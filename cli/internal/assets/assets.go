// Package assets holds the files of the rebar source tree that the binary
// writes out, so that it needs no install tree: the project bootstrap
// template (`rebar new`, `rebar init`, `rebar adopt`, `rebar migrate`) and
// the companions an install keeps beside the binary (`rebar install-shims`:
// the bin/ask* shims, bin/install and the shell completions).
//
// go:embed cannot reach outside the module (cli/), so files/ is a copy of
// Sources, kept in step by scripts/sync-bootstrap.sh: run it after editing
// a source; `sync-bootstrap.sh --check` (in ci-check) and
// TestEmbeddedCopyMatchesSource fail while the copy differs. Each file is
// stored as <path>.embed so that rebar's own scanners (freshness markers,
// doc links, contract files) and git (the template's .gitignore files) do
// not treat the copies as live files; FS strips the suffix.
package assets

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

//go:embed all:files
var files embed.FS

// Sources are the paths, relative to the rebar repo root, mirrored under
// files/: directories recursively. scripts/sync-bootstrap.sh has the same
// list (EMBED_SOURCES).
var Sources = []string{
	"templates/project-bootstrap",
	"bin/ask",
	"bin/ask-agent-loop",
	"bin/ask-mcp-server",
	"bin/ask-server",
	"bin/install",
	"bin/completions",
}

// BootstrapDir is the bootstrap template's path in FS.
const BootstrapDir = "templates/project-bootstrap"

// FS is the embedded tree, paths as in the rebar repo
// (templates/project-bootstrap/scripts/ci-check.sh, bin/ask, …).
func FS() fs.FS { return embedFS{} }

const suffix = ".embed"

// embedFS serves files/<name>.embed as <name>; directories as they are.
type embedFS struct{}

func (embedFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if f, err := files.Open(path.Join("files", name) + suffix); err == nil {
		return f, nil
	}
	f, err := files.Open(path.Join("files", name))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if st, err := f.Stat(); err != nil || !st.IsDir() {
		f.Close() // a stored name without the suffix is not a file of the tree
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return f, nil
}

func (embedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	entries, err := files.ReadDir(path.Join("files", name))
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	out := make([]fs.DirEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e)
		} else if n, ok := strings.CutSuffix(e.Name(), suffix); ok {
			out = append(out, entry{e, n})
		}
	}
	return out, nil
}

// entry is a stored file's DirEntry under its tree name.
type entry struct {
	fs.DirEntry
	name string
}

func (e entry) Name() string { return e.name }

// Bootstrap is the project bootstrap template, rooted at its top
// (scripts/ci-check.sh, .claude/settings.json, …).
func Bootstrap() fs.FS {
	sub, err := fs.Sub(FS(), BootstrapDir)
	if err != nil {
		panic(err)
	}
	return sub
}

// ReadFile returns an embedded file by its repo path.
func ReadFile(name string) ([]byte, error) { return fs.ReadFile(FS(), name) }

// ContractTemplate is architecture/CONTRACT-TEMPLATE.md (the template's copy,
// which sync-bootstrap.sh keeps identical to the repo root's).
func ContractTemplate() []byte {
	data, err := ReadFile(BootstrapDir + "/architecture/CONTRACT-TEMPLATE.md")
	if err != nil {
		panic(err)
	}
	return data
}

// Mode is the file mode a written-out copy of name (a repo path) gets:
// executable for shell scripts and the bin/ tools, else 0644. embed keeps
// no modes; TestEmbeddedCopyMatchesSource holds this rule to the source
// tree's modes.
func Mode(name string) fs.FileMode {
	if strings.HasSuffix(name, ".sh") {
		return 0o755
	}
	if dir := path.Dir(name); dir == "bin" {
		return 0o755
	}
	return 0o644
}

// Kit lists the files `rebar install-shims` writes beside the binary,
// relative to that directory (ask, completions/_ask, …), sorted.
func Kit() []string {
	var out []string
	_ = fs.WalkDir(FS(), "bin", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, strings.TrimPrefix(p, "bin/"))
		}
		return nil
	})
	return out
}
