package migrate

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// These tests run the real install scripts of this checkout against
// throwaway projects.

func installRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil || !isFile(filepath.Join(root, "scripts", "check-ground-truth.sh")) {
		t.Skip("rebar scripts not found next to the CLI source")
	}
	return root
}

const legacyGroundTruth = `#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
compute_metrics() {
  echo "go_files=$(find "$REPO_ROOT/src" -type f -name '*.go' | wc -l | tr -d ' ')"
  echo "shell_scripts=$(find "$REPO_ROOT/scripts" -maxdepth 1 -type f -name '*.sh' | wc -l | tr -d ' ')"
  EXTRA
}
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  failed=0
  while IFS='=' read -r key value; do
    doc=$(awk -F= -v k="$key" '$1 ~ "^[[:space:]]*" k "[[:space:]]*$" {gsub(/[[:space:]]/, "", $2); print $2}' "$REPO_ROOT/METRICS")
    if [ "$doc" = "$value" ]; then echo "OK: $key = $value"; else echo "DRIFT: $key — METRICS says $doc, repo has $value"; failed=1; fi
  done < <(compute_metrics)
  exit "$failed"
fi
`

// newProject lays out a tier-1 project with a customized ground-truth copy
// (extra is one more compute_metrics line), a customized steward.sh and a
// project script of its own.
func newProject(t *testing.T, extra, metrics string) string {
	t.Helper()
	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "# project settings\ntier = 1\n")
	testutil.WriteFile(t, p, "METRICS", metrics)
	testutil.WriteFile(t, p, "src/a.go", "package a\n\nvar A = 1\n")
	testutil.WriteFile(t, p, "src/b.go", "package a\n")
	testutil.WriteExecutable(t, p, "scripts/check-ground-truth.sh", strings.Replace(legacyGroundTruth, "EXTRA", extra, 1))
	testutil.WriteExecutable(t, p, "scripts/steward.sh", "#!/usr/bin/env bash\necho 'our own steward'\n")
	testutil.WriteExecutable(t, p, "scripts/deploy.sh", "#!/usr/bin/env bash\necho deploy\n")
	testutil.Git(t, p, "add", "-A")
	testutil.Git(t, p, "commit", "-q", "-m", "init")
	return p
}

// treeHash fingerprints every file under root except .git.
func treeHash(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		data, _ := os.ReadFile(path)
		info, _ := d.Info()
		rel, _ := filepath.Rel(root, path)
		lines = append(lines, fmt.Sprintf("%s %o %x", rel, info.Mode(), sha1.Sum(data)))
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func scriptPlan(p *Plan, name string) *ScriptPlan {
	for _, s := range p.Scripts {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func opts(project, install string) Options {
	return Options{ProjectRoot: project, RebarRoot: install, Timeout: time.Minute}
}

// Dry run changes nothing; --apply migrates the metrics, deletes the
// verified copy, keeps the unverified one, fixes METRICS for the deleted
// .sh file, and a second run proposes nothing.
func TestBuildPlanAndApply(t *testing.T) {
	install := installRoot(t)
	p := newProject(t, ":", "go_files = 2\nshell_scripts = 3\n")
	before := treeHash(t, p)

	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	if treeHash(t, p) != before {
		t.Fatal("BuildPlan (dry run) changed the project")
	}
	var buf bytes.Buffer
	Render(&buf, plan, false)
	out := buf.String()

	gt := scriptPlan(plan, "check-ground-truth.sh")
	if gt == nil || gt.Verdict != Delete {
		t.Fatalf("ground truth not verified:\n%s", out)
	}
	if st := scriptPlan(plan, "steward.sh"); st == nil || st.Verdict != Keep {
		t.Fatalf("customized steward.sh must be kept:\n%s", out)
	}
	if scriptPlan(plan, "deploy.sh") != nil {
		t.Fatal("deploy.sh is the project's own script, not a legacy copy")
	}
	for _, want := range []string{"metric.go_files.paths = src", "metric.shell_scripts.max_depth", "shell_scripts = 3 → 2", "tier: the project is tier 1", "+ checks_enable = check-ground-truth"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run output lacks %q:\n%s", want, out)
		}
	}

	results, err := Apply(plan, opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Regressed() {
			t.Fatalf("%s regressed after apply:\n%s", r.Name, r.Output)
		}
	}
	if isFile(filepath.Join(p, "scripts", "check-ground-truth.sh")) {
		t.Fatal("verified copy not deleted")
	}
	if !isFile(filepath.Join(p, "scripts", "steward.sh")) {
		t.Fatal("unverified copy deleted")
	}
	rc, _ := os.ReadFile(filepath.Join(p, ".rebarrc"))
	if !strings.HasPrefix(string(rc), "# project settings\ntier = 1\n") || !strings.Contains(string(rc), blockStart) {
		t.Fatalf(".rebarrc lost its original lines or the block:\n%s", rc)
	}
	if m, _ := os.ReadFile(filepath.Join(p, "METRICS")); string(m) != "go_files = 2\nshell_scripts = 2\n" {
		t.Fatalf("METRICS = %q", m)
	}

	again, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	if !again.Empty() {
		buf.Reset()
		Render(&buf, again, false)
		t.Fatalf("second run proposes changes:\n%s", buf.String())
	}
}

// A metric the translator cannot express keeps the copy, reported with its
// current value; the metrics it can express still move to .rebarrc.
func TestUntranslatableMetricKeepsCopy(t *testing.T) {
	install := installRoot(t)
	p := newProject(t, `echo "go_lines=$(cat "$REPO_ROOT"/src/*.go | wc -l | tr -d ' ')"`, "go_files = 2\nshell_scripts = 3\ngo_lines = 4\n")
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	gt := scriptPlan(plan, "check-ground-truth.sh")
	if gt.Verdict != Keep {
		t.Fatalf("copy with an untranslatable metric not kept: %+v", gt)
	}
	details := strings.Join(gt.Details, "\n")
	if !strings.Contains(details, "kept      go_lines = 4 — untranslatable") || !strings.Contains(details, "migrated  go_files = 2") {
		t.Fatalf("details:\n%s", details)
	}
	if _, ok := acceptedValue(plan.Added, "metric.go_files.paths"); !ok {
		t.Fatalf("translatable metric not proposed: %+v", plan.Added)
	}
	if len(plan.MetricsEdits) != 0 {
		t.Fatalf("nothing deleted, yet METRICS edits: %+v", plan.MetricsEdits)
	}
	if _, err := Apply(plan, opts(p, install)); err != nil {
		t.Fatal(err)
	}
	if !isFile(filepath.Join(p, "scripts", "check-ground-truth.sh")) {
		t.Fatal("copy deleted although a metric could not be verified")
	}
}

// A translated metric whose value differs is not accepted.
func TestMismatchedMetricIsRejected(t *testing.T) {
	install := installRoot(t)
	// `ls src/*` translates to "files in src", but ls also lists the
	// contents of a matching directory, so the values differ.
	p := newProject(t, `echo "listed=$(ls "$REPO_ROOT"/src/* | wc -l | tr -d ' ')"`, "go_files = 3\nshell_scripts = 3\n")
	testutil.WriteFile(t, p, "src/sub/c.go", "package sub\n")
	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	gt := scriptPlan(plan, "check-ground-truth.sh")
	if gt.Verdict != Keep || !strings.Contains(strings.Join(gt.Details, "\n"), "kept      listed = ") {
		t.Fatalf("copy deleted with a mismatched metric: %+v", gt)
	}
	if _, ok := acceptedValue(plan.Added, "metric.listed.paths"); ok {
		t.Fatal("mismatched metric proposed for .rebarrc")
	}
}

func TestRefusesTheInstallItself(t *testing.T) {
	install := installRoot(t)
	if _, err := BuildPlan(opts(install, install)); err == nil {
		t.Fatal("migrating the rebar install itself must fail")
	}
}

// A customized compute-registry.sh (hard-coded EXT- scope prefix, old
// 80-byte purpose cut): migrate finds the two settings that reproduce its
// output byte for byte, and nothing more; --regenerate-registry drops the
// byte cut instead.
func TestRegistrySettingsAreDerived(t *testing.T) {
	install := installRoot(t)
	src, err := os.ReadFile(filepath.Join(install, "scripts", "compute-registry.sh"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := string(src)
	for old, repl := range map[string]string{
		"\nPURPOSE_STYLE=ellipsis\n":          "\nPURPOSE_STYLE=bytes\n",
		`  for prefix in $SCOPE_PREFIXES; do`: "  id=\"${id#EXT-}\"\n  for prefix in $SCOPE_PREFIXES; do",
	} {
		if !strings.Contains(legacy, old) {
			t.Skipf("install compute-registry.sh changed shape (%q)", old)
		}
		legacy = strings.Replace(legacy, old, repl, 1)
	}

	p := testutil.InitRepo(t)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 2\n")
	long := strings.Repeat("Ünïcödé purpose text — ", 6)
	testutil.WriteFile(t, p, "architecture/CONTRACT-EXT-S1-AUTH.1.0.md", "# Auth\n\n**Status:** active\n\n## Purpose\n\n"+long+"\n")
	testutil.WriteFile(t, p, "architecture/CONTRACT-C2-STORE.1.0.md", "# Store\n\n**Status:** active\n\n## Purpose\n\nStores things.\n")
	testutil.WriteExecutable(t, p, "scripts/compute-registry.sh", legacy)
	testutil.Run(t, p, "bash", "scripts/compute-registry.sh")
	testutil.Git(t, p, "add", "-A")
	testutil.Git(t, p, "commit", "-q", "-m", "init")

	plan, err := BuildPlan(opts(p, install))
	if err != nil {
		t.Fatal(err)
	}
	reg := scriptPlan(plan, "compute-registry.sh")
	if reg == nil || reg.Verdict != Delete {
		t.Fatalf("registry copy not verified: %+v", reg)
	}
	got := map[string]string{}
	for _, s := range plan.Added {
		got[s.Key] = s.Value
	}
	if got["registry_scope_prefixes"] != "EXT-" || got["registry_purpose_style"] != "bytes" || len(got) != 2 {
		t.Fatalf("derived settings = %v, want exactly scope prefixes EXT- and purpose style bytes", got)
	}
	if !strings.Contains(strings.Join(reg.Details, "\n"), "registry_purpose_style = bytes was needed") {
		t.Fatalf("bytes not reported: %v", reg.Details)
	}

	o := opts(p, install)
	o.RegenerateRegistry = true
	plan, err = BuildPlan(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := acceptedValue(plan.Added, "registry_purpose_style"); ok || !plan.RegenRegistry {
		t.Fatalf("--regenerate-registry kept the byte cut: %+v", plan.Added)
	}
	if _, err := Apply(plan, o); err != nil {
		t.Fatal(err)
	}
	reg2, _ := os.ReadFile(filepath.Join(p, "architecture", "CONTRACT-REGISTRY.md"))
	if !strings.Contains(string(reg2), "...") {
		t.Fatalf("registry not regenerated with the ellipsis style:\n%s", reg2)
	}
}

func TestRegistryCandidates(t *testing.T) {
	src := `ARCH_DIR="$PROJECT_ROOT/specs"
contract_id() { local base="${1#CONTRACT-}"; echo "$base"; }
classify_prefix() {
  local id="$1"
  local scoped_id="${id#SDK-}"
  scoped_id="${scoped_id#MCP-}"
}
purpose=$(awk '/^## ([0-9]+\.[[:space:]]*)?(Purpose|Why this exists)/{found=1; next}' "$file")
git grep -l "CONTRACT:${id}" -- '*.go' '*.ts' '*.sh' | grep -v "^architecture/"
`
	got := map[string]string{}
	for _, s := range registryCandidates(src, []byte("registry_impl_counts = false\n")) {
		got[s.Key] = s.Value
	}
	want := map[string]string{
		"contracts_dir":            "specs",
		"registry_scope_prefixes":  "SDK- MCP-",
		"registry_purpose_heading": "Purpose|Why this exists",
		"registry_impl_globs":      "*.go *.ts *.sh",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("candidates = %v\nwant %v", got, want)
	}
}
