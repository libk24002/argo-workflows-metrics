package pod

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/conti/argo-workflows-metrics/pkg/collector"
	"github.com/conti/argo-workflows-metrics/pkg/health"
	"github.com/conti/argo-workflows-metrics/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const podInformerName = "pod"

type PodInformer struct {
	collector   *collector.PodCollector
	healthState *health.State
	informer    cache.SharedIndexInformer
	stopCh      chan struct{}
	stopOnce    sync.Once
	namespace   string
}

func NewPodInformer(
	client kubernetes.Interface,
	namespace string,
	resyncPeriod time.Duration,
	podCollector *collector.PodCollector,
	healthState *health.State,
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
		collector:   podCollector,
		healthState: healthState,
		informer:    informer,
		stopCh:      make(chan struct{}),
		namespace:   namespace,
	}

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

	<-ctx.Done()
	pi.Stop()
	return nil
}

func (pi *PodInformer) Stop() {
	klog.Info("Stopping pod informer")
	pi.stopOnce.Do(func() {
		close(pi.stopCh)
	})
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
	metrics.ExporterEventsTotal.WithLabelValues(podInformerName, "add").Inc()
	if pi.healthState != nil {
		pi.healthState.MarkPodEvent()
	}
	klog.V(4).Infof("Pod added: %s/%s", pod.Namespace, pod.Name)
	pi.collector.AddPod(pod)
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
	metrics.ExporterEventsTotal.WithLabelValues(podInformerName, "update").Inc()
	if pi.healthState != nil {
		pi.healthState.MarkPodEvent()
	}
	klog.V(4).Infof("Pod updated: %s/%s", pod.Namespace, pod.Name)
	pi.collector.AddPod(pod)
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
	metrics.ExporterEventsTotal.WithLabelValues(podInformerName, "delete").Inc()
	if pi.healthState != nil {
		pi.healthState.MarkPodEvent()
	}
	klog.V(4).Infof("Pod deleted: %s/%s", pod.Namespace, pod.Name)
	pi.collector.DeletePod(pod)
}

func (pi *PodInformer) isWorkflowPod(pod *corev1.Pod) bool {
	_, ok := pod.Labels["workflows.argoproj.io/workflow"]
	return ok
}
