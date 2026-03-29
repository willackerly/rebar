package signature

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// KeyPair holds an Ed25519 identity.
type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	KeyID      string
	Identity   string
	CreatedAt  time.Time
}

// TrustedKey is a public key in the trust store with role authorization.
type TrustedKey struct {
	KeyID     string     `json:"key_id"`
	Identity  string     `json:"identity"`
	PublicKey string     `json:"public_key"` // hex-encoded
	Roles     []string   `json:"roles"`
	TrustedAt time.Time  `json:"trusted_at"`
	TrustedBy string     `json:"trusted_by,omitempty"`
	Revoked   bool       `json:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// GenerateKeyPair creates a new Ed25519 keypair.
func GenerateKeyPair(identity string) (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating keypair: %w", err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  pub,
		KeyID:      KeyIDFromPublic(pub),
		Identity:   identity,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// KeyIDFromPublic computes a short fingerprint from a public key.
// Uses first 8 bytes of SHA-256(pubkey), hex-encoded (16 chars).
func KeyIDFromPublic(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:8])
}

// SavePrivateKey writes the private key as PEM to .rebar/keys/<keyid>.pem.
func SavePrivateKey(kp *KeyPair, keysDir string) error {
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return err
	}

	block := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: kp.PrivateKey.Seed(), // 32 bytes
	}
	data := pem.EncodeToMemory(block)

	path := filepath.Join(keysDir, kp.KeyID+".pem")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing private key: %w", err)
	}

	// Also write identity metadata
	meta := map[string]string{
		"key_id":     kp.KeyID,
		"identity":   kp.Identity,
		"created_at": kp.CreatedAt.Format(time.RFC3339),
		"public_key": hex.EncodeToString(kp.PublicKey),
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	metaPath := filepath.Join(keysDir, kp.KeyID+".json")
	return os.WriteFile(metaPath, append(metaData, '\n'), 0600)
}

// LoadPrivateKey reads the first available private key from .rebar/keys/.
func LoadPrivateKey(keysDir string) (*KeyPair, error) {
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return nil, fmt.Errorf("reading keys directory: %w", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".pem" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(keysDir, e.Name()))
		if err != nil {
			continue
		}

		block, _ := pem.Decode(data)
		if block == nil || block.Type != "ED25519 PRIVATE KEY" {
			continue
		}

		priv := ed25519.NewKeyFromSeed(block.Bytes)
		pub := priv.Public().(ed25519.PublicKey)
		keyID := KeyIDFromPublic(pub)

		// Load identity from metadata
		identity := ""
		metaPath := filepath.Join(keysDir, keyID+".json")
		if metaData, err := os.ReadFile(metaPath); err == nil {
			var meta map[string]string
			if json.Unmarshal(metaData, &meta) == nil {
				identity = meta["identity"]
			}
		}

		return &KeyPair{
			PrivateKey: priv,
			PublicKey:  pub,
			KeyID:      keyID,
			Identity:   identity,
		}, nil
	}

	return nil, fmt.Errorf("no private key found in %s — run 'rebar key init'", keysDir)
}

// ExportPublicKey returns the public key as hex-encoded bytes.
func ExportPublicKey(kp *KeyPair) string {
	return hex.EncodeToString(kp.PublicKey)
}
