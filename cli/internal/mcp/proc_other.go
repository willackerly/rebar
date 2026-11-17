//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd || linux)

package mcp

import "os/exec"

// inOwnGroup: no process groups here; cancelling cmd kills cmd only.
func inOwnGroup(cmd *exec.Cmd) {}

func killGroup(pid int) {}
