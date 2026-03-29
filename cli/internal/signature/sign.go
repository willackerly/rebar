package signature

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/willackerly/rebar/cli/internal/integrity"
)

// Sign produces a signature over a file hash bound to a role and repo.
// Signs: SHA256(fileHash || role || timestamp || repoID)
func Sign(fileHash, role, repoID string, kp *KeyPair) (*integrity.Signature, error) {
	ts := time.Now().UTC()
	message := buildSignatureMessage(fileHash, role, ts, repoID)

	sig := ed25519.Sign(kp.PrivateKey, message)

	return &integrity.Signature{
		KeyID:      kp.KeyID,
		Identity:   kp.Identity,
		Role:       role,
		Timestamp:  ts,
		HashSigned: fileHash,
		Sig:        base64.StdEncoding.EncodeToString(sig),
	}, nil
}

// VerifySig checks that a signature is valid for the given parameters.
func VerifySig(sig *integrity.Signature, fileHash, role, repoID string, pubKey ed25519.PublicKey) error {
	message := buildSignatureMessage(fileHash, role, sig.Timestamp, repoID)

	sigBytes, err := base64.StdEncoding.DecodeString(sig.Sig)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !ed25519.Verify(pubKey, message, sigBytes) {
		return fmt.Errorf("signature verification failed for key %s", sig.KeyID)
	}

	return nil
}

// buildSignatureMessage constructs the message to sign.
// Uses || as separator to prevent ambiguity.
func buildSignatureMessage(fileHash, role string, ts time.Time, repoID string) []byte {
	combined := fileHash + "||" + role + "||" + ts.Format(time.RFC3339Nano) + "||" + repoID
	h := sha256.Sum256([]byte(combined))
	return h[:]
}
