package suite

import (
	"fmt"
	"github.com/willackerly/rebar/cli/internal/config"

	"github.com/willackerly/rebar/cli/internal/checks"
)

func init() {
	checks.Register(&checks.Check{Name: "ci-check", Run: runCICheck})
}

// ciStep is one check of the full suite: its summary label, the variable
// that skips it, and the script (relative to scripts/) with its arguments.
type ciStep struct {
	label, skip, script string
	args                []string
}

// ciSteps are scripts/ci-check.sh's checks, in its order.
var ciSteps = []ciStep{
	{"Contract Headers", "SKIP_CONTRACT_HEADERS", "check-contract-headers.sh", nil},
	{"Contract References", "SKIP_CONTRACT_REFS", "check-contract-refs.sh", nil},
	{"Doc References", "SKIP_DOC_REFS", "check-doc-refs.sh", nil},
	{"TODO Tracking", "SKIP_TODOS", "check-todos.sh", nil},
	{"Doc Freshness", "SKIP_FRESHNESS", "check-freshness.sh", nil},
	{"Registry Consistency", "SKIP_REGISTRY", "compute-registry.sh", []string{"--check"}},
	{"JTBD Presence", "SKIP_JTBD", "check-jtbd-presence.sh", nil},
	{"Prefix Uniqueness", "SKIP_PREFIX_UNIQUENESS", "check-prefix-uniqueness.sh", nil},
	{"Ground Truth", "SKIP_GROUND_TRUTH", "check-ground-truth.sh", nil},
	{"Rebar Compliance", "SKIP_COMPLIANCE", "check-compliance.sh", nil},
	{"Decay Patterns", "SKIP_DECAY_PATTERNS", "check-decay-patterns.sh", nil},
	{"Bootstrap Sync", "SKIP_BOOTSTRAP_SYNC", "sync-bootstrap.sh", []string{"--check"}},
	{"Script Tests", "SKIP_SCRIPT_TESTS", "tests/run.sh", nil},
	{"Fix Commit Gate", "SKIP_FIX_COMMIT", "check-fix-commit.sh", nil},
	{"Bypass Flags Gate", "SKIP_BYPASS_FLAGS", "check-bypass-flags.sh", nil},
	{"Steward", "SKIP_STEWARD", "steward.sh", nil},
}

// maintainerSteps check the rebar source repo itself: they run its own
// scripts there and skip elsewhere.
var maintainerSteps = map[string]bool{"sync-bootstrap.sh": true, "tests/run.sh": true}

// SkipVars are the variables that skip a check of the full suite, in
// suite order, then SKIP_CHECKS_EXTRA (every checks_extra entry).
func SkipVars() []string {
	vars := make([]string, 0, len(ciSteps)+1)
	for _, s := range ciSteps {
		vars = append(vars, s.skip)
	}
	return append(vars, "SKIP_CHECKS_EXTRA")
}

// StepSkipVar is the variable that skips the full suite's step running
// script (relative to scripts/, e.g. check-todos.sh), "" when no step runs
// it.
func StepSkipVar(script string) string {
	for _, s := range ciSteps {
		if s.script == script {
			return s.skip
		}
	}
	return ""
}

// ciRun tallies the full suite's results.
type ciRun struct {
	ctx                     *checks.Context
	passed, failed, skipped int
	results                 []string
}

func (r *ciRun) skip(line string) {
	r.results = append(r.results, "SKIP  "+line)
	r.skipped++
}

func (r *ciRun) record(label string, code int) {
	if code == 0 {
		r.results = append(r.results, "PASS  "+label)
		r.passed++
	} else {
		r.results = append(r.results, "FAIL  "+label)
		r.failed++
	}
}

func (r *ciRun) header(label string) {
	echo(r.ctx.Stdout, "")
	echo(r.ctx.Stdout, "━━━ "+label+" ━━━")
}

// runCICheck is scripts/ci-check.sh: every check in order, a summary, and
// exit 1 on a failure in strict mode (the default; any first argument but
// --strict turns it off), then the project's checks_extra.
func runCICheck(ctx *checks.Context, args []string) int {
	strict := len(args) == 0 || args[0] == "" || args[0] == "--strict"
	warnOnce(ctx)
	out := ctx.Stdout
	echo(out, "Running contract and documentation checks...")

	r := &ciRun{ctx: ctx}
	for _, step := range ciSteps {
		s := resolve(ctx, step.script, false)
		if config.Getenv(step.skip) == "1" {
			r.skip(step.label)
			continue
		}
		if !s.found() {
			if maintainerSteps[step.script] && !config.IsRebarTree(ctx.Root) {
				// These check rebar's own source.
				r.skip(step.label + " (rebar source repo only)")
			} else {
				r.skip(step.label + " (script not found or not executable)")
			}
			continue
		}
		r.header(step.label)
		r.record(step.label, s.run(ctx, step.args, false))
	}
	for _, e := range Extras(ctx.Root, ExtraKey) {
		label := "extra: " + e.Name
		switch {
		case skipExtras():
			r.skip(label)
		case !e.Runnable(ctx.Root):
			r.results = append(r.results, "FAIL  "+label+" ("+e.Path+" not found or not executable)")
			r.failed++
		default:
			r.header(label)
			r.record(label, e.Run(ctx, nil))
		}
	}

	echo(out, "")
	echo(out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	echo(out, "SUMMARY")
	echo(out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, res := range r.results {
		echo(out, "  "+res)
	}
	echo(out, "")
	fmt.Fprintf(out, "  %d passed, %d failed, %d skipped\n", r.passed, r.failed, r.skipped)

	if r.failed > 0 && strict {
		echo(out, "")
		echo(out, "CI check failed. Fix the issues above or skip individual checks")
		echo(out, "with environment variables (e.g., SKIP_FRESHNESS=1).")
		return 1
	}
	return 0
}
