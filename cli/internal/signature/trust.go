package signature

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TrustStore manages the set of trusted public keys for a repository.
type TrustStore struct {
	Dir  string
	Keys []TrustedKey
}

// LoadTrustStore reads all trusted keys from .rebar/trusted-keys/.
func LoadTrustStore(trustedKeysDir string) (*TrustStore, error) {
	ts := &TrustStore{Dir: trustedKeysDir}

	entries, err := os.ReadDir(trustedKeysDir)
	if err != nil {
		if os.IsNotExist(err) {
			return ts, nil
		}
		return nil, err
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(trustedKeysDir, e.Name()))
		if err != nil {
			continue
		}
		var key TrustedKey
		if err := json.Unmarshal(data, &key); err != nil {
			continue
		}
		ts.Keys = append(ts.Keys, key)
	}

	return ts, nil
}

// AddKey adds a public key to the trust store.
func (ts *TrustStore) AddKey(key TrustedKey) error {
	if err := os.MkdirAll(ts.Dir, 0755); err != nil {
		return err
	}

	key.TrustedAt = time.Now().UTC()
	data, err := json.MarshalIndent(key, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(ts.Dir, key.KeyID+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing trusted key: %w", err)
	}

	ts.Keys = append(ts.Keys, key)
	return nil
}

// RevokeKey marks a key as revoked.
func (ts *TrustStore) RevokeKey(keyID string) error {
	for i := range ts.Keys {
		if ts.Keys[i].KeyID == keyID {
			now := time.Now().UTC()
			ts.Keys[i].Revoked = true
			ts.Keys[i].RevokedAt = &now

			data, _ := json.MarshalIndent(ts.Keys[i], "", "  ")
			path := filepath.Join(ts.Dir, keyID+".json")
			return os.WriteFile(path, append(data, '\n'), 0644)
		}
	}
	return fmt.Errorf("key %s not found in trust store", keyID)
}

// FindKey looks up a key by ID. Returns nil if not found or revoked.
func (ts *TrustStore) FindKey(keyID string) *TrustedKey {
	for _, k := range ts.Keys {
		if k.KeyID == keyID && !k.Revoked {
			return &k
		}
	}
	return nil
}

// IsAuthorized checks if a key is trusted for a specific role.
func (ts *TrustStore) IsAuthorized(keyID, role string) bool {
	key := ts.FindKey(keyID)
	if key == nil {
		return false
	}
	for _, r := range key.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// PublicKeyBytes returns the decoded public key for a trusted key.
func (tk *TrustedKey) PublicKeyBytes() (ed25519.PublicKey, error) {
	return hex.DecodeString(tk.PublicKey)
}
