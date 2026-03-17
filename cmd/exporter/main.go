package main

import (
	"context"
	"flag"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/conti/argo-workflows-metrics/pkg/collector"
	"github.com/conti/argo-workflows-metrics/pkg/informer"
	"github.com/conti/argo-workflows-metrics/pkg/informer/pod"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	kubeconfig   string
	namespace    string
	port         string
	resyncPeriod time.Duration
	version      string
)

func init() {
	pflag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (optional, uses in-cluster config if not provided)")
	pflag.StringVar(&namespace, "namespace", "", "Namespace to watch (empty for all namespaces)")
	pflag.StringVar(&port, "port", "8080", "Port to expose metrics on")
	pflag.DurationVar(&resyncPeriod, "resync-period", 5*time.Minute, "Resync period for informer")
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

	// Create collector
	wfCollector := collector.NewWorkflowCollector()
	podCollector := collector.NewPodCollector()

	// Create and start informer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wfInformer := informer.NewWorkflowInformer(wfClient, namespace, resyncPeriod, wfCollector)
	go func() {
		if err := wfInformer.Start(ctx); err != nil {
			klog.Fatalf("Failed to start workflow informer: %v", err)
		}
	}()

	// Create Kubernetes client for Pod informer
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create and start Pod informer
	podInformer := pod.NewPodInformer(kubeClient, namespace, resyncPeriod, podCollector)
	go func() {
		if err := podInformer.Start(ctx); err != nil {
			klog.Fatalf("Failed to start pod informer: %v", err)
		}
	}()

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", healthzHandler)
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

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	klog.Info("Shutting down gracefully...")

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
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, "OK"); err != nil {
		klog.Errorf("Failed to write healthz response: %v", err)
	}
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
</body>
</html>`); err != nil {
		klog.Errorf("Failed to write root response: %v", err)
	}
}
