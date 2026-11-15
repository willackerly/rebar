package migrate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// workspace is a scratch copy of the project that every probe runs in, so
// a dry run cannot change the project and a copy that writes files is
// caught (its run changes the workspace's status).
//
// In a git work tree it is a `git clone --shared` checked out at the
// project's HEAD (same branch name and commit, so checks that read history
// see the same thing), with the work tree's uncommitted and untracked files,
// and its ignored files (not ignored directories), laid on top. Elsewhere it
// is a plain copy.
type workspace struct {
	project string // the real project
	top     string // the real repo top level ("" outside git)
	clone   string // the scratch repo top level
	root    string // the project inside the scratch repo
	base    string // snapshot right after setup
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = scrubbedEnv()
	out, err := cmd.Output()
	return string(out), err
}

func newWorkspace(project, dir string) (*workspace, error) {
	w := &workspace{project: project, clone: filepath.Join(dir, "work")}
	if top, err := gitOut(project, "rev-parse", "--show-toplevel"); err == nil {
		w.top = strings.TrimSpace(top)
		if rt, err := filepath.EvalSymlinks(w.top); err == nil {
			w.top = rt
		}
	}
	if w.top != "" {
		if _, err := gitOut(w.top, "rev-parse", "--verify", "HEAD"); err != nil {
			w.top = "" // no commits yet: copy instead
		}
	}
	if w.top == "" {
		w.root = w.clone
		if err := copyTree(project, w.root); err != nil {
			return nil, err
		}
	} else {
		rel, err := filepath.Rel(w.top, project)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("project %s is not inside its git top level %s", project, w.top)
		}
		w.root = filepath.Join(w.clone, rel)
		head, _ := gitOut(w.top, "rev-parse", "HEAD")
		branch, _ := gitOut(w.top, "symbolic-ref", "-q", "--short", "HEAD")
		if out, err := exec.Command("git", "clone", "-q", "--shared", "--no-checkout", w.top, w.clone).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("making the scratch copy: %v: %s", err, out)
		}
		args := []string{"checkout", "-q", "--detach", strings.TrimSpace(head)}
		if b := strings.TrimSpace(branch); b != "" {
			args = []string{"checkout", "-q", "-B", b, strings.TrimSpace(head)}
		}
		if out, err := exec.Command("git", append([]string{"-C", w.clone}, args...)...).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("checking out the scratch copy: %v: %s", err, out)
		}
		if err := w.overlay(); err != nil {
			return nil, err
		}
	}
	w.base = w.snapshot()
	return w, nil
}

// overlay copies the work tree's changes, untracked files and ignored files
// onto the scratch checkout, and removes what the work tree deleted.
func (w *workspace) overlay() error {
	var paths []string
	for _, args := range [][]string{
		{"diff", "--name-only", "-z", "HEAD"},
		{"ls-files", "-z", "--others", "--exclude-standard"},
		{"ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory"},
	} {
		out, err := gitOut(w.top, args...)
		if err != nil {
			return fmt.Errorf("listing work tree changes: %v", err)
		}
		for _, p := range strings.Split(out, "\x00") {
			if p != "" && !strings.HasSuffix(p, "/") {
				paths = append(paths, p)
			}
		}
	}
	for _, p := range paths {
		src := filepath.Join(w.top, p)
		dst := filepath.Join(w.clone, p)
		info, err := os.Lstat(src)
		if os.IsNotExist(err) {
			_ = os.Remove(dst)
			continue
		}
		if err != nil {
			return err
		}
		if err := copyEntry(src, dst, info); err != nil {
			return err
		}
	}
	return nil
}

// snapshot fingerprints the workspace: git status (with untracked and
// ignored files) plus size and mtime of every listed path.
func (w *workspace) snapshot() string {
	if w.top == "" {
		return treeFingerprint(w.root)
	}
	out, err := gitOut(w.clone, "status", "--porcelain", "-z", "--untracked-files=all", "--ignored")
	if err != nil {
		return "error: " + err.Error()
	}
	var b strings.Builder
	for _, e := range strings.Split(out, "\x00") {
		if len(e) < 4 {
			continue
		}
		b.WriteString(e)
		if info, err := os.Lstat(filepath.Join(w.clone, e[3:])); err == nil {
			fmt.Fprintf(&b, " %d %d", info.Size(), info.ModTime().UnixNano())
		}
		b.WriteString("\n")
	}
	return b.String()
}

// changed lists what differs from the baseline snapshot.
func (w *workspace) changed(snap string) []string {
	base := map[string]bool{}
	for _, l := range strings.Split(w.base, "\n") {
		base[l] = true
	}
	var out []string
	for _, l := range strings.Split(snap, "\n") {
		if l != "" && !base[l] {
			f := strings.Fields(l)
			if len(f) >= 2 {
				out = append(out, f[1])
			}
		}
	}
	sort.Strings(out)
	return out
}

// restore puts the workspace back to its baseline.
func (w *workspace) restore() error {
	if w.top == "" {
		_ = os.RemoveAll(w.root)
		if err := copyTree(w.project, w.root); err != nil {
			return err
		}
		w.base = w.snapshot()
		return nil
	}
	for _, args := range [][]string{{"checkout", "-q", "-f", "HEAD", "--", "."}, {"clean", "-q", "-f", "-d", "-x"}} {
		if out, err := exec.Command("git", append([]string{"-C", w.clone}, args...)...).CombinedOutput(); err != nil {
			return fmt.Errorf("restoring the scratch copy: %v: %s", err, out)
		}
	}
	if err := w.overlay(); err != nil {
		return err
	}
	w.base = w.snapshot()
	return nil
}

// copyTree copies src to dst, skipping .git, keeping symlinks as symlinks.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		return copyEntry(p, target, info)
	})
}

func copyEntry(src, dst string, info os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	_ = os.Remove(dst)
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(link, dst)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

// treeFingerprint lists every file under root with size and mtime.
func treeFingerprint(root string) string {
	var lines []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		lines = append(lines, fmt.Sprintf("?? %s %d %d", rel, info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// sameBytes reports whether path currently holds data (absent counts as nil).
func sameBytes(path string, data []byte) bool {
	cur, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return data == nil
	}
	return err == nil && bytes.Equal(cur, data)
}
