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

func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// Run executes a bash script and captures its output.
// repoRoot is the consumer repo directory used as the working directory.
// env provides additional environment variables passed to the script.
func Run(scriptsDir, repoRoot, scriptName string, env map[string]string, args ...string) (*Result, error) {
	scriptPath := scriptsDir + "/" + scriptName
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("script not found: %s", scriptPath)
	}

	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = repoRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = buildEnv(env)

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
// repoRoot is the consumer repo directory used as the working directory.
// env provides additional environment variables passed to the script.
func RunPassthrough(scriptsDir, repoRoot, scriptName string, env map[string]string, args ...string) (int, error) {
	scriptPath := scriptsDir + "/" + scriptName
	if _, err := os.Stat(scriptPath); err != nil {
		return -1, fmt.Errorf("script not found: %s", scriptPath)
	}

	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = buildEnv(env)

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}
