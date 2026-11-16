package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// A .mcp.json whose servers run bin/ask-mcp-server — directly, through
// python3, from an install with a rebar beside it or not — is planned for
// rewriting to `rebar mcp serve` (same arguments, other keys and servers
// kept), shown by the dry run, and written by --apply only.
func TestMCPJSONRewrite(t *testing.T) {
	install := installRoot(t)
	p := testutil.InitRepo(t)
	withRebar := t.TempDir()
	testutil.WriteExecutable(t, withRebar, "bin/ask-mcp-server", "#!/bin/sh\n")
	testutil.WriteExecutable(t, withRebar, "bin/rebar", "#!/bin/sh\n")
	mcp := `{
  "mcpServers": {
    "rebar-ask": {
      "command": "` + withRebar + `/bin/ask-mcp-server",
      "args": ["--stdio", "--repos-dir", "/Users/x/dev"],
      "env": {"ASK_MODEL": "opus"}
    },
    "old-python": {"command": "python3", "args": ["/opt/rebar/bin/ask-mcp-server", "--stdio", "--repos", "/a,/b"]},
    "bare": {"command": "ask-mcp-server"},
    "other": {"command": "npx", "args": ["some-server"]}
  },
  "extra": true
}
`
	testutil.WriteFile(t, p, ".mcp.json", mcp)
	testutil.WriteFile(t, p, ".rebarrc", "tier = 1\n")
	testutil.Git(t, p, "add", "-A")
	testutil.Git(t, p, "commit", "-q", "-m", "init")

	o := opts(p, install)
	o.RebarBin = "/usr/local/bin/rebar"
	plan, err := BuildPlan(o)
	if err != nil {
		t.Fatal(err)
	}
	if plan.MCP == nil || len(plan.MCP.Servers) != 3 {
		t.Fatalf("plan.MCP = %+v", plan.MCP)
	}
	if plan.Empty() {
		t.Error("a plan with an .mcp.json rewrite is not empty")
	}
	var out bytes.Buffer
	Render(&out, plan, false)
	for _, want := range []string{
		".mcp.json: would point Claude Code to rebar's native MCP server",
		"rebar-ask: " + withRebar + "/bin/ask-mcp-server --stdio --repos-dir /Users/x/dev",
		"→ " + withRebar + "/bin/rebar mcp serve --stdio --repos-dir /Users/x/dev",
		"old-python: python3 /opt/rebar/bin/ask-mcp-server --stdio --repos /a,/b",
		"→ /usr/local/bin/rebar mcp serve --stdio --repos /a,/b",
		"bare: ask-mcp-server",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry run lacks %q:\n%s", want, out.String())
		}
	}
	if data, _ := os.ReadFile(filepath.Join(p, ".mcp.json")); string(data) != mcp {
		t.Fatal("the dry run changed .mcp.json")
	}

	if _, err := Apply(plan, o, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(p, ".mcp.json"))
	want := `{
  "mcpServers": {
    "rebar-ask": {
      "command": "` + withRebar + `/bin/rebar",
      "args": [
        "mcp",
        "serve",
        "--stdio",
        "--repos-dir",
        "/Users/x/dev"
      ],
      "env": {
        "ASK_MODEL": "opus"
      }
    },
    "old-python": {
      "command": "/usr/local/bin/rebar",
      "args": [
        "mcp",
        "serve",
        "--stdio",
        "--repos",
        "/a,/b"
      ]
    },
    "bare": {
      "command": "/usr/local/bin/rebar",
      "args": [
        "mcp",
        "serve"
      ]
    },
    "other": {
      "command": "npx",
      "args": [
        "some-server"
      ]
    }
  },
  "extra": true
}
`
	if string(got) != want {
		t.Errorf("applied .mcp.json:\n%s\nwant:\n%s", got, want)
	}
	testutil.Git(t, p, "commit", "-qam", "migrated")
	again, err := BuildPlan(o)
	if err != nil {
		t.Fatal(err)
	}
	if again.MCP != nil || !again.Empty() {
		t.Errorf("a migrated .mcp.json is planned again: %+v", again.MCP)
	}
}

// Nothing to do without .mcp.json or ask-mcp-server servers; an unreadable
// one is a note, not an error.
func TestPlanMCPNothingToDo(t *testing.T) {
	p := t.TempDir()
	if e, err := PlanMCP(p, "rebar"); e != nil || err != nil {
		t.Errorf("no .mcp.json: %+v %v", e, err)
	}
	testutil.WriteFile(t, p, ".mcp.json", `{"mcpServers": {"x": {"command": "rebar", "args": ["mcp", "serve"]}}}`)
	if e, err := PlanMCP(p, "rebar"); e != nil || err != nil {
		t.Errorf("already native: %+v %v", e, err)
	}
	testutil.WriteFile(t, p, ".mcp.json", `{"mcpServers": {`)
	if _, err := PlanMCP(p, "rebar"); err == nil {
		t.Error("broken .mcp.json: no error")
	}
}
