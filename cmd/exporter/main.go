package main

import (
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/conti/argo-workflows-metrics/pkg/collector"
	"github.com/conti/argo-workflows-metrics/pkg/health"
	"github.com/conti/argo-workflows-metrics/pkg/informer"
	"github.com/conti/argo-workflows-metrics/pkg/informer/pod"
	"github.com/conti/argo-workflows-metrics/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

var (
	kubeconfig   string
	namespace    string
	port         string
	resyncPeriod time.Duration
	startupGrace time.Duration
	eventStale   time.Duration
	wfDetails    bool
	podMetrics   bool
	workerCount  int
	reconcileDur time.Duration
	shardTotal   int
	shardIndex   int
	leaderElect  bool
	leaderID     string
	leaderNS     string
	leaseDur     time.Duration
	renewDur     time.Duration
	retryDur     time.Duration
	version      string
)

func init() {
	pflag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (optional, uses in-cluster config if not provided)")
	pflag.StringVar(&namespace, "namespace", "", "Namespace to watch (empty for all namespaces)")
	pflag.StringVar(&port, "port", "8080", "Port to expose metrics on")
	pflag.DurationVar(&resyncPeriod, "resync-period", 5*time.Minute, "Resync period for informer")
	pflag.DurationVar(&startupGrace, "startup-grace-period", 2*time.Minute, "Startup grace period before event staleness is evaluated")
	pflag.DurationVar(&eventStale, "event-stale-threshold", 30*time.Minute, "Max time without workflow/pod events before readiness fails")
	pflag.BoolVar(&wfDetails, "enable-workflow-detail-metrics", true, "Enable high-cardinality per-workflow metrics")
	pflag.BoolVar(&podMetrics, "enable-pod-container-metrics", true, "Enable per-container pod metrics")
	pflag.IntVar(&workerCount, "worker-count", 1, "Number of worker routines per informer queue")
	pflag.DurationVar(&reconcileDur, "full-reconcile-period", 10*time.Minute, "Full reconcile period for informer cache backfill")
	pflag.IntVar(&shardTotal, "shard-total", 1, "Total shard count for namespace-hash sharding")
	pflag.IntVar(&shardIndex, "shard-index", -1, "Shard index in [0, shard-total-1], default derived from pod hostname")
	pflag.BoolVar(&leaderElect, "leader-elect", true, "Enable leader election for high availability")
	pflag.StringVar(&leaderID, "leader-election-id", "argo-workflows-metrics", "Leader election lease name")
	pflag.StringVar(&leaderNS, "leader-election-namespace", "", "Leader election lease namespace (defaults to POD_NAMESPACE env)")
	pflag.DurationVar(&leaseDur, "leader-election-lease-duration", 30*time.Second, "Leader election lease duration")
	pflag.DurationVar(&renewDur, "leader-election-renew-deadline", 20*time.Second, "Leader election renew deadline")
	pflag.DurationVar(&retryDur, "leader-election-retry-period", 5*time.Second, "Leader election retry period")
	pflag.StringVar(&version, "version", "dev", "Version of the exporter")
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
}

func main() {
	pflag.Parse()

	klog.Info("Starting Argo Workflows Metrics Exporter")
	klog.Infof("Version: %s", version)

	// Build Kubernetes config
	config, err := buildConfig()
	if err != nil {
		klog.Fatalf("Failed to build config: %v", err)
	}

	// Create Argo Workflows client
	wfClient, err := wfclientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create Argo Workflows client: %v", err)
	}

	klog.Infof("Connected to Kubernetes cluster")
	if namespace == "" {
		klog.Info("Watching all namespaces")
	} else {
		klog.Infof("Watching namespace: %s", namespace)
	}
	klog.Infof("Workflow detail metrics enabled: %t", wfDetails)
	klog.Infof("Pod container metrics enabled: %t", podMetrics)
	klog.Infof("Informer worker count: %d", workerCount)
	klog.Infof("Full reconcile period: %s", reconcileDur)

	namespaceMatcher, mode, resolvedShardIndex, err := buildNamespaceMatcher(namespace, shardTotal, shardIndex)
	if err != nil {
		klog.Fatalf("Invalid sharding config: %v", err)
	}

	if mode == "shard" && leaderElect {
		klog.Fatalf("leader election and sharding cannot be enabled at the same time")
	}

	klog.Infof("Partition mode: %s (shard-total=%d shard-index=%d)", mode, shardTotal, resolvedShardIndex)
	metrics.ExporterShardInfo.WithLabelValues(mode, strconv.Itoa(shardTotal), strconv.Itoa(resolvedShardIndex)).Set(1)

	// Create collectors and health state
	wfCollector := collector.NewWorkflowCollector(wfDetails)
	podCollector := collector.NewPodCollector(podMetrics)
	healthState := health.NewState(startupGrace, eventStale, leaderElect)

	// Build informer objects
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wfInformer := informer.NewWorkflowInformer(
		wfClient,
		namespace,
		resyncPeriod,
		wfCollector,
		healthState,
		workerCount,
		reconcileDur,
		namespaceMatcher,
	)

	// Create Kubernetes client for Pod informer
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Build pod informer
	podInformer := pod.NewPodInformer(
		kubeClient,
		namespace,
		resyncPeriod,
		podCollector,
		healthState,
		workerCount,
		reconcileDur,
		namespaceMatcher,
	)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", healthzHandler(healthState))
	mux.HandleFunc("/readyz", readyzHandler(healthState))
	mux.HandleFunc("/", rootHandler)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start HTTP server
	go func() {
		klog.Infof("Starting HTTP server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	if leaderElect {
		go runLeaderElection(ctx, kubeClient, wfInformer, podInformer, healthState)
	} else {
		healthState.MarkLeader(true)
		go func() {
			if err := runControllers(ctx, wfInformer, podInformer); err != nil {
				klog.Fatalf("Failed to run controllers: %v", err)
			}
		}()
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	klog.Info("Shutting down gracefully...")
	healthState.MarkShuttingDown()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("HTTP server shutdown error: %v", err)
	}

	// Stop informer
	cancel()

	klog.Info("Exporter stopped")
}

func runControllers(ctx context.Context, wfInformer *informer.WorkflowInformer, podInformer *pod.PodInformer) error {
	errCh := make(chan error, 2)

	go func() {
		errCh <- wfInformer.Start(ctx)
	}()

	go func() {
		errCh <- podInformer.Start(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}
}

func runLeaderElection(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	wfInformer *informer.WorkflowInformer,
	podInformer *pod.PodInformer,
	healthState *health.State,
) {
	namespace := resolveLeaderElectionNamespace()
	identity, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Failed to resolve leader identity: %v", err)
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaderID,
			Namespace: namespace,
		},
		Client: kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: identity,
		},
	}

	klog.Infof("Leader election enabled: lease=%s namespace=%s identity=%s", leaderID, namespace, identity)

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   leaseDur,
		RenewDeadline:   renewDur,
		RetryPeriod:     retryDur,
		ReleaseOnCancel: true,
		Name:            leaderID,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaderCtx context.Context) {
				klog.Info("Leadership acquired, starting controller loops")
				healthState.MarkLeader(true)

				if err := runControllers(leaderCtx, wfInformer, podInformer); err != nil && leaderCtx.Err() == nil {
					klog.Fatalf("Controller loop failed while leader: %v", err)
				}
			},
			OnStoppedLeading: func() {
				healthState.MarkLeader(false)
				if ctx.Err() != nil {
					klog.Info("Leader election stopped due to shutdown")
					return
				}
				klog.Fatalf("Leadership lost")
			},
			OnNewLeader: func(current string) {
				klog.Infof("New leader elected: %s", current)
			},
		},
	})
}

func resolveLeaderElectionNamespace() string {
	if leaderNS != "" {
		return leaderNS
	}

	podNS := os.Getenv("POD_NAMESPACE")
	if podNS != "" {
		return podNS
	}

	if namespace != "" {
		return namespace
	}

	return "default"
}

func buildNamespaceMatcher(targetNamespace string, total, index int) (func(string) bool, string, int, error) {
	if total < 1 {
		return nil, "", 0, fmt.Errorf("shard-total must be >= 1")
	}

	baseMatcher := func(ns string) bool {
		if targetNamespace == "" {
			return true
		}
		return ns == targetNamespace
	}

	if total == 1 {
		if index > 0 {
			return nil, "", 0, fmt.Errorf("shard-index must be 0 or -1 when shard-total is 1")
		}
		return baseMatcher, "single", 0, nil
	}

	resolvedIndex, err := resolveShardIndex(total, index)
	if err != nil {
		return nil, "", 0, err
	}

	matcher := func(ns string) bool {
		if !baseMatcher(ns) {
			return false
		}
		if ns == "" {
			return false
		}
		return namespaceShard(ns, total) == resolvedIndex
	}

	return matcher, "shard", resolvedIndex, nil
}

func resolveShardIndex(total, configured int) (int, error) {
	if configured >= 0 {
		if configured >= total {
			return 0, fmt.Errorf("shard-index must be in [0, %d)", total)
		}
		return configured, nil
	}

	host, err := os.Hostname()
	if err != nil {
		return 0, fmt.Errorf("resolve hostname for shard index: %w", err)
	}

	lastDash := strings.LastIndex(host, "-")
	if lastDash < 0 || lastDash == len(host)-1 {
		return 0, fmt.Errorf("unable to derive shard-index from hostname %q", host)
	}

	autoIndex, err := strconv.Atoi(host[lastDash+1:])
	if err != nil {
		return 0, fmt.Errorf("invalid shard suffix in hostname %q: %w", host, err)
	}
	if autoIndex < 0 || autoIndex >= total {
		return 0, fmt.Errorf("derived shard index %d out of range [0, %d)", autoIndex, total)
	}
	return autoIndex, nil
}

func namespaceShard(namespace string, total int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(namespace))
	return int(h.Sum32() % uint32(total))
}

// buildConfig builds Kubernetes config from kubeconfig or in-cluster config
func buildConfig() (*rest.Config, error) {
	if kubeconfig != "" {
		klog.Infof("Using kubeconfig: %s", kubeconfig)
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	klog.Info("Using in-cluster config")
	return rest.InClusterConfig()
}

// healthzHandler handles health check requests
func healthzHandler(state *health.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		alive, reason := state.IsLive(time.Now())
		if alive {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		snapshot := state.Snapshot()
		response := fmt.Sprintf(
			"status=%s reason=%q leader_elect=%t is_leader=%t workflow_synced=%t pod_synced=%t last_workflow_event=%s last_pod_event=%s\n",
			boolToState(alive, "alive", "unhealthy"),
			reason,
			snapshot.LeaderElect,
			snapshot.IsLeader,
			snapshot.WorkflowSynced,
			snapshot.PodSynced,
			formatEventTime(snapshot.LastWorkflowEvt),
			formatEventTime(snapshot.LastPodEvt),
		)

		if _, err := io.WriteString(w, response); err != nil {
			klog.Errorf("Failed to write healthz response: %v", err)
		}
	}
}

func readyzHandler(state *health.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ready, reason := state.IsReady(time.Now())
		if ready {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		snapshot := state.Snapshot()
		response := fmt.Sprintf(
			"status=%s reason=%q leader_elect=%t is_leader=%t workflow_synced=%t pod_synced=%t last_workflow_event=%s last_pod_event=%s\n",
			boolToState(ready, "ready", "not-ready"),
			reason,
			snapshot.LeaderElect,
			snapshot.IsLeader,
			snapshot.WorkflowSynced,
			snapshot.PodSynced,
			formatEventTime(snapshot.LastWorkflowEvt),
			formatEventTime(snapshot.LastPodEvt),
		)

		if _, err := io.WriteString(w, response); err != nil {
			klog.Errorf("Failed to write readyz response: %v", err)
		}
	}
}

func boolToState(value bool, trueValue, falseValue string) string {
	if value {
		return trueValue
	}
	return falseValue
}

func formatEventTime(value time.Time) string {
	if value.IsZero() {
		return "never"
	}
	return value.Format(time.RFC3339)
}

// rootHandler handles root path requests
func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if _, err := io.WriteString(w, `<html>
<head><title>Argo Workflows Metrics Exporter</title></head>
<body>
<h1>Argo Workflows Metrics Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
<p><a href="/healthz">Health Check</a></p>
<p><a href="/readyz">Readiness Check</a></p>
</body>
</html>`); err != nil {
		klog.Errorf("Failed to write root response: %v", err)
	}
}
