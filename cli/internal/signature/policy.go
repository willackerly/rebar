package signature

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/willackerly/rebar/cli/internal/integrity"
)

// Policy defines signature requirements for a repository.
type Policy struct {
	Enabled         bool         `json:"enabled"`
	RequireOnCommit bool         `json:"require_on_commit"`
	RequireOnPush   bool         `json:"require_on_push"`
	RequireOnMerge  bool         `json:"require_on_merge_to_main"`
	Rules           []PolicyRule `json:"rules"`
	PostCheckout    *PostCheckoutPolicy `json:"post_checkout,omitempty"`
}

// PolicyRule specifies signature requirements for a file category.
type PolicyRule struct {
	Category         string   `json:"category"`
	RequireFromRoles []string `json:"require_signatures_from_roles"`
	MinSignatures    int      `json:"min_signatures"`
	RequireCISig     bool     `json:"require_ci_signature"`
}

// PostCheckoutPolicy controls verification after git clone/pull.
type PostCheckoutPolicy struct {
	AutoVerify     bool `json:"auto_verify"`
	BlockOnFailure bool `json:"block_on_failure"`
	WarnOnUnsigned bool `json:"warn_on_unsigned"`
}

// PolicyResult holds per-file and per-rule evaluation.
type PolicyResult struct {
	FileResults []FilePolicyResult
	RuleResults []RulePolicyResult
	Passed      bool
}

// FilePolicyResult is the policy evaluation for one file.
type FilePolicyResult struct {
	Path     string
	Category string
	Signed   bool
	SignedBy []string // key IDs
	Missing  []string // roles that should have signed but didn't
}

// RulePolicyResult is the evaluation of one policy rule.
type RulePolicyResult struct {
	Rule   PolicyRule
	Passed bool
	Detail string
}

// LoadPolicy reads .rebar/policy.json.
func LoadPolicy(rebarDir string) (*Policy, error) {
	path := filepath.Join(rebarDir, "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Policy{}, nil // no policy = no requirements
		}
		return nil, err
	}
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing policy: %w", err)
	}
	return &p, nil
}

// SavePolicy writes .rebar/policy.json.
func SavePolicy(rebarDir string, p *Policy) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rebarDir, "policy.json"), append(data, '\n'), 0644)
}

// EvaluatePolicy checks the manifest against policy rules using the trust store.
func EvaluatePolicy(policy *Policy, manifest *integrity.Manifest, ts *TrustStore, repoID string) *PolicyResult {
	result := &PolicyResult{Passed: true}

	if !policy.Enabled {
		return result
	}

	for _, rule := range policy.Rules {
		files, ok := manifest.Checksums[rule.Category]
		if !ok {
			continue
		}

		ruleResult := RulePolicyResult{Rule: rule, Passed: true}
		for path, entry := range files {
			fr := FilePolicyResult{
				Path:     path,
				Category: rule.Category,
			}

			// Check each required role
			for _, requiredRole := range rule.RequireFromRoles {
				found := false
				for _, sig := range entry.Signatures {
					if sig.Role == requiredRole && ts.IsAuthorized(sig.KeyID, requiredRole) {
						// Verify the signature
						trustedKey := ts.FindKey(sig.KeyID)
						if trustedKey != nil {
							pubKey, err := trustedKey.PublicKeyBytes()
							if err == nil {
								if err := VerifySig(&sig, entry.SHA256, requiredRole, repoID, pubKey); err == nil {
									found = true
									fr.SignedBy = append(fr.SignedBy, sig.KeyID)
								}
							}
						}
					}
				}
				if !found {
					fr.Missing = append(fr.Missing, requiredRole)
					ruleResult.Passed = false
					result.Passed = false
				}
			}

			fr.Signed = len(fr.Missing) == 0

			// Check CI signature requirement
			if rule.RequireCISig {
				ciSigned := false
				for _, sig := range entry.Signatures {
					if sig.Role == "ci" && ts.IsAuthorized(sig.KeyID, "ci") {
						ciSigned = true
						break
					}
				}
				if !ciSigned {
					fr.Missing = append(fr.Missing, "ci")
					ruleResult.Passed = false
					result.Passed = false
				}
			}

			// Check min signatures
			if len(entry.Signatures) < rule.MinSignatures {
				ruleResult.Passed = false
				result.Passed = false
				ruleResult.Detail = fmt.Sprintf("%s: %d/%d signatures", path, len(entry.Signatures), rule.MinSignatures)
			}

			result.FileResults = append(result.FileResults, fr)
		}

		result.RuleResults = append(result.RuleResults, ruleResult)
	}

	return result
}
