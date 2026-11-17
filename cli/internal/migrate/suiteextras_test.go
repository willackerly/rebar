package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/assets"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// releasedSuite is a suite script as rebar v3.0.0 shipped it (skips
// without the tag in this clone).
func releasedSuite(t *testing.T, name string) string {
	t.Helper()
	out, err := testutil.TryGit(installRoot(t), "show", "v3.0.0:scripts/"+name)
	if err != nil {
		t.Skip("v3.0.0 is not in this clone")
	}
	return out + "\n"
}

// suiteProject is a tier-1 project that rebar's full suite passes at tier
// 3 (migrate's probes run there), with a project check scripts/check-openapi.sh
// that exits openapiExit and the given suite copies.
func suiteProject(t *testing.T, openapiExit string, copies map[string]string) string {
	t.Helper()
	p := testutil.InitRepo(t)
	agents, _ := assets.ReadFile(assets.BootstrapDir + "/AGENTS.md")
	testutil.WriteFile(t, p, ".rebarrc", "tier = 1\n")
	testutil.WriteFile(t, p, ".rebar-version", "v3.1.0\n")
	testutil.WriteFile(t, p, "README.md", "# fx\n\n> **rebar v3.1.0** | **Tier 1: PARTIAL**\n")
	testutil.WriteFile(t, p, "AGENTS.md", string(agents))
	testutil.WriteExecutable(t, p, "scripts/check-openapi.sh", "#!/usr/bin/env bash\necho openapi\nexit "+openapiExit+"\n")
	for name, body := range copies {
		testutil.WriteExecutable(t, p, "scripts/"+name, body)
	}
	os.MkdirAll(filepath.Join(p, "architecture"), 0o755)
	testutil.Git(t, p, "add", "-A")
	if r := runNative(p, "", scrubbedEnv("REBAR_PROJECT_ROOT="+p), "compute-registry.sh"); !r.ok() {
		t.Fatalf("compute-registry: %+v", r)
	}
	commitAll(t, p)
	return p
}

const ciExtraLine = `run_check "OpenAPI"             SKIP_OPENAPI          "$SCRIPT_DIR/check-openapi.sh"` + "\n"

const preCommitExtraBlock = `echo ""
echo "Pre-commit: checking the OpenAPI spec..."
if [ -x "$BASE/check-openapi.sh" ]; then
  "$BASE/check-openapi.sh" || failed=$((failed + 1))
fi

`

func withExtra(t *testing.T, src, before, add string) string {
	t.Helper()
	i := strings.Index(src, before)
	if i < 0 {
		t.Fatalf("anchor %q not in the released suite", before)
	}
	return src[:i] + add + src[i:]
}

// A released suite plus calls to a project script becomes the thin wrapper
// with checks_extra / checks_extra_pre_commit; apply keeps the suites green.
func TestSuiteCopyWithExtras(t *testing.T) {
	ci := withExtra(t, releasedSuite(t, "ci-check.sh"), "\n# NOTE: Tag-to-CI", ciExtraLine)
	pc := withExtra(t, releasedSuite(t, "pre-commit.sh"), `if [ "$failed" -gt 0 ]`, preCommitExtraBlock)
	p := suiteProject(t, "0", map[string]string{"ci-check.sh": ci, "pre-commit.sh": pc})
	plan, err := BuildPlan(opts(p, ""))
	if err != nil {
		t.Fatal(err)
	}
	out := render(plan)
	for _, n := range []string{"ci-check.sh", "pre-commit.sh"} {
		if sp := scriptPlan(plan, n); sp == nil || sp.Verdict != Replace || !strings.Contains(sp.Reason, "scripts/check-openapi.sh") {
			t.Fatalf("%s not replaced:\n%s", n, out)
		}
	}
	for _, want := range []string{"checks_extra = scripts/check-openapi.sh", "checks_extra_pre_commit = scripts/check-openapi.sh"} {
		if !strings.Contains(string(plan.NewRC), want) {
			t.Errorf("proposed .rebarrc lacks %q:\n%s", want, plan.NewRC)
		}
	}
	if scriptPlan(plan, "check-openapi.sh") != nil {
		t.Error("the project's own check was treated as a rebar copy")
	}
	res, err := Apply(plan, opts(p, ""), ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RolledBack {
		t.Fatalf("apply rolled back: %+v", res.Checks)
	}
	wrapper, _ := os.ReadFile(filepath.Join(p, "scripts", "ci-check.sh"))
	if !isThinWrapper(wrapper) {
		t.Errorf("scripts/ci-check.sh is not the thin wrapper:\n%s", wrapper)
	}
}

// A suite copy is kept when it is more than a released version plus extra
// calls, or when rebar's suite with the extras exits otherwise.
func TestSuiteCopyKept(t *testing.T) {
	released := releasedSuite(t, "ci-check.sh")
	t.Run("other customization", func(t *testing.T) {
		ci := withExtra(t, strings.Replace(released, "Running contract and documentation checks...", "Running our checks...", 1), "\n# NOTE: Tag-to-CI", ciExtraLine)
		p := suiteProject(t, "0", map[string]string{"ci-check.sh": ci})
		plan, err := BuildPlan(opts(p, ""))
		if err != nil {
			t.Fatal(err)
		}
		if sp := scriptPlan(plan, "ci-check.sh"); sp == nil || sp.Verdict != Keep || !strings.Contains(sp.Reason, "more than those calls") {
			t.Fatalf("kept for the wrong reason:\n%s", render(plan))
		}
		if strings.Contains(string(plan.NewRC), "checks_extra") {
			t.Errorf("checks_extra proposed anyway:\n%s", plan.NewRC)
		}
	})
	t.Run("different exit", func(t *testing.T) {
		ci := withExtra(t, released, "\n# NOTE: Tag-to-CI", ciExtraLine)
		p := suiteProject(t, "0", map[string]string{"ci-check.sh": ci})
		// rebar's compliance check fails without AGENTS.md; the copy has
		// no check-compliance.sh to run and passes.
		os.Remove(filepath.Join(p, "AGENTS.md"))
		commitAll(t, p)
		plan, err := BuildPlan(opts(p, ""))
		if err != nil {
			t.Fatal(err)
		}
		if sp := scriptPlan(plan, "ci-check.sh"); sp == nil || sp.Verdict != Keep || !strings.Contains(sp.Reason, "exits 1 where the copy exits 0") {
			t.Fatalf("not kept on a different exit:\n%s", render(plan))
		}
	})
}
