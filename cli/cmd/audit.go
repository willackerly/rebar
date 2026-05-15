package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	auditAll  string // directory to scan for repos
	auditFix  bool   // auto-fix quick wins
)

var auditCmd = &cobra.Command{
	Use:   "audit [path]",
	Short: "Assess REBAR compliance for a project",
	Long: `Runs a structured compliance audit against REBAR v2.0.0.
Without arguments, audits the current directory.

  rebar audit              # audit current repo
  rebar audit /path/to/repo  # audit specific repo
  rebar audit --all ~/dev  # audit all repos in a directory
  rebar audit --fix        # audit + auto-fix quick wins`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAudit,
}

func init() {
	auditCmd.Flags().StringVar(&auditAll, "all", "", "scan all repos in this directory")
	auditCmd.Flags().BoolVar(&auditFix, "fix", false, "auto-fix quick wins (create .rebarrc, install hook, etc.)")
	// auditCmd is registered in root.go with GroupID
}

type auditResult struct {
	Section string
	Score   int // 0-10
	Weight  int // percentage
	Checks  []checkResult
}

type checkResult struct {
	Name   string
	Pass   bool
	Detail string
}

func runAudit(cmd *cobra.Command, args []string) error {
	if auditAll != "" {
		return runAuditAll(auditAll)
	}

	root := ""
	if len(args) > 0 {
		root = args[0]
	} else if cfg != nil {
		root = cfg.RepoRoot
	} else {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	score, results := auditRepo(root)

	if auditFix {
		fixed := applyFixes(root)
		if fixed > 0 {
			fmt.Printf("\n  Applied %d fix(es). Re-running audit...\n\n", fixed)
			score, results = auditRepo(root)
		}
	}

	printAuditReport(root, score, results)
	return nil
}

type repoScore struct {
	Name  string // display name (parent/child for nested)
	Path  string
	Score float64
	Tier  string
}

// skipForRepoScan returns true for directory names that should never be
// descended into during --all repo discovery (caches, deps, build output).
func skipForRepoScan(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "dist", "build", "target",
		"out", ".next", ".nuxt", "coverage":
		return true
	}
	return false
}

// collectRepos walks `dir` to find git repos, recursing into non-repo
// subdirectories up to `depth` levels. Each found repo gets a display name
// that includes its parent path relative to the original --all root, so
// `~/dev/opentdf/TDFLite` shows up as `opentdf/TDFLite` in the table.
func collectRepos(dir, prefix string, depth int, scores *[]repoScore) {
	if depth <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if skipForRepoScan(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		displayName := e.Name()
		if prefix != "" {
			displayName = prefix + "/" + displayName
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			// Git repo: audit it.
			score, _ := auditRepo(path)
			tier := "none"
			if data, err := os.ReadFile(filepath.Join(path, ".rebarrc")); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.Contains(line, "tier") {
						parts := strings.SplitN(line, "=", 2)
						if len(parts) == 2 {
							tier = strings.TrimSpace(parts[1])
						}
					}
				}
			}
			*scores = append(*scores, repoScore{Name: displayName, Path: path, Score: score, Tier: tier})
		} else if depth > 1 {
			// Not a repo — descend one more level to find nested repos
			// (e.g., ~/dev/opentdf/TDFLite under ~/dev).
			collectRepos(path, displayName, depth-1, scores)
		}
	}
}

func runAuditAll(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	var scores []repoScore
	collectRepos(dir, "", 2, &scores)

	sort.Slice(scores, func(i, j int) bool { return scores[i].Score > scores[j].Score })

	// Right-size the name column to the longest display name (min 20).
	nameWidth := 20
	for _, s := range scores {
		if len(s.Name) > nameWidth {
			nameWidth = len(s.Name)
		}
	}

	fmt.Println()
	fmt.Printf("  %-*s %6s  %s\n", nameWidth, "REPOSITORY", "SCORE", "TIER")
	fmt.Printf("  %-*s %6s  %s\n", nameWidth, strings.Repeat("─", 10), "─────", "────")
	for _, s := range scores {
		bar := scoreBar(s.Score)
		fmt.Printf("  %-*s %5.1f  %s  tier %s\n", nameWidth, s.Name, s.Score, bar, s.Tier)
	}
	fmt.Println()

	return nil
}

func auditRepo(root string) (float64, []auditResult) {
	results := []auditResult{
		auditStructure(root),
		auditContracts(root),
		auditEnforcement(root),
		auditAgents(root),
		auditLifecycle(root),
		auditAccuracy(root),
		auditTesting(root),
	}

	// Weighted score
	total := 0.0
	totalWeight := 0
	for _, r := range results {
		total += float64(r.Score) * float64(r.Weight) / 100.0
		totalWeight += r.Weight
	}
	if totalWeight > 0 {
		total = total * 100.0 / float64(totalWeight)
	}

	return total, results
}

// --- Section Auditors ---

func auditStructure(root string) auditResult {
	checks := []checkResult{
		fileExists(root, "README.md", "Project README"),
		fileContains(root, "README.md", "rebar v", "Rebar badge in README"),
		fileExists(root, "QUICKCONTEXT.md", "QUICKCONTEXT.md"),
		fileContains(root, "QUICKCONTEXT.md", "last-synced", "QUICKCONTEXT has last-synced date"),
		fileExists(root, "TODO.md", "TODO.md"),
		fileExists(root, "AGENTS.md", "AGENTS.md"),
		fileExists(root, ".rebarrc", ".rebarrc tier declaration"),
		fileExists(root, ".rebar-version", ".rebar-version file"),
	}
	return auditResult{Section: "Structural Presence", Score: scoreChecks(checks), Weight: 15, Checks: checks}
}

func auditContracts(root string) auditResult {
	archDir := filepath.Join(root, "architecture")
	checks := []checkResult{
		dirExists(root, "architecture", "architecture/ directory"),
	}

	// Count contract files
	contracts := countGlob(filepath.Join(archDir, "CONTRACT-*.md"))
	contracts -= countGlob(filepath.Join(archDir, "CONTRACT-TEMPLATE*.md"))
	contracts -= countGlob(filepath.Join(archDir, "CONTRACT-REGISTRY*.md"))
	contracts -= countGlob(filepath.Join(archDir, "CONTRACT-SEAM-TEMPLATE*.md"))
	if contracts < 0 {
		contracts = 0
	}
	checks = append(checks, checkResult{
		Name: fmt.Sprintf("Contract files (%d found)", contracts),
		Pass: contracts > 0,
	})

	// Check for CONTRACT: headers in source
	headerCount := countContractHeaders(root)
	checks = append(checks, checkResult{
		Name:   fmt.Sprintf("CONTRACT: headers in source (%d files)", headerCount),
		Pass:   headerCount > 0,
		Detail: fmt.Sprintf("%d source files with CONTRACT: headers", headerCount),
	})

	return auditResult{Section: "Contract System", Score: scoreChecks(checks), Weight: 20, Checks: checks}
}

func auditEnforcement(root string) auditResult {
	checks := []checkResult{
		dirExists(root, "scripts", "scripts/ directory"),
		hookInstalled(root),
		fileExists(root, "scripts/refresh-context.sh", "refresh-context.sh"),
	}

	// Check for enforcement scripts
	for _, script := range []string{"check-contract-refs.sh", "check-todos.sh"} {
		checks = append(checks, fileExists(root, filepath.Join("scripts", script), script))
	}

	return auditResult{Section: "Enforcement & Scripts", Score: scoreChecks(checks), Weight: 15, Checks: checks}
}

func auditAgents(root string) auditResult {
	checks := []checkResult{
		dirExists(root, "agents", "agents/ directory"),
		fileExists(root, "agents/subagent-guidelines.md", "subagent-guidelines.md"),
	}

	// Count agent roles
	roles := countAgentRoles(root)
	checks = append(checks, checkResult{
		Name: fmt.Sprintf("Agent roles (%d found)", roles),
		Pass: roles > 0,
	})

	return auditResult{Section: "Agent Coordination", Score: scoreChecks(checks), Weight: 10, Checks: checks}
}

func auditLifecycle(root string) auditResult {
	checks := []checkResult{
		fileContains(root, "AGENTS.md", "session", "AGENTS.md mentions session lifecycle"),
		fileContains(root, "QUICKCONTEXT.md", "What's Next", "QUICKCONTEXT has What's Next section"),
	}

	// Check TODO length
	todoLines := countFileLines(root, "TODO.md")
	openItems := countPattern(root, "TODO.md", "- [ ]")
	checks = append(checks, checkResult{
		Name:   fmt.Sprintf("TODO.md is concise (%d lines, %d open items)", todoLines, openItems),
		Pass:   todoLines < 200 || openItems < 30,
		Detail: fmt.Sprintf("%d total lines, %d open items", todoLines, openItems),
	})

	return auditResult{Section: "Session Lifecycle", Score: scoreChecks(checks), Weight: 10, Checks: checks}
}

func auditAccuracy(root string) auditResult {
	checks := []checkResult{}

	// Check QUICKCONTEXT freshness
	lastSynced := extractDateFromFile(root, "QUICKCONTEXT.md")
	if lastSynced != "" {
		syncTime, err := time.Parse("2006-01-02", lastSynced)
		if err == nil {
			days := int(time.Since(syncTime).Hours() / 24)
			checks = append(checks, checkResult{
				Name:   fmt.Sprintf("QUICKCONTEXT freshness (%d days old)", days),
				Pass:   days <= 7,
				Detail: fmt.Sprintf("last-synced: %s (%d days ago)", lastSynced, days),
			})
		}
	} else {
		checks = append(checks, checkResult{Name: "QUICKCONTEXT freshness", Pass: false, Detail: "no last-synced date found"})
	}

	// Check if METRICS file exists
	metricsExists := false
	for _, name := range []string{"METRICS", "METRICS.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			metricsExists = true
			break
		}
	}
	checks = append(checks, checkResult{Name: "METRICS file exists", Pass: metricsExists})

	return auditResult{Section: "Content Accuracy", Score: scoreChecks(checks), Weight: 20, Checks: checks}
}

func auditTesting(root string) auditResult {
	checks := []checkResult{
		fileContainsAny(root, "AGENTS.md", []string{"T0", "T1", "T2", "Typecheck", "Targeted", "Package"}, "Testing tiers defined"),
	}

	// Check for skipped tests
	skipCount := countSkippedTests(root)
	checks = append(checks, checkResult{
		Name:   fmt.Sprintf("No skipped tests (%d found)", skipCount),
		Pass:   skipCount == 0,
		Detail: fmt.Sprintf("%d skipped tests in codebase", skipCount),
	})

	return auditResult{Section: "Testing Cascade", Score: scoreChecks(checks), Weight: 10, Checks: checks}
}

// --- Check Helpers ---

func fileExists(root, rel, name string) checkResult {
	_, err := os.Stat(filepath.Join(root, rel))
	return checkResult{Name: name, Pass: err == nil}
}

func dirExists(root, rel, name string) checkResult {
	info, err := os.Stat(filepath.Join(root, rel))
	return checkResult{Name: name, Pass: err == nil && info.IsDir()}
}

func fileContains(root, rel, pattern, name string) checkResult {
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return checkResult{Name: name, Pass: false}
	}
	return checkResult{Name: name, Pass: strings.Contains(strings.ToLower(string(data)), strings.ToLower(pattern))}
}

func fileContainsAny(root, rel string, patterns []string, name string) checkResult {
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return checkResult{Name: name, Pass: false}
	}
	lower := strings.ToLower(string(data))
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return checkResult{Name: name, Pass: true}
		}
	}
	return checkResult{Name: name, Pass: false}
}

func hookInstalled(root string) checkResult {
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	info, err := os.Lstat(hookPath)
	if err != nil {
		return checkResult{Name: "Pre-commit hook installed", Pass: false}
	}
	return checkResult{Name: "Pre-commit hook installed", Pass: info.Mode()&os.ModeSymlink != 0 || info.Mode().IsRegular()}
}

func countGlob(pattern string) int {
	matches, _ := filepath.Glob(pattern)
	return len(matches)
}

func countContractHeaders(root string) int {
	cmd := exec.Command("grep", "-rl", "CONTRACT:", "--include=*.go", "--include=*.ts", "--include=*.tsx", "--include=*.py", "--include=*.rs", "--include=*.js", root)
	out, _ := cmd.Output()
	if len(out) == 0 {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(string(out)), "\n"))
}

func countAgentRoles(root string) int {
	agentsDir := filepath.Join(root, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(agentsDir, e.Name(), "AGENT.md")); err == nil {
				count++
			}
		}
	}
	return count
}

func countFileLines(root, rel string) int {
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return 0
	}
	return len(strings.Split(string(data), "\n"))
}

func countPattern(root, rel, pattern string) int {
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return 0
	}
	return strings.Count(string(data), pattern)
}

func extractDateFromFile(root, rel string) string {
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return ""
	}
	return extractDate(string(data), "last-synced:")
}

func countSkippedTests(root string) int {
	cmd := exec.Command("grep", "-rl", "-E", `\.skip\(|\.skip\b|xit\(|xdescribe\(|xtest\(`, "--include=*.test.*", "--include=*_test.*", "--include=*.spec.*", root)
	out, _ := cmd.Output()
	if len(out) == 0 {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// Filter out node_modules and .claude
	count := 0
	for _, l := range lines {
		if !strings.Contains(l, "node_modules") && !strings.Contains(l, ".claude/worktrees") {
			count++
		}
	}
	return count
}

func scoreChecks(checks []checkResult) int {
	if len(checks) == 0 {
		return 0
	}
	passed := 0
	for _, c := range checks {
		if c.Pass {
			passed++
		}
	}
	return passed * 10 / len(checks)
}

// --- Output ---

func printAuditReport(root string, score float64, results []auditResult) {
	name := filepath.Base(root)

	fmt.Println()
	fmt.Printf("  REBAR Compliance: %s — %.1f/10\n", name, score)
	fmt.Printf("  %s\n", scoreBar(score))
	fmt.Println()

	for _, r := range results {
		status := "PASS"
		if r.Score < 5 {
			status = "FAIL"
		} else if r.Score < 8 {
			status = "PARTIAL"
		}
		fmt.Printf("  %-25s %d/10  %s\n", r.Section, r.Score, status)
		for _, c := range r.Checks {
			mark := "  ✓"
			if !c.Pass {
				mark = "  ✗"
			}
			fmt.Printf("    %s %s\n", mark, c.Name)
		}
		fmt.Println()
	}
}

func scoreBar(score float64) string {
	filled := int(score)
	empty := 10 - filled
	if filled < 0 {
		filled = 0
	}
	if empty < 0 {
		empty = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}

// --- Auto-Fix ---

func applyFixes(root string) int {
	fixed := 0

	// .rebarrc
	rcPath := filepath.Join(root, ".rebarrc")
	if _, err := os.Stat(rcPath); os.IsNotExist(err) {
		os.WriteFile(rcPath, []byte("# REBAR Configuration\ntier = 1\n"), 0644)
		fmt.Println("  ✓ Created .rebarrc (tier 1)")
		fixed++
	}

	// .rebar-version
	versionPath := filepath.Join(root, ".rebar-version")
	if _, err := os.Stat(versionPath); os.IsNotExist(err) {
		os.WriteFile(versionPath, []byte("v2.0.0\n"), 0644)
		fmt.Println("  ✓ Created .rebar-version")
		fixed++
	}

	// Pre-commit hook
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	scriptPath := filepath.Join(root, "scripts", "pre-commit.sh")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		if _, err := os.Stat(scriptPath); err == nil {
			os.Symlink("../../scripts/pre-commit.sh", hookPath)
			fmt.Println("  ✓ Installed pre-commit hook")
			fixed++
		}
	}

	// refresh-context.sh
	refreshPath := filepath.Join(root, "scripts", "refresh-context.sh")
	if _, err := os.Stat(refreshPath); os.IsNotExist(err) {
		rebarRoot := findRebarRoot()
		if rebarRoot != "" {
			src := filepath.Join(rebarRoot, "scripts", "refresh-context.sh")
			if data, err := os.ReadFile(src); err == nil {
				os.MkdirAll(filepath.Join(root, "scripts"), 0755)
				os.WriteFile(refreshPath, data, 0755)
				fmt.Println("  ✓ Copied refresh-context.sh")
				fixed++
			}
		}
	}

	// architecture/ directory
	archDir := filepath.Join(root, "architecture")
	if _, err := os.Stat(archDir); os.IsNotExist(err) {
		os.MkdirAll(archDir, 0755)
		rebarRoot := findRebarRoot()
		if rebarRoot != "" {
			src := filepath.Join(rebarRoot, "architecture", "CONTRACT-TEMPLATE.md")
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(filepath.Join(archDir, "CONTRACT-TEMPLATE.md"), data, 0644)
			}
		}
		fmt.Println("  ✓ Created architecture/ with contract template")
		fixed++
	}

	return fixed
}
