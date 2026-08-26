// Command rdc-exporter is the entry point that wires the exporter together and
// runs it.
//
// This is the outermost Frameworks and Drivers layer: it parses flags, loads the
// catalog, constructs the RDC reader, Prometheus sink, and optional Kubernetes
// labeler adapters, and drives the collect use case on a fixed interval while
// serving the Prometheus /metrics endpoint. It contains glue and lifecycle code
// only; the collection rules live in the inner layers.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ROCm/rdc-exporter/internal/adapter/gpulabeler"
	"github.com/ROCm/rdc-exporter/internal/adapter/k8slabeler"
	"github.com/ROCm/rdc-exporter/internal/adapter/prommetric"
	"github.com/ROCm/rdc-exporter/internal/adapter/rdcsource"
	"github.com/ROCm/rdc-exporter/internal/bindings/rdc"
	"github.com/ROCm/rdc-exporter/internal/config/catalog"
	"github.com/ROCm/rdc-exporter/internal/domain/metric"
	"github.com/ROCm/rdc-exporter/internal/hostgpu"
	"github.com/ROCm/rdc-exporter/internal/usecase/collect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
)

// scrapeInterval is how often the exporter refreshes its metrics from RDC. It is
// the long-standing five-second cadence and changing it alters the resolution of
// every exported series.
const scrapeInterval = 5 * time.Second

func main() {
	var (
		addr           string
		catalogPath    string
		enableDebug    bool
		fields         []string
		fieldsFilePath string
		gpuIndexes     []int
		kubeletPath    string
		selfMonitoring bool
	)

	pflag.StringVarP(&addr, "listen-address", "l", ":5000", "Address to listen on for HTTP requests")
	pflag.BoolVarP(&enableDebug, "debug", "d", false, "Enable debug logging")
	pflag.StringVar(&catalogPath, "catalog", "", "Path to the catalog YAML file")
	pflag.StringSliceVarP(&fields, "fields", "e", nil, "Fields to scrape (e.g., 100,812)")
	pflag.StringVarP(&fieldsFilePath, "fields-file", "f", "", "Path to a file containing fields to scrape (one per line)")
	pflag.IntSliceVarP(&gpuIndexes, "gpu-indexes", "i", nil, "GPU indexes to scrape (e.g., 0,1,2)")
	pflag.StringVarP(&kubeletPath, "kubelet", "k", "", "Path to the kubelet socket (e.g., /var/lib/kubelet/pod-resources/kubelet.sock)")
	pflag.BoolVar(&selfMonitoring, "self-monitoring", false, "Enable self-monitoring metrics")
	pflag.Parse()

	var logOpts *slog.HandlerOptions
	if enableDebug {
		logOpts = &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, logOpts)))

	catalg, err := catalog.ParseCatalogYAML(catalogPath)
	if err != nil {
		slog.Error("Failed to parse catalog YAML", "error", err)
		return
	}

	if fieldsFilePath != "" {
		file, err := os.ReadFile(fieldsFilePath)
		if err != nil {
			slog.Error("Failed to read fields file", "error", err)
			return
		}
		for _, line := range strings.Split(string(file), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fields = append(fields, line)
			}
		}
	}

	if len(fields) == 0 {
		slog.Info("No fields specified, exporting default fields")
		fields = defaultFields()
	}

	catalg.FilterEntitiesByFields(fields)
	if len(catalg.Entities) == 0 {
		slog.Error("No valid entities found in the catalog")
		return
	}

	// Resolve a field's enum name through the RDC bindings only when the catalog
	// leaves a metric's name blank; injecting it here keeps the catalog and domain
	// layers free of cgo.
	fieldName := func(id int) string { return rdc.NewFieldIDFromInt(id).Name() }
	definitions, err := catalog.DefinitionsFromEntities(catalg.Entities, fieldName)
	if err != nil {
		slog.Error("Failed to build metric definitions", "error", err)
		return
	}

	fieldIDs := make([]metric.FieldID, len(definitions))
	for i, def := range definitions {
		fieldIDs[i] = def.FieldID
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reg := prometheus.NewRegistry()
	if selfMonitoring {
		// Best-effort self-monitoring on a fresh registry; registration of the
		// standard collectors cannot conflict here, so the errors are ignored.
		_ = reg.Register(collectors.NewGoCollector())
		_ = reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	// Workload attribution is optional. Keep the concrete labeler available for
	// cleanup while avoiding a non-nil interface that wraps a nil pointer.
	var workloadLabelProvider collect.LabelProvider
	if kubeletPath != "" {
		lb, err := k8slabeler.New(kubeletPath)
		if err != nil {
			slog.Error("Failed to create K8s labeler", "error", err)
			return
		}
		defer lb.Close()
		workloadLabelProvider = lb
	}

	// GPU UUID discovery is best-effort so a missing rocm-smi binary or one bad
	// card record cannot prevent the exporter from serving all other metrics.
	uuidByGPU, err := hostgpu.LoadUUIDs(ctx)
	if err != nil {
		slog.Warn("Failed to load one or more GPU UUIDs", "error", err, "loaded", len(uuidByGPU))
	}
	labelProvider := gpulabeler.New(uuidByGPU, workloadLabelProvider)
	additionalLabelKeys := labelProvider.LabelKeys()

	reader, err := rdcsource.New(rdcsource.DefaultConfig(), gpuIndexes, fieldIDs)
	if err != nil {
		slog.Error("Failed to create RDC reader", "error", err)
		return
	}
	defer reader.Close()

	sink, err := prommetric.New(reg, definitions, additionalLabelKeys)
	if err != nil {
		slog.Error("Failed to register metrics", "error", err)
		return
	}

	service := collect.NewService(definitions, reader, sink, labelProvider)

	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Info("Shutting down exporter")
				return
			default:
				if err := service.Collect(ctx); err != nil {
					slog.Error("Failed to collect metrics", "error", err)
				}
				time.Sleep(scrapeInterval)
			}
		}
	}()

	server := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/metrics", http.StatusFound)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
		}
	}()

	// Block until an interrupt or termination signal cancels ctx, then ask the
	// HTTP server to shut down before the deferred RDC/labeler cleanup releases
	// external resources.
	<-ctx.Done()
	slog.Info("Shutting down exporter and HTTP server...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "err", err)
	}

	slog.Info("Exporter exited")
}

// defaultFields is the metric set exported when the user does not request any
// fields. It is the established default selection and changing it changes what a
// default deployment exposes.
func defaultFields() []string {
	return []string{
		"RDC_FI_GPU_CLOCK",
		"RDC_FI_MEM_CLOCK",
		"RDC_FI_MEMORY_TEMP",
		"RDC_FI_GPU_TEMP",
		"RDC_FI_POWER_USAGE",
		"RDC_FI_GPU_UTIL",
		"RDC_FI_GPU_MEMORY_USAGE",
		"RDC_FI_GPU_MEMORY_TOTAL",
		"RDC_FI_ECC_CORRECT_TOTAL",
		"RDC_FI_ECC_UNCORRECT_TOTAL",
		"RDC_FI_PROF_OCCUPANCY_PERCENT",
		"RDC_FI_PROF_ACTIVE_CYCLES",
		"RDC_FI_PROF_ACTIVE_WAVES",
		"RDC_FI_PROF_ELAPSED_CYCLES",
		"RDC_FI_PROF_TENSOR_ACTIVE_PERCENT",
		"RDC_FI_PROF_GPU_UTIL_PERCENT",
		"RDC_FI_PROF_EVAL_MEM_R_BW",
		"RDC_FI_PROF_EVAL_MEM_W_BW",
		"RDC_FI_PROF_EVAL_FLOPS_16",
		"RDC_FI_PROF_EVAL_FLOPS_32",
		"RDC_FI_PROF_EVAL_FLOPS_64",
		"RDC_FI_PROF_VALU_PIPE_ISSUE_UTIL",
		"RDC_FI_PROF_SM_ACTIVE",
		"RDC_FI_PROF_OCC_PER_ACTIVE_CU",
		"RDC_FI_PROF_OCC_ELAPSED",
		"RDC_FI_PROF_EVAL_FLOPS_16_PERCENT",
		"RDC_FI_PROF_EVAL_FLOPS_32_PERCENT",
		"RDC_FI_PROF_EVAL_FLOPS_64_PERCENT",
		"RDC_HEALTH_RETIRED_PAGE_NUM",
	}
}
