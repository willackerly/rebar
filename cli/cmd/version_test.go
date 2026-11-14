package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The release workflow and setup-rebar.sh stamp Version through -ldflags -X;
// that only works while Version stays a plain package-level string var and
// the root command reads it.
func TestVersionIsStampable(t *testing.T) {
	if rootCmd.Version != Version {
		t.Fatalf("rootCmd.Version = %q, want Version (%q)", rootCmd.Version, Version)
	}
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`).MatchString(Version) {
		t.Fatalf("default Version %q is not a release tag", Version)
	}
}

// The exact ldflags string setup-rebar.sh and scripts/package-release.sh use.
func TestVersionLdflagsStamp(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the CLI")
	}
	bin := filepath.Join(t.TempDir(), "rebar")
	testutil.Run(t, "../", "go", "build", "-trimpath",
		"-ldflags", "-s -w -X github.com/willackerly/rebar/cli/cmd.Version=v0.0.0-stamp.1",
		"-o", bin, ".")
	if got := testutil.Run(t, t.TempDir(), bin, "version"); got != "rebar v0.0.0-stamp.1" {
		t.Fatalf("rebar version = %q, want %q", got, "rebar v0.0.0-stamp.1")
	}
}

func makeInstall(t *testing.T, root string) {
	t.Helper()
	testutil.WriteFile(t, root, "templates/project-bootstrap/README.md", "template\n")
	testutil.WriteExecutable(t, root, "scripts/steward.sh", "#!/usr/bin/env bash\n")
}

// A versioned install (~/.rebar/current → versions/<ver>) is found even when
// the running binary lives elsewhere, and wins over a pre-versioned ~/.rebar
// clone in the same directory; the clone alone is still found.
func TestFindRebarRootVersionedInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REBAR_ROOT", "")
	base := filepath.Join(home, ".rebar")

	makeInstall(t, base) // legacy clone layout
	if got := findRebarRoot(); got != base {
		t.Fatalf("legacy only: findRebarRoot() = %q, want %q", got, base)
	}

	makeInstall(t, filepath.Join(base, "versions", "v1.0.0"))
	if err := os.Symlink(filepath.Join("versions", "v1.0.0"), filepath.Join(base, "current")); err != nil {
		t.Fatal(err)
	}
	if got, want := findRebarRoot(), filepath.Join(base, "current"); got != want {
		t.Fatalf("versioned: findRebarRoot() = %q, want %q", got, want)
	}
}
