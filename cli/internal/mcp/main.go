package mcp

import (
	"context"
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
	port, maxAsks         int
	host, reposDir, repos string
	apiKey, askBin        string
	insecureNoAuth        bool
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
		fs.StringVar(&o.host, "host", "127.0.0.1", "Host to bind for HTTP (default: 127.0.0.1; another address needs --api-key)")
	} else {
		fs.IntVar(&o.port, "port", 7232, "Port to listen on (default: 7232)")
		fs.StringVar(&o.host, "host", "127.0.0.1", "Host to bind (default: 127.0.0.1; another address needs --api-key)")
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
	fs.IntVar(&o.maxAsks, "max-asks", DefaultMaxAsks, "HTTP: asks run at once at most; more get 503 (0: no limit)")
	fs.BoolVar(&o.insecureNoAuth, "insecure-no-auth", false, "Allow a non-loopback --host without --api-key (anyone who can reach it can run agents)")
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

// signalled is a context done at the first SIGINT/SIGTERM. Later signals
// are ignored: the shutdown is bounded (ShutdownGrace), and dying on a
// second one would orphan the ask processes it is stopping.
func signalled() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigs:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, func() { signal.Stop(sigs); cancel() }
}

// serveUntilSignal serves h on l until SIGINT/SIGTERM, then shuts down:
// it stops accepting connections and cancels the asks running (stop: their
// process groups get SIGTERM, then SIGKILL after KillDelay, and each
// request answers "cancelled"), lets the requests finish their answers for
// up to ShutdownGrace, then cancels what is left (request contexts) and
// closes the connections, and kills leftover ask processes.
func serveUntilSignal(l net.Listener, h http.Handler, onStop, stop func()) error {
	base, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	srv := newHTTPServer(h)
	srv.BaseContext = func(net.Listener) context.Context { return base }
	sig, unhook := signalled()
	defer unhook()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(l) }()
	select {
	case <-sig.Done():
		onStop()
		shut := make(chan struct{})
		go func() {
			defer close(shut)
			ctx, cancel := context.WithTimeout(context.Background(), ShutdownGrace)
			defer cancel()
			if srv.Shutdown(ctx) != nil {
				cancelRequests()
				srv.Close()
			}
		}()
		stop()
		<-shut
		KillChildren()
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
		// A client that closes its end of stdout must not kill the server
		// with SIGPIPE before it has stopped its asks: writes fail with
		// EPIPE instead, which ends the session like the end of input.
		signal.Ignore(syscall.SIGPIPE)
		fmt.Fprintln(stderr, "ask-mcp-server: starting MCP stdio transport")
		sig, unhook := signalled()
		defer unhook()
		if err := server.ServeStdio(sig, stdin, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "ask-mcp-server: %v\n", err)
			return 1
		}
		return 0
	}
	if err := checkBind(prog, o, stderr); err != nil {
		fmt.Fprintf(stderr, "ask-mcp-server: error: %v\n", err)
		return 2
	}
	server.Remote, server.Log, server.MaxAsks = true, stderr, o.maxAsks
	h, err := NewMCPHandler(server, o.apiKey, o.mcpOnly, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "ask-mcp-server: error: %v\n", err)
		return 1
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
	if err := serveUntilSignal(l, h, func() { fmt.Fprintln(stdout, "\nask-mcp-server: shutting down") }, server.Stop); err != nil {
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
	reg := NewRegistry(prog, stdout, stderr, true)
	fmt.Fprintln(stdout, "ask-server: scanning repos...")
	build(o, reg)
	if reg.Len() == 0 {
		fmt.Fprintln(stderr, "ask-server: error: no repos found with agents/ directories")
		return 1
	}
	fmt.Fprintf(stdout, "\nask-server: %d repos, %d agents\n", reg.Len(), len(reg.Agents()))
	if err := checkBind(prog, o, stderr); err != nil {
		fmt.Fprintf(stderr, "ask-server: error: %v\n", err)
		return 2
	}
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
	h := &AskServer{Registry: reg, APIKey: o.apiKey, AskCmd: askCmd(o.askBin, self), Log: stdout, ErrLog: stderr, MaxAsks: o.maxAsks}
	if err := serveUntilSignal(l, h, func() { fmt.Fprintln(stdout, "\nask-server: shutting down") }, h.Stop); err != nil {
		fmt.Fprintf(stderr, "ask-server: %v\n", err)
		return 1
	}
	return 0
}
