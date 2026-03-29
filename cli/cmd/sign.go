package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/integrity"
	"github.com/willackerly/rebar/cli/internal/signature"
)

var signRole string
var signAllVerified bool

var signCmd = &cobra.Command{
	Use:   "sign [files...]",
	Short: "Sign protected files with your Ed25519 identity",
	Long: `Produces digital signatures attesting that you (or your CI identity)
approve the current state of protected files.

Each signature covers: SHA256(fileHash || role || timestamp || repoID)`,
	RunE: runSign,
}

func init() {
	signCmd.Flags().StringVar(&signRole, "role", "", "role to sign as (required)")
	signCmd.Flags().BoolVar(&signAllVerified, "all-verified", false, "sign all verified files in this role's category")
}

func runSign(cmd *cobra.Command, args []string) error {
	if signRole == "" {
		return fmt.Errorf("--role required (e.g., architect, tester, ci)")
	}

	// Load private key
	keysDir := filepath.Join(cfg.RebarDir, "keys")
	kp, err := signature.LoadPrivateKey(keysDir)
	if err != nil {
		return err
	}

	// Load manifest
	manifest, err := integrity.LoadManifest(cfg.RebarDir)
	if err != nil {
		return fmt.Errorf("no manifest — run 'rebar init' first")
	}

	// Load repo ID
	repoID := manifest.RepoID

	// Determine which files to sign
	targets := map[string]map[string]integrity.FileEntry{}

	if signAllVerified {
		// Sign all files in the role's default category
		catForRole := map[string]string{
			"architect": integrity.CategoryContracts,
			"tester":    integrity.CategoryTests,
			"steward":   integrity.CategoryEnforcement,
			"ci":        integrity.CategoryEnforcement,
			"developer": "", // no default category
		}
		cat, ok := catForRole[signRole]
		if !ok || cat == "" {
			return fmt.Errorf("no default category for role %q — specify files explicitly", signRole)
		}
		if files, exists := manifest.Checksums[cat]; exists {
			targets[cat] = files
		}
	} else if len(args) > 0 {
		// Sign specific files
		for _, arg := range args {
			found := false
			for cat, files := range manifest.Checksums {
				if entry, ok := files[arg]; ok {
					if targets[cat] == nil {
						targets[cat] = map[string]integrity.FileEntry{}
					}
					targets[cat][arg] = entry
					found = true
				}
			}
			if !found {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s not in manifest, skipping\n", arg)
			}
		}
	} else {
		return fmt.Errorf("specify files to sign, or use --all-verified")
	}

	// Sign each file
	signed := 0
	for cat, files := range targets {
		for path, entry := range files {
			// Verify hash is current before signing
			fullPath := filepath.Join(cfg.RepoRoot, path)
			currentHash, err := integrity.HashFile(fullPath)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: cannot read %s, skipping\n", path)
				continue
			}
			if currentHash != entry.SHA256 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s hash changed since last commit — run 'rebar commit' first\n", path)
				continue
			}

			sig, err := signature.Sign(entry.SHA256, signRole, repoID, kp)
			if err != nil {
				return fmt.Errorf("signing %s: %w", path, err)
			}

			entry.Signatures = append(entry.Signatures, *sig)
			manifest.Checksums[cat][path] = entry
			signed++

			if verbose {
				fmt.Printf("  ✓ %s signed as %s by %s\n", path, signRole, kp.Identity)
			}
		}
	}

	if signed == 0 {
		return fmt.Errorf("no files signed")
	}

	// Save updated manifest
	manifest.GeneratedBy = fmt.Sprintf("rebar sign --role %s", signRole)
	manifest.GeneratedAt = time.Now().UTC()
	if err := manifest.Save(cfg.RebarDir); err != nil {
		return err
	}

	fmt.Printf("Signed %d file(s) as %s (identity: %s, key: %s)\n", signed, signRole, kp.Identity, kp.KeyID)

	return nil
}
