package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	srv, err := s.NewMCPServer()
	if err != nil {
		return err
	}
	p := newPending()
	f := &stdinFilter{lines: readLines(in), stop: ctx.Done(), log: errw, pending: p, shutdown: s.Stop,
		stopping: make(chan struct{}), closed: make(chan struct{})}
	w := &stdoutTracker{out: out, pending: p, broken: make(chan struct{})}
	f.reply = func(b []byte) { w.Write(b) }
	// The filter hands the SDK lines of at most maxLineLength bytes.
	t := &sdk.IOTransport{Reader: f, Writer: w, MaxLineLength: 2 * maxLineLength}
	done := make(chan error, 1)
	go func() { done <- srv.Run(context.Background(), t) }()
	defer KillChildren()
	select {
	case err = <-done:
	case <-w.broken:
		// The client closed stdout: nobody reads the answers any more.
		fmt.Fprintln(errw, "ask-mcp-server: stdout closed: shutting down")
		s.Stop()
		return nil
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
	if err == io.EOF || w.isBroken() {
		return nil
	}
	return err
}

// maxLineLength caps one line of input (one JSON-RPC message): the SDK's
// own limit. A longer line is dropped, and logged, and the session goes
// on; it is never buffered whole.
var maxLineLength = sdk.DefaultMaxLineLength

// line is one line of input, the size of a line dropped for its length,
// or the error that ended the input.
type line struct {
	b       []byte
	dropped int
	err     error
}

// readLines reads in line by line (the last line may lack its newline);
// the channel closes at the end of input. Reading runs on its own so a
// signal can end the session while a read blocks.
func readLines(in io.Reader) <-chan line {
	ch := make(chan line)
	go func() {
		defer close(ch)
		r := bufio.NewReaderSize(in, 64<<10)
		var buf []byte
		dropped := 0 // bytes of an over-long line read so far
		for {
			chunk, err := r.ReadSlice('\n')
			end := err == nil || (err != bufio.ErrBufferFull && len(chunk)+len(buf)+dropped > 0)
			if dropped == 0 && len(buf)+len(chunk) <= maxLineLength+1 {
				buf = append(buf, chunk...)
				if end && len(buf) > 0 {
					ch <- line{b: buf}
					buf = nil
				}
			} else {
				dropped += len(buf) + len(chunk)
				buf = nil
				if end {
					ch <- line{dropped: dropped}
					dropped = 0
				}
			}
			if err != nil && err != bufio.ErrBufferFull {
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
	reply    func([]byte)
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
			if l.dropped > 0 {
				fmt.Fprintf(f.log, "ask-mcp-server: ignored invalid message (longer than %d bytes): %d bytes\n", maxLineLength, l.dropped)
				continue
			}
			if msg := bytes.TrimSpace(l.b); len(msg) > 0 {
				if reason := f.check(msg); reason != "" {
					fmt.Fprintf(f.log, "ask-mcp-server: ignored invalid message (%s): %s\n", reason, abbreviate(msg))
					f.parseError(msg, reason)
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
	if f.pending.count() > 0 {
		fmt.Fprintf(f.log, "ask-mcp-server: %s: cancelling running requests\n", why)
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
// (MCP ids are strings or integers; the SDK would truncate 4.5 to 4), and
// so are integer ids beyond ±2^53. Requests read are recorded as pending
// until their answer is written (stdoutTracker), also when the client
// cancels them: the SDK still answers a cancelled call, so its id is in use
// until then.
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
		if len(id.ID) > 0 && id.ID[0] != '"' && string(id.ID) != "null" && !safeInteger(id.ID) {
			return "id beyond ±2^53 (it would not survive the round trip)"
		}
		req, ok := m.(*jsonrpc.Request)
		if !ok || !req.IsCall() {
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

// parseError answers a request too deeply nested for the JSON decoders
// (Go's and the SDK's stop at 1000 levels) with a parse error, when its id
// can be read: otherwise its client would wait for an answer forever. Other
// invalid lines get no answer (one with id null breaks Claude Code).
func (f *stdinFilter) parseError(msg []byte, reason string) {
	if f.reply == nil || !strings.Contains(reason, "nesting depth") {
		return
	}
	id, ok := topLevelID(msg)
	if !ok {
		return
	}
	var v any
	if json.Unmarshal(id, &v) != nil || id[0] != '"' && !safeInteger(id) {
		return
	}
	if rid, err := jsonrpc.MakeID(v); err != nil || f.pending.has(rid) {
		return // not an id, or one in use: an answer would be taken for its
	}
	f.reply([]byte(`{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32700,"message":"Parse error: JSON nested too deeply"}}` + "\n"))
}

// topLevelID is the raw "id" member of the JSON object msg, found without
// decoding the other members (however deep they nest).
func topLevelID(msg []byte) (json.RawMessage, bool) {
	i := skipSpace(msg, 0)
	if i >= len(msg) || msg[i] != '{' {
		return nil, false
	}
	i++
	for {
		i = skipSpace(msg, i)
		if i >= len(msg) || msg[i] != '"' {
			return nil, false
		}
		end := skipValue(msg, i)
		key := msg[i:end]
		i = skipSpace(msg, end)
		if i >= len(msg) || msg[i] != ':' {
			return nil, false
		}
		i = skipSpace(msg, i+1)
		end = skipValue(msg, i)
		if end <= i {
			return nil, false
		}
		if string(key) == `"id"` {
			return json.RawMessage(msg[i:end]), true
		}
		i = skipSpace(msg, end)
		if i >= len(msg) || msg[i] != ',' {
			return nil, false
		}
		i++
	}
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r' || b[i] == '\n') {
		i++
	}
	return i
}

// skipValue is the end of the JSON value starting at b[i] (strings, nested
// arrays and objects, scalars); i when there is none.
func skipValue(b []byte, i int) int {
	if i >= len(b) {
		return i
	}
	depth := 0
	for j := i; j < len(b); j++ {
		switch c := b[j]; {
		case c == '"':
			for j++; j < len(b) && b[j] != '"'; j++ {
				if b[j] == '\\' {
					j++
				}
			}
			if depth == 0 {
				return j + 1
			}
		case c == '{' || c == '[':
			depth++
		case c == '}' || c == ']':
			depth--
			if depth == 0 {
				return j + 1
			}
			if depth < 0 {
				return j
			}
		case depth == 0 && (c == ',' || c == ' ' || c == '\t' || c == '\r' || c == '\n'):
			return j
		}
	}
	return len(b)
}

// safeInteger reports whether a numeric id is an integer a float64 holds
// exactly (|n| <= 2^53): the SDK decodes ids as JSON numbers, so a larger
// one would come back altered and its answer would never match.
func safeInteger(raw json.RawMessage) bool {
	n, err := strconv.ParseInt(string(raw), 10, 64)
	return err == nil && n <= 1<<53 && n >= -(1<<53)
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
	mu      sync.Mutex // the SDK's writes and the filter's parse errors
	out     io.Writer
	pending *pending
	broken  chan struct{} // closed when stdout is gone (EPIPE)
	once    sync.Once
}

func (w *stdoutTracker) isBroken() bool {
	select {
	case <-w.broken:
		return true
	default:
		return false
	}
}

func (w *stdoutTracker) Write(b []byte) (int, error) {
	// Settle first: a client may reuse an id (or send what the negotiated
	// version allows) as soon as it reads the answer, which can be before
	// this goroutine returns from the write.
	w.settle(b)
	w.mu.Lock()
	n, err := w.out.Write(fixErrorCodes(b))
	w.mu.Unlock()
	if err == nil {
		n = len(b)
	} else if errors.Is(err, syscall.EPIPE) || errors.Is(err, os.ErrClosed) {
		w.once.Do(func() { close(w.broken) })
	}
	return n, err
}

// settle records what the responses in b (a message or a batch) answer:
// their ids are no longer in use, and an initialize result sets the
// protocol version.
func (w *stdoutTracker) settle(b []byte) {
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
}

func (w *stdoutTracker) Close() error { return nil }
