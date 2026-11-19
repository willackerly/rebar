package cmd

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/willackerly/rebar/cli/internal/assets"
)

// copyBootstrap copies the files under dir of the bootstrap template
// compiled into the binary (templates/project-bootstrap/<dir>; "." for all)
// to dst/<dir>, keeping the template's layout. A file that already exists
// is left alone. It returns the copied paths, relative to dst, in walk
// order.
func copyBootstrap(dir, dst string) []string {
	var copied []string
	tmpl := assets.Bootstrap()
	_ = fs.WalkDir(tmpl, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if _, err := os.Lstat(target); err == nil {
			return nil // never overwrite
		}
		data, err := fs.ReadFile(tmpl, p)
		if err != nil {
			return nil
		}
		if os.MkdirAll(filepath.Dir(target), 0o755) != nil {
			return nil
		}
		if os.WriteFile(target, data, assets.Mode(path.Join(assets.BootstrapDir, p))) == nil {
			copied = append(copied, p)
		}
		return nil
	})
	return copied
}

// writeContractTemplate writes architecture/CONTRACT-TEMPLATE.md (compiled
// into the binary) into archDir unless the file exists.
func writeContractTemplate(archDir string) bool {
	dst := filepath.Join(archDir, "CONTRACT-TEMPLATE.md")
	if _, err := os.Lstat(dst); err == nil {
		return false
	}
	return os.WriteFile(dst, assets.ContractTemplate(), 0o644) == nil
}
