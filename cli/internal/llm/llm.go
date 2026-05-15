// Package llm provides a backend for invoking language models.
// Default: Claude Code CLI (`claude --print`). Future: --local for LM Studio/ollama.
package llm

import (
	"fmt"
	"os/exec"
	"strings"
)

// Backend generates text from a prompt.
type Backend interface {
	Complete(prompt string) (string, error)
}

// Claude invokes the Claude Code CLI (`claude --print --model <model>`).
type Claude struct {
	Model string // e.g. "sonnet", "opus", "haiku"
}

// Local invokes an OpenAI-compatible API (LM Studio, ollama, etc.).
// Placeholder for future implementation.
type Local struct {
	Endpoint string // e.g. "http://localhost:1234/v1"
	Model    string // e.g. "qwen2.5-coder-32b"
}

// NewBackend returns the appropriate backend based on flags.
func NewBackend(local bool, localEndpoint, model string) Backend {
	if local {
		if localEndpoint == "" {
			localEndpoint = "http://localhost:1234/v1"
		}
		if model == "" {
			model = "default"
		}
		return &Local{Endpoint: localEndpoint, Model: model}
	}
	if model == "" {
		model = "sonnet"
	}
	return &Claude{Model: model}
}

// Complete sends a prompt to Claude Code CLI and returns the response.
func (c *Claude) Complete(prompt string) (string, error) {
	cmd := exec.Command("claude", "--print", "--model", c.Model, prompt)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude failed (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude not found — install Claude Code CLI or use --local: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Complete is a placeholder for future local LLM integration.
func (l *Local) Complete(prompt string) (string, error) {
	return "", fmt.Errorf("local LLM backend not yet implemented — coming soon\n  endpoint: %s\n  model: %s\n  For now, use Claude Code CLI (default) or install it: https://claude.ai/code", l.Endpoint, l.Model)
}
