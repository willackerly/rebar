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
	Short: "Write the ask shims, bin/install and shell completions beside rebar",
	Long: `Writes the companions a rebar install keeps next to the binary, compiled
into it: the ask, ask-agent-loop, ask-mcp-server and ask-server shims (they
run this binary; adopters' PATH ` + "`ask`" + ` and .mcp.json files name them),
bin/install (puts the directory on PATH and wires the completions) and
completions/.

setup-rebar.sh runs it for a binary release. After downloading the binary by
hand (or go install), run it once:

  rebar install-shims                 # into the directory holding rebar
  rebar install-shims --dir ~/bin     # elsewhere (the shims find rebar on PATH)

A file that exists with other content is left alone unless --force.`,
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

// installShims writes assets.Kit() into dir: files already identical are
// skipped, differing ones are reported as conflicts unless force. Each file
// is written to a temporary name and renamed into place, so a running shim
// is never rewritten in place.
func installShims(dir string, force bool) (written, conflicts []string, err error) {
	for _, rel := range assets.Kit() {
		src := path.Join("bin", rel)
		data, err := assets.ReadFile(src)
		if err != nil {
			return written, conflicts, err
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if cur, err := os.ReadFile(dst); err == nil {
			if bytes.Equal(cur, data) {
				continue
			}
			if !force {
				conflicts = append(conflicts, rel)
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return written, conflicts, err
		}
		tmp := dst + ".rebar-tmp"
		if err := os.WriteFile(tmp, data, assets.Mode(src)); err != nil {
			return written, conflicts, err
		}
		if err := os.Chmod(tmp, assets.Mode(src)); err != nil { // umask
			os.Remove(tmp)
			return written, conflicts, err
		}
		if err := os.Rename(tmp, dst); err != nil {
			os.Remove(tmp)
			return written, conflicts, err
		}
		written = append(written, rel)
	}
	return written, conflicts, nil
}
