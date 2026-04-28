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

var (
	newDescription string
	newLocal       bool
	newEndpoint    string
	newModel       string
)

var newCmd = &cobra.Command{
	Use:   "new <project-name> [--description \"...\"]",
	Short: "Create a new REBAR project from scratch",
	Long: `Creates a new directory with full REBAR v2.0.0 scaffolding.

  rebar new my-api --description "REST API for document signing with client-side crypto"
  rebar new my-app                # scaffold only, no LLM-generated contracts
  rebar new my-lib --local        # use local LLM for contract generation`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	newCmd.Flags().StringVarP(&newDescription, "description", "d", "", "project description (enables LLM-generated contracts and README)")
	newCmd.Flags().BoolVar(&newLocal, "local", false, "use local LLM instead of Claude API")
	newCmd.Flags().StringVar(&newEndpoint, "endpoint", "", "local LLM endpoint")
	newCmd.Flags().StringVar(&newModel, "model", "", "LLM model override")
	// newCmd is registered in root.go with GroupID
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Create directory
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("directory %q already exists", name)
	}

	fmt.Printf("\n  Creating REBAR project: %s\n\n", name)

	if err := os.MkdirAll(name, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	root, _ := filepath.Abs(name)

	// git init
	fmt.Println("  Phase 1: Initialize")
	gitInit := exec.Command("git", "init", root)
	if out, err := gitInit.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %s", string(out))
	}
	fmt.Println("  ✓ git init")

	// Configure git
	exec.Command("git", "-C", root, "config", "user.email", "dev@localhost").Run()
	exec.Command("git", "-C", root, "config", "user.name", "Developer").Run()

	// Phase 2: REBAR scaffolding
	fmt.Println("\n  Phase 2: REBAR Scaffolding")

	// .rebarrc
	os.WriteFile(filepath.Join(root, ".rebarrc"), []byte("# REBAR Configuration\ntier = 1\n"), 0644)
	fmt.Println("  ✓ .rebarrc")

	// .rebar-version
	os.WriteFile(filepath.Join(root, ".rebar-version"), []byte("v2.0.0\n"), 0644)
	fmt.Println("  ✓ .rebar-version")

	// .gitignore
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte(`# REBAR
.rebar/salt
.rebar/keys/

# Dependencies
node_modules/
vendor/

# Build
dist/
build/

# IDE
.vscode/
.idea/

# OS
.DS_Store
`), 0644)
	fmt.Println("  ✓ .gitignore")

	// Bootstrap v2 files
	bootstrapV2Files(root)

	// README.md
	readme := fmt.Sprintf("# %s\n\n> **rebar v2.0.0** | **Tier 1: PARTIAL**\n\n", name)
	if newDescription != "" {
		readme += newDescription + "\n\n"
	}
	readme += "## Getting Started\n\n```bash\n# TODO: add setup instructions\n```\n"
	os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0644)
	fmt.Println("  ✓ README.md")

	// AGENTS.md
	writeMinimalAgents(root, name)
	fmt.Println("  ✓ AGENTS.md")

	// CLAUDE.md
	writeMinimalClaude(root, name)
	fmt.Println("  ✓ CLAUDE.md")

	// architecture/
	archDir := filepath.Join(root, "architecture")
	os.MkdirAll(archDir, 0755)
	rebarRoot := findRebarRoot()
	if rebarRoot != "" {
		src := filepath.Join(rebarRoot, "architecture", "CONTRACT-TEMPLATE.md")
		if data, err := os.ReadFile(src); err == nil {
			os.WriteFile(filepath.Join(archDir, "CONTRACT-TEMPLATE.md"), data, 0644)
		}
	}
	fmt.Println("  ✓ architecture/")

	// scripts/
	scriptsDir := filepath.Join(root, "scripts")
	os.MkdirAll(scriptsDir, 0755)
	if rebarRoot != "" {
		src := filepath.Join(rebarRoot, "scripts", "refresh-context.sh")
		if data, err := os.ReadFile(src); err == nil {
			os.WriteFile(filepath.Join(scriptsDir, "refresh-context.sh"), data, 0755)
		}
	}
	fmt.Println("  ✓ scripts/")

	// Phase 3: LLM-generated content (if description provided)
	if newDescription != "" {
		fmt.Println("\n  Phase 3: AI-Generated Content (via Claude API)")

		backend := llm.NewBackend(newLocal, newEndpoint, newModel)

		prompt := fmt.Sprintf(`You are bootstrapping a new software project called "%s".
Description: %s

Generate the following as a single markdown document with clear section headers:

## README Content
Write a professional README.md body (after the title and rebar badge) with:
- Project description (2-3 sentences)
- Key features (3-5 bullet points)
- Architecture overview (1 paragraph)
- Getting started section (placeholder commands)

## Contract Proposal
Propose 2-3 architecture contracts based on the project description.
For each, provide:
- Contract ID and name (e.g., CONTRACT-S1-API.1.0)
- Purpose (1 sentence)
- Key interfaces (function signatures or API endpoints)
- Behavioral specifications (3-5 invariants)

## QUICKCONTEXT
Generate a QUICKCONTEXT.md body appropriate for a brand new project at Phase 0.
Include a "What's Next" section with 3-5 initial priorities.

Be practical and specific. Don't over-engineer — this is a fresh project.`, name, newDescription)

		fmt.Println("  Generating project content...")
		response, err := backend.Complete(prompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ LLM call failed: %v\n", err)
			fmt.Println("  Proceeding with scaffolding only.")
		} else {
			// Parse and write the generated content
			sections := parseLLMSections(response)

			if content, ok := sections["README Content"]; ok {
				readme := fmt.Sprintf("# %s\n\n> **rebar v2.0.0** | **Tier 1: PARTIAL**\n\n%s\n", name, content)
				os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0644)
				fmt.Println("  ✓ README.md (AI-generated)")
			}

			if content, ok := sections["QUICKCONTEXT"]; ok {
				qc := fmt.Sprintf("# Quick Context\n\n<!-- freshness: %s -->\n<!-- last-synced: %s -->\n\n%s\n",
					time.Now().Format("2006-01-02"), time.Now().Format("2006-01-02"), content)
				os.WriteFile(filepath.Join(root, "QUICKCONTEXT.md"), []byte(qc), 0644)
				fmt.Println("  ✓ QUICKCONTEXT.md (AI-generated)")
			}

			if content, ok := sections["Contract Proposal"]; ok {
				fmt.Println("\n  Proposed contracts:")
				for _, line := range strings.Split(content, "\n") {
					fmt.Printf("    %s\n", line)
				}
				fmt.Println()
				fmt.Println("  Create these using: architecture/CONTRACT-TEMPLATE.md")
			}
		}
	}

	// Initial commit
	fmt.Println("\n  Phase 4: Initial Commit")
	exec.Command("git", "-C", root, "add", "-A").Run()
	exec.Command("git", "-C", root, "commit", "-m", "feat: initialize project with REBAR v2.0.0").Run()
	fmt.Println("  ✓ Initial commit")

	// Final score
	fmt.Println("\n  Final Assessment")
	score, _ := auditRepo(root)
	fmt.Printf("  Compliance: %.1f/10\n", score)
	fmt.Printf("  %s\n", scoreBar(score))

	fmt.Printf("\n  Project ready at: %s\n", root)
	fmt.Printf("\n  Next steps:\n")
	fmt.Printf("    cd %s\n", name)
	fmt.Printf("    rebar context                    # see your project's current state\n")
	fmt.Printf("    ask architect \"where do I start?\" # talk to the architect agent\n")
	fmt.Printf("    ask who                          # list all available agents\n\n")

	return nil
}

// parseLLMSections splits LLM output by "## Header" sections.
func parseLLMSections(text string) map[string]string {
	sections := map[string]string{}
	lines := strings.Split(text, "\n")
	currentSection := ""
	var currentContent []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(strings.Join(currentContent, "\n"))
			}
			currentSection = strings.TrimPrefix(line, "## ")
			currentContent = nil
		} else if currentSection != "" {
			currentContent = append(currentContent, line)
		}
	}
	if currentSection != "" {
		sections[currentSection] = strings.TrimSpace(strings.Join(currentContent, "\n"))
	}

	return sections
}
