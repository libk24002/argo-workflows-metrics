package pod

import (
	"context"
	"time"

	"github.com/conti/argo-workflows-metrics/pkg/collector"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type PodInformer struct {
	collector *collector.PodCollector
	informer  cache.SharedIndexInformer
	stopCh    chan struct{}
	namespace string
}

func NewPodInformer(
	clientset interface{},
	namespace string,
	resyncPeriod time.Duration,
	podCollector *collector.PodCollector,
) *PodInformer {
	client := clientset.(kubernetes.Interface)

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
		collector: podCollector,
		informer:  informer,
		stopCh:    make(chan struct{}),
		namespace: namespace,
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
		return nil
	}

	klog.Info("Pod informer cache synced")

	<-ctx.Done()
	pi.Stop()
	return nil
}

func (pi *PodInformer) Stop() {
	klog.Info("Stopping pod informer")
	close(pi.stopCh)
}

func (pi *PodInformer) onAdd(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("Expected Pod object, got: %T", obj)
		return
	}
	if !pi.isWorkflowPod(pod) {
		return
	}
	klog.V(4).Infof("Pod added: %s/%s", pod.Namespace, pod.Name)
	pi.collector.AddPod(pod)
}

func (pi *PodInformer) onUpdate(oldObj, newObj interface{}) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		klog.Errorf("Expected Pod object, got: %T", newObj)
		return
	}
	if !pi.isWorkflowPod(pod) {
		return
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
			klog.Errorf("DeletedFinalStateUnknown contained non-Pod object: %T", t.Obj)
			return
		}
	default:
		klog.Errorf("Expected Pod or DeletedFinalStateUnknown, got: %T", obj)
		return
	}

	if !pi.isWorkflowPod(pod) {
		return
	}
	klog.V(4).Infof("Pod deleted: %s/%s", pod.Namespace, pod.Name)
	pi.collector.DeletePod(pod)
}

func (pi *PodInformer) isWorkflowPod(pod *corev1.Pod) bool {
	_, ok := pod.Labels["workflows.argoproj.io/workflow"]
	return ok
}
