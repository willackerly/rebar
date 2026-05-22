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
	RepoRoot          string
	RebarDir          string // .rebar/ in project
	RebarFrameworkDir string // framework install (versioned, e.g. ~/.rebar/versions/v3.1.0)
	Tier              int    // 1, 2, or 3
	Version           string // from .rebar-version
	ScriptsDir        string // framework scripts/ dir
	AgentsDir         string // agents/ in project
	BinDir            string // bin/ in framework
	ContractNamespace string // e.g. github.com/willackerly/rebar; empty = legacy/unmigrated
}

// Load reads configuration from .rebarrc and .rebar-version, respecting
// REBAR_TIER env override. Mirrors the logic in scripts/_rebar-config.sh.
func Load(repoRoot string) (*Config, error) {
	// Read version first to resolve framework dir
	versionBytes, _ := os.ReadFile(filepath.Join(repoRoot, ".rebar-version"))
	version := strings.TrimSpace(string(versionBytes))

	frameworkDir, err := findRebarDir(version)
	if err != nil {
		return nil, fmt.Errorf("finding rebar framework: %w", err)
	}

	c := &Config{
		RepoRoot:          repoRoot,
		RebarDir:          filepath.Join(repoRoot, ".rebar"),
		RebarFrameworkDir: frameworkDir,
		Version:           version,
		ScriptsDir:        filepath.Join(frameworkDir, "scripts"),
		AgentsDir:         filepath.Join(repoRoot, "agents"),
		BinDir:            filepath.Join(frameworkDir, "bin"),
		Tier:              3, // default: full enforcement
	}

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

	// Contract namespace (Go-module form, e.g. github.com/owner/repo).
	// Sourced from .rebarrc; REBAR_CONTRACT_NAMESPACE env var overrides.
	if env := strings.TrimSpace(os.Getenv("REBAR_CONTRACT_NAMESPACE")); env != "" {
		c.ContractNamespace = env
	} else {
		c.ContractNamespace = readRebarRCString(filepath.Join(repoRoot, ".rebarrc"), "contract_namespace")
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

// readRebarRCString reads a string-valued key from .rebarrc. Returns
// empty string if the file is missing, the key is absent, or the value
// is blank. Matches keys case-insensitively.
func readRebarRCString(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
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
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// findRebarDir locates the rebar framework installation directory.
// Precedence: REBAR_DIR env → ~/.rebar/versions/<version>/ → ~/.rebar/current/
func findRebarDir(version string) (string, error) {
	// 1. REBAR_DIR env (explicit override for testing/CI)
	if env := os.Getenv("REBAR_DIR"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}

	// 2. Versioned install (~/.rebar/versions/<version>/)
	if version != "" {
		versioned := filepath.Join(home, ".rebar", "versions", version)
		if _, err := os.Stat(versioned); err == nil {
			return versioned, nil
		}
	}

	// 3. Current symlink (~/.rebar/current/)
	current := filepath.Join(home, ".rebar", "current")
	if target, err := os.Readlink(current); err == nil {
		abs := filepath.Join(home, ".rebar", target)
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}

	// 4. Legacy single install (~/.rebar/)
	legacy := filepath.Join(home, ".rebar")
	if _, err := os.Stat(filepath.Join(legacy, "bin", "rebar")); err == nil {
		return legacy, nil
	}

	return "", fmt.Errorf("rebar framework not found — run setup-rebar.sh or set REBAR_DIR")
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
