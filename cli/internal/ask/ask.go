// Package ask is the native (Go) implementation of the ASK CLI, bin/ask
// (CONTRACT:S2-ASK-CLI.1.0), and of its agent loop, bin/ask-agent-loop:
// the same subcommands, flags, environment variables, output, exit codes
// and on-disk state (agents/<role>/{AGENT.md,memory.md,memory.log.md,
// .session-id,.pid,.progress,.last-stderr,inbox/,outbox/},
// shared/messages/events.jsonl, ~/.config/ask/projects and .rr-index).
// The LLM is the claude CLI found on PATH, invoked with the script's
// arguments, so a stub claude on PATH stands in for it in tests.
//
// The JSON the script built with jq is written byte for byte as jq writes
// it (package jqjson), and no jq is needed. Where the script's behaviour
// depended on the platform (GNU vs BSD sed/stat, bash 3.2 quirks) or it
// crashed, the port does what the script evidently meant; CHANGELOG lists
// those deviations.
package ask

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/willackerly/rebar/cli/internal/checks"
)

// Version is ASK's own version, reported by `ask --version`.
const Version = "1.0.0"

// rebarVersionFallback is reported by --version outside a project with a
// .rebar-version file.
const rebarVersionFallback = "v2.0.0"

// HandOffEnv is set by the bin/ask shim when it execs the rebar binary, so
// that an older rebar, whose `ask` runs bin/ask, cannot loop. Main clears
// it, so that the agent commands and servers ASK starts see a clean
// environment.
const HandOffEnv = "REBAR_ASK_HANDOFF"

// exit unwinds Run with an exit code, as the script's `exit N` did from
// any depth.
type exit int

// App is one run of the CLI.
type App struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Exe is the rebar binary, which background compaction runs.
	Exe string

	stdoutTTY bool
	stdinTTY  bool
	pwd       string // the logical working directory ($PWD)

	agentsDir, sharedDir, eventsLog string
	timeout                         string
	verbose, debug, writeMode       bool
	model, contextLimit             string
	registry, server, apiKey        string

	resolvedProjectDir string
	canonicalAgent     string
	c                  colours
}

// Main runs the CLI on args (after "ask") with the process's standard
// streams and returns the exit code.
func Main(args []string) int {
	os.Unsetenv(HandOffEnv)
	exe, _ := os.Executable()
	a := New(os.Stdin, os.Stdout, os.Stderr)
	a.Exe = exe
	return a.Run(args)
}

// RunIn runs the CLI on args in dir, as a child process started there
// would (PWD=dir), and restores the working directory afterwards.
func RunIn(dir string, args []string, stdout, stderr io.Writer) int {
	prev, err := os.Getwd()
	if err != nil {
		return 4
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintf(stderr, "ask: cd: %s: %v\n", dir, err)
		return 4
	}
	defer os.Chdir(prev)
	a := New(nil, stdout, stderr)
	a.pwd = dir
	return a.Run(args)
}

// New reads the settings from the environment, as the script's
// Configuration section does.
func New(stdin io.Reader, stdout, stderr io.Writer) *App {
	a := &App{Stdin: stdin, Stdout: stdout, Stderr: stderr}
	a.stdoutTTY = checks.IsTerminal(stdout)
	if f, ok := stdin.(*os.File); ok {
		a.stdinTTY = checks.IsTerminal(f)
	}
	a.pwd, _ = os.Getwd()
	a.agentsDir = envOr("ASK_AGENTS_DIR", "./agents")
	a.sharedDir = envOr("ASK_SHARED_DIR", "./shared")
	a.timeout = envOr("ASK_TIMEOUT", "120")
	a.verbose = envOr("ASK_VERBOSE", "0") == "1"
	a.debug = envOr("ASK_DEBUG", "0") == "1"
	a.model = envOr("ASK_MODEL", "sonnet")
	a.contextLimit = envOr("ASK_CONTEXT_LIMIT", "70")
	a.writeMode = envOr("ASK_WRITE", "0") == "1"
	a.registry = envOr("ASK_REGISTRY", os.Getenv("HOME")+"/.config/ask/projects")
	a.server = os.Getenv("ASK_SERVER")
	a.apiKey = os.Getenv("ASK_API_KEY")
	a.eventsLog = a.sharedDir + "/messages/events.jsonl"
	a.c = newColours(os.Getenv("NO_COLOR") == "" && a.stdoutTTY)
	return a
}

// envOr is ${name:-def}.
func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// Run runs the CLI on args and returns the exit code.
func (a *App) Run(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(exit)
			if !ok {
				panic(r)
			}
			code = int(e)
		}
	}()
	a.autoRegister()
	args = a.parseFlags(args)
	a.dispatch(args)
	return 0
}

func (a *App) exit(code int) { panic(exit(code)) }

// out and errf write to stdout and stderr.
func (a *App) out(format string, args ...any) { fmt.Fprintf(a.Stdout, format, args...) }
func (a *App) errf(format string, args ...any) {
	fmt.Fprintf(a.Stderr, format, args...)
}

// log, dbg and die are the script's helpers of the same names.
func (a *App) log(format string, args ...any) {
	if a.verbose {
		a.errf("[ask] "+format+"\n", args...)
	}
}

func (a *App) dbg(format string, args ...any) {
	if a.debug {
		a.errf("[ask:debug] "+format+"\n", args...)
	}
}

func (a *App) die(format string, args ...any) {
	a.errf("ask: error: "+format+"\n", args...)
	a.exit(4)
}

// parseFlags consumes the leading flags and returns the rest.
func (a *App) parseFlags(args []string) []string {
	for len(args) > 0 {
		arg := args[0]
		switch {
		case arg == "-v" || arg == "--verbose":
			a.verbose = true
		case arg == "-d" || arg == "--debug":
			a.debug, a.verbose = true, true
		case arg == "-w" || arg == "--write":
			a.writeMode = true
		case arg == "-V" || arg == "--version":
			v := rebarVersionFallback
			if data, err := os.ReadFile(".rebar-version"); err == nil {
				v = strings.NewReplacer(" ", "", "\n", "").Replace(string(data))
			}
			a.out("ask v%s (%s)\n", Version, v)
			a.exit(0)
		case arg == "-h" || arg == "--help":
			// The script's help case could not see these: its flag loop
			// rejected them first ("unknown flag '--help'").
			return []string{"help"}
		case arg == "--":
			return args[1:]
		case strings.HasPrefix(arg, "--") || (strings.HasPrefix(arg, "-") && utf8.RuneCountInString(arg) == 2):
			a.errf("ask: error: unknown flag '%s'\n\nTry: ask --help\n", arg)
			a.exit(4)
		default:
			return args
		}
		args = args[1:]
	}
	return args
}

func arg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

// dispatch is the script's Main Dispatch.
func (a *App) dispatch(args []string) {
	cmd := arg(args, 0)
	if cmd == "" {
		cmd = "help"
	}
	second := arg(args, 1)
	switch cmd {
	case "help", "--help", "-h":
		a.out("%s", helpText)
	case "who":
		a.who()
	case "where":
		a.where(second)
	case "status":
		if a.isRemoteQuery(second) {
			a.remoteStatus(splitSpec(second))
		} else {
			a.status(second)
		}
	case "log":
		if a.isRemoteQuery(second) {
			a.remoteLog(splitSpec(second))
		} else {
			a.showLog(second)
		}
	case "peek":
		a.peek(second)
	case "tail":
		a.tail()
	case "up":
		a.up(second)
	case "watch":
		a.watch(second)
	case "reset":
		if a.isRemoteQuery(second) {
			a.remoteReset(splitSpec(second))
		} else {
			a.reset(second)
		}
	case "compact":
		a.compact(second)
	case "register":
		a.register(second)
	case "projects":
		a.formatProjects()
	case "init":
		a.initProject()
	case "serve":
		a.serve(args[1:])
	case "agents":
		a.remoteAgents()
	default:
		a.askOrCommand(cmd, args[1:])
	}
	a.exit(0)
}

// splitSpec splits "repo:role" at the first colon.
func splitSpec(spec string) (string, string) {
	repo, role, _ := strings.Cut(spec, ":")
	return repo, role
}

// askOrCommand is the dispatcher's default case: ask a question or run an
// agent command.
func (a *App) askOrCommand(spec string, rest []string) {
	if a.isRemoteQuery(spec) {
		repo, role := splitSpec(spec)
		question := arg(rest, 0)
		if question == "" {
			a.errf("ask: error: question required for remote agent\nUsage: ask %s:%s \"<question>\"\n", repo, role)
			a.exit(4)
		}
		a.remoteAsk(repo, role, question)
		a.exit(0)
	}

	agent := a.resolveProjectAgent(spec)
	a.requireAgentExists(agent)
	agent = a.canonicalAgent
	agentDir := a.agentsDir + "/" + agent

	word := arg(rest, 0)
	if word == "" {
		if isExecutable(agentDir + "/commands/default.sh") {
			a.exit(a.runCommand(agentDir+"/commands/default.sh", nil))
		}
		if !a.stdinTTY && a.Stdin != nil {
			data, _ := io.ReadAll(a.Stdin)
			word = strings.TrimRight(string(data), "\n")
		}
		if word == "" {
			a.errf("ask: error: question or command required\n\nUsage:\n")
			a.errf("  ask %s \"<question>\"     — ask a question\n", agent)
			a.errf("  ask %s <command>          — run a command\n", agent)
			a.listAgentCommands(agent)
			a.exit(4)
		}
	}

	switch word {
	case "-v", "-d", "-w", "-V", "--verbose", "--debug", "--write", "--version", "--help", "-h":
		a.errf("ask: error: flag '%s' must come before the agent name\n\nUsage: ask %s %s \"<question>\"\n", word, word, agent)
		a.exit(4)
	}

	if script := agentDir + "/commands/" + word + ".sh"; isExecutable(script) {
		a.exit(a.runCommand(script, rest[1:]))
	}
	a.ask(agent, word)
}

// isExecutable is `[ -x path ]`.
func isExecutable(path string) bool {
	_, err := os.Stat(path)
	return err == nil && accessOK(path)
}

// isFile is `[ -f path ]` and isDir `[ -d path ]`.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// basename is basename(1).
func basename(p string) string {
	t := strings.TrimRight(p, "/")
	if t == "" {
		if p == "" {
			return ""
		}
		return "/"
	}
	return filepath.Base(t)
}

// atoi is the shell's reading of a number in [ ] tests: ok is false when
// s is not an integer.
func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil
}

const helpText = `ASK Scoped Knowledge — query an agent

Usage:
  ask <agent> "<question>"     Send a question, get a concise answer
  ask -v <agent> "<question>"  Verbose: answer + rationale + refs
  ask -w <agent> "<question>"  Write mode: agent can edit/create files
  ask project:agent "question" Query an agent in another project
  ask <agent>                    Run agent's default command (if configured)
  ask <agent> <command> [args]   Run a named agent command
  ask watch <agent>            Follow agent's incremental progress
  ask up <agent>               Bring an agent online
  ask who                      List available agents
  ask status <agent>           Check agent health
  ask log <agent>              View interaction history
  ask peek <agent>             Inspect pending inbox messages
  ask tail <agent>             Follow live event log
  ask where <agent>            Resolve agent directory path
  ask reset <agent>            Clear session (start fresh next question)
  ask compact <agent>          Compact memory log (summarize old entries)
  ask register [name]          Register current project (default: dir name)
  ask projects                 List registered projects
  ask init                     Initialize directory structure
  ask help                     Show this help

Enterprise (server mode):
  ask serve [flags]            Start enterprise server (see ask serve --help)
  ask agents                   List agents on remote server

Exit codes:
  0  Answer returned
  1  Agent could not answer
  2  Agent not available
  3  Timeout
  4  Internal error

Environment variables:
  ASK_AGENTS_DIR   Agents directory (default: ./agents)
  ASK_SHARED_DIR   Shared directory (default: ./shared)
  ASK_TIMEOUT      Timeout in seconds (default: 120)
  ASK_CONTEXT_LIMIT  Auto-reset threshold, percent (default: 70)
  ASK_VERBOSE      Set to 1 for debug output
  ASK_WRITE        Set to 1 to enable write mode by default
  ASK_SERVER       Remote server host:port (routes repo:agent queries over HTTP)
  ASK_API_KEY      API key for remote server auth
`
