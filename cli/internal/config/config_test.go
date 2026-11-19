package config

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestContractsDirFor(t *testing.T) {
	cases := []struct {
		name, rc, env, want string
	}{
		{"default", "tier = 1\n", "", "architecture"},
		{"configured", "tier = 1\ncontracts_dir = specs\n", "", "specs"},
		{"comment and slash", "contracts_dir = docs/contracts/   # here\n", "", "docs/contracts"},
		{"indented key ignored like the bash reader", "  contracts_dir = specs\n", "", "architecture"},
		{"escape falls back", "contracts_dir = ../elsewhere\n", "", "architecture"},
		{"absolute falls back", "contracts_dir = /tmp/x\n", "", "architecture"},
		{"env wins", "contracts_dir = specs\n", "env-specs", "env-specs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ".rebarrc"), []byte(tc.rc), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("REBAR_CONTRACTS_DIR", tc.env)
			if got := ContractsDirFor(root); got != tc.want {
				t.Fatalf("ContractsDirFor = %q, want %q", got, tc.want)
			}
			c, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if c.ContractsDir != tc.want {
				t.Fatalf("Load().ContractsDir = %q, want %q", c.ContractsDir, tc.want)
			}
		})
	}
}

func TestChecksEnableFor(t *testing.T) {
	cases := []struct {
		name, rc, env string
		want          []string
	}{
		{"unset", "tier = 1\n", "", nil},
		{"list", "tier = 1\nchecks_enable = check-ground-truth compute-registry\n", "", []string{"check-ground-truth", "compute-registry"}},
		{"comment, tabs, .sh", "checks_enable =\tcheck-ground-truth.sh   steward # why\n", "", []string{"check-ground-truth", "steward"}},
		{"first line wins", "checks_enable = steward\nchecks_enable = check-todos\n", "", []string{"steward"}},
		{"commented out", "# checks_enable = steward\n", "", nil},
		{"env wins", "checks_enable = steward\n", "check-freshness", []string{"check-freshness"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testutil.WriteFile(t, root, ".rebarrc", tc.rc)
			t.Setenv("REBAR_CHECKS_ENABLE", tc.env)
			t.Setenv("REBAR_TIER", "")
			if got := ChecksEnableFor(root); !slices.Equal(got, tc.want) {
				t.Fatalf("ChecksEnableFor = %q, want %q", got, tc.want)
			}
			c, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(c.ChecksEnable, tc.want) {
				t.Fatalf("Load().ChecksEnable = %q, want %q", c.ChecksEnable, tc.want)
			}
		})
	}
}

func TestCheckTierAndUnknownChecks(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "tier = 1\nchecks_enable = check-ground-truth no-such-check\n")
	t.Setenv("REBAR_CHECKS_ENABLE", "")
	t.Setenv("REBAR_TIER", "")
	c, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tier != 1 {
		t.Fatalf("Tier = %d, want 1 (checks_enable must not change it)", c.Tier)
	}
	for name, want := range map[string]int{
		"check-ground-truth":    3,
		"check-ground-truth.sh": 3,
		"check-freshness":       1,
	} {
		if got := c.CheckTier(name); got != want {
			t.Errorf("CheckTier(%q) = %d, want %d", name, got, want)
		}
	}
	if got := UnknownChecks(c.ChecksEnable); !slices.Equal(got, []string{"no-such-check"}) {
		t.Fatalf("UnknownChecks = %q", got)
	}
}

// KnownChecks is the scripts' list (_REBAR_KNOWN_CHECKS in
// scripts/_rebar-config.sh, deleted in rebar 4.0), so checks_enable names
// valid in a 3.x project stay valid.
func TestKnownChecksMatchScripts(t *testing.T) {
	bash := strings.Fields("check-bypass-flags check-compliance check-contract-headers check-contract-refs check-decay-patterns check-doc-refs check-fix-commit check-freshness check-ground-truth check-jtbd-presence check-prefix-uniqueness check-todos compute-registry steward sync-bootstrap")
	if !slices.Equal(bash, KnownChecks) {
		t.Fatalf("KnownChecks = %q\nbash list   = %q", KnownChecks, bash)
	}
}

func TestContractNamespace(t *testing.T) {
	cases := []struct {
		name, rc, env, want string
	}{
		{"unset", "tier = 1\n", "", ""},
		{"configured", "contract_namespace = github.com/org/repo\n", "", "github.com/org/repo"},
		{"comment and slash", "contract_namespace = github.com/org/repo/   # ours\n", "", "github.com/org/repo"},
		{"commented out", "# contract_namespace = github.com/org/repo\n", "", ""},
		{"invalid ignored", "contract_namespace = org repo\n", "", ""},
		{"colon ignored", "contract_namespace = org:repo\n", "", ""},
		{"ID.version lookalike ignored", "contract_namespace = acme.1.2\n", "", ""},
		{"env trimmed like the bash reader", "", "  github.com/org/repo/  ", "github.com/org/repo"},
		{"env wins", "contract_namespace = github.com/org/repo\n", "example.com/env", "example.com/env"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testutil.WriteFile(t, root, ".rebarrc", tc.rc)
			t.Setenv("REBAR_CONTRACT_NAMESPACE", tc.env)
			if got := ContractNamespace(root); got != tc.want {
				t.Fatalf("ContractNamespace = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLocalContractRef(t *testing.T) {
	const ns = "github.com/org/repo"
	cases := []struct {
		name, ref, namespace, want string
		local                      bool
	}{
		{"unset keeps short form", "AUTH.1.0", "", "AUTH.1.0", true},
		{"unset keeps qualified form verbatim", ns + ":AUTH.1.0", "", ns + ":AUTH.1.0", true},
		{"short form is local", "AUTH.1.0", ns, "AUTH.1.0", true},
		{"own namespace stripped", ns + ":AUTH.1.0", ns, "AUTH.1.0", true},
		{"own namespace numbered id", ns + ":C1-AUTH.2.1", ns, "C1-AUTH.2.1", true},
		{"other namespace is foreign", "github.com/org/other:AUTH.1.0", ns, "", false},
		{"namespace prefix is not a match", ns + "-fork:AUTH.1.0", ns, "", false},
		{"trailing colon is not a namespace", "AUTH.1.0:", ns, "AUTH.1.0:", true},
		{"ID.version before a colon is not a namespace", "MISSING.1.0:see.2", ns, "MISSING.1.0:see.2", true},
		{"dot is literal", "githubXcom/org/repo:AUTH.1.0", ns, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, local := LocalContractRef(tc.ref, tc.namespace)
			if got != tc.want || local != tc.local {
				t.Fatalf("LocalContractRef(%q, %q) = %q, %v; want %q, %v", tc.ref, tc.namespace, got, local, tc.want, tc.local)
			}
		})
	}
}

func TestRCValueOr(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "source_dirs = cli scripts  # mine\nsource_extensions =\n")
	t.Setenv("REBAR_TEST_SOURCE_DIRS", "")
	cases := []struct {
		name, key, env, envVal, want string
	}{
		{"value, comment dropped", "source_dirs", "", "", "cli scripts"},
		{"empty value falls back", "source_extensions", "", "", "go"},
		{"absent key falls back", "source_exclude_dirs", "", "", "go"},
		{"empty env ignored", "source_dirs", "REBAR_TEST_SOURCE_DIRS", "", "cli scripts"},
		{"env wins", "source_dirs", "REBAR_TEST_SOURCE_DIRS", "pkg", "pkg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv(tc.env, tc.envVal)
			}
			if got := RCValueOr(root, tc.key, tc.env, "go"); got != tc.want {
				t.Fatalf("RCValueOr(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

// TestRCFile: REBAR_RCFILE replaces <root>/.rebarrc for every reader, as
// in scripts/_rebar-config.sh (rebar migrate tries settings that way
// before writing them).
func TestRCFile(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "tier = 1\nsource_dirs = project\nchecks_enable = steward\n")
	alt := testutil.WriteFile(t, t.TempDir(), "alt.rc", "tier = 2\nsource_dirs = alt  # tried\n")
	keys := []string{"source_dirs", "checks_enable", "source_extensions"}

	goValues := func() []string {
		t.Helper()
		c, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		vals := []string{strconv.Itoa(c.Tier)}
		for _, k := range keys {
			vals = append(vals, RCValueOr(root, k, "", "-"))
		}
		return vals
	}

	for _, tc := range []struct {
		name, rcfile string
		want         []string
	}{
		{"project .rebarrc", "", []string{"1", "project", "steward", "-"}},
		{"REBAR_RCFILE", alt, []string{"2", "alt", "-", "-"}},
		{"REBAR_RCFILE missing", filepath.Join(root, "none.rc"), []string{"3", "-", "-", "-"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, kv := range os.Environ() {
				if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "REBAR_") {
					t.Setenv(k, "")
				}
			}
			if tc.rcfile != "" {
				t.Setenv("REBAR_RCFILE", tc.rcfile)
			}
			if got := goValues(); !slices.Equal(got, tc.want) {
				t.Fatalf("tier and %v:\n got  = %q\n want = %q", keys, got, tc.want)
			}
			if got := ChecksEnableFor(root); strings.Join(got, " ") != map[bool]string{true: "steward", false: ""}[tc.rcfile == ""] {
				t.Fatalf("ChecksEnableFor = %q", got)
			}
		})
	}
}
