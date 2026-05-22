package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	upgradeDryRun bool
	upgradeForce  bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade <version>",
	Short: "Upgrade project to a different rebar version",
	Long: `Migrates this project from its current rebar version (in .rebar-version)
to a target version. Runs version-specific migration scripts and updates
.rebar-version.

Goose-style migration: each version transition has an optional migration
script at $REBAR_DIR/migrations/<old>-to-<new>.sh that handles schema
changes, config rewrites, or file migrations.

By default this is a dry run. Use --force to apply.

Examples:
  rebar upgrade v3.1.0           Show migration plan
  rebar upgrade v3.1.0 --force   Execute migration
`,
	Args: cobra.ExactArgs(1),
	RunE: runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeDryRun, "dry-run", false, "show migration plan without applying (default behavior)")
	upgradeCmd.Flags().BoolVar(&upgradeForce, "force", false, "apply migration")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	targetVersion := args[0]
	root := cfg.RepoRoot

	// Read current version
	versionPath := filepath.Join(root, ".rebar-version")
	currentBytes, err := os.ReadFile(versionPath)
	if err != nil {
		return fmt.Errorf("reading .rebar-version: %w", err)
	}
	currentVersion := strings.TrimSpace(string(currentBytes))

	if currentVersion == targetVersion {
		fmt.Printf("Already at %s\n", targetVersion)
		return nil
	}

	// Resolve target framework dir
	home, _ := os.UserHomeDir()
	targetDir := filepath.Join(home, ".rebar", "versions", targetVersion)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("rebar %s not installed\nRun: curl ... | bash -s -- %s", targetVersion, targetVersion)
	}

	// Look for migration script
	migrationName := fmt.Sprintf("%s-to-%s.sh", currentVersion, targetVersion)
	migrationScript := filepath.Join(targetDir, "migrations", migrationName)

	hasMigration := false
	if _, err := os.Stat(migrationScript); err == nil {
		hasMigration = true
	}

	// Dry-run or force check
	isDryRun := !upgradeForce

	fmt.Printf("Upgrade: %s → %s\n", currentVersion, targetVersion)
	if hasMigration {
		fmt.Printf("  Migration script: %s\n", migrationName)
	} else {
		fmt.Println("  No migration script (version bump only)")
	}

	if isDryRun {
		fmt.Println("\nDRY RUN — pass --force to apply")
		if hasMigration {
			fmt.Println("\nMigration script preview:")
			data, _ := os.ReadFile(migrationScript)
			fmt.Println(string(data))
		}
		return nil
	}

	// Execute migration
	if hasMigration {
		fmt.Println("\nRunning migration...")
		c := exec.Command("bash", migrationScript, root)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		fmt.Println("✓ Migration complete")
	}

	// Update .rebar-version
	if err := os.WriteFile(versionPath, []byte(targetVersion+"\n"), 0644); err != nil {
		return fmt.Errorf("updating .rebar-version: %w", err)
	}
	fmt.Printf("✓ Updated .rebar-version to %s\n", targetVersion)

	// Verify post-upgrade health
	fmt.Println("\nRunning post-upgrade audit...")
	c := exec.Command(os.Args[0], "audit")
	c.Dir = root
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run() // ignore error — audit prints its own output

	return nil
}
