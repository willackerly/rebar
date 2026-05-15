package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/repo"
	"github.com/willackerly/rebar/cli/internal/scripts"
)

var (
	migrateNamespaceFlag string
	migrateWrite         bool
	migrateJSON          bool
)

var contractMigrateNamespaceCmd = &cobra.Command{
	Use:   "migrate-namespace",
	Short: "Rewrite contract references to use the repo namespace (Go-module form)",
	Long: `Rewrites every contract reference in this repo from the legacy
form (CONTRACT:<id>.<major>.<minor>) to the namespaced form
(CONTRACT:<host>/<org>/<repo>:<id>.<major>.<minor>).

The namespace is inferred from 'git remote get-url origin' unless
overridden by --namespace or the contract_namespace key in .rebarrc.

Scope of rewrites:
  - architecture/CONTRACT-*.md            title line + body references
  - source files (.go .ts .tsx .js .jsx .py .rs) in src/ internal/ cmd/
    client/ packages/ lib/ app/           CONTRACT: header references
  - scripts/*.sh                          CONTRACT: comment headers

Filenames are NOT renamed — the repo's filesystem path implicitly
namespaces the contract.

Templates and methodology docs (DESIGN.md, conventions.md, QUICKSTART.md,
QUICKCONTEXT.md, templates/project-bootstrap/) are skipped because they
contain illustrative examples that aren't real contracts.

By default this is a dry run. Use --write to apply.

Exit codes:
  0  no drift (already migrated) OR --write succeeded
  1  drift found in dry-run mode
  2  namespace could not be resolved
`,
	RunE: runMigrateNamespace,
}

func init() {
	contractMigrateNamespaceCmd.Flags().StringVar(&migrateNamespaceFlag, "namespace", "", "override namespace (e.g. github.com/owner/repo)")
	contractMigrateNamespaceCmd.Flags().BoolVar(&migrateWrite, "write", false, "apply changes (default: dry-run)")
	contractMigrateNamespaceCmd.Flags().BoolVar(&migrateJSON, "json", false, "machine-readable JSON output")
	contractCmd.AddCommand(contractMigrateNamespaceCmd)
}

// Matches a legacy reference: CONTRACT: followed by ID.MAJOR.MINOR with
// NO colon immediately after CONTRACT:NS (i.e. namespace not already
// present). The ID must start with an uppercase letter.
//
// Used in source files, contract bodies, and shell comment headers.
var legacyRefRE = regexp.MustCompile(`CONTRACT:([A-Z][A-Za-z0-9_-]*)\.(\d+)\.(\d+)\b`)

// Matches a legacy title line in a contract markdown file:
//
//	# CONTRACT-S1-STEWARD.1.0
//
// Capture group 1 = ID + version (e.g. S1-STEWARD.1.0).
var legacyTitleRE = regexp.MustCompile(`(?m)^# CONTRACT-([A-Z][A-Za-z0-9_-]*\.\d+\.\d+)\b`)

// Detects a reference that is ALREADY namespaced. Used for idempotence:
// if a file is fully migrated, the legacy regex matches nothing and the
// file is skipped.
var namespacedRefRE = regexp.MustCompile(`CONTRACT:[a-zA-Z0-9][a-zA-Z0-9_./-]+:[A-Z][A-Za-z0-9_-]*\.\d+\.\d+\b`)

type migrationChange struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"` // "contract-md", "source", "script"
	Count int    `json:"count"`
}

type migrationReport struct {
	Namespace        string            `json:"namespace"`
	NamespaceSource  string            `json:"namespace_source"` // "flag" | ".rebarrc" | "git-remote"
	DryRun           bool              `json:"dry_run"`
	Changes          []migrationChange `json:"changes"`
	TotalFiles       int               `json:"total_files"`
	TotalReplacements int              `json:"total_replacements"`
}

func runMigrateNamespace(cmd *cobra.Command, args []string) error {
	ns, source, err := resolveMigrationNamespace()
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate-namespace: %v\n", err)
		os.Exit(2)
	}

	// Guard: refuse if .rebarrc already declares a different namespace
	// (the user-provided override is the way to deliberately change it).
	existing := cfg.ContractNamespace
	if existing != "" && existing != ns && migrateNamespaceFlag == "" {
		fmt.Fprintf(os.Stderr, "migrate-namespace: .rebarrc declares contract_namespace=%q but inferred %q.\n", existing, ns)
		fmt.Fprintln(os.Stderr, "Pass --namespace=<ns> to force a change, or align .rebarrc with the remote.")
		os.Exit(2)
	}

	report := migrationReport{
		Namespace:       ns,
		NamespaceSource: source,
		DryRun:          !migrateWrite,
	}

	if err := scanContractDir(cfg.RepoRoot, ns, &report); err != nil {
		return err
	}
	if err := scanSourceDirs(cfg.RepoRoot, ns, &report); err != nil {
		return err
	}
	if err := scanScriptsDir(cfg.RepoRoot, ns, &report); err != nil {
		return err
	}

	if migrateWrite {
		if err := ensureRebarRCNamespace(filepath.Join(cfg.RepoRoot, ".rebarrc"), ns); err != nil {
			return fmt.Errorf("updating .rebarrc: %w", err)
		}
		// Regenerate registry so the ID column reflects the new namespace.
		if _, err := os.Stat(filepath.Join(cfg.ScriptsDir, "compute-registry.sh")); err == nil {
			_, _ = scripts.RunPassthrough(cfg.ScriptsDir, "compute-registry.sh")
		}
	}

	if migrateJSON {
		return emitMigrationJSON(&report)
	}
	emitMigrationText(&report)

	if !migrateWrite && report.TotalReplacements > 0 {
		os.Exit(1)
	}
	return nil
}

func resolveMigrationNamespace() (string, string, error) {
	if migrateNamespaceFlag != "" {
		return migrateNamespaceFlag, "flag", nil
	}
	if cfg.ContractNamespace != "" {
		return cfg.ContractNamespace, ".rebarrc", nil
	}
	ns, err := repo.InferNamespace(cfg.RepoRoot)
	if err != nil {
		return "", "", fmt.Errorf("could not infer namespace from git remote: %w\nPass --namespace=<host>/<org>/<repo> or set contract_namespace in .rebarrc", err)
	}
	return ns, "git-remote", nil
}

// scanContractDir walks architecture/CONTRACT-*.md and rewrites titles
// and bodies. Skips templates and the auto-generated registry.
func scanContractDir(repoRoot, ns string, report *migrationReport) error {
	archDir := filepath.Join(repoRoot, "architecture")
	entries, err := os.ReadDir(archDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading architecture dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "CONTRACT-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		// Skip non-contract files in this directory.
		switch name {
		case "CONTRACT-TEMPLATE.md", "CONTRACT-SEAM-TEMPLATE.md",
			"CONTRACT-REGISTRY.md", "CONTRACT-REGISTRY.template.md", "CONTRACT-GAPS.md":
			continue
		}

		path := filepath.Join(archDir, name)
		if err := rewriteFile(path, ns, "contract-md", report); err != nil {
			return err
		}
	}
	return nil
}

// scanSourceDirs walks the standard source directories with the same
// extension and skip rules as scripts/check-contract-headers.sh.
func scanSourceDirs(repoRoot, ns string, report *migrationReport) error {
	exts := map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".rs": true,
	}
	dirs := []string{"src", "internal", "cmd", "client", "packages", "lib", "app", "cli"}

	for _, d := range dirs {
		dir := filepath.Join(repoRoot, d)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		err = filepath.Walk(dir, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fi.IsDir() {
				base := filepath.Base(path)
				if base == "vendor" || base == "node_modules" || base == "dist" || base == "build" || base == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(path)
			if !exts[ext] {
				return nil
			}
			name := fi.Name()
			// Skip tests + generated files.
			if strings.HasSuffix(name, "_test.go") ||
				strings.HasSuffix(name, ".test.ts") || strings.HasSuffix(name, ".test.tsx") ||
				strings.HasSuffix(name, ".test.js") || strings.HasSuffix(name, ".spec.ts") ||
				strings.HasSuffix(name, ".spec.tsx") || strings.HasSuffix(name, ".spec.js") ||
				strings.Contains(name, "_generated") || strings.Contains(name, ".gen.") ||
				strings.HasSuffix(name, ".pb.go") || strings.HasSuffix(name, ".pb.ts") {
				return nil
			}
			return rewriteFile(path, ns, "source", report)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// scanScriptsDir walks scripts/ for shell scripts with CONTRACT: comment
// headers (e.g. scripts/steward.sh L3). Restricted to top-level .sh.
func scanScriptsDir(repoRoot, ns string, report *migrationReport) error {
	scriptsDir := filepath.Join(repoRoot, "scripts")
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading scripts dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		path := filepath.Join(scriptsDir, e.Name())
		if err := rewriteFile(path, ns, "script", report); err != nil {
			return err
		}
	}
	return nil
}

// rewriteFile reads a file, applies legacy → namespaced rewrites
// idempotently, optionally writes the result. Records change counts in
// the report.
func rewriteFile(path, ns, kind string, report *migrationReport) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	content := string(data)

	// Heuristic guard: skip files that are clearly templates (still
	// contain unfilled placeholders). The migrator can't reliably
	// distinguish a real contract from a template if the body has
	// {ID} or {NAMESPACE} placeholders.
	if strings.Contains(content, "{NAMESPACE}") || strings.Contains(content, "{ID}-{NAME}") {
		return nil
	}

	original := content
	replacements := 0

	// 1. Title line in contract markdown files.
	content = legacyTitleRE.ReplaceAllStringFunc(content, func(m string) string {
		// Avoid double-prefixing if the title is somehow already
		// namespaced (idempotence).
		if strings.Contains(m, "CONTRACT-"+ns+":") {
			return m
		}
		idVer := legacyTitleRE.FindStringSubmatch(m)[1]
		replacements++
		return "# CONTRACT-" + ns + ":" + idVer
	})

	// 2. Inline CONTRACT: references in any file.
	content = legacyRefRE.ReplaceAllStringFunc(content, func(m string) string {
		sub := legacyRefRE.FindStringSubmatch(m)
		// sub[1] = ID; sub[2] = major; sub[3] = minor.
		replacements++
		return "CONTRACT:" + ns + ":" + sub[1] + "." + sub[2] + "." + sub[3]
	})

	if replacements == 0 || content == original {
		return nil
	}

	report.Changes = append(report.Changes, migrationChange{
		Path:  relPath(path, cfg.RepoRoot),
		Kind:  kind,
		Count: replacements,
	})
	report.TotalFiles++
	report.TotalReplacements += replacements

	if migrateWrite {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func relPath(abs, root string) string {
	if rel, err := filepath.Rel(root, abs); err == nil {
		return rel
	}
	return abs
}

// ensureRebarRCNamespace appends contract_namespace=<ns> to .rebarrc if
// the key isn't already present. Idempotent.
func ensureRebarRCNamespace(path, ns string) error {
	var lines []string
	hasKey := false

	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if strings.EqualFold(strings.TrimSpace(parts[0]), "contract_namespace") {
					// Replace existing value to keep .rebarrc canonical.
					line = "contract_namespace = " + ns
					hasKey = true
				}
			}
			lines = append(lines, line)
		}
		f.Close()
	} else if !os.IsNotExist(err) {
		return err
	}

	if !hasKey {
		lines = append(lines, "")
		lines = append(lines, "# Contract namespace (host/org/repo). All CONTRACT: references in this repo")
		lines = append(lines, "# are prefixed with this value. Inferred from `git remote get-url origin`.")
		lines = append(lines, "contract_namespace = "+ns)
	}

	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0644)
}

func emitMigrationText(r *migrationReport) {
	mode := "DRY RUN — pass --write to apply"
	if !r.DryRun {
		mode = "applied"
	}
	fmt.Printf("migrate-namespace: %s\n", mode)
	fmt.Printf("  namespace: %s  (source: %s)\n", r.Namespace, r.NamespaceSource)
	if r.TotalReplacements == 0 {
		fmt.Println("  no drift — every CONTRACT: reference is already namespaced (or there are none)")
		return
	}

	// Group changes by kind for a readable summary.
	byKind := map[string][]migrationChange{}
	for _, c := range r.Changes {
		byKind[c.Kind] = append(byKind[c.Kind], c)
	}
	kindOrder := []string{"contract-md", "source", "script"}
	for _, k := range kindOrder {
		group, ok := byKind[k]
		if !ok {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].Path < group[j].Path })
		fmt.Printf("\n  %s (%d files):\n", k, len(group))
		for _, c := range group {
			fmt.Printf("    %-60s %d replacement(s)\n", c.Path, c.Count)
		}
	}
	fmt.Printf("\n  total: %d files, %d replacement(s)\n", r.TotalFiles, r.TotalReplacements)
	if r.DryRun {
		fmt.Println("\nRe-run with --write to apply.")
	} else {
		fmt.Println("\n.rebarrc updated, CONTRACT-REGISTRY.md regenerated.")
	}
}

func emitMigrationJSON(r *migrationReport) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
