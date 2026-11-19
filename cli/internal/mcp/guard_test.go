package mcp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The v3.1.2 request hardening, as the python servers do it.

func TestValidNameAndLoopback(t *testing.T) {
	for name, want := range map[string]bool{
		"web": true, "dsp_2_8": true, "my agent": true,
		"": false, ".hidden": false, "-x": false, "..": false, "a..b": false,
		"a/b": false, `a\b`: false, "repo:role": false, "a\x00": false, "a\x7f": false,
	} {
		if got := validName(name); got != want {
			t.Errorf("validName(%q) = %v", name, got)
		}
	}
	for host, want := range map[string]bool{
		"127.0.0.1": true, "localhost": true, "LOCALHOST": true, "::1": true, "[::1]": true, "127.0.0.2": true,
		"0.0.0.0": false, "": false, "example.com": false, "10.0.0.1": false,
	} {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v", host, got)
		}
	}
	for header, want := range map[string]bool{
		"": true, "localhost:7232": true, "127.0.0.1": true, "[::1]:7232": true,
		"evil.example:7232": false, "evil.example": false, "[::2]:1": false,
	} {
		r := httptest.NewRequest("GET", "/v1/health", nil)
		r.Host = header
		if got := hostHeaderIsLoopback(r); got != want {
			t.Errorf("Host %q: loopback = %v", header, got)
		}
	}
}

func TestCheckBindSafety(t *testing.T) {
	for _, tc := range []struct {
		o    options
		ok   bool
		says string
	}{
		{options{host: "127.0.0.1"}, true, ""},
		{options{host: "0.0.0.0", apiKey: "k"}, true, ""},
		{options{host: "0.0.0.0"}, false, "refusing to listen on non-loopback address '0.0.0.0' without an API key"},
		{options{host: "0.0.0.0", insecureNoAuth: true}, true, "WARNING: listening on 0.0.0.0 with no API key"},
	} {
		var errw bytes.Buffer
		if ok := checkBindSafety("ask-server", &tc.o, &errw); ok != tc.ok || !strings.Contains(errw.String(), tc.says) {
			t.Errorf("%+v: ok=%v, stderr %q", tc.o, ok, errw.String())
		}
	}
}

func TestRequestGuards(t *testing.T) {
	reg := NewRegistry("ask-server", &bytes.Buffer{}, &bytes.Buffer{}, false)
	h := &AskServer{Registry: reg, AskCmd: []string{"false"}, LoopbackOnly: true}
	post := func(body string, mod func(*http.Request)) int {
		r := httptest.NewRequest("POST", "/v1/ask", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Host = "localhost:7232"
		if mod != nil {
			mod(r)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if code := post(`{}`, func(r *http.Request) { r.Host = "evil.example" }); code != 403 {
		t.Errorf("foreign Host: %d, want 403", code)
	}
	if code := post(`{}`, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); code != 415 {
		t.Errorf("text/plain: %d, want 415", code)
	}
	if code := post(`{}`, func(r *http.Request) { r.Header.Del("Content-Length"); r.TransferEncoding = []string{"chunked"} }); code != 411 {
		t.Errorf("chunked: %d, want 411", code)
	}
	if code := post(`{}`, func(r *http.Request) { r.Header.Set("Content-Length", "140000000000000") }); code != 413 {
		t.Errorf("huge Content-Length: %d, want 413", code)
	}
	if code := post(`{}`, func(r *http.Request) { r.Header.Set("Content-Length", "10") }); code != 400 {
		t.Errorf("short body: %d, want 400", code)
	}
	if code := post(`[]`, func(r *http.Request) { r.Header.Set("Content-Length", "2") }); code != 400 {
		t.Errorf("array body: %d, want 400", code)
	}
	if !apiKeyMatches("Bearer k", "k") || apiKeyMatches("Bearer kk", "k") || apiKeyMatches("bearer k", "k") {
		t.Error("apiKeyMatches")
	}
	if got := strings.Join(askArgs("architect", "--help", true), " "); got != "-v --question -- architect --help" {
		t.Errorf("askArgs: %s", got)
	}
}
