package mcp

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestPyDumps(t *testing.T) {
	v := object{{"a", []any{1, "é\n\"", nil, true}}, {"b", object{}}, {"c", []any{}}, {"d", json.Number("4.5")}, {"e", "🚀\x7f"}}
	if got, want := pyDumps(v, 0), `{"a": [1, "\u00e9\n\"", null, true], "b": {}, "c": [], "d": 4.5, "e": "\ud83d\ude80\u007f"}`; got != want {
		t.Errorf("compact:\n%s\nwant:\n%s", got, want)
	}
	if got, want := pyDumps(object{{"k", []any{1, object{{"x", "y"}}}}}, 2), "{\n  \"k\": [\n    1,\n    {\n      \"x\": \"y\"\n    }\n  ]\n}"; got != want {
		t.Errorf("indent:\n%s\nwant:\n%s", got, want)
	}
}

func TestDescriptionAndTruthiness(t *testing.T) {
	root := t.TempDir()
	md := filepath.Join(root, "r", "agents", "x", "AGENT.md")
	os.MkdirAll(filepath.Dir(md), 0o755)
	os.WriteFile(md, []byte("# X\n\nyou are the X agent — you answer. YOU OWN things. Real text here.\n"), 0o644)
	reg := NewRegistry("t", io.Discard, io.Discard, true)
	reg.AddRepo("r", filepath.Join(root, "r"))
	s := &Server{Registry: reg}
	if got := s.description("r", "x"); got != "Query the x agent for r. Repo specifics: Real text here." {
		t.Errorf("description = %q", got)
	}
	for v, want := range map[string]bool{`0`: false, `0.0`: false, `""`: false, `"false"`: true, `[]`: false, `{}`: false, `null`: false, `1`: true} {
		d, _ := decode([]byte(v))
		if truthy(d) != want {
			t.Errorf("truthy(%s) = %v", v, !want)
		}
	}
	if _, err := decode([]byte(`{} {}`)); err == nil || !strings.Contains(err.Error(), "Extra data") {
		t.Errorf("trailing data: %v", err)
	}
}

func TestAliases(t *testing.T) {
	root := t.TempDir()
	md := filepath.Join(root, "TDFLite", "agents", "architect", "AGENT.md")
	os.MkdirAll(filepath.Dir(md), 0o755)
	os.WriteFile(md, []byte("x\n"), 0o644)
	reg := NewRegistry("t", io.Discard, io.Discard, true)
	reg.AddRepo("TDFLite", filepath.Join(root, "TDFLite"))
	s := &Server{Registry: reg}
	for in, want := range map[string]string{
		"ask_TDFLite_architect": "ask_TDFLite_architect",
		"ask_tdflite_ARCHITECT": "ask_TDFLite_architect",
		"ask_tdflite_product":   "ask_tdflite_product",
		"":                      "",
	} {
		if got := s.canonicalTool(in); got != want {
			t.Errorf("canonicalTool(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"ask://memory/tdflite:Architect": "ask://memory/TDFLite:architect",
		"ask://log/TDFLite:architect":    "ask://log/TDFLite:architect",
		"ask://memory/nope:architect":    "ask://memory/nope:architect",
		"ask://memory/tdflite":           "ask://memory/tdflite",
		"file:///etc/passwd":             "file:///etc/passwd",
	} {
		if got := s.canonicalURI(in); got != want {
			t.Errorf("canonicalURI(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestStdinFilter: invalid lines are reported and skipped, valid ones pass
// one per line, and end of input cancels the running calls and waits for
// the pending answers.
func TestStdinFilter(t *testing.T) {
	var log strings.Builder
	p := newPending()
	cancelled := make(chan bool, 1)
	f := &stdinFilter{lines: readLines(strings.NewReader("\n{\"jsonrpc\":\"2.0\",\"method\":\"ping\",\"id\":7}\nnot json\n[]\n{\"jsonrpc\":\"2.0\",\"method\":\"x\",\"id\":1.5}\n")),
		log: &log, pending: p, shutdown: func() { cancelled <- true }, stopping: make(chan struct{})}
	done := make(chan []byte)
	go func() {
		data, _ := io.ReadAll(f)
		done <- data
	}()
	select {
	case data := <-done:
		t.Fatalf("end of input before the answer: %q", data)
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case <-cancelled:
	default:
		t.Error("end of input did not cancel the running calls")
	}
	w := &stdoutTracker{out: io.Discard, pending: p}
	w.Write([]byte(`{"jsonrpc":"2.0","id":7,"result":{}}` + "\n"))
	if data := <-done; string(data) != `{"jsonrpc":"2.0","method":"ping","id":7}`+"\n" {
		t.Errorf("passed: %q", data)
	}
	for _, want := range []string{"(not JSON): not json", "(empty batch): []", "(fractional id)"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
}

// clientWriter is stdout as the client sees it: when an answer arrives it
// sends its next request at once, here a check of whether that request's
// id would be refused.
type clientWriter struct {
	p        *pending
	id       jsonrpc.ID
	inUseNow bool
}

func (c *clientWriter) Write(b []byte) (int, error) {
	c.inUseNow = c.p.has(c.id)
	return len(b), nil
}

// TestStdoutTrackerSettlesBeforeWriting: an id is free again by the time
// its answer reaches the client, which may reuse it right away (it used to
// be settled after the write: under load the reused id was refused as
// "still running", seen as a TestMCPCancelThenReuseID failure).
func TestStdoutTrackerSettlesBeforeWriting(t *testing.T) {
	p := newPending()
	id, err := jsonrpc.MakeID(float64(2))
	if err != nil {
		t.Fatal(err)
	}
	p.add(id)
	c := &clientWriter{p: p, id: id}
	w := &stdoutTracker{out: c, pending: p, broken: make(chan struct{})}
	w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{}}` + "\n"))
	if c.inUseNow {
		t.Error("id 2 still in use when its answer was written")
	}
	if p.has(id) {
		t.Error("id 2 still in use after its answer")
	}
}

// TestStdinFilterGrace: an answer that never comes holds end of input for
// ShutdownGrace only; a stop (signal) ends the input while a read blocks.
func TestStdinFilterGrace(t *testing.T) {
	defer func(g time.Duration) { ShutdownGrace = g }(ShutdownGrace)
	ShutdownGrace = 200 * time.Millisecond
	p := newPending()
	f := &stdinFilter{lines: readLines(strings.NewReader(`{"jsonrpc":"2.0","method":"ping","id":1}` + "\n")),
		log: io.Discard, pending: p, shutdown: func() {}, stopping: make(chan struct{})}
	start := time.Now()
	if data, _ := io.ReadAll(f); len(data) == 0 {
		t.Error("nothing passed")
	}
	if d := time.Since(start); d < ShutdownGrace || d > ShutdownGrace+time.Second {
		t.Errorf("end of input took %v, want about %v", d, ShutdownGrace)
	}

	pr, pw := io.Pipe()
	defer pw.Close()
	stop := make(chan struct{})
	var log strings.Builder
	f = &stdinFilter{lines: readLines(pr), stop: stop, log: &log, pending: newPending(), shutdown: func() {}, stopping: make(chan struct{})}
	done := make(chan error)
	go func() {
		_, err := f.Read(make([]byte, 10))
		done <- err
	}()
	close(stop)
	select {
	case err := <-done:
		if err != io.EOF || !strings.Contains(log.String(), "signal received: shutting down") {
			t.Errorf("stopped read: %v, log %q", err, log.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a stop did not end a blocked read")
	}
}

func TestFixErrorCodes(t *testing.T) {
	in := `{"jsonrpc":"2.0","id":0,"error":{"code":0,"message":"method \"tools/list\" is invalid during session initialization"}}` + "\n"
	want := `{"jsonrpc":"2.0","id":0,"error":{"code":-32600,"message":"method \"tools/list\" is invalid during session initialization"}}` + "\n"
	if got := string(fixErrorCodes([]byte(in))); got != want {
		t.Errorf("got %s", got)
	}
	for msg, code := range map[string]int{
		`duplicate \"initialize\" received`:                  -32600,
		`handling 'initialize': missing required \"params\"`: -32602,
		`something else`: -32603,
	} {
		in := `{"jsonrpc":"2.0","id":2,"error":{"code":0,"message":"` + msg + `"}}`
		want := `{"jsonrpc":"2.0","id":2,"error":{"code":` + strconv.Itoa(code) + `,"message":"` + msg + `"}}`
		if got := string(fixErrorCodes([]byte(in))); got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
	other := `{"jsonrpc":"2.0","id":0,"result":{"content":[{"type":"text","text":"\"error\":{\"code\":0, is invalid during session initialization"}]}}`
	if got := string(fixErrorCodes([]byte(other))); got != other {
		t.Errorf("rewrote a result: %s", got)
	}
}
