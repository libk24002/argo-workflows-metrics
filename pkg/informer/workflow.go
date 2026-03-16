package informer

import (
	"context"
	"time"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	wfinformers "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions"
	"github.com/conti/argo-workflows-metrics/pkg/collector"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// WorkflowInformer manages the workflow informer and event handlers
type WorkflowInformer struct {
	collector *collector.WorkflowCollector
	informer  cache.SharedIndexInformer
	stopCh    chan struct{}
}

// NewWorkflowInformer creates a new WorkflowInformer
func NewWorkflowInformer(
	wfClient wfclientset.Interface,
	namespace string,
	resyncPeriod time.Duration,
	collector *collector.WorkflowCollector,
) *WorkflowInformer {
	factory := wfinformers.NewSharedInformerFactoryWithOptions(
		wfClient,
		resyncPeriod,
		wfinformers.WithNamespace(namespace),
	)

	informer := factory.Argoproj().V1alpha1().Workflows().Informer()

	wi := &WorkflowInformer{
		collector: collector,
		informer:  informer,
		stopCh:    make(chan struct{}),
	}

	// Register event handlers
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    wi.onAdd,
		UpdateFunc: wi.onUpdate,
		DeleteFunc: wi.onDelete,
	}); err != nil {
		klog.Fatalf("Failed to add event handler: %v", err)
	}

	return wi
}

// Start starts the informer
func (wi *WorkflowInformer) Start(ctx context.Context) error {
	klog.Info("Starting workflow informer")
	go wi.informer.Run(wi.stopCh)

	// Wait for cache sync
	if !cache.WaitForCacheSync(wi.stopCh, wi.informer.HasSynced) {
		return nil
	}

	klog.Info("Workflow informer cache synced")

	// Wait for context cancellation
	<-ctx.Done()
	wi.Stop()
	return nil
}

// Stop stops the informer
func (wi *WorkflowInformer) Stop() {
	klog.Info("Stopping workflow informer")
	close(wi.stopCh)
}

// onAdd handles workflow add events
func (wi *WorkflowInformer) onAdd(obj interface{}) {
	wf, ok := obj.(*wfv1.Workflow)
	if !ok {
		klog.Errorf("Expected Workflow object, got: %T", obj)
		return
	}
	klog.V(4).Infof("Workflow added: %s/%s", wf.Namespace, wf.Name)
	wi.collector.AddWorkflow(wf)
}

// onUpdate handles workflow update events
func (wi *WorkflowInformer) onUpdate(oldObj, newObj interface{}) {
	wf, ok := newObj.(*wfv1.Workflow)
	if !ok {
		klog.Errorf("Expected Workflow object, got: %T", newObj)
		return
	}
	klog.V(4).Infof("Workflow updated: %s/%s", wf.Namespace, wf.Name)
	wi.collector.AddWorkflow(wf)
}

// onDelete handles workflow delete events
func (wi *WorkflowInformer) onDelete(obj interface{}) {
	wf, ok := obj.(*wfv1.Workflow)
	if !ok {
		// Handle DeletedFinalStateUnknown
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Expected Workflow or DeletedFinalStateUnknown, got: %T", obj)
			return
		}
		wf, ok = tombstone.Obj.(*wfv1.Workflow)
		if !ok {
			klog.Errorf("DeletedFinalStateUnknown contained non-Workflow object: %T", tombstone.Obj)
			return
		}
	}
	klog.V(4).Infof("Workflow deleted: %s/%s", wf.Namespace, wf.Name)
	wi.collector.DeleteWorkflow(wf)
}
