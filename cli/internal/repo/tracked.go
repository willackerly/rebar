package repo

import (
	"fmt"
	"os/exec"
	"strings"
)

// TrackedFiles returns git-tracked files in repoRoot matching the given
// glob patterns (e.g. "*.go", "*.ts"). Returns relative paths.
func TrackedFiles(repoRoot string, patterns ...string) ([]string, error) {
	args := append([]string{"-C", repoRoot, "ls-files", "--"}, patterns...)
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// TrackedFilesGrep returns git-tracked files matching patterns whose
// contents match the given grep regex. Equivalent to:
//
//	git ls-files <patterns> | xargs grep -lE <regex>
func TrackedFilesGrep(repoRoot, regex string, patterns ...string) ([]string, error) {
	shell := fmt.Sprintf(
		"git -C %q ls-files -- %s | xargs grep -lE %q 2>/dev/null",
		repoRoot, shellGlobs(patterns), regex)
	cmd := exec.Command("bash", "-c", shell)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("git ls-files | grep: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

func shellGlobs(patterns []string) string {
	quoted := make([]string, len(patterns))
	for i, p := range patterns {
		quoted[i] = "'" + p + "'"
	}
	return strings.Join(quoted, " ")
}
