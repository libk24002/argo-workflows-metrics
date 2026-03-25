package informer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	wfinformers "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions"
	"github.com/conti/argo-workflows-metrics/pkg/collector"
	"github.com/conti/argo-workflows-metrics/pkg/health"
	"github.com/conti/argo-workflows-metrics/pkg/metrics"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const workflowInformerName = "workflow"

const (
	workflowEventAdd    = "add"
	workflowEventUpdate = "update"
	workflowEventDelete = "delete"
	maxWorkflowRetries  = 5
)

type workflowEvent struct {
	eventType string
	workflow  *wfv1.Workflow
}

// WorkflowInformer manages the workflow informer and event handlers
type WorkflowInformer struct {
	collector           *collector.WorkflowCollector
	healthState         *health.State
	informer            cache.SharedIndexInformer
	queue               workqueue.RateLimitingInterface
	workerCount         int
	fullReconcilePeriod time.Duration
	namespaceMatcher    func(string) bool
	reconcileMu         sync.Mutex
	stopCh              chan struct{}
	stopOnce            sync.Once
}

// NewWorkflowInformer creates a new WorkflowInformer
func NewWorkflowInformer(
	wfClient wfclientset.Interface,
	namespace string,
	resyncPeriod time.Duration,
	collector *collector.WorkflowCollector,
	healthState *health.State,
	workerCount int,
	fullReconcilePeriod time.Duration,
	namespaceMatcher func(string) bool,
) *WorkflowInformer {
	factory := wfinformers.NewSharedInformerFactoryWithOptions(
		wfClient,
		resyncPeriod,
		wfinformers.WithNamespace(namespace),
	)

	informer := factory.Argoproj().V1alpha1().Workflows().Informer()

	wi := &WorkflowInformer{
		collector:           collector,
		healthState:         healthState,
		informer:            informer,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "workflow-events"),
		workerCount:         max(1, workerCount),
		fullReconcilePeriod: fullReconcilePeriod,
		namespaceMatcher:    namespaceMatcher,
		stopCh:              make(chan struct{}),
	}
	if wi.namespaceMatcher == nil {
		wi.namespaceMatcher = func(string) bool { return true }
	}
	metrics.ExporterQueueDepth.WithLabelValues(workflowInformerName).Set(0)

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
		metrics.ExporterInformerStartErrorsTotal.WithLabelValues(workflowInformerName).Inc()
		return errors.New("workflow informer cache sync failed")
	}

	if wi.healthState != nil {
		wi.healthState.MarkWorkflowSynced()
	}

	klog.Info("Workflow informer cache synced")

	for i := 0; i < wi.workerCount; i++ {
		go wait.UntilWithContext(ctx, wi.runWorker, time.Second)
	}

	if wi.fullReconcilePeriod > 0 {
		go wait.UntilWithContext(ctx, wi.runFullReconcile, wi.fullReconcilePeriod)
	}

	// Wait for context cancellation
	<-ctx.Done()
	wi.Stop()
	return nil
}

// Stop stops the informer
func (wi *WorkflowInformer) Stop() {
	klog.Info("Stopping workflow informer")
	wi.stopOnce.Do(func() {
		wi.queue.ShutDown()
		metrics.ExporterQueueDepth.WithLabelValues(workflowInformerName).Set(0)
		close(wi.stopCh)
	})
}

func (wi *WorkflowInformer) runWorker(ctx context.Context) {
	for wi.processNext(ctx) {
	}
}

func (wi *WorkflowInformer) processNext(ctx context.Context) bool {
	_ = ctx
	item, shutdown := wi.queue.Get()
	if shutdown {
		return false
	}
	defer wi.queue.Done(item)

	evt, ok := item.(workflowEvent)
	if !ok {
		metrics.ExporterReconcileErrorsTotal.WithLabelValues(workflowInformerName, "event").Inc()
		metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(workflowInformerName, "queue").Inc()
		wi.queue.Forget(item)
		wi.setQueueDepth()
		return true
	}

	started := time.Now()
	wi.reconcileMu.Lock()
	err := wi.handleWorkflowEvent(evt)
	wi.reconcileMu.Unlock()
	metrics.ExporterReconcileDurationSeconds.WithLabelValues(workflowInformerName, "event").Observe(time.Since(started).Seconds())

	if err != nil {
		metrics.ExporterReconcileErrorsTotal.WithLabelValues(workflowInformerName, "event").Inc()
		if wi.queue.NumRequeues(item) < maxWorkflowRetries {
			wi.queue.AddRateLimited(item)
		} else {
			klog.Errorf("Dropping workflow event after retries: %v", err)
			wi.queue.Forget(item)
		}
		wi.setQueueDepth()
		return true
	}

	wi.queue.Forget(item)
	wi.setQueueDepth()
	return true
}

func (wi *WorkflowInformer) handleWorkflowEvent(evt workflowEvent) error {
	if evt.workflow == nil {
		return errors.New("workflow event contains nil object")
	}

	switch evt.eventType {
	case workflowEventAdd, workflowEventUpdate:
		wi.collector.AddWorkflow(evt.workflow)
		return nil
	case workflowEventDelete:
		wi.collector.DeleteWorkflow(evt.workflow)
		return nil
	default:
		return fmt.Errorf("unsupported workflow event type: %s", evt.eventType)
	}
}

func (wi *WorkflowInformer) runFullReconcile(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	started := time.Now()
	status := "success"
	items := wi.informer.GetStore().List()
	workflows := make([]*wfv1.Workflow, 0, len(items))

	for _, item := range items {
		wf, ok := item.(*wfv1.Workflow)
		if !ok {
			status = "error"
			metrics.ExporterReconcileErrorsTotal.WithLabelValues(workflowInformerName, "full").Inc()
			metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(workflowInformerName, "full").Inc()
			continue
		}
		if !wi.namespaceMatcher(wf.Namespace) {
			continue
		}
		workflows = append(workflows, wf)
	}

	wi.reconcileMu.Lock()
	wi.collector.ReplaceWorkflows(workflows)
	wi.reconcileMu.Unlock()

	metrics.ExporterFullReconcileTotal.WithLabelValues(workflowInformerName, status).Inc()
	metrics.ExporterReconcileDurationSeconds.WithLabelValues(workflowInformerName, "full").Observe(time.Since(started).Seconds())
}

func (wi *WorkflowInformer) enqueue(evt workflowEvent) {
	wi.queue.Add(evt)
	wi.setQueueDepth()
}

func (wi *WorkflowInformer) setQueueDepth() {
	metrics.ExporterQueueDepth.WithLabelValues(workflowInformerName).Set(float64(wi.queue.Len()))
}

// onAdd handles workflow add events
func (wi *WorkflowInformer) onAdd(obj interface{}) {
	wf, ok := obj.(*wfv1.Workflow)
	if !ok {
		metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(workflowInformerName, "add").Inc()
		klog.Errorf("Expected Workflow object, got: %T", obj)
		return
	}
	if !wi.namespaceMatcher(wf.Namespace) {
		return
	}
	metrics.ExporterEventsTotal.WithLabelValues(workflowInformerName, "add").Inc()
	if wi.healthState != nil {
		wi.healthState.MarkWorkflowEvent()
	}
	klog.V(4).Infof("Workflow added: %s/%s", wf.Namespace, wf.Name)
	wi.enqueue(workflowEvent{eventType: workflowEventAdd, workflow: wf.DeepCopy()})
}

// onUpdate handles workflow update events
func (wi *WorkflowInformer) onUpdate(oldObj, newObj interface{}) {
	wf, ok := newObj.(*wfv1.Workflow)
	if !ok {
		metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(workflowInformerName, "update").Inc()
		klog.Errorf("Expected Workflow object, got: %T", newObj)
		return
	}
	if !wi.namespaceMatcher(wf.Namespace) {
		return
	}
	metrics.ExporterEventsTotal.WithLabelValues(workflowInformerName, "update").Inc()
	if wi.healthState != nil {
		wi.healthState.MarkWorkflowEvent()
	}
	klog.V(4).Infof("Workflow updated: %s/%s", wf.Namespace, wf.Name)
	wi.enqueue(workflowEvent{eventType: workflowEventUpdate, workflow: wf.DeepCopy()})
}

// onDelete handles workflow delete events
func (wi *WorkflowInformer) onDelete(obj interface{}) {
	wf, ok := obj.(*wfv1.Workflow)
	if !ok {
		// Handle DeletedFinalStateUnknown
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(workflowInformerName, "delete").Inc()
			klog.Errorf("Expected Workflow or DeletedFinalStateUnknown, got: %T", obj)
			return
		}
		wf, ok = tombstone.Obj.(*wfv1.Workflow)
		if !ok {
			metrics.ExporterEventHandlerErrorsTotal.WithLabelValues(workflowInformerName, "delete").Inc()
			klog.Errorf("DeletedFinalStateUnknown contained non-Workflow object: %T", tombstone.Obj)
			return
		}
	}
	if !wi.namespaceMatcher(wf.Namespace) {
		return
	}
	metrics.ExporterEventsTotal.WithLabelValues(workflowInformerName, "delete").Inc()
	if wi.healthState != nil {
		wi.healthState.MarkWorkflowEvent()
	}
	klog.V(4).Infof("Workflow deleted: %s/%s", wf.Namespace, wf.Name)
	wi.enqueue(workflowEvent{eventType: workflowEventDelete, workflow: wf.DeepCopy()})
}
