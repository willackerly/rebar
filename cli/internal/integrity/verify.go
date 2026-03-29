package integrity

import (
	"fmt"
	"path/filepath"
	"strings"
)

// VerifyResult holds the complete output of an integrity verification.
type VerifyResult struct {
	Files     []FileResult
	Missing   []string // in manifest but not on disk
	Untracked []string // on disk but not in manifest
	Ratchets  []RatchetResult
	Clean     bool
}

// FileResult is the verification status of a single file.
type FileResult struct {
	Path      string
	Category  string
	HashMatch bool
	HMACMatch bool
	HMACCheck bool // whether HMAC was checked (false if no salt)
	OldHash   string
	NewHash   string
	OldAssert *int
	NewAssert *int
	Detail    string
}

// Verify compares the manifest against the filesystem.
// If salt is nil, HMAC checks are skipped (Layer 1 only).
func Verify(repoRoot string, manifest *Manifest, salt []byte) (*VerifyResult, error) {
	result := &VerifyResult{Clean: true}

	// Scan current protected files on disk
	onDisk, err := ScanProtectedFiles(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("scanning files: %w", err)
	}

	// Build lookup of what's on disk
	diskSet := map[string]string{} // path → category
	for cat, paths := range onDisk {
		for _, p := range paths {
			diskSet[p] = cat
		}
	}

	// Build lookup of what's in manifest
	manifestSet := map[string]string{} // path → category
	for cat, files := range manifest.Checksums {
		for path := range files {
			manifestSet[path] = cat
		}
	}

	// Check each file in manifest
	for cat, files := range manifest.Checksums {
		for path, entry := range files {
			fullPath := filepath.Join(repoRoot, path)
			currentHash, err := HashFile(fullPath)
			if err != nil {
				result.Missing = append(result.Missing, path)
				result.Clean = false
				continue
			}

			fr := FileResult{
				Path:      path,
				Category:  cat,
				HashMatch: currentHash == entry.SHA256,
				OldHash:   entry.SHA256,
				NewHash:   currentHash,
				OldAssert: entry.AssertCount,
			}

			// Check assertion count for test files
			if cat == CategoryTests {
				count, err := CountAssertions(fullPath)
				if err == nil {
					fr.NewAssert = &count
				}
			}

			// Check role HMAC if salt available
			if salt != nil {
				fr.HMACCheck = true
				roleSalt := ComputeRoleSalt(salt, entry.Role)
				expectedHMAC := ComputeRoleHMAC(roleSalt, currentHash)
				fr.HMACMatch = expectedHMAC == entry.RoleHMAC
			}

			if !fr.HashMatch {
				if fr.HMACCheck && !fr.HMACMatch {
					fr.Detail = "MODIFIED outside rebar CLI"
				} else {
					fr.Detail = "MODIFIED (hash mismatch)"
				}
				result.Clean = false
			}

			result.Files = append(result.Files, fr)
		}
	}

	// Check for untracked protected files (on disk but not in manifest)
	for path := range diskSet {
		if _, exists := manifestSet[path]; !exists {
			result.Untracked = append(result.Untracked, path)
		}
	}

	// Compute and check ratchets
	computed, err := ComputeRatchets(repoRoot)
	if err == nil {
		result.Ratchets = CheckRatchets(manifest, computed)
		for _, r := range result.Ratchets {
			if r.Violated {
				result.Clean = false
			}
		}
	}

	return result, nil
}

// FormatResult produces human-readable verification output.
func FormatResult(r *VerifyResult) string {
	var b strings.Builder

	// Group files by category
	byCat := map[string][]FileResult{}
	for _, f := range r.Files {
		byCat[f.Category] = append(byCat[f.Category], f)
	}

	catNames := []string{CategoryEnforcement, CategoryContracts, CategoryTests}
	catLabels := map[string]string{
		CategoryEnforcement: "Enforcement scripts",
		CategoryContracts:   "Contracts",
		CategoryTests:       "Tests",
	}

	for _, cat := range catNames {
		files, ok := byCat[cat]
		if !ok || len(files) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s:\n", catLabels[cat])
		for _, f := range files {
			mark := "✓"
			detail := "hash OK"
			if !f.HashMatch {
				mark = "✗"
				detail = f.Detail
			}
			line := fmt.Sprintf("  %s %-45s %s", mark, filepath.Base(f.Path), detail)
			if f.NewAssert != nil && f.OldAssert != nil {
				line += fmt.Sprintf(", assertions: %d (was %d)", *f.NewAssert, *f.OldAssert)
				if *f.NewAssert < *f.OldAssert {
					line += " — RATCHET VIOLATION"
				}
			}
			fmt.Fprintln(&b, line)
		}
	}

	// Missing files
	if len(r.Missing) > 0 {
		fmt.Fprintln(&b, "\nMissing (in manifest but not on disk):")
		for _, p := range r.Missing {
			fmt.Fprintf(&b, "  ✗ %s\n", p)
		}
	}

	// Untracked files
	if len(r.Untracked) > 0 {
		fmt.Fprintln(&b, "\nUntracked (on disk but not in manifest):")
		for _, p := range r.Untracked {
			fmt.Fprintf(&b, "  ? %s\n", p)
		}
	}

	// Ratchets
	hasRatchets := false
	for _, r := range r.Ratchets {
		if r.Min > 0 || r.Current > 0 {
			hasRatchets = true
			break
		}
	}
	if hasRatchets {
		fmt.Fprintln(&b, "\nRatchets:")
		for _, r := range r.Ratchets {
			mark := "✓"
			extra := ""
			if r.Violated {
				mark = "✗"
				extra = "  VIOLATION"
			}
			fmt.Fprintf(&b, "  %s %s: %d (min %d)%s\n", mark, r.Name, r.Current, r.Min, extra)
		}
	}

	// Summary
	if r.Clean && len(r.Untracked) == 0 {
		fmt.Fprintln(&b, "\nRESULT: Integrity verified ✓")
	} else {
		violations := 0
		for _, f := range r.Files {
			if !f.HashMatch {
				violations++
			}
		}
		violations += len(r.Missing)
		for _, rr := range r.Ratchets {
			if rr.Violated {
				violations++
			}
		}
		if violations > 0 {
			fmt.Fprintf(&b, "\nRESULT: %d integrity violation(s) detected\n", violations)
		} else if len(r.Untracked) > 0 {
			fmt.Fprintf(&b, "\nRESULT: %d untracked protected file(s) — run rebar commit to track\n", len(r.Untracked))
		}
	}

	return b.String()
}
