package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// The stdio transport is the SDK's (newline-delimited JSON-RPC), behind two
// adapters that keep what the hand-written server guaranteed:
//
//   - stdinFilter hands the SDK only well-formed JSON-RPC messages. The SDK
//     ends the session at the first malformed line; this server reports it
//     on stderr and reads on (no reply: a response with id null breaks
//     Claude Code's client, CONTRACT:S3-MCP-SERVER.1.0).
//   - at end of input, or when ctx is done (SIGINT/SIGTERM), it stops
//     reading, cancels the tool calls still running and waits up to
//     ShutdownGrace for every request read to be answered (quick ones —
//     tools/list, resources — answer; cancelled calls answer with an
//     isError "cancelled" result): a client that exits never waits on the
//     server, and `printf '…' | rebar mcp serve --stdio` still prints the
//     answers of quick requests. Then the SDK session ends and leftover ask
//     processes are killed (shutdown.go).

// ServeStdio runs the MCP server over stdin/stdout; it returns at the end
// of input or, when ctx is done, after a shutdown.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out, errw io.Writer) error {
	p := newPending()
	f := &stdinFilter{lines: readLines(in), stop: ctx.Done(), log: errw, pending: p, shutdown: s.Stop,
		stopping: make(chan struct{}), closed: make(chan struct{})}
	t := &sdk.IOTransport{Reader: f, Writer: &stdoutTracker{out: out, pending: p}}
	done := make(chan error, 1)
	go func() { done <- s.NewMCPServer().Run(context.Background(), t) }()
	defer KillChildren()
	var err error
	select {
	case err = <-done:
	case <-f.stopping:
		// The session ends once the filter reports end of input; its
		// handlers were cancelled, so this does not take long, but never
		// longer than the grace (plus a moment to write the answers).
		select {
		case err = <-done:
		case <-time.After(time.Until(f.deadline) + 500*time.Millisecond):
			return nil
		}
	}
	if err == io.EOF {
		return nil
	}
	return err
}

// line is one line of input, or the error that ended it.
type line struct {
	b   []byte
	err error
}

// readLines reads in line by line (the last line may lack its newline);
// the channel closes at the end of input. Reading runs on its own so a
// signal can end the session while a read blocks.
func readLines(in io.Reader) <-chan line {
	ch := make(chan line)
	go func() {
		defer close(ch)
		r := bufio.NewReader(in)
		for {
			b, err := r.ReadBytes('\n')
			if len(b) > 0 {
				ch <- line{b: b}
			}
			if err != nil {
				if err != io.EOF {
					ch <- line{err: err}
				}
				return
			}
		}
	}()
	return ch
}

// pending is the set of requests read and not yet answered, by id, and
// the protocol version the session negotiated (from the initialize
// response).
type pending struct {
	mu      sync.Mutex
	cond    *sync.Cond
	ids     map[string]int
	closed  bool
	version string
}

func (p *pending) setVersion(v string) {
	p.mu.Lock()
	p.version = v
	p.mu.Unlock()
}

// batchesAllowed: JSON-RPC batches were removed in 2025-06-18; the SDK ends
// a session that sends one after negotiating it or later.
func (p *pending) batchesAllowed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.version < "2025-06-18"
}

func newPending() *pending {
	p := &pending{ids: map[string]int{}}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// idKey identifies a request id (string and number ids differ).
func idKey(id jsonrpc.ID) string { return fmt.Sprintf("%T:%v", id.Raw(), id.Raw()) }

func (p *pending) add(id jsonrpc.ID) {
	p.mu.Lock()
	p.ids[idKey(id)]++
	p.mu.Unlock()
}

func (p *pending) done(key string) {
	p.mu.Lock()
	if p.ids[key] > 1 {
		p.ids[key]--
	} else {
		delete(p.ids, key)
	}
	p.cond.Broadcast()
	p.mu.Unlock()
}

// has reports whether a request with id is unanswered.
func (p *pending) has(id jsonrpc.ID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ids[idKey(id)] > 0
}

// wait blocks until every request is answered, the transport closes, or
// deadline passes (a request the SDK never answers must not keep the
// server alive).
func (p *pending) wait(deadline time.Time) {
	timer := time.AfterFunc(time.Until(deadline), p.close)
	defer timer.Stop()
	p.mu.Lock()
	for len(p.ids) > 0 && !p.closed {
		p.cond.Wait()
	}
	p.mu.Unlock()
}

// count is the number of requests not yet answered.
func (p *pending) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, c := range p.ids {
		n += c
	}
	return n
}

func (p *pending) close() {
	p.mu.Lock()
	p.closed = true
	p.cond.Broadcast()
	p.mu.Unlock()
}

// stdinFilter is the SDK's reader: the valid messages of lines, one per
// line, until the end of input or stop.
type stdinFilter struct {
	lines    <-chan line
	stop     <-chan struct{}
	log      io.Writer
	pending  *pending
	shutdown func() // cancels the running tool calls
	buf      []byte

	stopping chan struct{} // closed when the shutdown starts
	deadline time.Time     // the end of the grace
	ended    bool
	closed   chan struct{} // closed by Close (the SDK ended the session)
	once     sync.Once
}

func (f *stdinFilter) Read(b []byte) (int, error) {
	for len(f.buf) == 0 {
		if f.ended {
			return 0, io.EOF
		}
		select {
		case l, ok := <-f.lines:
			if !ok {
				f.end("end of input")
				continue
			}
			if l.err != nil {
				return 0, l.err
			}
			if msg := bytes.TrimSpace(l.b); len(msg) > 0 {
				if reason := f.check(msg); reason != "" {
					fmt.Fprintf(f.log, "ask-mcp-server: ignored invalid message (%s): %s\n", reason, abbreviate(msg))
					continue
				}
				f.buf = append(msg, '\n')
			}
		case <-f.stop:
			f.end("signal received")
		case <-f.closed:
			return 0, io.EOF
		}
	}
	n := copy(b, f.buf)
	f.buf = f.buf[n:]
	return n, nil
}

// end is the shutdown: no more requests are read, running tool calls are
// cancelled, and the answers still due get ShutdownGrace.
func (f *stdinFilter) end(why string) {
	f.deadline = time.Now().Add(ShutdownGrace)
	close(f.stopping)
	if why != "end of input" {
		fmt.Fprintf(f.log, "ask-mcp-server: %s: shutting down\n", why)
	}
	if n := f.pending.count(); n > 0 {
		fmt.Fprintf(f.log, "ask-mcp-server: %s: cancelling %d running request(s)\n", why, n)
	}
	f.shutdown()
	f.pending.wait(f.deadline)
	f.ended = true
}

func (f *stdinFilter) Close() error {
	f.pending.close()
	if f.closed != nil {
		f.once.Do(func() { close(f.closed) })
	}
	return nil
}

// check is why msg would end the SDK's session ("" when it would not), as
// its reader decides: a JSON-RPC 2.0 message or (before 2025-06-18) a
// non-empty batch of them with distinct ids. Fractional ids are refused too
// (MCP ids are strings or integers; the SDK would truncate 4.5 to 4). Requests read are recorded as pending, and a
// notifications/cancelled settles the request it names (the SDK does not
// answer a cancelled request).
func (f *stdinFilter) check(msg []byte) string {
	if !json.Valid(msg) {
		return "not JSON"
	}
	var batch []json.RawMessage
	raws := []json.RawMessage{msg}
	if json.Unmarshal(msg, &batch) == nil {
		if len(batch) == 0 {
			return "empty batch"
		}
		if !f.pending.batchesAllowed() {
			return "batches are not supported after 2025-06-18"
		}
		raws = batch
	}
	var calls []jsonrpc.ID
	seen := map[string]bool{}
	for _, raw := range raws {
		m, err := jsonrpc.DecodeMessage(raw)
		if err != nil {
			return strings.TrimPrefix(err.Error(), "unmarshaling jsonrpc message: ")
		}
		var id struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(raw, &id) == nil && len(id.ID) > 0 && id.ID[0] != '"' && strings.ContainsAny(string(id.ID), ".eE") {
			return "fractional id"
		}
		req, ok := m.(*jsonrpc.Request)
		if !ok || !req.IsCall() {
			if ok && req.Method == "notifications/cancelled" {
				f.cancelled(req.Params)
			}
			continue
		}
		if seen[idKey(req.ID)] {
			return "duplicate id in batch"
		}
		if f.pending.has(req.ID) {
			// The SDK would answer with id null, which Claude Code rejects.
			return "id of a request still running"
		}
		seen[idKey(req.ID)] = true
		calls = append(calls, req.ID)
	}
	for _, id := range calls {
		f.pending.add(id)
	}
	return ""
}

// cancelled settles the request a notifications/cancelled names.
func (f *stdinFilter) cancelled(params json.RawMessage) {
	var p struct {
		RequestID any `json:"requestId"`
	}
	if json.Unmarshal(params, &p) != nil || p.RequestID == nil {
		return
	}
	if id, err := jsonrpc.MakeID(p.RequestID); err == nil {
		f.pending.done(idKey(id))
	}
}

// abbreviate is msg cut to 200 bytes for the log.
func abbreviate(msg []byte) string {
	if len(msg) > 200 {
		return string(msg[:200]) + "…"
	}
	return string(msg)
}

// stdoutTracker is the SDK's writer: stdout, settling the requests whose
// responses go out.
type stdoutTracker struct {
	out     io.Writer
	pending *pending
}

func (w *stdoutTracker) Write(b []byte) (int, error) {
	n, err := w.out.Write(fixNotInitialized(b))
	if err == nil {
		n = len(b)
	}
	var batch []json.RawMessage
	raws := []json.RawMessage{bytes.TrimSpace(b)}
	if json.Unmarshal(raws[0], &batch) == nil {
		raws = batch
	}
	for _, raw := range raws {
		if m, derr := jsonrpc.DecodeMessage(raw); derr == nil {
			if resp, ok := m.(*jsonrpc.Response); ok {
				var init struct {
					ProtocolVersion string `json:"protocolVersion"`
				}
				if json.Unmarshal(resp.Result, &init) == nil && init.ProtocolVersion != "" {
					w.pending.setVersion(init.ProtocolVersion)
				}
				w.pending.done(idKey(resp.ID))
			}
		}
	}
	return n, err
}

func (w *stdoutTracker) Close() error { return nil }
