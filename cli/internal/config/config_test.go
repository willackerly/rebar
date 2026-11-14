package config

import (
	"os"
	"path/filepath"
	"testing"
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
