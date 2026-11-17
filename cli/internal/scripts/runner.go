// Package scripts runs a project's own scripts: legacy copies of rebar's
// scripts (which rebar no longer ships; they keep working with a
// deprecation notice) and other project scripts `rebar run` finds.
package scripts

import (
	"os"
	"os/exec"
)

// projectCommand builds a bash invocation of scriptPath that checks
// projectRoot: it runs there, with REBAR_PROJECT_ROOT set, as `rebar
// check` and `rebar run` have always run scripts.
func projectCommand(scriptPath, projectRoot string, args ...string) *exec.Cmd {
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "REBAR_PROJECT_ROOT="+projectRoot)
	return cmd
}

// RunInProject runs scriptPath against projectRoot with stdio connected to
// the terminal, and returns its exit code.
func RunInProject(scriptPath, projectRoot string, args ...string) (int, error) {
	cmd := projectCommand(scriptPath, projectRoot, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return exitCode(cmd.Run())
}

// OutputInProject runs scriptPath against projectRoot and returns its
// combined output and exit code.
func OutputInProject(scriptPath, projectRoot string, args ...string) ([]byte, int, error) {
	out, err := projectCommand(scriptPath, projectRoot, args...).CombinedOutput()
	code, err := exitCode(err)
	return out, code, err
}

func exitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}
