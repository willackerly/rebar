package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/config"
)

// releaseVersion is the release string of this source tree. Bump it with
// each release; the release workflow refuses a tag that disagrees with it.
const releaseVersion = "v3.1.0-rc.2"

// Version is what `rebar version` reports. Release builds and setup-rebar.sh
// source builds stamp it with
//
//	go build -ldflags "-X github.com/willackerly/rebar/cli/cmd.Version=<ver>"
//
// (a tag, or <branch>@<sha7> for a branch build). Unstamped, it is
// releaseVersion.
var Version = releaseVersion

// semverTag matches the release tags check-compliance.sh accepts in a badge.
var semverTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

// badgeVersion is the version new/adopt write into a project's README badge,
// which check-compliance.sh validates: the stamped Version when it is a
// release tag, else this tree's releaseVersion, so a branch build does not
// write "feat/x@abc1234" into an adopter's badge.
func badgeVersion() string {
	if semverTag.MatchString(Version) {
		return Version
	}
	return releaseVersion
}

var (
	verbose  bool
	jsonOut  bool
	repoRoot string
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:     "rebar",
	Version: Version,
	Short:   "REBAR — contract-driven development framework for AI-powered teams",
	Long: `REBAR — contract-driven development framework for AI-powered teams.

  Get started:
    rebar new my-project -d "description"   Create a new REBAR project
    rebar adopt                              Add REBAR to an existing project
    rebar init                               Initialize integrity tracking

  Daily workflow:
    rebar context [role]                     View context files for a role
    rebar commit -m "message"                Enforced commit with integrity
    rebar audit                              Check compliance score
    rebar audit --all ~/dev                  Fleet-wide scorecard

  Agent coordination:
    rebar ask <agent> "question"             Query a role-based agent
    rebar agent start --role dev "task"       Run agent in sealed envelope

  Quality & integrity:
    rebar status                             Health dashboard
    rebar verify                             Check file integrity
    rebar check                              Run all enforcement checks
    rebar run <script>                       Run one of rebar's scripts
    rebar migrate [--apply]                  Move legacy script copies into .rebarrc`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip repo detection for commands that handle their own root
		name := cmd.Name()
		if name == "help" || name == "version" {
			return nil
		}
		// These commands handle their own repo root
		if name == "init" || name == "new" || name == "audit" || name == "adopt" {
			if cmd.Parent() != nil && cmd.Parent().Name() == "rebar" {
				return nil
			}
		}

		var err error
		if repoRoot == "" {
			// Like the scripts: REBAR_PROJECT_ROOT names the project (the
			// thin wrappers `rebar migrate` leaves set it, so a copied
			// wrapper still checks the directory it sits in).
			repoRoot = os.Getenv("REBAR_PROJECT_ROOT")
		}
		if repoRoot == "" {
			cwd, _ := os.Getwd()
			repoRoot, err = config.FindRepoRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a rebar repository: %w\nRun 'rebar init' to initialize or 'rebar new <name>' to create a project", err)
			}
		}

		cfg, err = config.Load(repoRoot)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI. Cobra's own error printing is off (SilenceErrors),
// so the error is printed here, once, on stderr; without this a failing
// command (e.g. "rebar install not found (set REBAR_ROOT …)") exited 1 silently.
func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(rootCmd.ErrOrStderr(), "rebar:", err)
	}
	return err
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "machine-readable JSON output")
	rootCmd.PersistentFlags().StringVar(&repoRoot, "repo-root", "", "override repo root detection")

	// Group commands by purpose using cobra groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "start", Title: "Getting Started:"},
		&cobra.Group{ID: "daily", Title: "Daily Workflow:"},
		&cobra.Group{ID: "agents", Title: "Agent Coordination:"},
		&cobra.Group{ID: "quality", Title: "Quality & Integrity:"},
		&cobra.Group{ID: "keys", Title: "Signing & Keys:"},
	)

	// Getting Started
	newCmd.GroupID = "start"
	adoptCmd.GroupID = "start"
	initCmd.GroupID = "start"
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(initCmd)

	// Daily Workflow
	contextCmd.GroupID = "daily"
	commitCmd.GroupID = "daily"
	auditCmd.GroupID = "daily"
	pushCmd.GroupID = "daily"
	docCmd.GroupID = "daily"
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(docCmd)

	// Agent Coordination
	askCmd.GroupID = "agents"
	agentCmd.GroupID = "agents"
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(agentCmd)

	// Quality & Integrity
	statusCmd.GroupID = "quality"
	verifyCmd.GroupID = "quality"
	checkCmd.GroupID = "quality"
	runCmd.GroupID = "quality"
	diffCmd.GroupID = "quality"
	contractCmd.GroupID = "quality"
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(contractCmd)

	// Signing & Keys
	signCmd.GroupID = "keys"
	keyCmd.GroupID = "keys"
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(keyCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print rebar version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("rebar " + rootCmd.Version)
		},
	})
}
