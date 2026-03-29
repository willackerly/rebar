package integrity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest is the central integrity tracking document at .rebar/integrity.json.
type Manifest struct {
	SchemaVersion string                         `json:"schema_version"`
	GeneratedAt   time.Time                      `json:"generated_at"`
	GeneratedBy   string                         `json:"generated_by"`
	RepoID        string                         `json:"repo_id"`
	Checksums     map[string]map[string]FileEntry `json:"checksums"` // category → path → entry
	Ratchets      map[string]Ratchet             `json:"ratchets"`
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

// Ratchet is a monotonically non-decreasing metric.
type Ratchet struct {
	Min     int `json:"min"`
	Current int `json:"current"`
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
		Ratchets: map[string]Ratchet{
			"total_assertions": {Min: 0, Current: 0},
			"contract_count":   {Min: 0, Current: 0},
			"test_file_count":  {Min: 0, Current: 0},
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
func (m *Manifest) Save(rebarDir string) error {
	m.GeneratedAt = time.Now().UTC()
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
