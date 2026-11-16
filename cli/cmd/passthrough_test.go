package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// resetRootFlags clears the root persistent flags after a test that sets
// them: they are package variables shared by every Execute.
func resetRootFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		repoRoot, verbose, jsonOut = "", false, false
		rootCmd.SetArgs(nil)
	})
}

func TestHoistRootFlags(t *testing.T) {
	resetRootFlags(t)
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"repo-root before run", []string{"--repo-root", "/p", "run", "check-todos"}, []string{"run", "check-todos"}},
		{"= form and bools", []string{"--repo-root=/p", "-v", "--json", "run", "steward"}, []string{"run", "steward"}},
		{"after run, before the script", []string{"run", "-v", "--repo-root", "/p", "steward"}, []string{"run", "steward"}},
		{"after the script passes through", []string{"--repo-root", "/p", "run", "s", "--repo-root", "x", "-v"}, []string{"run", "s", "--repo-root", "x", "-v"}},
		{"ask keeps its own -v", []string{"-v", "ask", "-v", "architect", "q"}, []string{"ask", "-v", "architect", "q"}},
		{"push keeps git flags", []string{"--repo-root", "/p", "push", "-v", "origin"}, []string{"push", "-v", "origin"}},
		{"parsed commands untouched", []string{"-v", "status"}, []string{"-v", "status"}},
		{"unknown flag untouched", []string{"--bogus", "run", "x"}, []string{"--bogus", "run", "x"}},
		{"missing value untouched", []string{"--repo-root"}, []string{"--repo-root"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := hoistRootFlags(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("hoistRootFlags(%q) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}

	repoRoot, verbose = "", false
	if _, err := hoistRootFlags([]string{"--repo-root", "/p", "-v", "run", "x"}); err != nil {
		t.Fatal(err)
	}
	if repoRoot != "/p" || !verbose {
		t.Fatalf("flags not parsed: repoRoot=%q verbose=%v", repoRoot, verbose)
	}
}

// End to end: `rebar --repo-root X run <script>` used to fail with
// `no script "--repo-root"`. The script runs in X and gets exactly the
// arguments after its name.
func TestRunAfterRootFlags(t *testing.T) {
	resetRootFlags(t)
	t.Setenv("REBAR_PROJECT_ROOT", "")
	project := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")
	t.Setenv("PROBE_OUT", out)
	testutil.WriteExecutable(t, project, "scripts/probe.sh",
		"#!/usr/bin/env bash\nprintf '%s|%s\\n' \"$PWD\" \"$*\" > \"$PROBE_OUT\"\n")

	for _, args := range [][]string{
		{"--repo-root", project, "run", "probe", "a", "--json", "-v"},
		{"-v", "--repo-root=" + project, "run", "probe", "a", "--json", "-v"},
		{"run", "--repo-root", project, "probe", "a", "--json", "-v"},
	} {
		repoRoot = ""
		if err := executeArgs(args); err != nil {
			t.Fatalf("rebar %s: %v", strings.Join(args, " "), err)
		}
		got, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		want := project + "|a --json -v"
		if real, err := filepath.EvalSymlinks(project); err == nil && strings.HasPrefix(string(got), real) {
			want = real + "|a --json -v"
		}
		if strings.TrimSpace(string(got)) != want {
			t.Fatalf("rebar %s: script saw %q, want %q", strings.Join(args, " "), got, want)
		}
	}
}
