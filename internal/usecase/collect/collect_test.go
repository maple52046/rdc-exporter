package collect

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ROCm/rdc-exporter/internal/domain/metric"
)

// fakeReader returns a fixed set of samples (and optional error) so the use case
// flow can be exercised without the RDC library.
type fakeReader struct {
	samples []metric.Sample
	err     error
}

func (f *fakeReader) ReadSamples(context.Context) ([]metric.Sample, error) {
	return f.samples, f.err
}

// fakeLabels records refresh calls and serves a static GPU-to-label mapping.
type fakeLabels struct {
	keys       []string
	byGPU      map[metric.GPUIndex][]string
	refreshErr error
	refreshed  int
}

func (f *fakeLabels) LabelKeys() []string { return f.keys }

func (f *fakeLabels) LabelsFor(gpuIndex metric.GPUIndex) []string {
	return f.byGPU[gpuIndex]
}

func (f *fakeLabels) Refresh(context.Context) error {
	f.refreshed++
	return f.refreshErr
}

// fakeSink captures published points for assertions.
type fakeSink struct {
	published []metric.Point
	err       error
}

func (f *fakeSink) Publish(points []metric.Point) error {
	f.published = points
	return f.err
}

func TestCollectScalesAndPublishes(t *testing.T) {
	reader := &fakeReader{samples: []metric.Sample{
		{GPUIndex: 0, FieldID: 502, Value: 206141652992},
		{GPUIndex: 1, FieldID: 100, Value: 1500},
	}}
	sink := &fakeSink{}
	defs := []metric.Definition{
		{FieldID: 502, Name: "gpu_memory_total", Scale: 0.000001},
		{FieldID: 100, Name: "gpu_clock", Scale: 1},
	}
	svc := NewService(defs, reader, sink, nil)

	if err := svc.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	want := []metric.Point{
		{FieldID: 502, GPUIndex: 0, Value: 206141.652992, Labels: []string{"0"}},
		{FieldID: 100, GPUIndex: 1, Value: 1500, Labels: []string{"1"}},
	}
	if got := sink.published; !equalPoints(got, want) {
		t.Errorf("Collect() published = %+v, want %+v", got, want)
	}
}

func TestCollectAttachesAdditionalLabels(t *testing.T) {
	reader := &fakeReader{samples: []metric.Sample{
		{GPUIndex: 0, FieldID: 100, Value: 10},
	}}
	sink := &fakeSink{}
	labels := &fakeLabels{
		keys: []string{"pod", "namespace", "container"},
		byGPU: map[metric.GPUIndex][]string{
			0: {"app-a", "user1", "main"},
		},
	}
	defs := []metric.Definition{{FieldID: 100, Name: "gpu_clock", Scale: 1}}
	svc := NewService(defs, reader, sink, labels)

	if err := svc.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	if labels.refreshed != 1 {
		t.Errorf("Collect() refreshed labeler %d times, want 1", labels.refreshed)
	}
	wantLabels := []string{"0", "app-a", "user1", "main"}
	if got := sink.published[0].Labels; !slices.Equal(got, wantLabels) {
		t.Errorf("Collect() labels = %v, want %v", got, wantLabels)
	}
}

func TestCollectSkipsUndefinedFields(t *testing.T) {
	reader := &fakeReader{samples: []metric.Sample{
		{GPUIndex: 0, FieldID: 100, Value: 10},
		{GPUIndex: 0, FieldID: 999, Value: 20},
	}}
	sink := &fakeSink{}
	defs := []metric.Definition{{FieldID: 100, Name: "gpu_clock", Scale: 1}}
	svc := NewService(defs, reader, sink, nil)

	if err := svc.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}
	if len(sink.published) != 1 {
		t.Fatalf("Collect() published %d points, want 1", len(sink.published))
	}
	if sink.published[0].FieldID != 100 {
		t.Errorf("Collect() published field %d, want 100", sink.published[0].FieldID)
	}
}

func TestCollectEmptyReadReturnsErrNoSamples(t *testing.T) {
	svc := NewService(nil, &fakeReader{samples: nil}, &fakeSink{}, nil)
	err := svc.Collect(context.Background())
	if !errors.Is(err, ErrNoSamples) {
		t.Errorf("Collect() error = %v, want ErrNoSamples", err)
	}
}

func TestCollectPropagatesPortErrors(t *testing.T) {
	readerErr := errors.New("reader boom")
	refreshErr := errors.New("refresh boom")
	sinkErr := errors.New("sink boom")

	tests := []struct {
		name    string
		reader  *fakeReader
		labels  *fakeLabels
		sink    *fakeSink
		wantErr error
	}{
		{
			name:    "reader error",
			reader:  &fakeReader{err: readerErr},
			sink:    &fakeSink{},
			wantErr: readerErr,
		},
		{
			name:    "refresh error",
			reader:  &fakeReader{samples: []metric.Sample{{FieldID: 100}}},
			labels:  &fakeLabels{refreshErr: refreshErr},
			sink:    &fakeSink{},
			wantErr: refreshErr,
		},
		{
			name:    "sink error",
			reader:  &fakeReader{samples: []metric.Sample{{FieldID: 100}}},
			sink:    &fakeSink{err: sinkErr},
			wantErr: sinkErr,
		},
	}

	defs := []metric.Definition{{FieldID: 100, Scale: 1}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var lp LabelProvider
			if tc.labels != nil {
				lp = tc.labels
			}
			svc := NewService(defs, tc.reader, tc.sink, lp)
			err := svc.Collect(context.Background())
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Collect() error = %v, want wrap of %v", err, tc.wantErr)
			}
		})
	}
}

func equalPoints(got, want []metric.Point) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].FieldID != want[i].FieldID ||
			got[i].GPUIndex != want[i].GPUIndex ||
			got[i].Value != want[i].Value ||
			!slices.Equal(got[i].Labels, want[i].Labels) {
			return false
		}
	}
	return true
}
