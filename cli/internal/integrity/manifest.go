package integrity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest is the central integrity tracking document at .rebar/integrity.json.
//
// The file is merge-friendly by construction: the header (generated_at,
// generated_by) is written once by `rebar init` and never restamped, and the
// ratchet floors are derived from the per-file entries (see DerivedFloors), so
// a commit only changes the entries of the files it touches. Two branches that
// touch different protected files merge without conflict.
type Manifest struct {
	SchemaVersion string                          `json:"schema_version"`
	GeneratedAt   time.Time                       `json:"generated_at"`
	GeneratedBy   string                          `json:"generated_by"`
	RepoID        string                          `json:"repo_id"`
	Checksums     map[string]map[string]FileEntry `json:"checksums"` // category → path → entry
	// Ratchets holds legacy stored floors from manifests written before floors
	// were derived. They are kept as a frozen lower bound and never raised.
	Ratchets map[string]Ratchet `json:"ratchets,omitempty"`
}

// FileEntry tracks a single protected file's integrity state.
type FileEntry struct {
	SHA256      string      `json:"sha256"`
	Role        string      `json:"role"`
	RoleHMAC    string      `json:"role_hmac"`
	ModifiedAt  time.Time   `json:"modified_at"`
	AssertCount *int        `json:"assertion_count,omitempty"`
	Signatures  []Signature `json:"signatures,omitempty"`
}

// Signature is an Ed25519 attestation (Phase 5).
type Signature struct {
	KeyID      string    `json:"key_id"`
	Identity   string    `json:"identity"`
	Role       string    `json:"role"`
	Timestamp  time.Time `json:"timestamp"`
	HashSigned string    `json:"hash_signed"`
	Sig        string    `json:"signature"`
}

// Ratchet is a legacy stored floor for a monotonically non-decreasing metric.
// Current is read from old manifests but no longer written.
type Ratchet struct {
	Min     int `json:"min"`
	Current int `json:"current,omitempty"`
}

// Categories of protected files.
const (
	CategoryEnforcement = "enforcement"
	CategoryContracts   = "contracts"
	CategoryTests       = "tests"
)

// NewManifest creates an empty manifest with defaults.
func NewManifest(repoID string) *Manifest {
	return &Manifest{
		SchemaVersion: "1.0",
		GeneratedAt:   time.Now().UTC(),
		GeneratedBy:   "rebar init",
		RepoID:        repoID,
		Checksums: map[string]map[string]FileEntry{
			CategoryEnforcement: {},
			CategoryContracts:   {},
			CategoryTests:       {},
		},
	}
}

// ManifestPath returns the path to integrity.json.
func ManifestPath(rebarDir string) string {
	return filepath.Join(rebarDir, "integrity.json")
}

// LoadManifest reads .rebar/integrity.json.
func LoadManifest(rebarDir string) (*Manifest, error) {
	data, err := os.ReadFile(ManifestPath(rebarDir))
	if err != nil {
		return nil, fmt.Errorf("loading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &m, nil
}

// Save writes the manifest atomically (write to temp, rename).
//
// Save does not restamp GeneratedAt: a per-save timestamp changes the same
// line on every branch, so every pair of PRs would conflict on it.
func (m *Manifest) Save(rebarDir string) error {
	for name, r := range m.Ratchets {
		r.Current = 0
		m.Ratchets[name] = r
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	data = append(data, '\n')

	path := ManifestPath(rebarDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming manifest: %w", err)
	}
	return nil
}
