package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveScript(t *testing.T) {
	project := t.TempDir()
	install := t.TempDir()
	writeScript(t, filepath.Join(install, "scripts", "ci-check.sh"), "#!/usr/bin/env bash\n")
	writeScript(t, filepath.Join(install, "scripts", "steward.sh"), "#!/usr/bin/env bash\n")

	// No project copy: the install's script runs.
	got, ok := resolveScript(project, install, "ci-check.sh")
	if !ok || got.Path != filepath.Join(install, "scripts", "ci-check.sh") || got.Legacy {
		t.Fatalf("no project copy: got %+v ok=%v", got, ok)
	}

	// A thin wrapper is skipped, or `rebar check` would call itself.
	writeScript(t, filepath.Join(project, "scripts", "ci-check.sh"),
		"#!/usr/bin/env bash\n# rebar-thin-wrapper\nexec rebar check \"$@\"\n")
	got, _ = resolveScript(project, install, "ci-check")
	if got.Path != filepath.Join(install, "scripts", "ci-check.sh") {
		t.Fatalf("thin wrapper: got %+v", got)
	}

	// A full legacy copy wins and is flagged.
	writeScript(t, filepath.Join(project, "scripts", "steward.sh"), "#!/usr/bin/env bash\necho custom\n")
	got, _ = resolveScript(project, install, "steward")
	if got.Path != filepath.Join(project, "scripts", "steward.sh") || !got.Legacy {
		t.Fatalf("legacy copy: got %+v", got)
	}

	// Inside the rebar repo itself the project copy is the install.
	got, _ = resolveScript(install, install, "steward.sh")
	if got.Legacy {
		t.Fatalf("rebar repo: got %+v", got)
	}

	// Neither has it.
	if _, ok := resolveScript(project, install, "nope"); ok {
		t.Fatal("missing script resolved")
	}
	if _, ok := resolveScript(project, "", "ci-check"); ok {
		t.Fatal("thin wrapper resolved with no install")
	}
}

func TestIsThinWrapperNeedsOwnLine(t *testing.T) {
	dir := t.TempDir()
	quoted := filepath.Join(dir, "quoted.sh")
	writeScript(t, quoted, "#!/usr/bin/env bash\ngrep -q '# rebar-thin-wrapper' \"$f\"\n")
	if isThinWrapper(quoted) {
		t.Fatal("a script that mentions the marker is not a thin wrapper")
	}
}

func TestIsRebarSourceDir(t *testing.T) {
	dir := t.TempDir()
	if isRebarSourceDir(dir) {
		t.Fatal("empty dir matched")
	}
	writeScript(t, filepath.Join(dir, "scripts", "steward.sh"), "")
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
