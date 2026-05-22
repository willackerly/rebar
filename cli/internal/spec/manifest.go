package spec

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const ManifestFile = ".spec-manifest.json"

// LoadManifest reads the spec manifest from the output directory
func LoadManifest(outDir string) (*Manifest, error) {
	path := filepath.Join(outDir, ManifestFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// No manifest yet, return empty
		return &Manifest{
			Version:  "1.0",
			LastSync: time.Now(),
			Mappings: []SpecMapping{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &m, nil
}

// SaveManifest writes the spec manifest to the output directory
func SaveManifest(outDir string, m *Manifest) error {
	m.LastSync = time.Now()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	path := filepath.Join(outDir, ManifestFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// FindMapping finds the mapping for a given contract path
func (m *Manifest) FindMapping(contractPath string) *SpecMapping {
	for i := range m.Mappings {
		if m.Mappings[i].Contract == contractPath {
			return &m.Mappings[i]
		}
	}
	return nil
}

// UpdateMapping updates or adds a mapping in the manifest
func (m *Manifest) UpdateMapping(mapping SpecMapping) {
	for i := range m.Mappings {
		if m.Mappings[i].Contract == mapping.Contract {
			m.Mappings[i] = mapping
			return
		}
	}
	m.Mappings = append(m.Mappings, mapping)
}

// RemoveMapping removes a mapping from the manifest
func (m *Manifest) RemoveMapping(contractPath string) {
	for i := range m.Mappings {
		if m.Mappings[i].Contract == contractPath {
			m.Mappings = append(m.Mappings[:i], m.Mappings[i+1:]...)
			return
		}
	}
}

// ComputeChecksum calculates SHA256 checksum of a file
func ComputeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// HasContractChanged checks if contract checksum differs from manifest
func HasContractChanged(contractPath string, mapping *SpecMapping) (bool, error) {
	if mapping == nil {
		return true, nil // No mapping = new contract
	}

	current, err := ComputeChecksum(contractPath)
	if err != nil {
		return false, err
	}

	return current != mapping.ContractChecksum, nil
}

// HasSpecChanged checks if any exported spec checksum differs from manifest
func HasSpecChanged(repoRoot string, mapping *SpecMapping) (bool, []string, error) {
	changed := []string{}

	for _, exp := range mapping.Exports {
		absPath := filepath.Join(repoRoot, exp.Path)
		current, err := ComputeChecksum(absPath)
		if os.IsNotExist(err) {
			changed = append(changed, exp.Path)
			continue
		}
		if err != nil {
			return false, nil, err
		}

		if current != exp.Checksum {
			changed = append(changed, exp.Path)
		}
	}

	return len(changed) > 0, changed, nil
}
