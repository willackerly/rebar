package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestResolveScript(t *testing.T) {
	project := t.TempDir()

	// Nothing in the project: nothing to run but the built-in checks.
	if _, ok := resolveScript(project, "ci-check.sh"); ok {
		t.Fatal("resolved a script the project does not have")
	}

	// A thin wrapper is skipped, or `rebar check` would call itself.
	testutil.WriteExecutable(t, project, "scripts/ci-check.sh",
		"#!/usr/bin/env bash\n# rebar-thin-wrapper\nexec rebar check \"$@\"\n")
	if got, ok := resolveScript(project, "ci-check"); ok {
		t.Fatalf("thin wrapper: got %+v", got)
	}

	// A full copy of one of rebar's scripts wins and is flagged legacy.
	testutil.WriteExecutable(t, project, "scripts/steward.sh", "#!/usr/bin/env bash\necho custom\n")
	got, ok := resolveScript(project, "steward")
	if !ok || got.Path != filepath.Join(project, "scripts", "steward.sh") || !got.Legacy {
		t.Fatalf("legacy copy: got %+v ok=%v", got, ok)
	}

	// The project's own other scripts resolve, not legacy.
	testutil.WriteExecutable(t, project, "scripts/deploy.sh", "#!/usr/bin/env bash\n")
	if got, ok := resolveScript(project, "deploy.sh"); !ok || got.Legacy {
		t.Fatalf("own script: got %+v ok=%v", got, ok)
	}

	// Inside a rebar source checkout nothing is legacy.
	src := t.TempDir()
	testutil.WriteFile(t, src, "templates/project-bootstrap/README.md", "x\n")
	testutil.WriteFile(t, src, "cli/internal/assets/assets.go", "package assets\n")
	testutil.WriteExecutable(t, src, "scripts/steward.sh", "#!/usr/bin/env bash\n")
	if got, _ := resolveScript(src, "steward.sh"); got.Legacy {
		t.Fatalf("rebar source: got %+v", got)
	}
}

func TestIsThinWrapperNeedsOwnLine(t *testing.T) {
	dir := t.TempDir()
	quoted := filepath.Join(dir, "quoted.sh")
	testutil.WriteExecutable(t, filepath.Dir(quoted), filepath.Base(quoted), "#!/usr/bin/env bash\ngrep -q '# rebar-thin-wrapper' \"$f\"\n")
	if isThinWrapper(quoted) {
		t.Fatal("a script that mentions the marker is not a thin wrapper")
	}
}

func TestIsRebarSourceDir(t *testing.T) {
	dir := t.TempDir()
	if isRebarSourceDir(dir) {
		t.Fatal("empty dir matched")
	}
	testutil.WriteExecutable(t, dir, "scripts/steward.sh", "")
	if isRebarSourceDir(dir) {
		t.Fatal("an adopter with scripts/steward.sh but no templates/ matched")
	}
	if err := os.MkdirAll(filepath.Join(dir, "templates", "project-bootstrap"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isRebarSourceDir(dir) {
		t.Fatal("rebar source layout not matched")
	}
}
