package spec

import (
	"fmt"
	"os"
	"path/filepath"
)

// Sync performs bidirectional synchronization between contracts and specs
func Sync(opts SyncOptions) error {
	fmt.Println("Synchronizing contracts <-> specs\n")

	// Load manifest
	manifest, err := LoadManifest(filepath.Join(opts.RepoRoot, opts.OutDir))
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if len(manifest.Mappings) == 0 {
		fmt.Println("No mappings in manifest. Run 'rebar spec export' first.")
		return nil
	}

	contractsExported := 0
	specsImported := 0
	conflicts := []SyncConflict{}

	// Check each mapping for changes
	for _, mapping := range manifest.Mappings {
		contractPath := filepath.Join(opts.RepoRoot, mapping.Contract)

		// Check if contract exists
		if _, err := os.Stat(contractPath); os.IsNotExist(err) {
			fmt.Printf("  ⚠ Contract removed: %s\n", mapping.Contract)
			continue
		}

		// Check if contract changed
		contractChanged, err := HasContractChanged(contractPath, &mapping)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", mapping.Contract, err)
			continue
		}

		// Check if specs changed
		specsChanged, changedPaths, err := HasSpecChanged(&mapping)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", mapping.Contract, err)
			continue
		}

		// Handle different cases
		if contractChanged && specsChanged {
			// Conflict: both sides changed
			conflicts = append(conflicts, SyncConflict{
				Contract: mapping.Contract,
				Spec:     fmt.Sprintf("%d specs", len(changedPaths)),
				Reason:   "both-modified",
			})
			fmt.Printf("  ⚠ Conflict: %s (both contract and specs changed)\n", mapping.Contract)
			for _, spec := range changedPaths {
				fmt.Printf("      - %s\n", spec)
			}

		} else if contractChanged {
			// Contract changed, re-export
			fmt.Printf("  → %s (contract updated, re-exporting)\n", mapping.Contract)
			if !opts.DryRun {
				// Re-export this contract
				exportOpts := ExportOptions{
					RepoRoot:    opts.RepoRoot,
					ContractDir: opts.ContractDir,
					OutDir:      opts.OutDir,
					Patterns:    []string{mapping.Contract},
					Force:       true,
				}
				if err := Export(exportOpts); err != nil {
					fmt.Printf("      ✗ export failed: %v\n", err)
				} else {
					contractsExported++
				}
			}

		} else if specsChanged {
			// Specs changed, prompt to import
			fmt.Printf("  ← %s (specs updated)\n", mapping.Contract)
			for _, spec := range changedPaths {
				fmt.Printf("      - %s\n", spec)
			}

			if !opts.Force && !opts.DryRun {
				fmt.Printf("      ⚠ Manual review required. Use 'rebar spec import %s' to update contract.\n", changedPaths[0])
			} else if !opts.DryRun {
				// Auto-import
				importOpts := ImportOptions{
					RepoRoot:    opts.RepoRoot,
					ContractDir: opts.ContractDir,
					SpecPaths:   changedPaths,
					Force:       true,
				}
				if err := Import(importOpts); err != nil {
					fmt.Printf("      ✗ import failed: %v\n", err)
				} else {
					specsImported++
				}
			}

		} else {
			// No changes
			fmt.Printf("  ✓ %s (in sync)\n", mapping.Contract)
		}
	}

	fmt.Printf("\n")
	if len(conflicts) > 0 {
		fmt.Printf("⚠ %d conflicts require manual resolution:\n", len(conflicts))
		for _, conflict := range conflicts {
			fmt.Printf("  - %s\n", conflict.Contract)
		}
	} else {
		fmt.Printf("✓ Sync complete (%d exported, %d imported)\n", contractsExported, specsImported)
	}

	if opts.DryRun {
		fmt.Println("(dry-run: no changes applied)")
	}

	return nil
}
