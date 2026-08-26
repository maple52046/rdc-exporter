// Package collect contains the application flow that gathers GPU field readings,
// applies the domain scaling and labelling rules, and publishes the resulting
// metric points.
//
// This is a Use Cases layer package. It orchestrates the core domain
// (internal/domain/metric) through ports it defines for itself, and it must not
// import the RDC bindings, the Prometheus client, gRPC, or HTTP. Concrete
// implementations of the ports live in the interface-adapter layer and are
// injected by the process entry point. Because every external capability is
// reached through a port, the flow here is testable with in-memory fakes,
// without a real GPU, the RDC library, or a Prometheus registry.
package collect

import (
	"context"
	"errors"
	"fmt"

	"github.com/ROCm/rdc-exporter/internal/domain/metric"
)

// ErrNoSamples reports that a collection produced no field readings.
//
// The reader returning zero samples usually means the link to the RDC library
// is stale and the watch needs to be re-established. Callers may treat this as a
// transient, retryable condition rather than a fatal error. It is returned
// unwrapped so callers can match it with errors.Is.
var ErrNoSamples = errors.New("collect: no field samples available")

// FieldReader reads the latest raw field values for the GPUs and fields the
// exporter is configured to collect.
//
// Implementations are interface adapters over the RDC library. ReadSamples
// returns one Sample per available (GPU, field) reading with the raw, unscaled
// value; scaling is the use case's responsibility. Implementations should honor
// context cancellation between readings and must not return RDC/cgo types in the
// samples. A successful read may legitimately return an empty slice, which the
// use case translates into ErrNoSamples.
type FieldReader interface {
	ReadSamples(ctx context.Context) ([]metric.Sample, error)
}

// LabelProvider supplies the additional labels attached to each GPU's series.
//
// It is optional: when no provider is configured the exporter emits only the
// mandatory gpu_index label. Refresh updates the provider's view of the world
// when needed (for example by querying the kubelet) and is called once at the
// start of each collection so that LabelsFor reflects the current mapping.
// LabelKeys is the fixed, ordered set of additional label names and must stay
// aligned with the values returned by LabelsFor.
type LabelProvider interface {
	// LabelKeys returns the additional label names in a stable order. The order
	// must match the values returned by LabelsFor.
	LabelKeys() []string
	// LabelsFor returns the additional label values for one GPU, positionally
	// aligned with LabelKeys. A GPU with no known labels yields empty strings,
	// never a shorter slice, so series keep a consistent label cardinality.
	LabelsFor(gpuIndex metric.GPUIndex) []string
	// Refresh updates the provider's GPU-to-label mapping. It respects context
	// cancellation and returns an error if the underlying source cannot be read.
	Refresh(ctx context.Context) error
}

// MetricSink publishes prepared metric points to the outside world.
//
// Implementations are interface adapters over a metrics backend (the Prometheus
// registry). Publish receives points whose values are already scaled and whose
// labels are already assembled; it must route each point to the series selected
// by its FieldID and is responsible for any backend-specific bookkeeping, such
// as removing a series whose label values changed since the previous Publish.
type MetricSink interface {
	Publish(points []metric.Point) error
}

// Service runs a single collection of GPU metrics on demand.
//
// Service owns the application flow and the metric definitions used to scale raw
// readings; it depends on external systems only through the FieldReader,
// LabelProvider, and MetricSink ports. A Service is safe to call repeatedly (one
// call per scrape interval) but is not designed for concurrent Collect calls.
type Service struct {
	// definitions maps each collected field to its scaling and identity rule.
	definitions map[metric.FieldID]metric.Definition
	// reader pulls raw field samples from the RDC library.
	reader FieldReader
	// labels supplies additional labels; nil disables additional labelling.
	labels LabelProvider
	// sink publishes the prepared points.
	sink MetricSink
}

// NewService wires the collection flow from its definitions and ports.
//
// definitions describe every metric to export and provide the scaling rule per
// field; duplicate field IDs keep the last definition. labels may be nil, in
// which case only the gpu_index label is emitted. reader and sink are required.
func NewService(definitions []metric.Definition, reader FieldReader, sink MetricSink, labels LabelProvider) *Service {
	byField := make(map[metric.FieldID]metric.Definition, len(definitions))
	for _, def := range definitions {
		byField[def.FieldID] = def
	}
	return &Service{
		definitions: byField,
		reader:      reader,
		labels:      labels,
		sink:        sink,
	}
}

// Collect performs one collection cycle: refresh labels, read raw samples, scale
// and label them, then publish the resulting points.
//
// When a LabelProvider is configured it is refreshed first so the labels match
// the readings taken in the same cycle. A sample whose field has no definition
// is skipped, since it has no scaling rule or series to update. An empty read is
// reported as ErrNoSamples; all other failures are wrapped with the failing
// stage for localization. Collect respects ctx cancellation through the ports it
// calls.
func (s *Service) Collect(ctx context.Context) error {
	if s.labels != nil {
		if err := s.labels.Refresh(ctx); err != nil {
			return fmt.Errorf("refresh labels: %w", err)
		}
	}

	samples, err := s.reader.ReadSamples(ctx)
	if err != nil {
		return fmt.Errorf("read samples: %w", err)
	}
	if len(samples) == 0 {
		return ErrNoSamples
	}

	points := make([]metric.Point, 0, len(samples))
	for _, sample := range samples {
		def, ok := s.definitions[sample.FieldID]
		if !ok {
			// A reading without a matching definition has no scale or series, so
			// there is nothing meaningful to publish for it.
			continue
		}

		var dynamic []string
		if s.labels != nil {
			dynamic = s.labels.LabelsFor(sample.GPUIndex)
		}

		points = append(points, metric.Point{
			FieldID:  sample.FieldID,
			GPUIndex: sample.GPUIndex,
			Value:    def.ApplyScale(sample.Value),
			Labels:   metric.LabelValues(sample.GPUIndex, dynamic),
		})
	}

	if err := s.sink.Publish(points); err != nil {
		return fmt.Errorf("publish points: %w", err)
	}
	return nil
}
