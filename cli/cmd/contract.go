package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/scripts"
)

var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "Contract management",
}

var contractVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify contract references and headers",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Checking contract references...")
		exitCode, err := scripts.RunPassthrough(cfg.ScriptsDir, "check-contract-refs.sh")
		if err != nil {
			return err
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}

		fmt.Println("\nChecking contract headers...")
		exitCode, err = scripts.RunPassthrough(cfg.ScriptsDir, "check-contract-headers.sh")
		if err != nil {
			return err
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}

		return nil
	},
}

// --- federation: drift-check + upstream -------------------------------------

// ConsumesEntry is one parsed `## owner/contract_id.version` section from
// CONSUMES.md. Only the fields drift-check + upstream actually need are
// captured; everything else stays as prose.
type ConsumesEntry struct {
	OwnerRepo      string
	ContractID     string
	VersionPinned  string
	PinDate        string
	NotifyOnChange string // "true" | "false" | ""
	Rationale      string
}

var contractDriftCheckCmd = &cobra.Command{
	Use:   "drift-check",
	Short: "Compare CONSUMES.md pins against upstream current versions",
	Long: `Reads CONSUMES.md in the current repo, finds each consumed
contract's owner repo (via the ASK CLI registry), and compares the
pinned version against the owner's current version (highest semver in
their architecture/CONTRACT-<id>.<ver>.md files).

CI-friendly exit codes (CHARTER §1.6):
  0  all pins match upstream current (or 0 minor versions behind)
  1  any pin is N+ minor versions behind upstream
  2  any pinned contract was removed upstream
  3  no CONSUMES.md (consumer hasn't opted in to federation)
  4  CONSUMES.md parse error

Run from the consumer repo:
  rebar contract drift-check
  rebar contract drift-check --json   # machine-readable output
`,
	RunE: runDriftCheck,
}

var driftCheckJSON bool

var contractUpstreamCmd = &cobra.Command{
	Use:   "upstream <local-extension-contract>",
	Short: "Propose a local extension contract for upstream absorption",
	Long: `Files a feature request in the upstream owner's repo via the
existing ask_<owner>_featurerequest gate, proposing your local extension
be considered for absorption into the next major version of their
contract. The owner triages on their own schedule (CHARTER §1.6).

Argument: path to your local extension contract file (e.g.,
architecture/CONTRACT-C2-AGENTS-MYRBAC.1.0.md).

The CONSUMES.md entry that the extension augments is auto-detected
from the extension's frontmatter or its extension_contracts field in
CONSUMES.md.
`,
	Args: cobra.ExactArgs(1),
	RunE: runUpstream,
}

func init() {
	contractCmd.AddCommand(contractVerifyCmd)

	contractDriftCheckCmd.Flags().BoolVar(&driftCheckJSON, "json", false, "machine-readable JSON output")
	contractCmd.AddCommand(contractDriftCheckCmd)

	contractCmd.AddCommand(contractUpstreamCmd)
}

// parseConsumes reads CONSUMES.md and returns the parsed entries.
// Format: H2 sections "## <owner>/<id>.<version>", followed by
// "- **field:** value" lines. Tolerant of extra prose; ignores unknown
// fields.
func parseConsumes(path string) ([]ConsumesEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	headerRE := regexp.MustCompile(`^##\s+([A-Za-z0-9_-]+)/([A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*)\.(\d+\.\d+(?:\.\d+)?)\s*$`)
	fieldRE := regexp.MustCompile(`^-\s+\*\*([a-z_]+):\*\*\s*(.+?)\s*$`)

	var entries []ConsumesEntry
	var cur *ConsumesEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := headerRE.FindStringSubmatch(line); m != nil {
			if cur != nil {
				entries = append(entries, *cur)
			}
			cur = &ConsumesEntry{
				OwnerRepo:     m[1],
				ContractID:    m[2],
				VersionPinned: m[3],
			}
			continue
		}
		if cur == nil {
			continue
		}
		if m := fieldRE.FindStringSubmatch(line); m != nil {
			val := strings.TrimSpace(m[2])
			// Trim inline-comment portion if present (e.g., "true   # OPTIONAL hint")
			if idx := strings.Index(val, "#"); idx >= 0 {
				val = strings.TrimSpace(val[:idx])
			}
			switch m[1] {
			case "version_pinned":
				cur.VersionPinned = val
			case "pin_date":
				cur.PinDate = val
			case "notify_on_change":
				cur.NotifyOnChange = val
			case "rationale":
				cur.Rationale = val
			}
		}
	}
	if cur != nil {
		entries = append(entries, *cur)
	}
	return entries, scanner.Err()
}

// resolveOwnerRepoPath looks up an owner repo's filesystem path from the
// ASK CLI registry (~/.config/ask/projects).
func resolveOwnerRepoPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	registry := filepath.Join(home, ".config", "ask", "projects")
	f, err := os.Open(registry)
	if err != nil {
		return "", fmt.Errorf("ASK registry not found at %s — run `ask register <name>` in the owner repo (or set REBAR_REPOS): %w", registry, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if strings.EqualFold(k, name) {
			return v, nil
		}
	}
	return "", fmt.Errorf("owner repo %q not registered (run `ask register %s` in that repo, or set REBAR_REPOS)", name, name)
}

// listOwnerVersions scans the owner repo's architecture/ for
// CONTRACT-<id>.<ver>.md files and returns sorted semver strings.
func listOwnerVersions(ownerPath, contractID string) ([]string, error) {
	archDir := filepath.Join(ownerPath, "architecture")
	pattern := fmt.Sprintf("CONTRACT-%s.*.md", contractID)
	matches, err := filepath.Glob(filepath.Join(archDir, pattern))
	if err != nil {
		return nil, err
	}

	verRE := regexp.MustCompile(`^CONTRACT-` + regexp.QuoteMeta(contractID) + `\.(\d+\.\d+(?:\.\d+)?)\.md$`)
	var versions []string
	for _, m := range matches {
		base := filepath.Base(m)
		if mm := verRE.FindStringSubmatch(base); mm != nil {
			versions = append(versions, mm[1])
		}
	}
	sortSemver(versions)
	return versions, nil
}

// sortSemver sorts a slice of semver strings ascending in-place.
// Bash 3.2-equivalent simplicity: lexical compare on zero-padded fields.
func sortSemver(vs []string) {
	sort.Slice(vs, func(i, j int) bool {
		return semverLess(vs[i], vs[j])
	})
}

func semverLess(a, b string) bool {
	pa := semverParts(a)
	pb := semverParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func semverParts(v string) [3]int {
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

type driftResult struct {
	Entry   ConsumesEntry
	Current string
	Status  string // current | minor-behind | major-behind | removed | error
	Detail  string
}

func runDriftCheck(cmd *cobra.Command, args []string) error {
	consumesPath := filepath.Join(cfg.RepoRoot, "CONSUMES.md")
	if _, err := os.Stat(consumesPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "drift-check: no CONSUMES.md in this repo (consumer hasn't opted into federation)")
		os.Exit(3)
	}

	entries, err := parseConsumes(consumesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drift-check: parse error: %v\n", err)
		os.Exit(4)
	}

	if len(entries) == 0 {
		fmt.Println("drift-check: CONSUMES.md exists but declares no consumed contracts")
		return nil
	}

	var results []driftResult
	worstStatus := "current"

	for _, e := range entries {
		ownerPath, err := resolveOwnerRepoPath(e.OwnerRepo)
		if err != nil {
			results = append(results, driftResult{Entry: e, Status: "error", Detail: err.Error()})
			if rankStatus("error") > rankStatus(worstStatus) {
				worstStatus = "error"
			}
			continue
		}
		versions, err := listOwnerVersions(ownerPath, e.ContractID)
		if err != nil || len(versions) == 0 {
			results = append(results, driftResult{Entry: e, Status: "removed", Detail: "no CONTRACT-" + e.ContractID + ".*.md in owner repo"})
			if rankStatus("removed") > rankStatus(worstStatus) {
				worstStatus = "removed"
			}
			continue
		}
		current := versions[len(versions)-1]
		status := compareVersions(e.VersionPinned, current)
		results = append(results, driftResult{Entry: e, Current: current, Status: status})
		if rankStatus(status) > rankStatus(worstStatus) {
			worstStatus = status
		}
	}

	if driftCheckJSON {
		emitDriftJSON(results)
	} else {
		emitDriftText(results)
	}

	// Exit code mapping per CHARTER §1.6:
	//   minor-behind = informational (exit 0 with warning) — current decision
	//   major-behind = failure (exit 1) — owner shipped breaking change
	//   removed      = upstream removed (exit 2) — needs investigation
	//   error        = registry/parse error (exit 4)
	switch worstStatus {
	case "current", "minor-behind":
		return nil
	case "major-behind":
		os.Exit(1)
	case "removed":
		os.Exit(2)
	case "error":
		os.Exit(4)
	}
	return nil
}

func compareVersions(pinned, current string) string {
	pp := semverParts(pinned)
	cc := semverParts(current)
	if pp[0] < cc[0] {
		return "major-behind"
	}
	if pp[0] == cc[0] && pp[1] < cc[1] {
		return "minor-behind"
	}
	return "current"
}

func rankStatus(s string) int {
	switch s {
	case "current":
		return 0
	case "minor-behind":
		return 1
	case "major-behind":
		return 2
	case "removed":
		return 3
	case "error":
		return 4
	}
	return 0
}

func emitDriftText(results []driftResult) {
	for _, r := range results {
		switch r.Status {
		case "current":
			fmt.Printf("  [ok]      %s/%s pinned %s (current)\n", r.Entry.OwnerRepo, r.Entry.ContractID, r.Entry.VersionPinned)
		case "minor-behind":
			fmt.Printf("  [warn]    %s/%s pinned %s, upstream %s (minor behind — additive bump expected)\n",
				r.Entry.OwnerRepo, r.Entry.ContractID, r.Entry.VersionPinned, r.Current)
		case "major-behind":
			fmt.Printf("  [DRIFT]   %s/%s pinned %s, upstream %s (MAJOR behind — likely breaking)\n",
				r.Entry.OwnerRepo, r.Entry.ContractID, r.Entry.VersionPinned, r.Current)
		case "removed":
			fmt.Printf("  [REMOVED] %s/%s pinned %s, no upstream version found (%s)\n",
				r.Entry.OwnerRepo, r.Entry.ContractID, r.Entry.VersionPinned, r.Detail)
		case "error":
			fmt.Printf("  [ERROR]   %s/%s — %s\n", r.Entry.OwnerRepo, r.Entry.ContractID, r.Detail)
		}
	}
	fmt.Println()
	fmt.Printf("Run `rebar contract upstream <ext>` to propose a local extension for upstream absorption.\n")
}

func emitDriftJSON(results []driftResult) {
	fmt.Print("[")
	for i, r := range results {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf(`{"owner_repo":%q,"contract_id":%q,"pinned":%q,"current":%q,"status":%q,"detail":%q}`,
			r.Entry.OwnerRepo, r.Entry.ContractID, r.Entry.VersionPinned, r.Current, r.Status, r.Detail)
	}
	fmt.Println("]")
}

func runUpstream(cmd *cobra.Command, args []string) error {
	extPath := args[0]
	if !filepath.IsAbs(extPath) {
		extPath = filepath.Join(cfg.RepoRoot, extPath)
	}

	extData, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("reading extension contract: %w", err)
	}

	// Look up which upstream this extension augments by scanning CONSUMES.md
	// for an entry whose extension_contracts: list contains this file's
	// basename. Falls back to scanning the file itself for any "augments"
	// or "extends:" prose mention.
	consumesPath := filepath.Join(cfg.RepoRoot, "CONSUMES.md")
	upstream, _ := findUpstreamForExtension(consumesPath, extPath)

	consumerName := filepath.Base(cfg.RepoRoot)
	extBase := filepath.Base(extPath)

	var msg strings.Builder
	msg.WriteString("Upstream extension proposal — auto-filed via `rebar contract upstream`.\n\n")
	if upstream != "" {
		msg.WriteString(fmt.Sprintf("Augments upstream: %s\n", upstream))
	}
	msg.WriteString(fmt.Sprintf("Source repo: %s\n", consumerName))
	msg.WriteString(fmt.Sprintf("Extension contract: %s\n\n", extBase))
	msg.WriteString("Use case: " + extractFirstParagraph(string(extData)) + "\n\n")
	msg.WriteString("Suggested triage: consider absorbing into the next major version of the upstream contract, or rejecting with rationale (CHARTER §3 acceptance gates).\n\n")
	msg.WriteString("Full extension contract:\n```markdown\n")
	if len(extData) > 4000 {
		msg.Write(extData[:4000])
		msg.WriteString(fmt.Sprintf("\n... (%d bytes truncated; see %s in source repo)\n", len(extData)-4000, extBase))
	} else {
		msg.Write(extData)
	}
	msg.WriteString("\n```\n")

	// Determine owner repo to dispatch to. If we resolved an upstream from
	// CONSUMES.md, that's the owner. Otherwise prompt the caller.
	if upstream == "" {
		return fmt.Errorf("could not resolve upstream owner — add this extension to the relevant CONSUMES.md entry's extension_contracts: list, or specify owner via prose in the extension file")
	}
	owner := strings.SplitN(upstream, "/", 2)[0]

	askBin := findAskBinForUpstream()
	if askBin == "" {
		return fmt.Errorf("ask CLI not found; run bin/install or set ASK_BIN")
	}

	target := owner + ":featurerequest"
	fmt.Printf("Dispatching upstream proposal to %s ...\n", target)
	c := exec.Command(askBin, target, msg.String())
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("ask invocation failed: %w", err)
	}
	fmt.Println("\nDone. Owner will triage on their schedule (CHARTER §1.6 — owner-pulled reconciliation).")
	return nil
}

// findUpstreamForExtension scans CONSUMES.md for any section whose
// extension_contracts: list mentions extPath's basename. Returns the
// "owner/contract.version" string (e.g., "rebar/C1-AGENTS.2.0") or "".
func findUpstreamForExtension(consumesPath, extPath string) (string, error) {
	f, err := os.Open(consumesPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	extName := filepath.Base(extPath)
	// Strip the .md and the version suffix to make matching forgiving:
	// CONTRACT-C2-AGENTS-MYRBAC.1.0.md → C2-AGENTS-MYRBAC
	idMatch := regexp.MustCompile(`^CONTRACT-(.+?)\.\d+\.\d+(?:\.\d+)?\.md$`).FindStringSubmatch(extName)
	idStripped := extName
	if idMatch != nil {
		idStripped = idMatch[1]
	}

	headerRE := regexp.MustCompile(`^##\s+([A-Za-z0-9_-]+)/([A-Za-z0-9_.-]+)\s*$`)
	currentHeader := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := headerRE.FindStringSubmatch(line); m != nil {
			currentHeader = m[1] + "/" + m[2]
			continue
		}
		if currentHeader != "" && strings.Contains(line, "extension_contracts") {
			// Read subsequent lines until we hit a non-list-item or new
			// section. Look for our id as a prefix.
			for scanner.Scan() {
				sub := scanner.Text()
				if !strings.HasPrefix(strings.TrimSpace(sub), "-") {
					break
				}
				if strings.Contains(sub, idStripped) || strings.Contains(sub, extName) {
					return currentHeader, nil
				}
			}
		}
	}
	return "", nil
}

func extractFirstParagraph(s string) string {
	lines := strings.Split(s, "\n")
	var collected []string
	started := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if !started {
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			started = true
			collected = append(collected, t)
			continue
		}
		if t == "" || strings.HasPrefix(t, "#") {
			break
		}
		collected = append(collected, t)
	}
	out := strings.Join(collected, " ")
	if len(out) > 300 {
		out = out[:300] + "…"
	}
	return out
}

// findAskBinForUpstream — same lookup chain used by ensureAgentsScaffolding
// in init.go, copied here to avoid coupling. (If a refactor shared a single
// helper across cmd/, this would deduplicate.)
func findAskBinForUpstream() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "ask")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if rebarRoot := findRebarRoot(); rebarRoot != "" {
		candidate := filepath.Join(rebarRoot, "bin", "ask")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath("ask"); err == nil {
		return p
	}
	return ""
}

