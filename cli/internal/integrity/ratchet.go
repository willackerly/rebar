package integrity

import (
	"fmt"
	"path/filepath"
	"sort"
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

// DerivedFloors computes ratchet floors from the manifest's per-file entries:
// the sum of recorded assertion counts, and the number of tracked contracts and
// test files. Deriving floors (instead of storing a raised minimum) keeps
// commits from rewriting a shared aggregate line that every branch touches.
func DerivedFloors(manifest *Manifest) map[string]int {
	totalAssertions := 0
	for _, entry := range manifest.Checksums[CategoryTests] {
		if entry.AssertCount != nil {
			totalAssertions += *entry.AssertCount
		}
	}
	return map[string]int{
		"total_assertions": totalAssertions,
		"contract_count":   len(manifest.Checksums[CategoryContracts]),
		"test_file_count":  len(manifest.Checksums[CategoryTests]),
	}
}

// CheckRatchets compares computed values against the manifest's floors.
// A floor is the larger of the derived floor and any legacy stored minimum.
// Returns results sorted by name, with Violated=true if current < floor.
func CheckRatchets(manifest *Manifest, computed map[string]int) []RatchetResult {
	floors := DerivedFloors(manifest)
	for name, r := range manifest.Ratchets {
		if r.Min > floors[name] {
			floors[name] = r.Min
		}
	}

	names := make([]string, 0, len(floors))
	for name := range floors {
		names = append(names, name)
	}
	sort.Strings(names)

	var results []RatchetResult
	for _, name := range names {
		current := computed[name]
		results = append(results, RatchetResult{
			Name:     name,
			Min:      floors[name],
			Current:  current,
			Violated: current < floors[name],
		})
	}
	return results
}
