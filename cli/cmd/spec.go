package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/spec"
)

var (
	specOutDir      string
	specFormat      string
	specForce       bool
	specDryRun      bool
	specContractDir string
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Export, import, and sync framework-agnostic specifications",
	Long: `Convert REBAR contracts to/from standard formats (Gherkin, Mermaid, OpenAPI).

Enables interoperability with other agentic frameworks and tools.

Examples:
  rebar spec export                    # export all contracts to specs/
  rebar spec export --out build/specs  # custom output directory
  rebar spec import specs/gherkin/*.feature  # import Gherkin scenarios
  rebar spec sync                      # bidirectional sync with change detection`,
}

var specExportCmd = &cobra.Command{
	Use:   "export [contract-path...]",
	Short: "Export contracts to standard formats",
	Long: `Export REBAR contracts to framework-agnostic specifications.

Output directory structure:
  specs/
    gherkin/      Behavior scenarios (.feature)
    mermaid/      Architecture diagrams (.mmd)
    openapi/      API contracts (.yaml)
    schemas/      Data schemas (.json)
    adrs/         Decision records (.md)
    .spec-manifest.json  Sync tracking

Supported formats:
  - Gherkin: Behavior scenarios (from Scenarios section)
  - Mermaid: Architecture diagrams (from mermaid code blocks)
  - OpenAPI: API contracts (from API section)
  - JSON Schema: Data models (from Data section)
  - ADR: Architecture decisions (from Decisions section)

Examples:
  rebar spec export
  rebar spec export architecture/CONTRACT-*.md
  rebar spec export --format gherkin
  rebar spec export --out build/specs`,
	RunE: runSpecExport,
}

var specImportCmd = &cobra.Command{
	Use:   "import <spec-file...>",
	Short: "Import standard specs as REBAR contracts",
	Long: `Generate or update REBAR contracts from external specifications.

Auto-detects format from file extension:
  .feature   -> Gherkin scenarios
  .mmd       -> Mermaid diagrams
  .yaml      -> OpenAPI (if paths/components present)
  .json      -> JSON Schema (if $schema present)
  .md        -> ADR (if decision record format)

Examples:
  rebar spec import specs/gherkin/*.feature
  rebar spec import specs/openapi/api.yaml
  rebar spec import specs/**/*                  # import all`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSpecImport,
}

var specSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Bidirectional sync between contracts and specs",
	Long: `Synchronize REBAR contracts with exported specifications.

Detects changes on both sides using checksums in .spec-manifest.json:
  - Contract changed → re-export to specs/
  - Spec changed → prompt to update contract
  - Both changed → conflict warning, manual resolution

Examples:
  rebar spec sync                 # sync all
  rebar spec sync --dry-run       # show what would change
  rebar spec sync --force         # overwrite without prompting`,
	RunE: runSpecSync,
}

func init() {
	// Parent command flags
	specCmd.PersistentFlags().StringVar(&specOutDir, "out", "specs", "output directory for exported specs")
	specCmd.PersistentFlags().StringVar(&specContractDir, "contracts", "architecture", "directory containing contracts")

	// Export flags
	specExportCmd.Flags().StringVar(&specFormat, "format", "", "export only this format (gherkin, mermaid, openapi, schema, adr)")
	specExportCmd.Flags().BoolVar(&specForce, "force", false, "overwrite existing spec files")
	specExportCmd.Flags().BoolVar(&specDryRun, "dry-run", false, "show what would be exported without writing")

	// Import flags
	specImportCmd.Flags().BoolVar(&specForce, "force", false, "overwrite existing contracts without prompting")
	specImportCmd.Flags().BoolVar(&specDryRun, "dry-run", false, "show what would be imported without writing")

	// Sync flags
	specSyncCmd.Flags().BoolVar(&specForce, "force", false, "resolve conflicts by overwriting")
	specSyncCmd.Flags().BoolVar(&specDryRun, "dry-run", false, "show changes without applying")

	specCmd.AddCommand(specExportCmd)
	specCmd.AddCommand(specImportCmd)
	specCmd.AddCommand(specSyncCmd)
	rootCmd.AddCommand(specCmd)
}

func runSpecExport(cmd *cobra.Command, args []string) error {
	if repoRoot == "" {
		return fmt.Errorf("not in a REBAR repository")
	}

	opts := spec.ExportOptions{
		RepoRoot:    repoRoot,
		ContractDir: specContractDir,
		OutDir:      specOutDir,
		Format:      specFormat,
		Force:       specForce,
		DryRun:      specDryRun,
		Patterns:    args,
	}

	return spec.Export(opts)
}

func runSpecImport(cmd *cobra.Command, args []string) error {
	if repoRoot == "" {
		return fmt.Errorf("not in a REBAR repository")
	}

	opts := spec.ImportOptions{
		RepoRoot:    repoRoot,
		ContractDir: specContractDir,
		SpecPaths:   args,
		Force:       specForce,
		DryRun:      specDryRun,
	}

	return spec.Import(opts)
}

func runSpecSync(cmd *cobra.Command, args []string) error {
	if repoRoot == "" {
		return fmt.Errorf("not in a REBAR repository")
	}

	opts := spec.SyncOptions{
		RepoRoot:    repoRoot,
		ContractDir: specContractDir,
		OutDir:      specOutDir,
		Force:       specForce,
		DryRun:      specDryRun,
	}

	return spec.Sync(opts)
}
