// Package k8slabeler adapts the kubelet pod resources API into the
// collect.LabelProvider port, attributing each GPU to the Kubernetes workload
// currently using it.
//
// This is an interface-adapter package. It owns the gRPC connection to the
// kubelet and the mapping from host GPU device identifiers to GPU indexes; it
// translates that into the pod, namespace, and container labels the collection
// flow attaches to metrics. It depends on the kubelet/gRPC frameworks and on
// host GPU discovery, and exposes only the framework-free LabelProvider surface
// plus Close, which the entry point uses to release the connection.
package k8slabeler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ROCm/rdc-exporter/internal/domain/metric"
	"github.com/ROCm/rdc-exporter/internal/hostgpu"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	podresources "k8s.io/kubelet/pkg/apis/podresources/v1"
)

// Workload label names exported for GPUs attributed to a Kubernetes workload.
// Their order is the contract used by both LabelKeys and LabelsFor and matches
// what existing scrape configurations expect, so it must not change casually.
const (
	labelPod       = "pod"
	labelNamespace = "namespace"
	labelContainer = "container"
)

// amdGPUResource is the device plugin resource name for AMD GPUs. Only container
// devices advertised under this resource are treated as GPUs.
const amdGPUResource = "amd.com/gpu"

// Labeler resolves Kubernetes pod/namespace/container labels for each GPU by
// querying the kubelet pod resources API.
//
// It is constructed once with a snapshot of host GPU devices and then refreshed
// on each collection. Labeler holds a live gRPC connection that must be released
// with Close. Refresh and LabelsFor are called in sequence by a single
// collection loop and are not designed for concurrent use.
type Labeler struct {
	// client queries the kubelet pod resources API.
	client podresources.PodResourcesListerClient
	// conn is the underlying gRPC connection, released by Close.
	conn *grpc.ClientConn

	// gpuDevices maps a container device identifier (PCI address or render node
	// path) to the host GPU index it belongs to.
	gpuDevices map[string]int
	// labels holds the current label values per GPU index; it is rebuilt on
	// every Refresh.
	labels map[int]map[string]string
}

// New connects to the kubelet pod resources API at endpoint and snapshots the
// host's AMD GPUs.
//
// endpoint may be given with or without the unix:// scheme; it is normalized to
// a Unix socket target. The gRPC client is created without transport security
// because the pod resources socket is a local, host-trusted endpoint. New
// returns an error if the connection cannot be created or host GPU discovery
// fails; on success the caller owns the Labeler and must call Close.
func New(endpoint string) (*Labeler, error) {
	if !strings.HasPrefix(endpoint, "unix://") {
		endpoint = "unix://" + endpoint
	}

	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to kubelet pod resources API at %s: %w", endpoint, err)
	}

	devices, err := hostgpu.GetAMDGpuDevices()
	if err != nil {
		return nil, fmt.Errorf("discover AMD GPU devices: %w", err)
	}

	labeler := &Labeler{
		conn:       conn,
		client:     podresources.NewPodResourcesListerClient(conn),
		gpuDevices: make(map[string]int),
		labels:     make(map[int]map[string]string),
	}

	for _, gpu := range devices {
		labeler.labels[gpu.Index] = make(map[string]string)
		if gpu.PCIAddress != nil {
			labeler.gpuDevices[*gpu.PCIAddress] = gpu.Index
		}
		if gpu.RenderDevice != nil {
			labeler.gpuDevices[*gpu.RenderDevice] = gpu.Index
		}
	}

	return labeler, nil
}

// Close releases the gRPC connection to the kubelet. It is safe to call when the
// connection was never established and returns an error only if the underlying
// close fails.
func (l *Labeler) Close() error {
	if l.conn != nil {
		if err := l.conn.Close(); err != nil {
			return fmt.Errorf("close kubelet connection: %w", err)
		}
	}
	return nil
}

// LabelKeys returns the workload label names in their fixed order: pod,
// namespace, container. The order matches the values produced by LabelsFor.
func (l *Labeler) LabelKeys() []string {
	return []string{labelPod, labelNamespace, labelContainer}
}

// LabelsFor returns the pod, namespace, and container values for gpuIndex,
// positionally aligned with LabelKeys. A GPU with no current attribution yields
// empty strings rather than a shorter slice, so every series keeps a consistent
// label cardinality.
func (l *Labeler) LabelsFor(gpuIndex metric.GPUIndex) []string {
	keys := l.LabelKeys()
	values := make([]string, len(keys))
	if gpuLabels, ok := l.labels[int(gpuIndex)]; ok {
		for i, key := range keys {
			values[i] = gpuLabels[key]
		}
	}
	return values
}

// Refresh rebuilds the GPU-to-label mapping from the current kubelet state.
//
// All GPU labels are reset first so a GPU released by a workload no longer
// carries stale pod/namespace/container labels, then each container's GPU
// allocations are mapped back to a GPU index and labelled. A device ID that does
// not match a known host GPU is warned and skipped. Refresh respects ctx
// cancellation through the kubelet query and returns an error only when that
// query fails.
func (l *Labeler) Refresh(ctx context.Context) error {
	containers, err := l.listContainers(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	for gpuIndex := range l.labels {
		l.labels[gpuIndex] = make(map[string]string)
	}

	for _, container := range containers {
		if len(container.GPUs) == 0 {
			continue
		}
		for _, gpuAddr := range container.GPUs {
			gpuIndex, ok := l.gpuDevices[gpuAddr]
			if !ok {
				slog.Warn("GPU device not found in labeler", "container", deref(container.Name), "gpu", gpuAddr)
				continue
			}
			l.labels[gpuIndex][labelPod] = deref(container.PodName)
			l.labels[gpuIndex][labelNamespace] = deref(container.Namespace)
			l.labels[gpuIndex][labelContainer] = deref(container.Name)
		}
	}

	return nil
}

// strPtr returns a pointer to a copy of s. It lets container records distinguish
// an unset field from an empty string without a shared helper package.
func strPtr(s string) *string {
	return &s
}

// deref returns the string a points to, or "" when a is nil, so a missing label
// becomes an empty value instead of a nil dereference.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
