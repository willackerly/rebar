package ask

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpers(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"", "abc", 3}, {"architect", "archtect", 1}, {"kitten", "sitting", 3}} {
		if got := levenshtein(tc.a, tc.b); got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	for in, want := range map[string]string{"  /a/b  ": "/a/b", "a   b\tc": "a b c", `"my dir"/x`: "my dir/x", `'x`: "", `a\ b`: "a b", "": ""} {
		if got := xargsEcho(in); got != want {
			t.Errorf("xargsEcho(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"/a/./b/../c/": "/a/c", "/../x": "/x"} {
		if got := cleanPath(in); got != want {
			t.Errorf("cleanPath(%q) = %q, want %q", in, got, want)
		}
	}
	if got := cutField2("a = /p=q"); got != " /p" {
		t.Errorf("cutField2 = %q", got)
	}
	if got := wrap("one two three four\n\n  five six", 9); got != "one two\nthree\nfour\n\n  five\n  six" {
		t.Errorf("wrap = %q", got)
	}
}

func TestJQReadings(t *testing.T) {
	cases := []struct {
		data, want string
		keys       []string
		def        any
	}{
		{`{"id":"a"}`, "a", []string{"id"}, nil},
		{`{"id":null}`, "null", []string{"id"}, nil},
		{`{"id":null}`, "unknown", []string{"id"}, "unknown"},
		{`{"id":false}`, "", []string{"id"}, jqEmpty},
		{`"str"`, "FALLBACK", []string{"id"}, "unknown"},
		{`{"id":1}{"id":2}`, "1\n2", []string{"id"}, nil},
		{`{"id":1} oops`, "1\nFALLBACK", []string{"id"}, nil},
		{`{"a":{"b":[1]}}`, "[\n  1\n]", []string{"a", "b"}, nil},
	}
	for _, tc := range cases {
		if got := jqRaw([]byte(tc.data), tc.keys, tc.def, "FALLBACK"); got != tc.want {
			t.Errorf("jqRaw(%s, %v, %v) = %q, want %q", tc.data, tc.keys, tc.def, got, tc.want)
		}
	}
	if got := responseText(`"just text"`); got != `"just text"` {
		t.Errorf("responseText(string) = %q", got)
	}
	if got := responseText(`{"result":null,"content":"c"}`); got != "c" {
		t.Errorf("responseText(content) = %q", got)
	}
	usage := `{"modelUsage":{"m":{"inputTokens":100,"cacheReadInputTokens":50,"cacheCreationInputTokens":null,"contextWindow":1000}}}`
	if got := contextPercent(usage); got != "15" {
		t.Errorf("contextPercent = %q", got)
	}
	if got := contextPercent(`{"modelUsage":{}}`); got != "" {
		t.Errorf("contextPercent(no models) = %q", got)
	}
	if got := tokenUsage(`{"modelUsage":{}}`); got != "tokens: in=null out=null cached=null window=null" {
		t.Errorf("tokenUsage(no models) = %q", got)
	}
	if got := tokenUsage(`not json`); got != "tokens: unavailable" {
		t.Errorf("tokenUsage(not json) = %q", got)
	}
	if !hasAnswer([]byte(`{"answer":"x"}`)) || hasAnswer([]byte(`{"answer":null}`)) || hasAnswer([]byte(`"x"`)) {
		t.Error("hasAnswer")
	}
}

func TestCallerFacingDescription(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "AGENT.md")
	os.WriteFile(md, []byte("# T\n\nYou are the x agent. You don't guess. Answers questions.\nMore.\n\nLater.\n"), 0o644)
	if got := callerFacingDescription(md); got != "Answers questions. More." {
		t.Errorf("description = %q", got)
	}
}

// The loop started by `ask up` (Loop.Run) answers, then stops cleanly.
func TestLoopStops(t *testing.T) {
	dir := t.TempDir()
	stop := make(chan struct{})
	close(stop)
	var out bytes.Buffer
	l := &Loop{Name: "x", Dir: dir, EventsLog: filepath.Join(dir, "events"), Model: "m", Stdout: &out, Stderr: &out, Stop: stop}
	if code := l.Run(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".pid")); !os.IsNotExist(err) {
		t.Error("the pid file outlived the loop")
	}
	if !strings.Contains(out.String(), "[x] online") || strings.Count(out.String(), "[x] offline") != 2 {
		t.Errorf("output:\n%s", out.String())
	}
}
