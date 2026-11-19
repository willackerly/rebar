package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/migrate"
)

var hookScriptRe = regexp.MustCompile(`scripts/([A-Za-z0-9_-]+\.sh)\b`)

// hookCommand is what a git hook should run instead of the deleted script
// name: the built-in suite or check.
func hookCommand(name string) string {
	switch name {
	case "pre-commit.sh":
		return "rebar check --pre-commit"
	case "ci-check.sh":
		return "rebar check"
	case "cold-start-checks.sh":
		return "rebar check --cold-start"
	}
	if _, ok := checks.Lookup(name); ok {
		return "rebar run " + strings.TrimSuffix(name, ".sh") + ` "$@"`
	}
	return ""
}

// warnBrokenHooks tells the user, on w, about git hooks of the project at
// root that stopped checking anything: a symlink whose target is gone (git
// skips such a hook without a word), a hook file that calls a
// scripts/<check>.sh that is gone, and a hook that runs the project's copy
// of a suite which would skip rebar's checks (decideLegacySuite).
func warnBrokenHooks(w io.Writer, root string) {
	for _, dir := range migrate.HookDirs(root) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasSuffix(e.Name(), ".sample") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			label := path
			if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
				label = rel
			}
			if e.Type()&os.ModeSymlink != 0 {
				target, _ := os.Readlink(path)
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil {
					fix := "restore it or remove the hook"
					if c := hookCommand(filepath.Base(target)); c != "" {
						fix = fmt.Sprintf("replace it: printf '#!/bin/sh\\nexec %s\\n' > %s && chmod +x %s", c, label, label)
					}
					fmt.Fprintf(w, "rebar: git hook %s links to %s, which does not exist, so git skips the hook without a word; %s\n", label, target, fix)
					continue
				}
				warnHookRunsLegacySuite(w, root, label, resolved)
				continue
			}
			info, err := e.Info()
			if err != nil || !info.Mode().IsRegular() || info.Size() > 512*1024 {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			seen := map[string]bool{}
			for _, line := range strings.Split(string(data), "\n") {
				if f := strings.Fields(line); len(f) == 0 || strings.HasPrefix(f[0], "#") || f[0] == "echo" || f[0] == "printf" {
					continue
				}
				for _, m := range hookScriptRe.FindAllStringSubmatch(line, -1) {
					name := m[1]
					if seen[name] || hookCommand(name) == "" || isFile(filepath.Join(root, "scripts", name)) {
						continue
					}
					seen[name] = true
					fmt.Fprintf(w, "rebar: git hook %s calls scripts/%s, which does not exist, so the hook may skip it and pass; have it run `%s` instead\n",
						label, name, strings.TrimSuffix(hookCommand(name), ` "$@"`))
				}
			}
		}
	}
}

// warnHookRunsLegacySuite warns when the hook at label is a link to the
// project's copy of a suite that would skip rebar's checks: git runs the
// copy itself, without rebar.
func warnHookRunsLegacySuite(w io.Writer, root, label, target string) {
	scriptsDir, err := filepath.EvalSymlinks(filepath.Join(root, "scripts"))
	if err != nil || filepath.Dir(target) != scriptsDir {
		return
	}
	s, ok := resolveScript(root, filepath.Base(target))
	if !ok || !s.Legacy || !legacySuites[filepath.Base(target)] {
		return
	}
	if missing := missingSiblings(root, s.Path); len(missing) > 0 {
		fmt.Fprintf(w, "rebar: git hook %s runs the project's own scripts/%s, which skips scripts/%s (missing or not executable) and passes; run `rebar migrate`, or point the hook at `%s`\n",
			label, filepath.Base(target), strings.Join(missing, ", scripts/"), hookCommand(filepath.Base(target)))
	}
}
