package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/config"
)

var (
	verbose  bool
	jsonOut  bool
	repoRoot string
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:     "rebar",
	Version: "v2.0.0",
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
    rebar check                              Run all enforcement checks`,
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

func Execute() error {
	return rootCmd.Execute()
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
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(pushCmd)

	// Agent Coordination
	askCmd.GroupID = "agents"
	agentCmd.GroupID = "agents"
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(agentCmd)

	// Quality & Integrity
	statusCmd.GroupID = "quality"
	verifyCmd.GroupID = "quality"
	checkCmd.GroupID = "quality"
	diffCmd.GroupID = "quality"
	contractCmd.GroupID = "quality"
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(checkCmd)
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
			fmt.Println("rebar v2.0.0")
		},
	})
}
