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
//   - at end of input it waits until every request read has been answered
//     (the SDK answers calls concurrently and cancels the unanswered ones
//     when its reader stops), so `printf '…' | rebar mcp serve --stdio`
//     prints every response.

// ServeStdio runs the MCP server over stdin/stdout; it returns at the end
// of input.
func (s *Server) ServeStdio(in io.Reader, out, errw io.Writer) error {
	p := newPending()
	t := &sdk.IOTransport{
		Reader: &stdinFilter{in: bufio.NewReader(in), log: errw, pending: p},
		Writer: &stdoutTracker{out: out, pending: p},
	}
	err := s.NewMCPServer().Run(context.Background(), t)
	if err == io.EOF {
		return nil
	}
	return err
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
// limit passes (a request the SDK never answers must not keep the server
// alive).
func (p *pending) wait(limit time.Duration) {
	timer := time.AfterFunc(limit, p.close)
	defer timer.Stop()
	p.mu.Lock()
	for len(p.ids) > 0 && !p.closed {
		p.cond.Wait()
	}
	p.mu.Unlock()
}

func (p *pending) close() {
	p.mu.Lock()
	p.closed = true
	p.cond.Broadcast()
	p.mu.Unlock()
}

// stdinFilter is the SDK's reader: the valid messages of in, one per line.
type stdinFilter struct {
	in      *bufio.Reader
	log     io.Writer
	pending *pending
	buf     []byte
	eof     bool
}

func (f *stdinFilter) Read(b []byte) (int, error) {
	for len(f.buf) == 0 {
		if f.eof {
			f.pending.wait(AskTimeout + 10*time.Second)
			return 0, io.EOF
		}
		line, err := f.in.ReadBytes('\n')
		if err == io.EOF {
			f.eof = true
		} else if err != nil {
			return 0, err
		}
		if msg := bytes.TrimSpace(line); len(msg) > 0 {
			if reason := f.check(msg); reason != "" {
				fmt.Fprintf(f.log, "ask-mcp-server: ignored invalid message (%s): %s\n", reason, abbreviate(msg))
				continue
			}
			f.buf = append(msg, '\n')
		}
	}
	n := copy(b, f.buf)
	f.buf = f.buf[n:]
	return n, nil
}

func (f *stdinFilter) Close() error {
	f.pending.close()
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
	n, err := w.out.Write(b)
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
