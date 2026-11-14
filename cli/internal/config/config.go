package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	RepoRoot   string
	RebarDir   string // .rebar/
	Tier       int    // 1, 2, or 3
	Version    string // from .rebar-version
	ScriptsDir string // scripts/
	AgentsDir  string // agents/
	BinDir     string // bin/
	// ContractsDir holds the contracts (CONTRACT-*.md, CONTRACT-REGISTRY.md,
	// the steward's .state/), relative to RepoRoot. It names the contracts
	// location only: some repos keep other design docs in architecture/.
	ContractsDir string
}

// DefaultContractsDir is where contracts live unless .rebarrc says otherwise.
const DefaultContractsDir = "architecture"

// Load reads configuration from .rebarrc and .rebar-version, respecting
// REBAR_TIER env override. Mirrors the logic in scripts/_rebar-config.sh.
func Load(repoRoot string) (*Config, error) {
	c := &Config{
		RepoRoot:   repoRoot,
		RebarDir:   filepath.Join(repoRoot, ".rebar"),
		ScriptsDir: filepath.Join(repoRoot, "scripts"),
		AgentsDir:  filepath.Join(repoRoot, "agents"),
		BinDir:     filepath.Join(repoRoot, "bin"),
		Tier:       3, // default: full enforcement
	}
	c.ContractsDir = ContractsDirFor(repoRoot)

	// REBAR_TIER env takes precedence
	if env := os.Getenv("REBAR_TIER"); env != "" {
		t, err := strconv.Atoi(env)
		if err == nil && t >= 1 && t <= 3 {
			c.Tier = t
		}
	} else {
		// Read from .rebarrc
		tier, err := readRebarRC(filepath.Join(repoRoot, ".rebarrc"))
		if err == nil && tier >= 1 && tier <= 3 {
			c.Tier = tier
		}
	}

	// Read version
	versionBytes, err := os.ReadFile(filepath.Join(repoRoot, ".rebar-version"))
	if err == nil {
		c.Version = strings.TrimSpace(string(versionBytes))
	}

	return c, nil
}

// FindRepoRoot walks up from dir looking for .rebarrc, .rebar/, or .git.
func FindRepoRoot(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		// Check for .rebar/ directory first (strongest signal)
		if info, err := os.Stat(filepath.Join(dir, ".rebar")); err == nil && info.IsDir() {
			return dir, nil
		}
		// Then .rebarrc
		if _, err := os.Stat(filepath.Join(dir, ".rebarrc")); err == nil {
			return dir, nil
		}
		// Then .git (fallback)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a rebar repository (no .rebar/, .rebarrc, or .git found)")
		}
		dir = parent
	}
}

// readRebarRC parses a .rebarrc file for the tier setting.
// Format: key = value lines, comments with #.
func readRebarRC(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if strings.EqualFold(key, "tier") || strings.EqualFold(key, "rebar_tier") {
			return strconv.Atoi(val)
		}
	}
	return 0, fmt.Errorf("tier not found in .rebarrc")
}

// EnsureRebarDir creates the .rebar/ directory structure.
func EnsureRebarDir(repoRoot string) error {
	dirs := []string{
		filepath.Join(repoRoot, ".rebar"),
		filepath.Join(repoRoot, ".rebar", "keys"),
		filepath.Join(repoRoot, ".rebar", "trusted-keys"),
		filepath.Join(repoRoot, ".rebar", "envelopes"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return nil
}

// ContractsDirFor returns the contracts directory of the repo at repoRoot,
// relative to it: env REBAR_CONTRACTS_DIR, then .rebarrc contracts_dir, then
// "architecture". Mirrors _rebar_contracts_dir in scripts/_rebar-config.sh.
// A value that is absolute or leaves the repo falls back to the default.
func ContractsDirFor(repoRoot string) string {
	dir := os.Getenv("REBAR_CONTRACTS_DIR")
	if dir == "" {
		dir = RCValue(repoRoot, "contracts_dir")
	}
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "." || filepath.IsAbs(dir) || dir == ".." || strings.HasPrefix(dir, ".."+string(filepath.Separator)) {
		return DefaultContractsDir
	}
	return dir
}

// RCValue returns the value of key in repoRoot/.rebarrc ("" if absent),
// parsed like scripts/_rebar-config.sh: the first `key = value` line wins
// and `#` starts a comment.
func RCValue(repoRoot, key string) string {
	f, err := os.Open(filepath.Join(repoRoot, ".rebarrc"))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimRight(k, " \t") != key {
			continue
		}
		if i := strings.Index(v, "#"); i >= 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	return ""
}
