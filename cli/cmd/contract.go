package cmd

import (
	"fmt"
	"os"

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

func init() {
	contractCmd.AddCommand(contractVerifyCmd)
}
