package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/llm"
)

var _ = time.Now // ensure time is used

var (
	adoptTier     int
	adoptLocal    bool
	adoptEndpoint string
	adoptModel    string
)

var adoptCmd = &cobra.Command{
	Use:   "adopt [path]",
	Short: "Adopt REBAR in an existing project",
	Long: `Assess, scaffold, and optionally propose contracts for an existing project.

  rebar adopt              # adopt in current directory
  rebar adopt --tier 1     # minimal: scaffold only, no contract proposals
  rebar adopt --tier 2     # full: scaffold + contracts via Claude API
  rebar adopt --local      # use local LLM instead of Claude API`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdopt,
}

func init() {
	adoptCmd.Flags().IntVar(&adoptTier, "tier", 2, "target tier (1=scaffold only, 2=scaffold+contracts)")
	adoptCmd.Flags().BoolVar(&adoptLocal, "local", false, "use local LLM (LM Studio/ollama) instead of Claude API")
	adoptCmd.Flags().StringVar(&adoptEndpoint, "endpoint", "", "local LLM endpoint (default: http://localhost:1234/v1)")
	adoptCmd.Flags().StringVar(&adoptModel, "model", "", "LLM model override")
	// adoptCmd is registered in root.go with GroupID
}

func runAdopt(cmd *cobra.Command, args []string) error {
	root := ""
	if len(args) > 0 {
		root = args[0]
	} else {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	name := filepath.Base(root)
	fmt.Printf("\n  Adopting REBAR v2.0.0 for: %s\n\n", name)

	// Phase 1: Assess current state
	fmt.Println("  Phase 1: Assessment")
	score, results := auditRepo(root)
	fmt.Printf("  Current compliance: %.1f/10\n\n", score)

	// Phase 2: Scaffold (always — equivalent to rebar init + fixes)
	fmt.Println("  Phase 2: Scaffolding")
	fixed := applyFixes(root)

	// Bootstrap v2 files via init logic
	bootstrapped := bootstrapV2Files(root)
	fixed += bootstrapped

	// Ensure AGENTS.md exists with session lifecycle
	agentsPath := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		writeMinimalAgents(root, name)
		fmt.Println("  ✓ Created AGENTS.md")
		fixed++
	}

	// Ensure CLAUDE.md exists
	claudePath := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		writeMinimalClaude(root, name)
		fmt.Println("  ✓ Created CLAUDE.md")
		fixed++
	}

	// Ensure README has badge
	readmePath := filepath.Join(root, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		if !strings.Contains(string(data), "rebar v") {
			// Insert badge after first line
			lines := strings.SplitN(string(data), "\n", 2)
			if len(lines) == 2 {
				badged := lines[0] + "\n\n> **rebar v2.0.0** | **Tier " + fmt.Sprintf("%d", adoptTier) + "**\n" + lines[1]
				os.WriteFile(readmePath, []byte(badged), 0644)
				fmt.Println("  ✓ Added rebar badge to README.md")
				fixed++
			}
		}
	}

	if fixed == 0 {
		fmt.Println("  (all scaffolding already in place)")
	}

	// Phase 3: Contract proposals (tier 2 only, requires LLM)
	if adoptTier >= 2 {
		fmt.Println("\n  Phase 3: Contract Proposals (via Claude API)")

		backend := llm.NewBackend(adoptLocal, adoptEndpoint, adoptModel)

		// Gather codebase summary for the LLM
		summary := gatherCodebaseSummary(root)
		if summary == "" {
			fmt.Println("  (no source files found — skipping contract proposals)")
		} else {
			prompt := fmt.Sprintf(`You are analyzing a codebase to propose REBAR architecture contracts.

Project: %s
Codebase summary:
%s

Based on this codebase, propose 2-4 architecture contracts that would be most valuable.
For each contract, provide:
1. Contract ID and name (e.g., CONTRACT-C1-AUTH.1.0)
2. One-line purpose
3. Key behavioral specifications (3-5 items)
4. Which source files implement it

Format as a numbered list. Be specific about file paths and behaviors.
Only propose contracts for code that actually exists — don't invent components.`, name, summary)

			fmt.Println("  Analyzing codebase...")
			response, err := backend.Complete(prompt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ LLM call failed: %v\n", err)
				fmt.Println("  Skipping contract proposals. Run 'rebar adopt --tier 2' again to retry.")
			} else {
				fmt.Println("\n  Proposed contracts:")
				fmt.Println()
				// Indent the response
				for _, line := range strings.Split(response, "\n") {
					fmt.Printf("    %s\n", line)
				}
				fmt.Println()
				fmt.Println("  To create these contracts, use the template at architecture/CONTRACT-TEMPLATE.md")
				fmt.Println("  Then add CONTRACT: headers to the implementing source files.")
			}
		}
	}

	// Re-assess
	fmt.Println("\n  Final Assessment")
	newScore, _ := auditRepo(root)
	fmt.Printf("  Compliance: %.1f/10 (was %.1f/10)\n", newScore, score)
	fmt.Printf("  %s\n\n", scoreBar(newScore))

	// Print unused results to satisfy compiler
	_ = results

	return nil
}

func gatherCodebaseSummary(root string) string {
	var sb strings.Builder

	// Find source files
	exts := []string{"*.go", "*.ts", "*.tsx", "*.py", "*.rs", "*.js", "*.jsx"}
	var sourceFiles []string
	for _, ext := range exts {
		cmd := exec.Command("find", root, "-name", ext, "-not", "-path", "*/node_modules/*", "-not", "-path", "*/.claude/*", "-not", "-path", "*/.git/*", "-not", "-path", "*/vendor/*")
		out, _ := cmd.Output()
		for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if f != "" {
				rel, _ := filepath.Rel(root, f)
				sourceFiles = append(sourceFiles, rel)
			}
		}
	}

	if len(sourceFiles) == 0 {
		return ""
	}

	sb.WriteString(fmt.Sprintf("Source files (%d total):\n", len(sourceFiles)))
	// Show first 30 files
	limit := 30
	if len(sourceFiles) < limit {
		limit = len(sourceFiles)
	}
	for _, f := range sourceFiles[:limit] {
		sb.WriteString(fmt.Sprintf("  %s\n", f))
	}
	if len(sourceFiles) > 30 {
		sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(sourceFiles)-30))
	}

	// Show directory structure
	sb.WriteString("\nDirectory structure:\n")
	cmd := exec.Command("find", root, "-maxdepth", "2", "-type", "d",
		"-not", "-path", "*/node_modules/*", "-not", "-path", "*/.git/*",
		"-not", "-path", "*/.claude/*", "-not", "-path", "*/vendor/*")
	out, _ := cmd.Output()
	for _, d := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if d != "" {
			rel, _ := filepath.Rel(root, d)
			if rel != "" && rel != "." {
				sb.WriteString(fmt.Sprintf("  %s/\n", rel))
			}
		}
	}

	// Show README if it exists (first 20 lines)
	readmePath := filepath.Join(root, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		lines := strings.Split(string(data), "\n")
		limit := 20
		if len(lines) < limit {
			limit = len(lines)
		}
		sb.WriteString("\nREADME.md (first 20 lines):\n")
		for _, l := range lines[:limit] {
			sb.WriteString(fmt.Sprintf("  %s\n", l))
		}
	}

	return sb.String()
}

func writeMinimalAgents(root, name string) {
	content := fmt.Sprintf(`# Agent Guidelines

<!-- freshness: %s -->

**How AI agents work effectively in %s.**

## Quick Start for New Agents

1. **README.md** — what is this project?
2. **QUICKCONTEXT.md** — what's true right now?
3. **VERIFY:** `+"`"+`git log --since='7 days' --oneline | head -20`+"`"+`
4. **TODO.md** — what needs doing?
5. **This file** — how do we work together?

## Session Lifecycle

See `+"`"+`practices/session-lifecycle.md`+"`"+` for the full protocol.

| Stage | Trigger | Key Actions |
|-------|---------|-------------|
| **Start** | New session | Cold Start Quad + staleness verification |
| **Checkpoint** | Every 10 commits or 2 hours | Update QUICKCONTEXT, commit WIP |
| **End** | Session closing | Update QUICKCONTEXT, update TODO, write wrapup |

## Contract-Driven Development

**Don't implement without a contract. Don't modify code without checking its contract.**

## Testing Cascade

| Tier | Name | Speed | When |
|------|------|-------|------|
| T0 | Typecheck | <5s | Every edit |
| T1 | Targeted | <10s | Every change |
| T2 | Package | <30s | Before commit |
| T3 | Cross-package | <60s | Before push |
| T4 | Visual/E2E | <2min | UI changes |
| T5 | Full suite | <10min | Release prep |
`, time.Now().Format("2006-01-02"), name)

	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(content), 0644)
}

func writeMinimalClaude(root, name string) {
	content := fmt.Sprintf(`# Claude Code Configuration

## Project: %s

## Cold Start (every session)

1. README.md — project overview
2. QUICKCONTEXT.md — current state
3. **VERIFY:** `+"`"+`git log --since='7 days' --oneline | head -20`+"`"+`
4. TODO.md — active work
5. AGENTS.md — coordination guidelines

## Session End

1. Update QUICKCONTEXT.md with current state
2. Update TODO.md (mark completed, add discovered)
3. Clean up: `+"`"+`git worktree prune`+"`"+`
`, name)

	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(content), 0644)
}
