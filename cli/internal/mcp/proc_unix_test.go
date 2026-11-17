//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// TestRunAskCancelKillsGroup: cancelling an ask ends it and everything it
// started, even processes that ignore SIGTERM (SIGKILL after KillDelay),
// and runAskContext reports context.Canceled.
func TestRunAskCancelKillsGroup(t *testing.T) {
	for _, tc := range []struct {
		name, trap string
	}{
		{"honours SIGTERM", ""},
		{"ignores SIGTERM", "trap '' TERM"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := filepath.Join(dir, "ask")
			os.WriteFile(script, []byte("#!/bin/sh\n"+tc.trap+"\nsleep 300 &\necho $! > grandchild.pid\necho $$ > child.pid\nwait\n"), 0o755)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error)
			go func() {
				_, _, _, err := runAskContext(ctx, []string{script}, dir, time.Minute)
				done <- err
			}()
			child := testutil.ReadPIDFile(t, filepath.Join(dir, "child.pid"), 10*time.Second)
			grandchild := testutil.ReadPIDFile(t, filepath.Join(dir, "grandchild.pid"), 10*time.Second)
			start := time.Now()
			cancel()
			select {
			case err := <-done:
				if err != context.Canceled {
					t.Errorf("err = %v, want context.Canceled", err)
				}
				if d := time.Since(start); d > KillDelay+2*time.Second {
					t.Errorf("cancel took %v", d)
				}
			case <-time.After(KillDelay + 10*time.Second):
				t.Fatal("cancelled ask still running")
			}
			testutil.WaitGone(t, "ask", child, 2*time.Second)
			testutil.WaitGone(t, "its child", grandchild, 2*time.Second)
		})
	}
}

// TestKillChildren: the last step of a shutdown kills ask groups still
// running.
func TestKillChildren(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ask")
	os.WriteFile(script, []byte("#!/bin/sh\nsleep 300 &\necho $! > grandchild.pid\nwait\n"), 0o755)
	done := make(chan error)
	go func() {
		_, _, _, err := runAskContext(context.Background(), []string{script}, dir, time.Minute)
		done <- err
	}()
	grandchild := testutil.ReadPIDFile(t, filepath.Join(dir, "grandchild.pid"), 10*time.Second)
	KillChildren()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("ask survived KillChildren")
	}
	testutil.WaitGone(t, "ask's child", grandchild, 2*time.Second)
}
