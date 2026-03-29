package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/signature"
)

var keyIdentity string
var keyRoles string

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage Ed25519 signing keys",
}

var keyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a new signing keypair",
	RunE: func(cmd *cobra.Command, args []string) error {
		identity := keyIdentity
		if identity == "" {
			// Default to git user
			out, _ := exec.Command("git", "config", "user.email").Output()
			identity = strings.TrimSpace(string(out))
			if identity == "" {
				return fmt.Errorf("--identity required (or set git user.email)")
			}
		}

		kp, err := signature.GenerateKeyPair(identity)
		if err != nil {
			return err
		}

		keysDir := filepath.Join(cfg.RebarDir, "keys")
		if err := signature.SavePrivateKey(kp, keysDir); err != nil {
			return err
		}

		fmt.Printf("Keypair generated\n")
		fmt.Printf("  Key ID:     %s\n", kp.KeyID)
		fmt.Printf("  Identity:   %s\n", kp.Identity)
		fmt.Printf("  Public key: %s\n", signature.ExportPublicKey(kp))
		fmt.Printf("  Stored at:  %s\n", filepath.Join(keysDir, kp.KeyID+".pem"))
		fmt.Printf("\nTo trust this key in a repo:\n")
		fmt.Printf("  rebar key trust --identity %q --role <roles> --pubkey %s\n", identity, signature.ExportPublicKey(kp))

		return nil
	},
}

var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trusted public keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		ts, err := signature.LoadTrustStore(filepath.Join(cfg.RebarDir, "trusted-keys"))
		if err != nil {
			return err
		}

		if len(ts.Keys) == 0 {
			fmt.Println("No trusted keys. Use 'rebar key trust' to add one.")
			return nil
		}

		fmt.Printf("%-18s %-30s %-20s %s\n", "KEY_ID", "IDENTITY", "ROLES", "STATUS")
		fmt.Println(strings.Repeat("─", 90))
		for _, k := range ts.Keys {
			status := "active"
			if k.Revoked {
				status = "REVOKED"
			}
			fmt.Printf("%-18s %-30s %-20s %s\n", k.KeyID, k.Identity, strings.Join(k.Roles, ","), status)
		}

		// Also show local private key if available
		keysDir := filepath.Join(cfg.RebarDir, "keys")
		if kp, err := signature.LoadPrivateKey(keysDir); err == nil {
			fmt.Printf("\nLocal signing key: %s (%s)\n", kp.KeyID, kp.Identity)
		}

		return nil
	},
}

var trustPubkey string

var keyTrustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Add a public key to the trust store",
	RunE: func(cmd *cobra.Command, args []string) error {
		if trustPubkey == "" && len(args) > 0 {
			// Read from file
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading public key file: %w", err)
			}
			trustPubkey = strings.TrimSpace(string(data))
		}

		if trustPubkey == "" {
			return fmt.Errorf("provide --pubkey or a file containing the public key")
		}

		identity := keyIdentity
		if identity == "" {
			return fmt.Errorf("--identity required")
		}

		roles := []string{}
		if keyRoles != "" {
			roles = strings.Split(keyRoles, ",")
		}
		if len(roles) == 0 {
			return fmt.Errorf("--role required (comma-separated: architect,tester,developer,steward,ci)")
		}

		// Validate public key
		pubBytes, err := hex.DecodeString(trustPubkey)
		if err != nil || len(pubBytes) != 32 {
			return fmt.Errorf("invalid public key (expected 64 hex chars)")
		}

		keyID := signature.KeyIDFromPublic(pubBytes)

		ts, err := signature.LoadTrustStore(filepath.Join(cfg.RebarDir, "trusted-keys"))
		if err != nil {
			return err
		}

		// Check for duplicate
		if existing := ts.FindKey(keyID); existing != nil {
			return fmt.Errorf("key %s already trusted (identity: %s)", keyID, existing.Identity)
		}

		key := signature.TrustedKey{
			KeyID:     keyID,
			Identity:  identity,
			PublicKey: trustPubkey,
			Roles:     roles,
		}

		if err := ts.AddKey(key); err != nil {
			return err
		}

		fmt.Printf("Trusted key added\n")
		fmt.Printf("  Key ID:   %s\n", keyID)
		fmt.Printf("  Identity: %s\n", identity)
		fmt.Printf("  Roles:    %s\n", strings.Join(roles, ", "))

		return nil
	},
}

var keyRevokeCmd = &cobra.Command{
	Use:   "revoke [key-id]",
	Short: "Revoke a trusted key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ts, err := signature.LoadTrustStore(filepath.Join(cfg.RebarDir, "trusted-keys"))
		if err != nil {
			return err
		}
		if err := ts.RevokeKey(args[0]); err != nil {
			return err
		}
		fmt.Printf("Key %s revoked\n", args[0])
		return nil
	},
}

func init() {
	keyInitCmd.Flags().StringVar(&keyIdentity, "identity", "", "identity label (e.g., email or CI name)")
	keyTrustCmd.Flags().StringVar(&keyIdentity, "identity", "", "identity label (required)")
	keyTrustCmd.Flags().StringVar(&keyRoles, "role", "", "authorized roles (comma-separated)")
	keyTrustCmd.Flags().StringVar(&trustPubkey, "pubkey", "", "hex-encoded public key")

	keyCmd.AddCommand(keyInitCmd)
	keyCmd.AddCommand(keyListCmd)
	keyCmd.AddCommand(keyTrustCmd)
	keyCmd.AddCommand(keyRevokeCmd)
}
