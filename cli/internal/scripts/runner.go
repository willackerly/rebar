package scripts

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Result captures the output of running a bash script.
type Result struct {
	Script   string
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// Run executes a bash script and captures its output.
func Run(scriptsDir, scriptName string, args ...string) (*Result, error) {
	scriptPath := scriptsDir + "/" + scriptName
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("script not found: %s", scriptPath)
	}

	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = scriptsDir + "/.." // run from repo root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("running %s: %w", scriptName, err)
		}
	}

	return &Result{
		Script:   scriptName,
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}, nil
}

// RunPassthrough executes a script with stdout/stderr connected to the terminal.
func RunPassthrough(scriptsDir, scriptName string, args ...string) (int, error) {
	scriptPath := scriptsDir + "/" + scriptName
	if _, err := os.Stat(scriptPath); err != nil {
		return -1, fmt.Errorf("script not found: %s", scriptPath)
	}

	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = scriptsDir + "/.."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// projectCommand builds a bash invocation of scriptPath that checks
// projectRoot: it runs there, and REBAR_PROJECT_ROOT tells scripts that live
// in a rebar install (not in the project) which repo to check. env is added
// last (the CLI passes checks.ScriptEnv, the native hand-off settings).
func projectCommand(scriptPath, projectRoot string, env []string, args ...string) *exec.Cmd {
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = projectRoot
	cmd.Env = append(append(os.Environ(), "REBAR_PROJECT_ROOT="+projectRoot), env...)
	return cmd
}

// RunInProject runs scriptPath against projectRoot, with env added to the
// environment, stdio connected to the terminal, and returns its exit code.
func RunInProject(scriptPath, projectRoot string, env []string, args ...string) (int, error) {
	cmd := projectCommand(scriptPath, projectRoot, env, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return exitCode(cmd.Run())
}

// OutputInProject runs scriptPath against projectRoot, with env added to
// the environment, and returns its combined output and exit code.
func OutputInProject(scriptPath, projectRoot string, env []string, args ...string) ([]byte, int, error) {
	out, err := projectCommand(scriptPath, projectRoot, env, args...).CombinedOutput()
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
