package metric

import (
	"slices"
	"testing"
)

func TestLabelKeys(t *testing.T) {
	tests := []struct {
		name       string
		additional []string
		want       []string
	}{
		{name: "no additional keys", additional: nil, want: []string{GPUIndexLabel}},
		{
			name:       "additional keys follow gpu index",
			additional: []string{"pod", "namespace", "container", "UUID"},
			want:       []string{GPUIndexLabel, "pod", "namespace", "container", "UUID"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LabelKeys(tc.additional)
			if !slices.Equal(got, tc.want) {
				t.Errorf("LabelKeys(%v) = %v, want %v", tc.additional, got, tc.want)
			}
		})
	}
}

func TestLabelValues(t *testing.T) {
	tests := []struct {
		name       string
		gpuIndex   GPUIndex
		additional []string
		want       []string
	}{
		{name: "only gpu index", gpuIndex: 3, additional: nil, want: []string{"3"}},
		{
			name:       "additional values follow gpu index",
			gpuIndex:   0,
			additional: []string{"app", "user1", "main", "GPU-0000000000001234"},
			want:       []string{"0", "app", "user1", "main", "GPU-0000000000001234"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LabelValues(tc.gpuIndex, tc.additional)
			if !slices.Equal(got, tc.want) {
				t.Errorf("LabelValues(%v, %v) = %v, want %v", tc.gpuIndex, tc.additional, got, tc.want)
			}
		})
	}
}
