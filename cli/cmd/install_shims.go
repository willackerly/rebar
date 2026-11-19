package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/assets"
)

var (
	shimsDir   string
	shimsForce bool
)

var installShimsCmd = &cobra.Command{
	Use:   "install-shims",
	Short: "Write the ask shims, rebar-install and shell completions beside rebar",
	Long: `Writes the companions a rebar install keeps next to the binary, compiled
into it: the ask, ask-agent-loop, ask-mcp-server and ask-server shims (they
run this binary; adopters' PATH ` + "`ask`" + ` and .mcp.json files name them),
rebar-install (puts the directory on PATH and wires the completions) and
completions/. An install file left there by rebar before v3.1.2 (now
rebar-install: a file named install on PATH shadows install(1)) is removed.

setup-rebar.sh runs it for a binary release. After downloading the binary by
hand (or go install), run it once:

  rebar install-shims                 # into the directory holding rebar
  rebar install-shims --dir ~/bin     # elsewhere (the shims find rebar on PATH)

A file that exists with other content (or is a symlink) is left alone
unless --force, which replaces it (a symlink itself, never its target).`,
	Args: cobra.NoArgs,
	RunE: runInstallShims,
}

func init() {
	installShimsCmd.Flags().StringVar(&shimsDir, "dir", "", "target directory (default: the directory holding this rebar binary)")
	installShimsCmd.Flags().BoolVar(&shimsForce, "force", false, "overwrite files that differ")
}

func runInstallShims(cmd *cobra.Command, args []string) error {
	dir := shimsDir
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("cannot locate the rebar binary (%v); pass --dir", err)
		}
		dir = filepath.Dir(exe)
	}
	written, conflicts, err := installShims(dir, shimsForce)
	for _, f := range written {
		fmt.Fprintf(cmd.OutOrStdout(), "  wrote %s\n", filepath.Join(dir, f))
	}
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("left %d file(s) in %s that differ from this rebar's: %s (--force overwrites)", len(conflicts), dir, strings.Join(conflicts, ", "))
	}
	if len(written) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "rebar install-shims: %s is up to date\n", dir)
	}
	return nil
}

// legacyInstallMarker is the second line of bin/install as rebar shipped
// it up to v3.1.x (renamed bin/rebar-install: a file named install on PATH
// shadows install(1)).
const legacyInstallMarker = "# install — Add rebar's bin/ to PATH"

// installShims writes assets.Kit() into dir: files already identical are
// skipped, differing ones (or symlinks) are reported as conflicts unless
// force. Each file is written to a new temporary file (os.CreateTemp, so a
// planted name or symlink is never followed) and renamed into place, so a
// running shim is never rewritten in place and a symlink at the target is
// replaced, not written through. A bin/install left by an older rebar is
// removed (removed lists it).
func installShims(dir string, force bool) (written, conflicts []string, err error) {
	for _, rel := range assets.Kit() {
		src := path.Join("bin", rel)
		data, err := assets.ReadFile(src)
		if err != nil {
			return written, conflicts, err
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if info, err := os.Lstat(dst); err == nil {
			if info.Mode().IsRegular() {
				if cur, err := os.ReadFile(dst); err == nil && bytes.Equal(cur, data) {
					continue
				}
			}
			if !force {
				conflicts = append(conflicts, rel)
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return written, conflicts, err
		}
		if err := writeReplacing(dst, data, assets.Mode(src)); err != nil {
			return written, conflicts, err
		}
		written = append(written, rel)
	}
	if removeLegacyInstall(dir) {
		written = append(written, "install (removed: renamed rebar-install)")
	}
	return written, conflicts, nil
}

// writeReplacing writes data to a fresh temporary file beside dst and
// renames it over dst.
func writeReplacing(dst string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".rebar-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp, mode) // CreateTemp makes it 0600
	}
	if werr == nil {
		werr = os.Rename(tmp, dst)
	}
	if werr != nil {
		os.Remove(tmp)
	}
	return werr
}

// removeLegacyInstall deletes dir/install when it is rebar's own old
// bin/install (a regular file with its header) and rebar-install is there.
func removeLegacyInstall(dir string) bool {
	old := filepath.Join(dir, "install")
	info, err := os.Lstat(old)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "rebar-install")); err != nil {
		return false
	}
	data, err := os.ReadFile(old)
	if err != nil {
		return false
	}
	lines := strings.SplitN(string(data), "\n", 3)
	if len(lines) < 2 || !strings.HasPrefix(lines[1], legacyInstallMarker) {
		return false
	}
	return os.Remove(old) == nil
}
