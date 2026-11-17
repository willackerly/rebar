package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
// one per line, and end of input waits for the pending answers.
func TestStdinFilter(t *testing.T) {
	var log strings.Builder
	p := newPending()
	f := &stdinFilter{in: bufio.NewReader(strings.NewReader("\n{\"jsonrpc\":\"2.0\",\"method\":\"ping\",\"id\":7}\nnot json\n[]\n{\"jsonrpc\":\"2.0\",\"method\":\"x\",\"id\":1.5}\n")), log: &log, pending: p}
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
