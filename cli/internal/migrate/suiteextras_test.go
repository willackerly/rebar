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
	return releasedSuiteAt(t, "v3.0.0", name)
}

func releasedSuiteAt(t *testing.T, tag, name string) string {
	t.Helper()
	out, err := testutil.TryGit(installRoot(t), "show", tag+":scripts/"+name)
	if err != nil {
		t.Skip(tag + " is not in this clone")
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
	// v3.0.0's pre-commit.sh skips everything ("Cannot find check
	// scripts") without a check-todos.sh beside it.
	todos := releasedSuite(t, "check-todos.sh")
	p := suiteProject(t, "0", map[string]string{"ci-check.sh": ci, "pre-commit.sh": pc, "check-todos.sh": todos})
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

// v3.1 suites name run_check's script bare; _rebar_script resolves it in
// scripts/, so it is a plain call too.
func TestSuiteCopyWithBareRunCheck(t *testing.T) {
	ci := withExtra(t, releasedSuiteAt(t, "v3.1.1", "ci-check.sh"), "\n# NOTE: Tag-to-CI", `run_check "OpenAPI"             SKIP_OPENAPI          check-openapi.sh`+"\n")
	p := suiteProject(t, "0", map[string]string{"ci-check.sh": ci})
	plan, err := BuildPlan(opts(p, ""))
	if err != nil {
		t.Fatal(err)
	}
	if sp := scriptPlan(plan, "ci-check.sh"); sp == nil || sp.Verdict != Replace {
		t.Fatalf("not replaced:\n%s", render(plan))
	}
	if !strings.Contains(string(plan.NewRC), "checks_extra = scripts/check-openapi.sh") {
		t.Errorf("proposed .rebarrc lacks checks_extra:\n%s", plan.NewRC)
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
	// Calls whose failure the copy treats otherwise than a checks_extra
	// entry (fatal, unconditional) keep the copy, with the reason.
	pcReleased := releasedSuite(t, "pre-commit.sh")
	const pcAnchor = `if [ "$failed" -gt 0 ]`
	const ciAnchor = "\n# NOTE: Tag-to-CI"
	todos := releasedSuite(t, "check-todos.sh")
	for _, tc := range []struct {
		name, suite, src, anchor, add, want string
	}{
		{"or true", "pre-commit.sh", pcReleased, pcAnchor,
			"\"$BASE/check-openapi.sh\" || true\n", "is not a plain fatal call"},
		{"warning only", "pre-commit.sh", pcReleased, pcAnchor,
			"if ! \"$BASE/check-openapi.sh\"; then\n  echo \"warning: OpenAPI spec drifted\"\nfi\n", "is not a plain fatal call"},
		{"CI only", "ci-check.sh", released, ciAnchor,
			"if [ -n \"${CI:-}\" ]; then\n" + ciExtraLine + "fi\n", "runs inside `if [ -n \"${CI:-}\" ]; then`"},
		{"guard with more", "pre-commit.sh", pcReleased, pcAnchor,
			"if [ -x \"$BASE/check-openapi.sh\" ]; then\n  \"$BASE/check-openapi.sh\" || failed=$((failed + 1))\n  rm -f openapi.cache\nfi\n", "runs inside `if [ -x"},
		{"extra statement on the line", "pre-commit.sh", pcReleased, pcAnchor,
			"\"$BASE/check-openapi.sh\" || failed=$((failed + 1)); export OPENAPI_DONE=1\n", "is not a plain fatal call"},
		{"arguments", "ci-check.sh", released, ciAnchor,
			`run_check "OpenAPI" SKIP_OPENAPI "$SCRIPT_DIR/check-openapi.sh" --lenient` + "\n", "is not a plain fatal call"},
		{"extra statements elsewhere", "pre-commit.sh", pcReleased, pcAnchor,
			"export OPENAPI_STRICT=1\n\"$BASE/check-openapi.sh\" || failed=$((failed + 1))\n", "more than those calls"},
		// v3.0.0's run_check runs its argument as given: a bare name is
		// not found from the project root, so the copy skips the check
		// (and passes) even when it fails, which only the forced failure
		// shows.
		{"failure ignored", "ci-check.sh", released, ciAnchor,
			`run_check "OpenAPI" SKIP_OPENAPI check-openapi.sh` + "\n", "when they fail the copy exits 0 where rebar's suite exits 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := suiteProject(t, "0", map[string]string{tc.suite: withExtra(t, tc.src, tc.anchor, tc.add), "check-todos.sh": todos})
			plan, err := BuildPlan(opts(p, ""))
			if err != nil {
				t.Fatal(err)
			}
			if sp := scriptPlan(plan, tc.suite); sp == nil || sp.Verdict != Keep || !strings.Contains(sp.Reason, tc.want) {
				t.Fatalf("not kept with %q:\n%s", tc.want, render(plan))
			}
			if strings.Contains(string(plan.NewRC), "checks_extra") {
				t.Errorf("checks_extra proposed anyway:\n%s", plan.NewRC)
			}
		})
	}
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

// Planning and apply run legacy copies with the same `rebar` first on PATH
// (Options.RebarOnPath), so what planning verified is what apply runs.
func TestRebarOnPath(t *testing.T) {
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 1\n")
	testutil.WriteExecutable(t, p, "scripts/check-todos.sh", "#!/usr/bin/env bash\n# customized\necho \"rebar=$(readlink \"$(command -v rebar)\")\"\n")
	commitAll(t, p)
	stub := testutil.WriteExecutable(t, t.TempDir(), "rebar-stub", "#!/bin/sh\nexit 0\n")
	o := opts(p, "")
	o.RebarOnPath = stub
	plan, err := BuildPlan(o)
	if err != nil {
		t.Fatal(err)
	}
	sp := scriptPlan(plan, "check-todos.sh")
	if sp == nil || sp.Before == nil || !strings.Contains(sp.Before.Stdout, "rebar="+stub) {
		t.Fatalf("planning did not put RebarOnPath first on PATH:\n%s\n%+v", render(plan), sp)
	}
	res, err := Apply(plan, o, ApplyOptions{AllowDirty: true})
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	for _, c := range res.Checks {
		if c.Name == "check-todos.sh" {
			ran = true
			if !strings.Contains(c.Output, "rebar="+stub) {
				t.Errorf("apply ran %s with another rebar: %q", c.Ran, c.Output)
			}
		}
	}
	if !ran {
		t.Errorf("apply did not run check-todos.sh: %+v", res.Checks)
	}
}
