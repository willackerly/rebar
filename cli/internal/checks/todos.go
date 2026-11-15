package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// check-todos: the two-tag TODO system. A TODO tag in source is untracked
// work and fails; a TRACKED-TASK tag is tracked in TODO.md and only counted.
// Native port of scripts/check-todos.sh.
func init() {
	register(&Check{Name: "check-todos", Run: runTodos})
}

// The tags, spelled so that this file does not trip the check it implements
// (the script is excluded by name; a Go source tree is not).
const (
	todoTag    = "TO" + "DO:"
	trackedTag = "TRACKED" + "-TASK:"
)

// todosExcludedFiles are never scanned: the task list itself and the
// script, by file name.
var todosExcludedFiles = []string{"TODO.md", "check-todos.sh"}

func runTodos(ctx *Context, args []string) int {
	scope := sourcescan.ScopeFor(ctx.Root)
	wd := ctx.WorkDir()

	dirs := args
	if len(dirs) == 0 {
		for _, d := range scope.Dirs {
			p := d
			if !filepath.IsAbs(p) {
				p = filepath.Join(wd, d)
			}
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				dirs = append(dirs, d)
			}
		}
	}
	if len(dirs) == 0 {
		fmt.Fprintln(ctx.Stdout, "No source directories found. Pass directories as arguments.")
		return 0
	}

	filter := sourcescan.Filter{
		Include:     scope.ExtensionGlobs(),
		Exclude:     todosExcludedFiles,
		ExcludeDirs: scope.ExcludeDirs,
	}
	var untracked []string
	tracked := 0
	for _, l := range sourcescan.Search(filter.Files(wd, dirs), func(text string) bool {
		return strings.Contains(text, todoTag) || strings.Contains(text, trackedTag)
	}) {
		if strings.Contains(l.Text, trackedTag) {
			tracked++
			continue
		}
		untracked = append(untracked, l.String())
	}

	if len(untracked) == 0 {
		fmt.Fprintf(ctx.Stdout, "OK: 0 untracked TODOs. %d tracked tasks.\n", tracked)
		return 0
	}
	w := ctx.Stdout
	fmt.Fprintf(w, "FAIL: %d untracked TODO(s) found:\n\n", len(untracked))
	for _, l := range untracked {
		fmt.Fprintln(w, l)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Before committing, you must either:")
	fmt.Fprintln(w, "  1. Fix the TODO immediately (remove it), OR")
	fmt.Fprintf(w, "  2. Add it to TODO.md and convert to: %s description\n", trackedTag)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Also found %d tracked tasks (OK).\n", tracked)
	return 1
}
