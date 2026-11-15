// Package sourcescan selects and searches a project's source files the way
// rebar's source scanners (check-todos, check-contract-refs, check-registry,
// compute-registry) do, without shelling out to grep.
//
// The scripts select files with `grep -r --include=*.<ext> --exclude=<name>
// --exclude-dir=<name>`; Filter reproduces that selection and Search its
// `-n` output (path:line:text). Where grep implementations disagree, this
// package picks one behaviour on every platform (see Filter.Files and
// Search) instead of inheriting the host's grep.
package sourcescan

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/willackerly/rebar/cli/internal/config"
)

// Defaults for the .rebarrc source scope keys, as in the scripts.
const (
	DefaultSourceDirs        = "src internal cmd client packages lib app"
	DefaultSourceExtensions  = "go ts tsx js py rs jsx"
	DefaultSourceExcludeDirs = "node_modules vendor dist"
)

// alwaysExcluded are skipped whatever source_exclude_dirs says: git
// metadata, and agent worktrees under .claude/ that hold stale copies of
// the source (conventions.md).
var alwaysExcluded = []string{".git", ".claude"}

// Scope is a project's configured source tree.
type Scope struct {
	// Dirs are the source directories (.rebarrc source_dirs), relative to
	// the project root and not checked for existence.
	Dirs []string
	// Extensions are the file extensions to scan (.rebarrc
	// source_extensions), without a leading dot.
	Extensions []string
	// ExcludeDirs are directory names skipped anywhere in the tree: .rebarrc
	// source_exclude_dirs, then .git and .claude.
	ExcludeDirs []string
}

// ScopeFor reads the source scope of the project at root. Mirrors
// check-todos.sh and _rebar_source_exclude_dirs: an absent or empty key
// takes the default, and none of the keys has an environment override.
func ScopeFor(root string) Scope {
	var s Scope
	s.Dirs = strings.Fields(config.RCValueOr(root, "source_dirs", "", DefaultSourceDirs))
	for _, ext := range strings.Fields(config.RCValueOr(root, "source_extensions", "", DefaultSourceExtensions)) {
		s.Extensions = append(s.Extensions, strings.TrimPrefix(ext, "."))
	}
	s.ExcludeDirs = append(strings.Fields(config.RCValueOr(root, "source_exclude_dirs", "", DefaultSourceExcludeDirs)), alwaysExcluded...)
	return s
}

// ExtensionGlobs returns the --include patterns for the scope's extensions.
func (s Scope) ExtensionGlobs() []string {
	globs := make([]string, 0, len(s.Extensions))
	for _, ext := range s.Extensions {
		globs = append(globs, "*."+ext)
	}
	return globs
}

// Filter selects files by base name, like grep's --include, --exclude and
// --exclude-dir. Patterns are shell globs (path.Match).
type Filter struct {
	// Include, when non-empty, admits only files matching one of them.
	Include []string
	// Exclude drops matching files, even ones Include admits.
	Exclude []string
	// ExcludeDirs drops matching directories, operands included.
	ExcludeDirs []string
}

// File is one selected file.
type File struct {
	// Display is the path grep would print: the operand as given, then the
	// path below it joined with "/".
	Display string
	// Path is the file's location on disk.
	Path string
}

// Files returns the files under operands (files or directories, relative
// to base unless absolute) that f selects: directories in lexical order,
// operands in the order given. Missing operands are skipped silently, as
// grep 2>/dev/null does.
//
// Symbolic links met while walking are skipped, as grep -r does. An
// operand that is a symlink is followed, as GNU grep does (BSD grep skips
// it).
func (f Filter) Files(base string, operands []string) []File {
	var files []File
	for _, op := range operands {
		display := trimTrailingSlashes(op)
		p := op
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if f.admitsFile(path.Base(filepath.ToSlash(display))) {
				files = append(files, File{Display: display, Path: p})
			}
			continue
		}
		// grep matches an operand by the name it was given ("." never).
		if f.excludesDir(filepath.Base(filepath.Clean(op))) {
			continue
		}
		files = append(files, f.walk(p, display)...)
	}
	return files
}

func (f Filter) walk(dir, display string) []File {
	// WalkDir does not descend into a symlinked root.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	var files []File
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p == dir {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if f.excludesDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !f.admitsFile(d.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		files = append(files, File{Display: display + "/" + filepath.ToSlash(rel), Path: p})
		return nil
	})
	return files
}

// admitsFile applies Include and Exclude to a base name. With both given,
// an excluded name stays out (grep: the last matching option wins, and the
// scripts list --include first).
func (f Filter) admitsFile(name string) bool {
	if matchAny(f.Exclude, name) {
		return false
	}
	return len(f.Include) == 0 || matchAny(f.Include, name)
}

func (f Filter) excludesDir(name string) bool {
	return matchAny(f.ExcludeDirs, name)
}

func matchAny(patterns []string, name string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool {
		ok, err := path.Match(p, name)
		return err == nil && ok
	})
}

// trimTrailingSlashes drops trailing slashes from an operand ("src/" →
// "src"), keeping a lone "/".
func trimTrailingSlashes(op string) string {
	for len(op) > 1 && strings.HasSuffix(op, "/") {
		op = op[:len(op)-1]
	}
	return op
}

// Line is a matching line of a file.
type Line struct {
	File File
	// Number is 1-based.
	Number int
	// Text is the line without its "\n"; a "\r" before it is kept, as grep
	// prints it.
	Text string
}

// String formats the line as grep -n does: path:line:text.
func (l Line) String() string {
	return l.File.Display + ":" + strconv.Itoa(l.Number) + ":" + l.Text
}

// Search returns the lines of files for which match is true, in file
// order. Matching is on bytes, independent of the locale: grep under a
// UTF-8 locale skips lines (BSD) or whole files (GNU) that are not valid
// UTF-8. Binary files, those containing a NUL byte, are skipped, as GNU
// grep does (BSD grep prints "Binary file ... matches" instead).
// Unreadable files are skipped.
func Search(files []File, match func(text string) bool) []Line {
	var lines []Line
	for _, f := range files {
		data, err := os.ReadFile(f.Path)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		n := 0
		for len(data) > 0 {
			n++
			var line []byte
			if i := bytes.IndexByte(data, '\n'); i >= 0 {
				line, data = data[:i], data[i+1:]
			} else {
				line, data = data, nil
			}
			if text := string(line); match(text) {
				lines = append(lines, Line{File: f, Number: n, Text: text})
			}
		}
	}
	return lines
}
