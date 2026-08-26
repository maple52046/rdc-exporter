package gpulabeler

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ROCm/rdc-exporter/internal/domain/metric"
)

// fakeWorkloadLabeler supplies workload labels without a kubelet connection.
type fakeWorkloadLabeler struct {
	refreshErr error
	refreshed  int
}

func (f *fakeWorkloadLabeler) LabelKeys() []string {
	return []string{"pod", "namespace", "container"}
}

func (f *fakeWorkloadLabeler) LabelsFor(metric.GPUIndex) []string {
	return []string{"app", "default", "main"}
}

func (f *fakeWorkloadLabeler) Refresh(context.Context) error {
	f.refreshed++
	return f.refreshErr
}

func TestLabelerWithoutWorkload(t *testing.T) {
	uuidByGPU := map[int]string{0: "GPU-0000000000001234"}
	labeler := New(uuidByGPU, nil)
	uuidByGPU[0] = "GPU-mutated"

	if got, want := labeler.LabelKeys(), []string{UUIDLabel}; !slices.Equal(got, want) {
		t.Errorf("LabelKeys() = %v, want %v", got, want)
	}
	if got, want := labeler.LabelsFor(0), []string{"GPU-0000000000001234"}; !slices.Equal(got, want) {
		t.Errorf("LabelsFor(0) = %v, want %v", got, want)
	}
	if got, want := labeler.LabelsFor(1), []string{""}; !slices.Equal(got, want) {
		t.Errorf("LabelsFor(1) = %v, want %v", got, want)
	}
	if err := labeler.Refresh(context.Background()); err != nil {
		t.Errorf("Refresh() returned error: %v", err)
	}
}

func TestLabelerCombinesAndRefreshesWorkloadLabels(t *testing.T) {
	workload := &fakeWorkloadLabeler{}
	labeler := New(map[int]string{0: "GPU-0000000000001234"}, workload)

	if got, want := labeler.LabelKeys(), []string{"pod", "namespace", "container", UUIDLabel}; !slices.Equal(got, want) {
		t.Errorf("LabelKeys() = %v, want %v", got, want)
	}
	if got, want := labeler.LabelsFor(0), []string{"app", "default", "main", "GPU-0000000000001234"}; !slices.Equal(got, want) {
		t.Errorf("LabelsFor(0) = %v, want %v", got, want)
	}
	if err := labeler.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() returned error: %v", err)
	}
	if workload.refreshed != 1 {
		t.Errorf("Refresh() called workload provider %d times, want 1", workload.refreshed)
	}
}

func TestLabelerWrapsWorkloadRefreshError(t *testing.T) {
	wantErr := errors.New("kubelet unavailable")
	labeler := New(nil, &fakeWorkloadLabeler{refreshErr: wantErr})

	if err := labeler.Refresh(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("Refresh() error = %v, want wrap of %v", err, wantErr)
	}
}
