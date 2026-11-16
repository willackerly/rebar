package ask

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/willackerly/rebar/cli/internal/jqjson"
)

// Loop is an agent's inbox loop: `ask up <agent>` without a run.sh, and
// bin/ask-agent-loop. It answers each inbox/<id>.json question with claude
// into outbox/<id>.json, logs the answer to the event log and
// memory.log.md, and removes the question, until SIGINT or SIGTERM.
type Loop struct {
	Name      string // the agent, as the messages name it
	Dir       string // the agent directory, as printed
	EventsLog string
	Model     string
	Stdout    io.Writer
	Stderr    io.Writer
	// Interval is the pause between inbox scans (default 2s).
	Interval time.Duration
	// Stop, when closed, ends the loop as a signal does (tests).
	Stop <-chan struct{}
}

// loopFallback is the outbox answer when claude fails.
const loopFallback = `{"answer": "error", "rationale": "claude invocation failed", "refs": []}`

// Run runs the loop and returns its exit status.
func (l *Loop) Run() int {
	inbox, outbox := l.Dir+"/inbox", l.Dir+"/outbox"
	_ = os.MkdirAll(inbox, 0o755)
	_ = os.MkdirAll(outbox, 0o755)
	pidfile := l.Dir + "/.pid"
	_ = os.WriteFile(pidfile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	offline := func() int {
		// The script's trap ran on the signal and again on the exit it made.
		for i := 0; i < 2; i++ {
			os.Remove(pidfile)
			fmt.Fprintf(l.Stdout, "[%s] offline\n", l.Name)
		}
		return 0
	}

	fmt.Fprintf(l.Stdout, "[%s] online, watching %s (pid %d)\n", l.Name, inbox, os.Getpid())
	interval := l.Interval
	if interval == 0 {
		interval = 2 * time.Second
	}
	for {
		for _, msg := range globJSON(inbox) {
			l.answer(msg, outbox)
			select {
			case <-sigs:
				return offline()
			case <-l.Stop:
				return offline()
			default:
			}
		}
		select {
		case <-sigs:
			return offline()
		case <-l.Stop:
			return offline()
		case <-time.After(interval):
		}
	}
}

// answer handles one inbox message.
func (l *Loop) answer(msg, outbox string) {
	data, _ := os.ReadFile(msg)
	id := jqRaw(data, []string{"id"}, nil, "unknown")
	question := jqRaw(data, []string{"body"}, nil, "")
	from := jqRaw(data, []string{"from"}, "unknown", "unknown")
	fmt.Fprintf(l.Stdout, "[%s] processing (%s): %s\n", l.Name, id, question)

	context := agentContext(l.Dir)
	prompt := "You are the " + l.Name + " agent.\n\nYour role and context:\n" + context +
		"\n\nQuestion from " + from + ": " + question + `

Respond as JSON with these fields:
{
  "answer": "your concise answer",
  "rationale": "your reasoning",
  "refs": ["relevant/file.md"]
}

Rules:
- answer should be direct and concise
- rationale explains your reasoning briefly
- refs lists relevant files (empty array if none)
- respond ONLY with the JSON object, no other text`

	cmd := exec.Command("claude", "--print", "--model", l.Model, prompt)
	out, err := cmd.Output()
	response := string(out)
	if err != nil {
		response += loopFallback + "\n"
	}
	response = strings.TrimRight(response, "\n")

	_ = os.WriteFile(outbox+"/"+id+".json", []byte(response+"\n"), 0o644)
	ts := timestamp()
	appendEvent(l.EventsLog, event(id, l.Name, from, "answer", ts, response))
	first := response
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	answer := jqRaw([]byte(response+"\n"), []string{"answer"}, nil, first)
	appendFile(l.Dir+"/memory.log.md", "---\n**"+ts+"** Q: "+question+"\nA: "+answer+"\n")
	os.Remove(msg)
	fmt.Fprintf(l.Stdout, "[%s] answered: %s\n", l.Name, id)
}

// agentContext is AGENT.md, then memory.md under "--- Current Memory ---".
func agentContext(dir string) string {
	context, _ := readTrimmed(dir + "/AGENT.md")
	if memory, ok := readTrimmed(dir + "/memory.md"); ok && isFile(dir+"/memory.md") {
		context += "\n\n--- Current Memory ---\n" + memory
	}
	return context
}

// event is the event-log entry jq -n builds: {id, from, to, type,
// timestamp, body}, pretty-printed.
func event(id, from, to, typ, ts, body string) string {
	return jqjson.Pretty(jqjson.Object{
		{Key: "id", Val: jqjson.RepairUTF8(id)},
		{Key: "from", Val: jqjson.RepairUTF8(from)},
		{Key: "to", Val: jqjson.RepairUTF8(to)},
		{Key: "type", Val: typ},
		{Key: "timestamp", Val: ts},
		{Key: "body", Val: jqjson.RepairUTF8(body)},
	})
}

// appendEvent appends an entry to the event log, creating its directory.
func appendEvent(log, entry string) {
	_ = os.MkdirAll(filepath.Dir(log), 0o755)
	appendFile(log, entry)
}

func appendFile(path, text string) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	f.WriteString(text)
	f.Close()
}

// RunAgentLoop is bin/ask-agent-loop: the loop for the agent directory
// args[0] (default "."), its shared directory ASK_SHARED_DIR or the one
// beside its agents/ directory.
func RunAgentLoop(args []string, stdout, stderr io.Writer) int {
	os.Unsetenv(HandOffEnv)
	dir := "."
	if len(args) > 0 && args[0] != "" {
		dir = args[0]
	}
	// $(cd "$dir" && pwd): the logical path.
	abs := filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		wd, _ := os.Getwd()
		abs = filepath.Clean(filepath.Join(wd, dir))
	}
	if !isDir(abs) {
		fmt.Fprintf(stderr, "ask-agent-loop: cd: %s: No such file or directory\n", dir)
		return 1
	}
	name := filepath.Base(abs)
	shared := os.Getenv("ASK_SHARED_DIR")
	if shared == "" {
		shared = filepath.Dir(filepath.Dir(abs)) + "/shared"
	}

	if !isFile(abs + "/AGENT.md") {
		fmt.Fprintf(stderr, "Error: %s/AGENT.md not found\n", abs)
		return 4
	}
	if _, err := exec.LookPath("claude"); err != nil {
		fmt.Fprintln(stderr, "Error: claude CLI not found")
		return 4
	}
	loop := &Loop{
		Name: name, Dir: abs, EventsLog: shared + "/messages/events.jsonl",
		Model: envOr("ASK_MODEL", "sonnet"), Stdout: stdout, Stderr: stderr,
	}
	return loop.Run()
}
