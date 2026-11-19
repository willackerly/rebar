package mcp

import (
	"bytes"
	"strings"
	"testing"
)

// --help never prints ASK_API_KEY (or ASK_BIN), which used to show up as the
// flags' "(default …)"; both still apply when the flag is not given, and a
// default is shown once.
func TestHelpHidesEnvDefaults(t *testing.T) {
	t.Setenv("ASK_API_KEY", "hunter2-secret")
	t.Setenv("ASK_BIN", "/opt/secret-ask")
	for _, mcp := range []bool{true, false} {
		var out, errb bytes.Buffer
		if _, code, done := parse("prog", "desc", "", mcp, []string{"--help"}, &out, &errb); !done || code != 0 {
			t.Fatalf("mcp=%v: --help = code %d, done %v", mcp, code, done)
		}
		help := out.String()
		for _, secret := range []string{"hunter2-secret", "/opt/secret-ask"} {
			if strings.Contains(help, secret) {
				t.Errorf("mcp=%v: --help shows %q:\n%s", mcp, secret, help)
			}
		}
		if n := strings.Count(help, "7232"); n != 1 {
			t.Errorf("mcp=%v: the port default appears %d times:\n%s", mcp, n, help)
		}
		o, _, done := parse("prog", "desc", "", mcp, []string{"--repos", "/r"}, &out, &errb)
		if done || o.apiKey != "hunter2-secret" || o.askBin != "/opt/secret-ask" {
			t.Errorf("mcp=%v: env not applied: %+v", mcp, o)
		}
		o, _, _ = parse("prog", "desc", "", mcp, []string{"--repos", "/r", "--api-key", "", "--ask-bin", "x"}, &out, &errb)
		if o.apiKey != "" || o.askBin != "x" {
			t.Errorf("mcp=%v: explicit flags lost to env: %+v", mcp, o)
		}
	}
}

func TestClientHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := ClientMain([]string{"--help"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "Usage: test-mcp-client") || errb.Len() != 0 {
		t.Errorf("client --help = %d, %q, %q", code, out.String(), errb.String())
	}
	out.Reset()
	if code := ClientMain([]string{"--bogus"}, &out, &errb); code != 1 || !strings.Contains(out.String(), "Usage:") {
		t.Errorf("client --bogus = %d, %q", code, out.String())
	}
}
