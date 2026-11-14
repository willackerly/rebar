package config

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// KnownChecks must match the bash list and name real scripts, or the CLI and
// the scripts would disagree about which names are valid.
func TestKnownChecksMatchScripts(t *testing.T) {
	scriptsDir := filepath.Join("..", "..", "..", "scripts")
	data, err := os.ReadFile(filepath.Join(scriptsDir, "_rebar-config.sh"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^_REBAR_KNOWN_CHECKS="([^"]*)"`).FindSubmatch(data)
	if m == nil {
		t.Fatal("_REBAR_KNOWN_CHECKS not found in scripts/_rebar-config.sh")
	}
	if bash := strings.Fields(string(m[1])); !slices.Equal(bash, KnownChecks) {
		t.Fatalf("KnownChecks = %q\nbash list   = %q", KnownChecks, bash)
	}
	for _, name := range KnownChecks {
		if _, err := os.Stat(filepath.Join(scriptsDir, name+".sh")); err != nil {
			t.Errorf("known check %s has no script: %v", name, err)
		}
	}
}
