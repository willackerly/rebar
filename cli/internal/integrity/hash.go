package integrity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// HashFile returns the hex-encoded SHA-256 of a file's contents.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeRoleSalt derives a role-specific salt from the repo salt.
// roleSalt = HMAC-SHA256(repoSalt, "role:<name>")
func ComputeRoleSalt(repoSalt []byte, role string) []byte {
	mac := hmac.New(sha256.New, repoSalt)
	mac.Write([]byte("role:" + role))
	return mac.Sum(nil)
}

// ComputeRoleHMAC produces a role-keyed HMAC of a file hash.
// roleHMAC = HMAC-SHA256(roleSalt, fileHash)
func ComputeRoleHMAC(roleSalt []byte, fileHash string) string {
	mac := hmac.New(sha256.New, roleSalt)
	mac.Write([]byte(fileHash))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateSalt creates a cryptographically random 256-bit salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}
	return salt, nil
}

// SaveSalt writes the salt as hex to .rebar/salt.
func SaveSalt(rebarDir string, salt []byte) error {
	return os.WriteFile(saltPath(rebarDir), []byte(hex.EncodeToString(salt)+"\n"), 0600)
}

// LoadSalt reads the hex-encoded salt from .rebar/salt.
func LoadSalt(rebarDir string) ([]byte, error) {
	data, err := os.ReadFile(saltPath(rebarDir))
	if err != nil {
		return nil, err
	}
	s := string(data)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return hex.DecodeString(s)
}

func saltPath(rebarDir string) string {
	return rebarDir + "/salt"
}
