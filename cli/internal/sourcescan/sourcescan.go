// Package sourcescan selects and searches a project's source files the way
// rebar's source scanners (check-todos, check-contract-refs, check-registry,
// compute-registry) do, without shelling out to grep.
//
// The scripts select files with `grep -r --include=*.<ext> --exclude=<name>
// --exclude-dir=<name>`; Filter reproduces that selection and Search its
// `-nI` output (path:line:text). Where grep implementations disagree, this
// package picks one behaviour on every platform (see Filter, Filter.Files
// and Search) instead of inheriting the host's grep.
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

// Filter selects files like grep's --include, --exclude and --exclude-dir.
// Patterns are fnmatch(3) globs, as grep reads them (see fnmatch): `[!x]`
// negates, POSIX classes work, `*` also matches `/`.
//
// A file or directory met while walking is matched by its name. An operand
// is matched by its path as given and by every tail of it that follows a
// `/` (for ./src/gen: src/gen and gen). Trailing slashes on ExcludeDirs
// patterns are ignored. This is GNU grep's --exclude-dir; BSD grep (macOS)
// differs in three ways, where this package follows GNU:
//   - below an operand it also matches the path from the operand, `*`
//     crossing `/`: `src/gen` or `*/gen` skips src/gen while walking src;
//   - it matches an operand only by its whole path or its last component
//     (`src/gen` does not exclude the operand ./src/gen);
//   - it keeps a pattern's trailing slash, so `gen/` matches nothing.
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
// operand that is a symlink to a directory is followed, as GNU grep -r
// does; BSD grep -r (macOS) skips it, so a symlinked source directory was
// not scanned there.
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
		if f.excludesOperand(op) {
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

// excludesDir reports whether a directory met while walking, by name, is
// excluded.
func (f Filter) excludesDir(name string) bool {
	return slices.ContainsFunc(f.ExcludeDirs, func(p string) bool {
		return fnmatch(dirPattern(p), name)
	})
}

// excludesOperand reports whether a directory operand is excluded: some
// ExcludeDirs pattern matches its path as given (cleaned) or a tail of it
// after a `/`. "." is never excluded.
func (f Filter) excludesOperand(op string) bool {
	p := filepath.ToSlash(filepath.Clean(op))
	if p == "." {
		return false
	}
	tails := []string{p}
	for i := 0; i+1 < len(p); i++ {
		if p[i] == '/' && p[i+1] != '/' {
			tails = append(tails, p[i+1:])
		}
	}
	return slices.ContainsFunc(f.ExcludeDirs, func(pat string) bool {
		return slices.ContainsFunc(tails, func(tail string) bool { return fnmatch(dirPattern(pat), tail) })
	})
}

// dirPattern drops an --exclude-dir pattern's trailing slashes.
func dirPattern(p string) string {
	return trimTrailingSlashes(p)
}

func matchAny(patterns []string, name string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool { return fnmatch(p, name) })
}

// trimTrailingSlashes drops trailing slashes from an operand ("src/" →
// "src"), keeping a lone "/".
func trimTrailingSlashes(op string) string {
	for len(op) > 1 && strings.HasSuffix(op, "/") {
		op = op[:len(op)-1]
	}
	return op
}

// BinaryBlock is how much of a file BSD grep -I inspects for a NUL byte
// when it decides whether the file is binary (its first buffer).
const BinaryBlock = 32 << 10

// TextPart returns the part of a file's contents that is searched: nothing
// when its first BinaryBlock bytes hold a NUL (a binary file, which grep -I
// skips), else all of it.
//
// That is BSD grep's rule. GNU grep reads a larger first buffer (more than
// 70 KiB on ubuntu-latest), skipping the whole file when that buffer holds
// a NUL, and stops searching at a NUL further in; lines it so misses are
// reported here.
func TextPart(data []byte) []byte {
	head := data
	if len(head) > BinaryBlock {
		head = head[:BinaryBlock]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return nil
	}
	return data
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
// UTF-8. Binary files are skipped (see TextPart).
// Unreadable files are skipped.
func Search(files []File, match func(text string) bool) []Line {
	var lines []Line
	for _, f := range files {
		data, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		data = TextPart(data)
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
