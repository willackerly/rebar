// Package hooks manages git hook installation and merging for rebar.
//
// Architecture: CONTRACT:S2-ASK-CLI.1.0
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallPreCommit installs .git/hooks/pre-commit, merging with existing
// hook if present.
//
// strategy: "symlink" (default) creates symlink to framework's pre-commit.sh
//           "merge" prepends rebar shim to existing hook with markers
//           "force" overwrites existing hook
func InstallPreCommit(repoRoot, frameworkDir, strategy string) error {
	hookPath := filepath.Join(repoRoot, ".git", "hooks", "pre-commit")
	frameworkScript := filepath.Join(frameworkDir, "scripts", "pre-commit.sh")

	if _, err := os.Stat(frameworkScript); os.IsNotExist(err) {
		return fmt.Errorf("framework pre-commit.sh not found at %s", frameworkScript)
	}

	hookType, err := DetectExistingHook(repoRoot)
	if err != nil {
		return err
	}

	switch hookType {
	case "none":
		relPath, _ := filepath.Rel(filepath.Dir(hookPath), frameworkScript)
		if err := os.Symlink(relPath, hookPath); err != nil {
			return fmt.Errorf("creating symlink: %w", err)
		}
		return nil

	case "rebar-symlink":
		return nil

	case "custom":
		if strategy == "force" {
			os.Remove(hookPath)
			relPath, _ := filepath.Rel(filepath.Dir(hookPath), frameworkScript)
			return os.Symlink(relPath, hookPath)
		}
		return prependRebarShim(hookPath, frameworkScript)

	case "husky", "pre-commit-py":
		return injectRebarShim(hookPath, frameworkScript)

	case "other-symlink":
		return fmt.Errorf("existing symlink points elsewhere; re-run with --force to replace")
	}
	return nil
}

// DetectExistingHook inspects .git/hooks/pre-commit and returns its type.
// Returns: "none", "rebar-symlink", "other-symlink", "husky", "pre-commit-py", "custom"
func DetectExistingHook(repoRoot string) (string, error) {
	hookPath := filepath.Join(repoRoot, ".git", "hooks", "pre-commit")
	info, err := os.Lstat(hookPath)
	if os.IsNotExist(err) {
		return "none", nil
	}
	if err != nil {
		return "", fmt.Errorf("stat hook: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(hookPath)
		if strings.Contains(target, "scripts/pre-commit.sh") {
			return "rebar-symlink", nil
		}
		return "other-symlink", nil
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		return "", err
	}
	content := string(data)

	if strings.Contains(content, "# === REBAR ===") {
		return "rebar-symlink", nil
	}
	if strings.Contains(content, "husky") || strings.Contains(content, ".husky/") {
		return "husky", nil
	}
	if strings.Contains(content, "pre-commit run") || strings.Contains(content, "pre_commit") {
		return "pre-commit-py", nil
	}
	return "custom", nil
}

// prependRebarShim adds rebar enforcement delegation to an existing hook.
func prependRebarShim(hookPath, frameworkScript string) error {
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return err
	}
	existing := string(data)

	if strings.Contains(existing, "# === REBAR ===") {
		return nil
	}

	var shebang string
	if strings.HasPrefix(existing, "#!") {
		idx := strings.Index(existing, "\n")
		if idx > 0 {
			shebang = existing[:idx+1]
			existing = existing[idx+1:]
		}
	}
	if shebang == "" {
		shebang = "#!/bin/bash\n"
	}

	merged := shebang
	merged += "# === REBAR === (auto-managed)\n"
	merged += fmt.Sprintf("%q || exit 1\n", frameworkScript)
	merged += "# === END REBAR ===\n\n"
	merged += existing

	return os.WriteFile(hookPath, []byte(merged), 0755)
}

// injectRebarShim inserts rebar delegation into framework-managed hooks
// (husky, pre-commit-py). Injects after shebang, before framework invocation.
func injectRebarShim(hookPath, frameworkScript string) error {
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return err
	}
	content := string(data)

	if strings.Contains(content, "# === REBAR ===") {
		return nil
	}

	lines := strings.Split(content, "\n")
	var out []string
	injected := false

	for i, line := range lines {
		out = append(out, line)
		if i == 0 && strings.HasPrefix(line, "#!") {
			out = append(out, "# === REBAR === (auto-managed)")
			out = append(out, fmt.Sprintf("%q || exit 1", frameworkScript))
			out = append(out, "# === END REBAR ===")
			out = append(out, "")
			injected = true
		}
	}

	if !injected {
		return fmt.Errorf("could not find injection point in %s", hookPath)
	}

	return os.WriteFile(hookPath, []byte(strings.Join(out, "\n")), 0755)
}
