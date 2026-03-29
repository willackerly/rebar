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
	Use:   "rebar",
	Short: "REBAR — integrity-enforced development framework",
	Long:  `Unified CLI for the REBAR framework: integrity verification, enforced commits, agent execution, and digital signatures.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip repo detection for commands that handle their own root
		name := cmd.Name()
		if name == "help" || name == "version" {
			return nil
		}
		// rebar init handles its own repo root
		if name == "init" && cmd.Parent() != nil && cmd.Parent().Name() == "rebar" {
			return nil
		}

		var err error
		if repoRoot == "" {
			cwd, _ := os.Getwd()
			repoRoot, err = config.FindRepoRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a rebar repository: %w\nRun 'rebar init' to initialize", err)
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

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(keyCmd)
	rootCmd.AddCommand(contractCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print rebar version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("rebar v0.1.0")
		},
	})
}
