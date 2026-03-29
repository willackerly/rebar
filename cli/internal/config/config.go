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
}

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
