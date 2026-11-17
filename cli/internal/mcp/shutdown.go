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

// Stop cancels the server's running tool calls and /v1/ask requests (and
// any started later).
func (s *Server) Stop() {
	s.stop.init()
	s.stop.cancel()
}

// withStop is ctx, also cancelled when the server stops.
func (s *Server) withStop(ctx context.Context) (context.Context, context.CancelFunc) {
	s.stop.init()
	ctx, cancel := context.WithCancel(ctx)
	unhook := context.AfterFunc(s.stop.ctx, cancel)
	return ctx, func() { unhook(); cancel() }
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

// KillChildren sends SIGKILL to the process group of every ask command
// still running: the last step of a shutdown.
func KillChildren() {
	children.Lock()
	defer children.Unlock()
	for pid := range children.pids {
		killGroup(pid)
	}
}
