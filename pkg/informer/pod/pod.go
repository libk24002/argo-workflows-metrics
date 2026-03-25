package pod

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/conti/argo-workflows-metrics/pkg/collector"
	"github.com/conti/argo-workflows-metrics/pkg/health"
	"github.com/conti/argo-workflows-metrics/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const podInformerName = "pod"

const (
	podEventAdd    = "add"
	podEventUpdate = "update"
	podEventDelete = "delete"
	maxPodRetries  = 5
)

type podEvent struct {
	eventType string
	pod       *corev1.Pod
}

type PodInformer struct {
	collector           *collector.PodCollector
	healthState         *health.State
	informer            cache.SharedIndexInformer
	queue               workqueue.RateLimitingInterface
	workerCount         int
	fullReconcilePeriod time.Duration
	namespaceMatcher    func(string) bool
	reconcileMu         sync.Mutex
	stopCh              chan struct{}
	stopOnce            sync.Once
	namespace           string
}

func NewPodInformer(
	client kubernetes.Interface,
	namespace string,
	resyncPeriod time.Duration,
	podCollector *collector.PodCollector,
	healthState *health.State,
	workerCount int,
	fullReconcilePeriod time.Duration,
	namespaceMatcher func(string) bool,
) *PodInformer {
	var factory informers.SharedInformerFactory
	if namespace != "" {
		factory = informers.NewSharedInformerFactoryWithOptions(
			client,
			resyncPeriod,
			informers.WithNamespace(namespace),
		)
	} else {
		factory = informers.NewSharedInformerFactory(
			client,
			resyncPeriod,
		)
	}

	informer := factory.Core().V1().Pods().Informer()

	pi := &PodInformer{
		collector:           podCollector,
		healthState:         healthState,
		informer:            informer,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod-events"),
		workerCount:         max(1, workerCount),
		fullReconcilePeriod: fullReconcilePeriod,
		namespaceMatcher:    namespaceMatcher,
		stopCh:              make(chan struct{}),
		namespace:           namespace,
	}
	if pi.namespaceMatcher == nil {
		pi.namespaceMatcher = func(string) bool { return true }
	}
	metrics.ExporterQueueDepth.WithLabelValues(podInformerName).Set(0)

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pi.onAdd,
		UpdateFunc: pi.onUpdate,
		DeleteFunc: pi.onDelete,
	}); err != nil {
		klog.Fatalf("Failed to add pod event handler: %v", err)
	}

	return pi
}

func (pi *PodInformer) Start(ctx context.Context) error {
	klog.Info("Starting pod informer")
	go pi.informer.Run(pi.stopCh)

	if !cache.WaitForCacheSync(pi.stopCh, pi.informer.HasSynced) {
		metrics.ExporterInformerStartErrorsTotal.WithLabelValues(podInformerName).Inc()
		return errors.New("pod informer cache sync failed")
	}

	if pi.healthState != nil {
		pi.healthState.MarkPodSynced()
	}

	klog.Info("Pod informer cache synced")

	for i := 0; i < pi.workerCount; i++ {
		go wait.UntilWithContext(ctx, pi.runWorker, time.Second)
	}

	if pi.fullReconcilePeriod > 0 {
		go wait.UntilWithContext(ctx, pi.runFullReconcile, pi.fullReconcilePeriod)
	}

	<-ctx.Done()
	pi.Stop()
	return nil
}

func (pi *PodInformer) Stop() {
	klog.Info("Stopping pod informer")
	pi.stopOnce.Do(func() {
		pi.queue.ShutDown()
		metrics.ExporterQueueDepth.WithLabelValues(podInformerName).Set(0)
		close(pi.stopCh)
	})
}

func (pi *PodInformer) runWorker(ctx context.Context) {
	for pi.processNext(ctx) {
	}
}

func (pi *PodInformer) processNext(ctx context.Context) bool {
	_ = ctx
	item, shutdown := pi.queue.Get()
	if shutdown {
		return false
	}
	defer pi.queue.Done(item)

	evt, ok := item.(podEvent)
	if !ok {
		metrics.ExporterReconcileErrorsTotal.WithLabelValues(podInformerName, "event").Inc()
		metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(podInformerName, "queue").Inc()
		pi.queue.Forget(item)
		pi.setQueueDepth()
		return true
	}

	started := time.Now()
	pi.reconcileMu.Lock()
	err := pi.handlePodEvent(evt)
	pi.reconcileMu.Unlock()
	metrics.ExporterReconcileDurationSeconds.WithLabelValues(podInformerName, "event").Observe(time.Since(started).Seconds())

	if err != nil {
		metrics.ExporterReconcileErrorsTotal.WithLabelValues(podInformerName, "event").Inc()
		if pi.queue.NumRequeues(item) < maxPodRetries {
			pi.queue.AddRateLimited(item)
		} else {
			klog.Errorf("Dropping pod event after retries: %v", err)
			pi.queue.Forget(item)
		}
		pi.setQueueDepth()
		return true
	}

	pi.queue.Forget(item)
	pi.setQueueDepth()
	return true
}

func (pi *PodInformer) handlePodEvent(evt podEvent) error {
	if evt.pod == nil {
		return errors.New("pod event contains nil object")
	}

	switch evt.eventType {
	case podEventAdd, podEventUpdate:
		pi.collector.AddPod(evt.pod)
		return nil
	case podEventDelete:
		pi.collector.DeletePod(evt.pod)
		return nil
	default:
		return fmt.Errorf("unsupported pod event type: %s", evt.eventType)
	}
}

func (pi *PodInformer) runFullReconcile(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	started := time.Now()
	status := "success"
	items := pi.informer.GetStore().List()

	pi.reconcileMu.Lock()
	for _, item := range items {
		podObj, ok := item.(*corev1.Pod)
		if !ok {
			status = "error"
			metrics.ExporterReconcileErrorsTotal.WithLabelValues(podInformerName, "full").Inc()
			metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(podInformerName, "full").Inc()
			continue
		}
		if !pi.isWorkflowPod(podObj) {
			continue
		}
		if !pi.namespaceMatcher(podObj.Namespace) {
			continue
		}
		pi.collector.AddPod(podObj)
	}
	pi.reconcileMu.Unlock()

	metrics.ExporterFullReconcileTotal.WithLabelValues(podInformerName, status).Inc()
	metrics.ExporterReconcileDurationSeconds.WithLabelValues(podInformerName, "full").Observe(time.Since(started).Seconds())
}

func (pi *PodInformer) enqueue(evt podEvent) {
	pi.queue.Add(evt)
	pi.setQueueDepth()
}

func (pi *PodInformer) setQueueDepth() {
	metrics.ExporterQueueDepth.WithLabelValues(podInformerName).Set(float64(pi.queue.Len()))
}

func (pi *PodInformer) onAdd(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(podInformerName, "add").Inc()
		klog.Errorf("Expected Pod object, got: %T", obj)
		return
	}
	if !pi.isWorkflowPod(pod) {
		return
	}
	if !pi.namespaceMatcher(pod.Namespace) {
		return
	}
	metrics.ExporterEventsTotal.WithLabelValues(podInformerName, "add").Inc()
	if pi.healthState != nil {
		pi.healthState.MarkPodEvent()
	}
	klog.V(4).Infof("Pod added: %s/%s", pod.Namespace, pod.Name)
	pi.enqueue(podEvent{eventType: podEventAdd, pod: pod.DeepCopy()})
}

func (pi *PodInformer) onUpdate(oldObj, newObj interface{}) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(podInformerName, "update").Inc()
		klog.Errorf("Expected Pod object, got: %T", newObj)
		return
	}
	if !pi.isWorkflowPod(pod) {
		return
	}
	if !pi.namespaceMatcher(pod.Namespace) {
		return
	}
	metrics.ExporterEventsTotal.WithLabelValues(podInformerName, "update").Inc()
	if pi.healthState != nil {
		pi.healthState.MarkPodEvent()
	}
	klog.V(4).Infof("Pod updated: %s/%s", pod.Namespace, pod.Name)
	pi.enqueue(podEvent{eventType: podEventUpdate, pod: pod.DeepCopy()})
}

func (pi *PodInformer) onDelete(obj interface{}) {
	var pod *corev1.Pod
	switch t := obj.(type) {
	case *corev1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*corev1.Pod)
		if !ok {
			metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(podInformerName, "delete").Inc()
			klog.Errorf("DeletedFinalStateUnknown contained non-Pod object: %T", t.Obj)
			return
		}
	default:
		metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(podInformerName, "delete").Inc()
		klog.Errorf("Expected Pod or DeletedFinalStateUnknown, got: %T", obj)
		return
	}

	if !pi.isWorkflowPod(pod) {
		return
	}
	if !pi.namespaceMatcher(pod.Namespace) {
		return
	}
	metrics.ExporterEventsTotal.WithLabelValues(podInformerName, "delete").Inc()
	if pi.healthState != nil {
		pi.healthState.MarkPodEvent()
	}
	klog.V(4).Infof("Pod deleted: %s/%s", pod.Namespace, pod.Name)
	pi.enqueue(podEvent{eventType: podEventDelete, pod: pod.DeepCopy()})
}

func (pi *PodInformer) isWorkflowPod(pod *corev1.Pod) bool {
	_, ok := pod.Labels["workflows.argoproj.io/workflow"]
	return ok
}
