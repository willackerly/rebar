package mcp

import (
	"context"
	"sync"
	"time"
)

// Shutdown. When the client goes away (end of input on stdio) or the server
// gets SIGINT/SIGTERM, the server stops reading requests, cancels the tool
// calls still running, gives them ShutdownGrace to answer, and exits 0. A
// cancelled call's ask process group gets SIGTERM, then SIGKILL after
// KillDelay; whatever is left when the server exits is killed too, so no
// ask, claude or git outlives it.
//
// Two seconds because that is how long MCP clients wait: Claude Code's
// stdio client (the TypeScript SDK's StdioClientTransport) closes stdin,
// waits 2 s for the server to exit, then sends SIGTERM and SIGKILL 2 s
// apart. Exiting within the first 2 s means the client never has to signal
// the server, and its children are never orphaned. Nothing a tool call
// does is worth keeping once its client is gone — an answer nobody reads —
// so there is no reason to wait longer.
var (
	ShutdownGrace = 2 * time.Second
	KillDelay     = time.Second
	// LingerLimit is how long what a finished ask left running in its
	// process group (its background memory compaction) may keep running;
	// the server's exit ends it sooner.
	LingerLimit = 300 * time.Second
	lingerPoll  = 250 * time.Millisecond
)

// stopper is the context every tool call of a server runs under, cancelled
// when the server shuts down.
type stopper struct {
	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *stopper) init() {
	s.once.Do(func() { s.ctx, s.cancel = context.WithCancel(context.Background()) })
}

// Stop cancels everything started under s (and anything started later).
func (s *stopper) Stop() {
	s.init()
	s.cancel()
}

// with is ctx, also cancelled when s stops.
func (s *stopper) with(ctx context.Context) (context.Context, context.CancelFunc) {
	s.init()
	ctx, cancel := context.WithCancel(ctx)
	unhook := context.AfterFunc(s.ctx, cancel)
	return ctx, func() { unhook(); cancel() }
}

// Stop cancels the server's running tool calls and /v1/ask requests (and
// any started later).
func (s *Server) Stop() { s.stop.Stop() }

// withStop is ctx, also cancelled when the server stops.
func (s *Server) withStop(ctx context.Context) (context.Context, context.CancelFunc) {
	return s.stop.with(ctx)
}

// children are the process groups of the ask commands running.
var children = struct {
	sync.Mutex
	pids map[int]bool
}{pids: map[int]bool{}}

func addChild(pid int) {
	children.Lock()
	children.pids[pid] = true
	children.Unlock()
}

func removeChild(pid int) {
	children.Lock()
	delete(children.pids, pid)
	children.Unlock()
}

// settle is the end of the ask command leading process group pid: when it
// was cancelled or timed out (ended), the rest of its group is killed now;
// otherwise what it left running stays in the kill set until it exits, or
// is killed after LingerLimit or by KillChildren.
func settle(pid int, ended bool) {
	if ended {
		killGroup(pid)
	}
	if ended || !groupAlive(pid) {
		removeChild(pid)
		return
	}
	deadline := time.Now().Add(LingerLimit)
	go func() {
		for groupAlive(pid) {
			if time.Now().After(deadline) {
				killGroup(pid)
				break
			}
			time.Sleep(lingerPoll)
		}
		removeChild(pid)
	}()
}

// KillChildren sends SIGKILL to the process group of every ask command
// still running, and to what finished asks left running: the last step of
// a shutdown.
func KillChildren() {
	children.Lock()
	defer children.Unlock()
	for pid := range children.pids {
		killGroup(pid)
	}
}
