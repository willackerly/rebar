package suite

import (
	"github.com/willackerly/rebar/cli/internal/checks"
)

func init() {
	checks.Register(&checks.Check{Name: "pre-commit", Run: runPreCommit})
}

// runPreCommit is scripts/pre-commit.sh: the fast checks (TODO tracking,
// contract references, ground truth), then the project's
// checks_extra_pre_commit; exit 1 when any failed. Arguments are ignored.
func runPreCommit(ctx *checks.Context, _ []string) int {
	warnOnce(ctx)
	out := ctx.Stdout
	failed := 0

	echo(out, "Pre-commit: checking TODO tracking...")
	if s := resolve(ctx, "check-todos.sh", false); s.found() && s.run(ctx, nil, false) != 0 {
		failed++
	}

	echo(out, "")
	echo(out, "Pre-commit: checking contract references...")
	if s := resolve(ctx, "check-contract-refs.sh", false); s.found() && s.run(ctx, nil, false) != 0 {
		failed++
	}

	// Optional: verify numeric claims haven't drifted.
	if s := resolve(ctx, "check-ground-truth.sh", false); s.found() {
		echo(out, "")
		echo(out, "Pre-commit: checking ground truth metrics...")
		if s.run(ctx, nil, false) != 0 {
			failed++
		}
	}

	if !skipExtras() {
		for _, e := range Extras(ctx.Root, ExtraPreCommitKey) {
			echo(out, "")
			echo(out, "Pre-commit: running extra check "+e.Name+"...")
			if !e.Runnable(ctx.Root) {
				echo(out, "FAIL: "+e.Path+" not found or not executable (.rebarrc "+ExtraPreCommitKey+")")
				failed++
				continue
			}
			if e.Run(ctx, nil) != 0 {
				failed++
			}
		}
	}

	if failed > 0 {
		echo(out, "")
		echo(out, "Pre-commit checks failed. Fix the issues above before committing.")
		echo(out, "To skip (not recommended): git commit --no-verify")
		return 1
	}
	echo(out, "")
	echo(out, "Pre-commit checks passed.")
	return 0
}
