package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The release workflow and setup-rebar.sh stamp Version through -ldflags -X;
// that only works while Version stays a plain package-level string var and
// the root command reads it. Unstamped, it is the tree's release string.
func TestVersionIsStampable(t *testing.T) {
	if rootCmd.Version != Version {
		t.Fatalf("rootCmd.Version = %q, want Version (%q)", rootCmd.Version, Version)
	}
	if Version != releaseVersion {
		t.Fatalf("unstamped Version = %q, want releaseVersion (%q)", Version, releaseVersion)
	}
	if !semverTag.MatchString(releaseVersion) {
		t.Fatalf("releaseVersion %q is not a release tag", releaseVersion)
	}
}

// Badges (validated by check-compliance.sh) carry a release tag even when
// the binary is a branch build.
func TestBadgeVersion(t *testing.T) {
	saved := Version
	t.Cleanup(func() { Version = saved })
	for stamp, want := range map[string]string{
		"v9.9.9":                     "v9.9.9",
		"v9.9.9-beta.2":              "v9.9.9-beta.2",
		"feat/versioned@abc1234":     releaseVersion,
		"0123456789abcdef@0123456":   releaseVersion,
		"v9.9.9-beta.2-local.test.1": releaseVersion, // check-compliance rejects '-' inside the pre-release
	} {
		Version = stamp
		if got := badgeVersion(); got != want {
			t.Errorf("Version %q: badgeVersion() = %q, want %q", stamp, got, want)
		}
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

// .mcp.json must name rebar through ~/.rebar/current, not the version
// directory: on Linux os.Executable() resolves the current symlink, and a
// recorded versions/<v>/bin path breaks when that version is removed.
func TestRebarCommandUsesCurrent(t *testing.T) {
	base := filepath.Join(t.TempDir(), ".rebar")
	for _, v := range []string{"v1.0.0", "v2.0.0"} {
		testutil.WriteExecutable(t, base, "versions/"+v+"/bin/rebar", "#!/bin/sh\n")
	}
	if err := os.Symlink(filepath.Join("versions", "v1.0.0"), filepath.Join(base, "current")); err != nil {
		t.Fatal(err)
	}
	// The executable as Linux reports it: the resolved version path.
	got := rebarCommandFrom(filepath.Join(base, "versions", "v1.0.0", "bin", "rebar"))
	if want := filepath.Join(base, "current", "bin", "rebar"); got != want {
		t.Fatalf("active version: got %q, want %q", got, want)
	}
	// A version that is not current keeps its own path.
	got = rebarCommandFrom(filepath.Join(base, "versions", "v2.0.0", "bin", "rebar"))
	if want := filepath.Join(base, "versions", "v2.0.0", "bin", "rebar"); got != want {
		t.Fatalf("inactive version: got %q, want %q", got, want)
	}
	// Outside a versioned install nothing changes.
	plain := t.TempDir()
	testutil.WriteExecutable(t, plain, "bin/rebar", "#!/bin/sh\n")
	if got, want := rebarCommandFrom(filepath.Join(plain, "bin", "rebar")), filepath.Join(plain, "bin", "rebar"); got != want {
		t.Fatalf("checkout: got %q, want %q", got, want)
	}
}

// rebar new/init write a .mcp.json that runs `rebar mcp serve --stdio`.
func TestMCPConfigRunsNativeServer(t *testing.T) {
	root := filepath.Join(t.TempDir(), "proj")
	os.MkdirAll(root, 0o755)
	if !ensureMCPConfig(root) {
		t.Fatal(".mcp.json not written")
	}
	data, _ := os.ReadFile(filepath.Join(root, ".mcp.json"))
	var cfg struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	srv := cfg.MCPServers["rebar-ask"]
	want := []string{"mcp", "serve", "--stdio", "--repos-dir", filepath.Dir(root)}
	if srv.Command == "" || strings.Join(srv.Args, " ") != strings.Join(want, " ") {
		t.Errorf(".mcp.json:\n%s", data)
	}
	if ensureMCPConfig(root) {
		t.Error("an existing .mcp.json was rewritten")
	}
}
