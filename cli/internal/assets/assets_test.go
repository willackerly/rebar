package assets

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot is the rebar checkout holding this package (cli/internal/assets);
// the test skips outside one (a module copy without the source tree).
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, BootstrapDir)); err != nil {
		t.Skip("not in a rebar checkout")
	}
	return root
}

// The embedded copy is the source files byte for byte, nothing more, and
// Mode gives each the source's executable bit. On failure, run
// scripts/sync-bootstrap.sh.
func TestEmbeddedCopyMatchesSource(t *testing.T) {
	root := repoRoot(t)
	want := map[string]bool{}
	for _, src := range Sources {
		err := filepath.WalkDir(filepath.Join(root, src), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			want[rel] = true
			data, _ := os.ReadFile(p)
			got, err := ReadFile(rel)
			switch {
			case err != nil:
				t.Errorf("%s is not embedded (run scripts/sync-bootstrap.sh)", rel)
			case !bytes.Equal(got, data):
				t.Errorf("embedded %s differs from the source (run scripts/sync-bootstrap.sh)", rel)
			}
			info, _ := d.Info()
			if exec := info.Mode()&0o111 != 0; exec != (Mode(rel)&0o111 != 0) {
				t.Errorf("%s: executable=%v in the tree, but Mode gives %v", rel, exec, Mode(rel))
			}
			return nil
		})
		if err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
	_ = fs.WalkDir(FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !want[p] {
			t.Errorf("embedded %s has no source (run scripts/sync-bootstrap.sh)", p)
		}
		return nil
	})
}

// sync-bootstrap.sh mirrors the same list.
func TestSyncScriptListsSources(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "scripts", "sync-bootstrap.sh"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^EMBED_SOURCES="([^"]*)"`).FindSubmatch(data)
	if m == nil {
		t.Fatal("sync-bootstrap.sh has no EMBED_SOURCES line")
	}
	if got, want := string(m[1]), strings.Join(Sources, " "); got != want {
		t.Fatalf("sync-bootstrap.sh EMBED_SOURCES = %q, want %q", got, want)
	}
}

func TestKitAndBootstrap(t *testing.T) {
	kit := strings.Join(Kit(), " ")
	for _, want := range []string{"ask", "ask-agent-loop", "ask-mcp-server", "ask-server", "rebar-install", "completions/_rebar", "completions/rebar.bash"} {
		if !strings.Contains(" "+kit+" ", " "+want+" ") {
			t.Errorf("Kit() = %q lacks %s", kit, want)
		}
	}
	if _, err := fs.ReadFile(Bootstrap(), "scripts/ci-check.sh"); err != nil {
		t.Errorf("bootstrap template: %v", err)
	}
	if _, err := fs.ReadFile(Bootstrap(), ".claude/settings.json"); err != nil {
		t.Errorf("bootstrap dotfiles not embedded: %v", err)
	}
	if !bytes.Contains(ContractTemplate(), []byte("CONTRACT")) {
		t.Error("ContractTemplate() looks wrong")
	}
}
