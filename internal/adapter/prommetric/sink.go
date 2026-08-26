// Package prommetric adapts the Prometheus client into the collect.MetricSink
// port.
//
// This is an interface-adapter package: it registers one Prometheus gauge per
// metric definition and updates those gauges from domain metric.Point values. It
// is the only place the collection flow touches the Prometheus client, keeping
// prometheus types out of the use-case and domain layers. The package depends on
// the inner domain and on the Prometheus framework; nothing inner depends on it.
package prommetric

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/ROCm/rdc-exporter/internal/domain/metric"
	"github.com/prometheus/client_golang/prometheus"
)

// Sink registers and updates the Prometheus gauges that back the exported
// metrics.
//
// One GaugeVec is created per field at construction; Publish then sets values on
// those vectors. Sink also remembers the label values last published for each
// (field, GPU) so it can drop a series whose labels changed and avoid leaking
// stale label combinations. Sink is driven by a single collection loop and is
// not safe for concurrent Publish calls; the underlying GaugeVec setters are
// concurrency-safe, but the label bookkeeping is not.
type Sink struct {
	// gauges holds one vector per registered field, keyed by field id.
	gauges map[metric.FieldID]*prometheus.GaugeVec
	// lastLabels records the most recent label values per field and GPU so a
	// changed label set can delete its previous series before the new one is set.
	lastLabels map[metric.FieldID]map[metric.GPUIndex][]string
}

// New registers a gauge for every definition and returns a ready Sink.
//
// All series share the same label names: the mandatory gpu_index label followed
// by additionalLabelKeys (which may include workload and GPU identity labels).
// Definitions are expected to carry a resolved Name and Help; duplicate field
// ids register a single gauge and the extras are ignored. Registration failure
// (for example a duplicate or invalid metric name) is returned as an error so
// the caller can fail startup instead of panicking.
func New(reg *prometheus.Registry, definitions []metric.Definition, additionalLabelKeys []string) (*Sink, error) {
	labelKeys := metric.LabelKeys(additionalLabelKeys)

	sink := &Sink{
		gauges:     make(map[metric.FieldID]*prometheus.GaugeVec, len(definitions)),
		lastLabels: make(map[metric.FieldID]map[metric.GPUIndex][]string),
	}

	for _, def := range definitions {
		if _, exists := sink.gauges[def.FieldID]; exists {
			continue
		}

		gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: def.Name,
			Help: def.Help,
		}, labelKeys)

		if err := reg.Register(gauge); err != nil {
			return nil, fmt.Errorf("register gauge %q: %w", def.Name, err)
		}

		sink.gauges[def.FieldID] = gauge
		sink.lastLabels[def.FieldID] = make(map[metric.GPUIndex][]string)
	}

	return sink, nil
}

// Publish sets each point's value on its field's gauge.
//
// A point whose field has no registered gauge is logged and skipped. When a
// (field, GPU) series' label values differ from the previous Publish, the old
// series is deleted first so changed additional labels (for example a GPU moving
// to a new pod) do not leave a stale time series behind. Publish always returns
// nil today; it returns an error to satisfy the MetricSink contract and to allow
// future backends to report failures.
func (s *Sink) Publish(points []metric.Point) error {
	for _, point := range points {
		gauge, ok := s.gauges[point.FieldID]
		if !ok {
			slog.Warn("No gauge registered for field", "fieldID", point.FieldID)
			continue
		}

		if previous, seen := s.lastLabels[point.FieldID][point.GPUIndex]; seen && !slices.Equal(previous, point.Labels) {
			gauge.DeleteLabelValues(previous...)
		}

		gauge.WithLabelValues(point.Labels...).Set(point.Value)
		s.lastLabels[point.FieldID][point.GPUIndex] = point.Labels
	}
	return nil
}
