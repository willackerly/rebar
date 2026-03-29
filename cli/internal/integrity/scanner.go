package integrity

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultRoleForCategory returns the expected role for a file category.
func DefaultRoleForCategory(category string) string {
	switch category {
	case CategoryEnforcement:
		return "steward"
	case CategoryContracts:
		return "architect"
	case CategoryTests:
		return "tester"
	default:
		return "developer"
	}
}

// ScanProtectedFiles discovers all protected files in the repo, grouped by category.
// Returns relative paths (relative to repoRoot).
func ScanProtectedFiles(repoRoot string) (map[string][]string, error) {
	result := map[string][]string{
		CategoryEnforcement: {},
		CategoryContracts:   {},
		CategoryTests:       {},
	}

	// Enforcement: scripts/*.sh
	scripts, err := filepath.Glob(filepath.Join(repoRoot, "scripts", "*.sh"))
	if err != nil {
		return nil, err
	}
	for _, s := range scripts {
		rel, _ := filepath.Rel(repoRoot, s)
		result[CategoryEnforcement] = append(result[CategoryEnforcement], rel)
	}

	// Contracts: architecture/CONTRACT-*.md (exclude templates and .impl.md)
	contracts, err := filepath.Glob(filepath.Join(repoRoot, "architecture", "CONTRACT-*.md"))
	if err != nil {
		return nil, err
	}
	for _, c := range contracts {
		base := filepath.Base(c)
		if base == "CONTRACT-TEMPLATE.md" || base == "CONTRACT-REGISTRY.template.md" {
			continue
		}
		if strings.HasSuffix(base, ".impl.md") {
			continue
		}
		rel, _ := filepath.Rel(repoRoot, c)
		result[CategoryContracts] = append(result[CategoryContracts], rel)
	}

	// Tests: find test files recursively
	// Patterns: *.test.*, *.spec.*, *_test.go
	testDirs := []string{"tests", "test", "src", "internal", "cmd", "client", "packages", "lib", "app"}
	seen := map[string]bool{}

	for _, dir := range testDirs {
		base := filepath.Join(repoRoot, dir)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name == "node_modules" || name == "vendor" || name == "dist" || name == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if isTestFile(info.Name()) {
				rel, _ := filepath.Rel(repoRoot, path)
				if !seen[rel] {
					seen[rel] = true
					result[CategoryTests] = append(result[CategoryTests], rel)
				}
			}
			return nil
		})
	}

	return result, nil
}

func isTestFile(name string) bool {
	// *.test.ts, *.test.js, *.test.tsx, *.spec.ts, etc.
	if strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") {
		return true
	}
	// *_test.go
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	// *_test.py
	if strings.HasSuffix(name, "_test.py") || strings.HasPrefix(name, "test_") {
		return true
	}
	return false
}

// Patterns that count as assertions in test files.
var assertionPatterns = regexp.MustCompile(
	`(?i)\b(assert|expect|should|it\s*\(|test\s*\(|describe\s*\(|check\s*\()\b`,
)

// CountAssertions counts assertion-like patterns in a test file.
// This is a heuristic — it counts lines containing assertion keywords.
func CountAssertions(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if assertionPatterns.MatchString(line) {
			count++
		}
	}
	return count, scanner.Err()
}
