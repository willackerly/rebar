package mcp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/pflag"
)

// options are the flags both servers take (argparse in the originals).
type options struct {
	stdio, mcpOnly        bool
	port                  int
	host, reposDir, repos string
	apiKey, askBin        string
}

// parse reads args as the python argparse parsers did; it returns the
// exit code when the run should end (help: 0, usage error: 2).
func parse(prog, description, epilog string, mcp bool, args []string, stdout, stderr io.Writer) (*options, int, bool) {
	o := &options{}
	fs := pflag.NewFlagSet(prog, pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	help := fs.BoolP("help", "h", false, "show this help message and exit")
	if mcp {
		fs.BoolVar(&o.stdio, "stdio", false, "Use stdio transport (for MCP clients)")
		fs.IntVar(&o.port, "port", 7232, "Port for HTTP transport (default: 7232)")
		fs.StringVar(&o.host, "host", "0.0.0.0", "Host to bind for HTTP (default: 0.0.0.0)")
	} else {
		fs.IntVar(&o.port, "port", 7232, "Port to listen on (default: 7232)")
		fs.StringVar(&o.host, "host", "0.0.0.0", "Host to bind (default: 0.0.0.0)")
	}
	fs.StringVar(&o.reposDir, "repos-dir", "", "Directory containing repos (each subdir with agents/)")
	fs.StringVar(&o.repos, "repos", "", "Comma-separated paths to specific repos")
	if mcp {
		fs.BoolVar(&o.mcpOnly, "mcp-only", false, "Only serve MCP endpoints, not classic ASK API")
		fs.StringVar(&o.apiKey, "api-key", os.Getenv("ASK_API_KEY"), "API key for auth")
		fs.StringVar(&o.askBin, "ask-bin", os.Getenv("ASK_BIN"), "Path to ask CLI (default: rebar ask)")
	} else {
		fs.StringVar(&o.apiKey, "api-key", os.Getenv("ASK_API_KEY"), "API key for auth (or set ASK_API_KEY)")
		fs.StringVar(&o.askBin, "ask-bin", os.Getenv("ASK_BIN"), "Path to ask CLI (default: rebar ask)")
	}
	fs.SortFlags = false
	usage := "usage: " + prog + " [-h] " + strings.Join(usageFlags(fs), " ") + "\n"
	err := fs.Parse(args)
	if err == nil && fs.NArg() > 0 {
		err = fmt.Errorf("unrecognized arguments: %s", strings.Join(fs.Args(), " "))
	}
	if err != nil {
		fmt.Fprint(stderr, usage)
		fmt.Fprintf(stderr, "%s: error: %v\n", prog, err)
		return nil, 2, true
	}
	if *help {
		fmt.Fprintf(stdout, "%s\n%s\n\noptions:\n%s%s", usage, description, fs.FlagUsages(), epilog)
		return nil, 0, true
	}
	if o.reposDir == "" && o.repos == "" {
		fmt.Fprint(stderr, usage)
		fmt.Fprintf(stderr, "%s: error: Provide --repos-dir or --repos\n", prog)
		return nil, 2, true
	}
	return o, 0, false
}

func usageFlags(fs *pflag.FlagSet) []string {
	var out []string
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		if f.Value.Type() == "bool" {
			out = append(out, "[--"+f.Name+"]")
		} else {
			out = append(out, "[--"+f.Name+" "+strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))+"]")
		}
	})
	return out
}

// askCmd is the ask command tools/call and /v1/ask run: --ask-bin
// (ASK_BIN), else this binary's `ask`.
func askCmd(askBin, self string) []string {
	if askBin != "" {
		return []string{askBin}
	}
	return []string{self, "ask"}
}

// build fills a registry from the options.
func build(o *options, reg *Registry) {
	if o.reposDir != "" {
		reg.ScanDir(o.reposDir)
	}
	if o.repos != "" {
		reg.AddRepos(strings.Split(o.repos, ","))
	}
}

// listen binds host:port.
func listen(o *options) (net.Listener, error) {
	return net.Listen("tcp", net.JoinHostPort(o.host, strconv.Itoa(o.port)))
}

// serveUntilSignal serves h on l until SIGINT/SIGTERM.
func serveUntilSignal(l net.Listener, h http.Handler, onStop func()) error {
	srv := &http.Server{Handler: h}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(l) }()
	select {
	case <-sigs:
		onStop()
		srv.Close()
		return nil
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// ServeMain is bin/ask-mcp-server (`rebar mcp serve`): self is this
// binary (the default ask command is `<self> ask`).
func ServeMain(args []string, self string, stdin io.Reader, stdout, stderr io.Writer) int {
	const prog = "ask-mcp-server"
	o, code, done := parse(prog, "MCP-enabled ASK server", `
Examples:
  ask-mcp-server --port 7232 --repos-dir /srv/repos
  ask-mcp-server --stdio
  ask-mcp-server --port 7232 --repos-dir /srv/repos --mcp-only
`, true, args, stdout, stderr)
	if done {
		return code
	}
	reg := NewRegistry(prog, stderr, stderr, true)
	if v := os.Getenv("ASK_SERVER"); v != "" {
		fmt.Fprintf(stderr, "ask-mcp-server: note: ASK_SERVER=%s is set in env; tool/call subprocesses will clear it and route locally\n", v)
	}
	fmt.Fprintln(stderr, "ask-mcp-server: scanning repos...")
	build(o, reg)
	if reg.Len() == 0 {
		fmt.Fprintln(stderr, "ask-mcp-server: error: no repos found with agents/ directories")
		return 1
	}
	fmt.Fprintf(stderr, "\nask-mcp-server: %d repos, %d agents\n", reg.Len(), len(reg.Agents()))
	server := &Server{Registry: reg, AskCmd: askCmd(o.askBin, self)}
	if o.stdio {
		fmt.Fprintln(stderr, "ask-mcp-server: starting MCP stdio transport")
		if err := server.ServeStdio(stdin, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "ask-mcp-server: %v\n", err)
			return 1
		}
		return 0
	}
	l, err := listen(o)
	if err != nil {
		fmt.Fprintf(stderr, "ask-mcp-server: error: %v\n", err)
		return 1
	}
	mode, auth := "MCP + classic ASK API", "no auth"
	if o.mcpOnly {
		mode = "MCP-only"
	}
	if o.apiKey != "" {
		auth = "with API key"
	}
	fmt.Fprintf(stdout, "ask-mcp-server: listening on %s:%d (%s, %s)\n", o.host, o.port, mode, auth)
	fmt.Fprintf(stdout, "ask-mcp-server: MCP endpoint: http://localhost:%d/mcp\n", o.port)
	if !o.mcpOnly {
		fmt.Fprintf(stdout, "ask-mcp-server: classic API: http://localhost:%d/v1/agents\n", o.port)
	}
	fmt.Fprintln(stdout)
	h := &MCPHandler{Server: server, APIKey: o.apiKey, MCPOnly: o.mcpOnly, Log: stderr}
	if err := serveUntilSignal(l, h, func() { fmt.Fprintln(stdout, "\nask-mcp-server: shutting down") }); err != nil {
		fmt.Fprintf(stderr, "ask-mcp-server: %v\n", err)
		return 1
	}
	return 0
}

// AskServerMain is bin/ask-server (`rebar ask serve`).
func AskServerMain(args []string, self string, stdout, stderr io.Writer) int {
	const prog = "ask-server"
	o, code, done := parse(prog, "Enterprise ASK server — host agents from multiple repos on one endpoint", `
Examples:
  ask-server --port 7232 --repos-dir /srv/repos
  ask-server --port 7232 --repos /srv/repos/billing,/srv/repos/auth
  ask-server --port 7232 --repos-dir /srv/repos --api-key SECRET

Then from any machine:
  ASK_SERVER=host:7232 ask billing:architect "What contracts exist?"
`, false, args, stdout, stderr)
	if done {
		return code
	}
	reg := NewRegistry(prog, stdout, stderr, false)
	fmt.Fprintln(stdout, "ask-server: scanning repos...")
	build(o, reg)
	if reg.Len() == 0 {
		fmt.Fprintln(stderr, "ask-server: error: no repos found with agents/ directories")
		return 1
	}
	fmt.Fprintf(stdout, "\nask-server: %d repos, %d agents\n", reg.Len(), len(reg.Agents()))
	l, err := listen(o)
	if err != nil {
		fmt.Fprintf(stderr, "ask-server: error: %v\n", err)
		return 1
	}
	auth := "no auth (use --api-key for production)"
	if o.apiKey != "" {
		auth = "with API key"
	}
	fmt.Fprintf(stdout, "ask-server: listening on %s:%d (%s)\n", o.host, o.port, auth)
	fmt.Fprintf(stdout, "ask-server: try: curl http://localhost:%d/v1/agents\n\n", o.port)
	h := &AskServer{Registry: reg, APIKey: o.apiKey, AskCmd: askCmd(o.askBin, self), Log: stdout}
	if err := serveUntilSignal(l, h, func() { fmt.Fprintln(stdout, "\nask-server: shutting down") }); err != nil {
		fmt.Fprintf(stderr, "ask-server: %v\n", err)
		return 1
	}
	return 0
}
