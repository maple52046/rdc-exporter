// Package gpulabeler combines stable GPU identity with optional workload
// attribution and presents both through the collect.LabelProvider port.
//
// This is an interface-adapter package. It owns no host commands or Kubernetes
// connections: those outer capabilities supply a UUID snapshot and an optional
// workload provider. Its sole responsibility is keeping the label names and
// values positionally aligned for the collection use case.
package gpulabeler

import (
	"context"
	"fmt"

	"github.com/ROCm/rdc-exporter/internal/domain/metric"
	"github.com/ROCm/rdc-exporter/internal/usecase/collect"
)

// UUIDLabel is intentionally uppercase for compatibility with the Python RDC
// exporter and DCGM-style GPU identity labels.
const UUIDLabel = "UUID"

// Labeler appends a stable UUID to any labels supplied by an optional workload
// provider.
//
// UUIDs are snapshotted at construction because physical GPU identity is not
// expected to change during the process lifetime. Refresh delegates only to the
// workload provider. Labeler follows that provider's concurrency contract and
// is driven sequentially by the collection loop.
type Labeler struct {
	uuids    map[metric.GPUIndex]string
	workload collect.LabelProvider
}

// New creates a provider that always declares the UUID label, even when uuidByGPU
// is empty or workload is nil. Missing GPU indexes therefore produce an empty
// UUID value without changing Prometheus label cardinality. The UUID map is
// copied so later caller mutations cannot change exported identities.
func New(uuidByGPU map[int]string, workload collect.LabelProvider) *Labeler {
	uuids := make(map[metric.GPUIndex]string, len(uuidByGPU))
	for gpuIndex, uuid := range uuidByGPU {
		if gpuIndex >= 0 {
			uuids[metric.GPUIndex(gpuIndex)] = uuid
		}
	}
	return &Labeler{uuids: uuids, workload: workload}
}

// LabelKeys preserves the workload provider's established order and appends
// UUID last, retaining the UUID position used by the Python exporter.
func (l *Labeler) LabelKeys() []string {
	var keys []string
	if l.workload != nil {
		keys = append(keys, l.workload.LabelKeys()...)
	}
	return append(keys, UUIDLabel)
}

// LabelsFor returns values aligned with LabelKeys. Unknown UUIDs are represented
// by an empty string so discovery failures do not remove the label or prevent
// the remaining metrics from being exported.
func (l *Labeler) LabelsFor(gpuIndex metric.GPUIndex) []string {
	var values []string
	if l.workload != nil {
		values = append(values, l.workload.LabelsFor(gpuIndex)...)
	}
	return append(values, l.uuids[gpuIndex])
}

// Refresh updates workload attribution while retaining the startup UUID
// snapshot. It respects context through the wrapped provider and adds stage
// context to any returned error; without a workload provider it is a no-op.
func (l *Labeler) Refresh(ctx context.Context) error {
	if l.workload == nil {
		return nil
	}
	if err := l.workload.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh workload labels: %w", err)
	}
	return nil
}
