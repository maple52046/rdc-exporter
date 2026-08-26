package hostgpu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// rocmSMICard is the portion of a rocm-smi card record needed to derive the
// stable GPU identity exported with metrics. Keeping the external JSON shape in
// this driver package prevents the command's schema from leaking inward.
type rocmSMICard struct {
	UniqueID string `json:"Unique ID"`
}

// LoadUUIDs queries rocm-smi once and returns DCGM-compatible UUID labels keyed
// by the GPU indexes reported as cardN.
//
// The command is bound to ctx and is expected to be available through PATH.
// Individual malformed card records are omitted while valid records are
// returned alongside a joined error, allowing callers to retain partial
// discovery results. A command or top-level JSON failure returns no results.
func LoadUUIDs(ctx context.Context) (map[int]string, error) {
	command := exec.CommandContext(ctx, "rocm-smi", "--json", "--showuniqueid")
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return nil, fmt.Errorf("run rocm-smi: %w: %s", err, stderr)
			}
		}
		return nil, fmt.Errorf("run rocm-smi: %w", err)
	}

	return parseROCMSMIUUIDs(output)
}

// parseROCMSMIUUIDs converts the rocm-smi JSON shape into the UUID convention
// used by DCGM-compatible dashboards: GPU- followed by 16 lowercase hex digits.
// It deliberately ignores non-card top-level records because some rocm-smi
// versions include metadata alongside card entries.
func parseROCMSMIUUIDs(data []byte) (map[int]string, error) {
	var cards map[string]rocmSMICard
	if err := json.Unmarshal(data, &cards); err != nil {
		return nil, fmt.Errorf("parse rocm-smi JSON: %w", err)
	}

	cardNames := make([]string, 0, len(cards))
	for cardName := range cards {
		cardNames = append(cardNames, cardName)
	}
	sort.Strings(cardNames)

	uuids := make(map[int]string)
	var parseErrors []error
	for _, cardName := range cardNames {
		card := cards[cardName]
		indexText, isCard := strings.CutPrefix(cardName, "card")
		if !isCard {
			continue
		}

		gpuIndex, err := strconv.Atoi(indexText)
		if err != nil || gpuIndex < 0 {
			parseErrors = append(parseErrors, fmt.Errorf("parse GPU index from %q", cardName))
			continue
		}

		hexID, ok := strings.CutPrefix(card.UniqueID, "0x")
		if !ok || hexID == "" {
			parseErrors = append(parseErrors, fmt.Errorf("parse unique ID for %s: expected 0x-prefixed hexadecimal value", cardName))
			continue
		}

		uniqueID, err := strconv.ParseUint(hexID, 16, 64)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("parse unique ID for %s: %w", cardName, err))
			continue
		}
		uuids[gpuIndex] = fmt.Sprintf("GPU-%016x", uniqueID)
	}

	return uuids, errors.Join(parseErrors...)
}
