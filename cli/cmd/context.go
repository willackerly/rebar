package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/willackerly/rebar/cli/internal/config"
)

var contextCmd = &cobra.Command{
	Use:   "context [role]",
	Short: "Print role-relevant context files in reading order",
	Long: `Cats the files relevant to a given role, in the order they should be read.
Without a role argument, prints the Cold Start Quad (README, QUICKCONTEXT, TODO, AGENTS).

Available roles:
  (none)       Cold Start Quad — README → QUICKCONTEXT → TODO → AGENTS
  architect    + DESIGN.md, architecture contracts
  product      + product/ personas, features, user stories
  security     + security contracts, CLAUDE.md
  developer    + contracts for current branch changes
  session-start  Cold Start Quad + staleness verification output`,
	Args: cobra.MaximumNArgs(1),
	RunE: runContext,
}

// roleFiles defines which files to cat for each role, in order.
// Glob patterns are expanded at runtime.
var roleFiles = map[string][]string{
	"": {
		"README.md",
		"QUICKCONTEXT.md",
		"TODO.md",
		"AGENTS.md",
	},
	"architect": {
		"README.md",
		"QUICKCONTEXT.md",
		"TODO.md",
		"AGENTS.md",
		"DESIGN.md",
		"architecture/CONTRACT-*.md",
	},
	"product": {
		"README.md",
		"QUICKCONTEXT.md",
		"TODO.md",
		"AGENTS.md",
		"product/personas/*.md",
		"product/features/*.feature",
		"product/features/*.md",
		"product/user-stories/*.md",
	},
	"security": {
		"README.md",
		"QUICKCONTEXT.md",
		"TODO.md",
		"AGENTS.md",
		"CLAUDE.md",
		"architecture/CONTRACT-*SECURITY*.md",
		"architecture/CONTRACT-*AUTH*.md",
		"architecture/CONTRACT-*CRYPTO*.md",
		"docs/THREAT_MODEL.md",
	},
	"developer": {
		"README.md",
		"QUICKCONTEXT.md",
		"TODO.md",
		"AGENTS.md",
		"CLAUDE.md",
	},
}

func runContext(cmd *cobra.Command, args []string) error {
	role := ""
	if len(args) > 0 {
		role = strings.ToLower(args[0])
	}

	// Special case: session-start runs the refresh script then the Cold Start Quad
	if role == "session-start" {
		return runSessionStart()
	}

	rolePatterns, ok := roleFiles[role]
	if !ok {
		available := make([]string, 0, len(roleFiles))
		for r := range roleFiles {
			if r != "" {
				available = append(available, r)
			}
		}
		sort.Strings(available)
		return fmt.Errorf("unknown role %q — available: %s, session-start", role, strings.Join(available, ", "))
	}

	// Role patterns name the default contracts dir; branch contract patterns
	// already name the configured one, so only the former are rewritten.
	patterns := withContractsDirAll(rolePatterns)

	// For developer role, also include contracts for files changed on current branch
	if role == "developer" {
		branchContracts := findBranchContracts()
		if len(branchContracts) > 0 {
			patterns = append(patterns, branchContracts...)
		}
	}

	printed := 0
	for _, pattern := range patterns {
		fullPattern := filepath.Join(cfg.RepoRoot, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil || len(matches) == 0 {
			// Try as a literal file
			path := filepath.Join(cfg.RepoRoot, pattern)
			if _, err := os.Stat(path); err == nil {
				matches = []string{path}
			} else {
				continue
			}
		}

		sort.Strings(matches)
		for _, path := range matches {
			relPath, _ := filepath.Rel(cfg.RepoRoot, path)

			// Skip template files, state files, and non-text
			if strings.Contains(relPath, ".state/") ||
				strings.HasSuffix(relPath, ".template.md") ||
				strings.HasPrefix(filepath.Base(relPath), ".") {
				continue
			}

			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			if printed > 0 {
				fmt.Println()
			}
			fmt.Printf("═══ %s ═══\n\n", relPath)
			fmt.Print(string(content))
			if !strings.HasSuffix(string(content), "\n") {
				fmt.Println()
			}
			printed++
		}
	}

	if printed == 0 {
		fmt.Println("No context files found for this role.")
		fmt.Println("Ensure the Cold Start Quad exists: README.md, QUICKCONTEXT.md, TODO.md, AGENTS.md")
	}

	// Nudge: if agents/ exists but the architect has never been queried,
	// surface the next-most-useful command. Caller is likely a fresh
	// adopter who just ran `rebar new` or `rebar context` for the first
	// time and hasn't discovered ASK yet.
	logPath := filepath.Join(cfg.RepoRoot, "agents", "architect", "memory.log.md")
	if info, err := os.Stat(logPath); err == nil && info.Size() == 0 {
		fmt.Println()
		fmt.Println("Try: ask architect \"what should I implement first?\"")
	}

	return nil
}

// findBranchContracts finds CONTRACT: references in files changed on the current branch.
func findBranchContracts() []string {
	// Get files changed on current branch vs main
	diff := gitOutput("diff", "--name-only", "main...HEAD")
	if diff == "" {
		// Maybe we're on main — check uncommitted changes
		diff = gitOutput("diff", "--name-only", "HEAD")
	}
	if diff == "" {
		return nil
	}

	namespace := config.ContractNamespace(cfg.RepoRoot)
	contractsDir := config.ContractsDirFor(cfg.RepoRoot)
	contracts := map[string]bool{}
	for _, file := range strings.Split(diff, "\n") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		path := filepath.Join(cfg.RepoRoot, file)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Look for CONTRACT: references in file headers (first 10 lines)
		lines := strings.Split(string(content), "\n")
		limit := 10
		if len(lines) < limit {
			limit = len(lines)
		}
		for _, line := range lines[:limit] {
			// Every reference on the line counts: a foreign reference may
			// precede a local one. The literal CONTRACT prefix is elided in
			// these comments so check-contract-refs.sh doesn't read them.
			const marker = "CONTRACT:"
			for rest := line; ; {
				idx := strings.Index(rest, marker)
				if idx < 0 {
					break
				}
				rest = rest[idx+len(marker):]
				fields := strings.Fields(rest)
				if len(fields) == 0 {
					break
				}
				// A reference qualified with another repo's contract
				// namespace has no contract file here.
				ref, ok := config.LocalContractRef(fields[0], namespace)
				if !ok {
					continue
				}
				// The contract ID (the C1-BLOBSTORE portion of C1-BLOBSTORE.2.1)
				// as a glob under the contracts dir.
				id := strings.Split(ref, ".")[0]
				pattern := filepath.ToSlash(filepath.Join(contractsDir, "CONTRACT-"+id+"*.md"))
				contracts[pattern] = true
			}
		}
	}

	result := make([]string, 0, len(contracts))
	for pattern := range contracts {
		result = append(result, pattern)
	}
	sort.Strings(result)
	return result
}

func runSessionStart() error {
	// First, the freshness summary (refresh-context is built in).
	fmt.Println("═══ Freshness Check ═══")
	fmt.Println()
	out := gitOutput("log", "--since=7 days", "--oneline")
	if out != "" {
		fmt.Println("Recent commits (last 7 days):")
		fmt.Println(out)
	} else {
		fmt.Println("No commits in the last 7 days.")
	}
	fmt.Println()

	// Check for worktrees
	worktrees := gitOutput("worktree", "list")
	wlines := strings.Split(worktrees, "\n")
	if len(wlines) > 1 {
		fmt.Printf("WARNING: %d worktree(s) exist — verify they're active:\n", len(wlines)-1)
		fmt.Println(worktrees)
	} else {
		fmt.Println("Worktrees: clean (main only)")
	}
	fmt.Println()

	// Then print the Cold Start Quad
	return runContext(&cobra.Command{}, []string{})
}

// contextCmd is registered in root.go with GroupID
