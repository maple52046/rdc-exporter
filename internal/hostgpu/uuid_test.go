package hostgpu

import (
	"strings"
	"testing"
)

func TestParseROCMSMIUUIDs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        map[int]string
		wantErrText string
	}{
		{
			name: "formats and pads card UUIDs",
			input: `{
				"card0": {"Unique ID": "0x1234"},
				"card2": {"Unique ID": "0xabcdef0123456789"},
				"system": {"Driver version": "test"}
			}`,
			want: map[int]string{
				0: "GPU-0000000000001234",
				2: "GPU-abcdef0123456789",
			},
		},
		{
			name:        "keeps valid cards when another record is malformed",
			input:       `{"card0":{"Unique ID":"0x1"},"card1":{"Unique ID":"not-hex"}}`,
			want:        map[int]string{0: "GPU-0000000000000001"},
			wantErrText: "parse unique ID for card1",
		},
		{
			name:        "rejects invalid JSON",
			input:       `{`,
			wantErrText: "parse rocm-smi JSON",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseROCMSMIUUIDs([]byte(tc.input))
			if tc.wantErrText == "" && err != nil {
				t.Fatalf("parseROCMSMIUUIDs() returned error: %v", err)
			}
			if tc.wantErrText != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrText)) {
				t.Fatalf("parseROCMSMIUUIDs() error = %v, want text %q", err, tc.wantErrText)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseROCMSMIUUIDs() returned %v, want %v", got, tc.want)
			}
			for gpuIndex, wantUUID := range tc.want {
				if gotUUID := got[gpuIndex]; gotUUID != wantUUID {
					t.Errorf("parseROCMSMIUUIDs() UUID for GPU %d = %q, want %q", gpuIndex, gotUUID, wantUUID)
				}
			}
		})
	}
}
