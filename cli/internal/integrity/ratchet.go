package integrity

import (
	"fmt"
	"path/filepath"
)

// RatchetResult reports whether a ratchet was violated.
type RatchetResult struct {
	Name     string
	Min      int
	Current  int
	Violated bool
}

// ComputeRatchets scans the repo and computes current ratchet values.
func ComputeRatchets(repoRoot string) (map[string]int, error) {
	files, err := ScanProtectedFiles(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("scanning files for ratchets: %w", err)
	}

	totalAssertions := 0
	for _, testFile := range files[CategoryTests] {
		count, err := CountAssertions(filepath.Join(repoRoot, testFile))
		if err != nil {
			continue // skip unreadable files
		}
		totalAssertions += count
	}

	return map[string]int{
		"total_assertions": totalAssertions,
		"contract_count":   len(files[CategoryContracts]),
		"test_file_count":  len(files[CategoryTests]),
	}, nil
}

// CheckRatchets compares computed values against manifest minimums.
// Returns results for each ratchet, with Violated=true if current < min.
func CheckRatchets(manifest *Manifest, computed map[string]int) []RatchetResult {
	var results []RatchetResult
	for name, ratchet := range manifest.Ratchets {
		current, ok := computed[name]
		if !ok {
			current = 0
		}
		results = append(results, RatchetResult{
			Name:     name,
			Min:      ratchet.Min,
			Current:  current,
			Violated: current < ratchet.Min,
		})
	}
	return results
}

// UpdateRatchets updates the manifest with computed values.
// Min ratchets up: min = max(old min, current).
func UpdateRatchets(manifest *Manifest, computed map[string]int) {
	for name, current := range computed {
		r, ok := manifest.Ratchets[name]
		if !ok {
			r = Ratchet{}
		}
		r.Current = current
		if current > r.Min {
			r.Min = current
		}
		manifest.Ratchets[name] = r
	}
}
