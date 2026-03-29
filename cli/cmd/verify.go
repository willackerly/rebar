package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/integrity"
	"github.com/willackerly/rebar/cli/internal/signature"
)

var (
	verifyStrict       bool
	verifySignatures   bool
	verifyRequireRoles string
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check integrity of protected files",
	Long:  `Compares SHA-256 hashes and role HMACs against the manifest. Detects unauthorized modifications, missing files, and ratchet violations.`,
	RunE:  runVerify,
}

func init() {
	verifyCmd.Flags().BoolVar(&verifyStrict, "strict", false, "exit 1 on any integrity issue")
	verifyCmd.Flags().BoolVar(&verifySignatures, "signatures", false, "also verify digital signatures")
	verifyCmd.Flags().StringVar(&verifyRequireRoles, "require-roles", "", "require signatures from these roles (comma-separated)")
}

func runVerify(cmd *cobra.Command, args []string) error {
	manifest, err := integrity.LoadManifest(cfg.RebarDir)
	if err != nil {
		return fmt.Errorf("no integrity manifest found — run 'rebar init' first\n%w", err)
	}

	// Salt is optional — HMAC checks skip if unavailable
	salt, saltErr := integrity.LoadSalt(cfg.RebarDir)
	if saltErr != nil && verbose {
		fmt.Fprintln(os.Stderr, "note: no salt found, skipping HMAC verification")
	}

	result, err := integrity.Verify(cfg.RepoRoot, manifest, salt)
	if err != nil {
		return err
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	header := "Integrity Check"
	if verifySignatures {
		header = "Integrity + Authenticity Check"
	}
	fmt.Printf("%s — %s\n", header, time.Now().UTC().Format(time.RFC3339))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Print(integrity.FormatResult(result))

	// Signature verification
	if verifySignatures {
		ts, err := signature.LoadTrustStore(filepath.Join(cfg.RebarDir, "trusted-keys"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nwarning: could not load trust store: %v\n", err)
		} else {
			fmt.Println("\nSignature verification:")
			sigIssues := 0
			repoID := manifest.RepoID

			catLabels := map[string]string{
				integrity.CategoryEnforcement: "Enforcement scripts",
				integrity.CategoryContracts:   "Contracts",
				integrity.CategoryTests:       "Tests",
			}

			for _, cat := range []string{integrity.CategoryEnforcement, integrity.CategoryContracts, integrity.CategoryTests} {
				files, ok := manifest.Checksums[cat]
				if !ok || len(files) == 0 {
					continue
				}
				fmt.Printf("\n  %s:\n", catLabels[cat])

				for path, entry := range files {
					if len(entry.Signatures) == 0 {
						fmt.Printf("    ✗ %-40s NO SIGNATURE\n", filepath.Base(path))
						sigIssues++
						continue
					}

					for _, sig := range entry.Signatures {
						trustedKey := ts.FindKey(sig.KeyID)
						if trustedKey == nil {
							fmt.Printf("    ✗ %-40s sig by %s: UNTRUSTED KEY\n", filepath.Base(path), sig.KeyID)
							sigIssues++
							continue
						}

						if !ts.IsAuthorized(sig.KeyID, sig.Role) {
							fmt.Printf("    ✗ %-40s sig by %s: ROLE MISMATCH (signed as %s, not authorized)\n",
								filepath.Base(path), trustedKey.Identity, sig.Role)
							sigIssues++
							continue
						}

						pubKey, err := trustedKey.PublicKeyBytes()
						if err != nil {
							fmt.Printf("    ✗ %-40s sig by %s: INVALID KEY\n", filepath.Base(path), trustedKey.Identity)
							sigIssues++
							continue
						}

						if err := signature.VerifySig(&sig, entry.SHA256, sig.Role, repoID, pubKey); err != nil {
							fmt.Printf("    ✗ %-40s sig by %s: INVALID SIGNATURE\n", filepath.Base(path), trustedKey.Identity)
							sigIssues++
							continue
						}

						fmt.Printf("    ✓ %-40s sig: %s (%s) — %s\n",
							filepath.Base(path), trustedKey.Identity, sig.Role, sig.Timestamp.Format("2006-01-02T15:04:05Z"))
					}
				}
			}

			// Check required roles
			if verifyRequireRoles != "" {
				requiredRoles := strings.Split(verifyRequireRoles, ",")
				fmt.Printf("\n  Required roles: ")
				allMet := true
				for _, role := range requiredRoles {
					role = strings.TrimSpace(role)
					found := false
					for _, files := range manifest.Checksums {
						for _, entry := range files {
							for _, sig := range entry.Signatures {
								if sig.Role == role && ts.IsAuthorized(sig.KeyID, role) {
									found = true
									break
								}
							}
							if found {
								break
							}
						}
						if found {
							break
						}
					}
					if found {
						fmt.Printf("%s ✓  ", role)
					} else {
						fmt.Printf("%s ✗  ", role)
						allMet = false
						sigIssues++
					}
				}
				fmt.Println()
				if !allMet {
					result.Clean = false
				}
			}

			if sigIssues > 0 {
				fmt.Printf("\n  %d authenticity issue(s)\n", sigIssues)
				result.Clean = false
			} else if len(ts.Keys) > 0 {
				fmt.Println("\n  All signatures verified ✓")
			}
		}
	}

	if verifyStrict && !result.Clean {
		os.Exit(1)
	}

	return nil
}
