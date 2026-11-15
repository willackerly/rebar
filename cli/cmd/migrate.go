package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/migrate"
)

var (
	migrateAllowDirty bool
	migrateApply      bool
	migrateRegenerate bool
	migrateTimeout    time.Duration
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Move legacy copies of rebar scripts into .rebarrc settings",
	Long: `Find the project's own copies of rebar scripts (the files the per-script
override runs instead of the install's), and decide for each one by running
it: the legacy copy and the install's script run against the project, and a
copy whose behavior the install reproduces, with settings derived from the
copy, can go.

  check-ground-truth.sh  each metric compute_metrics() emits is translated
                         to metric.<key>.* lines and kept only if the
                         install computes the same value
  compute-registry.sh    settings are kept only if the install's --stdout is
                         byte-identical to the copy's
  other checks           same exit status and findings, on the project and
                         (for source scanners) on a probe tree
  everything else        deleted only if byte-identical to a released rebar
                         version (ci-check.sh / pre-commit.sh become the thin
                         wrappers)

Without --apply nothing changes: the proposed .rebarrc lines, the files to
remove and the files kept (with reasons) are printed. --apply writes them,
reruns the checks, and exits 1 if one that passed before now fails.`,
	Args: cobra.NoArgs,
	RunE: runMigrate,
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateApply, "apply", false, "write .rebarrc, delete the verified copies and rerun the checks")
	migrateCmd.Flags().BoolVar(&migrateRegenerate, "regenerate-registry", false, "use the ellipsis purpose style even if the registry changes (default: keep its bytes)")
	migrateCmd.Flags().BoolVar(&migrateAllowDirty, "allow-dirty", false, "let --apply run with uncommitted or untracked files (their changes can't be undone with git)")
	migrateCmd.Flags().DurationVar(&migrateTimeout, "timeout", 3*time.Minute, "time limit for each script run")
	rootCmd.AddCommand(migrateCmd)
}

// runMigrate prints its own errors: main exits 1 without printing.
func runMigrate(cmd *cobra.Command, args []string) error {
	err := migrateMain(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rebar migrate:", err)
	}
	return err
}

func migrateMain(cmd *cobra.Command) error {
	opts := migrate.Options{
		ProjectRoot:        cfg.RepoRoot,
		RebarRoot:          findRebarRoot(),
		RegenerateRegistry: migrateRegenerate,
		Timeout:            migrateTimeout,
		Progress:           os.Stderr,
	}
	fmt.Fprintln(os.Stderr, "rebar migrate: probes run in a scratch copy of the project; nothing here changes until --apply")
	plan, err := migrate.BuildPlan(opts)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	migrate.Render(out, plan, migrateApply)
	if !migrateApply {
		if plan.Empty() {
			fmt.Fprintln(out, "Nothing to migrate.")
		} else {
			fmt.Fprintln(out, "Dry run: nothing changed. Run `rebar migrate --apply` to apply.")
		}
		return nil
	}
	if plan.Empty() {
		fmt.Fprintln(out, "Nothing to migrate.")
		return nil
	}
	ao := migrate.ApplyOptions{AllowDirty: migrateAllowDirty}
	if exe, err := os.Executable(); err == nil && filepath.Base(exe) == "rebar" {
		ao.RebarBin = exe
	}
	res, err := migrate.Apply(plan, opts, ao)
	if err != nil {
		return err
	}
	migrate.RenderChecks(out, res.Checks, plan.Tier)
	if res.RolledBack {
		fmt.Fprintln(out, "\nA check that passed before migration fails now (see above). Every change was rolled back; the project is as it was.")
		os.Exit(1)
	}
	fmt.Fprint(out, "\nApplied. Review with `git status` / `git diff`")
	if !migrateAllowDirty {
		fmt.Fprint(out, " (the tree was clean, so `git checkout -- .` undoes it)")
	}
	fmt.Fprintln(out, ", then commit (`rebar commit` if the project tracks .rebar/integrity.json).")
	return nil
}
