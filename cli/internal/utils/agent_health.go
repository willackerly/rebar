package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/gitx"
)

// agentHealth holds what scripts/agent-health.sh sets up when it loads.
type agentHealth struct {
	ctx      *checks.Context
	dir      string
	id       string
	progress string
}

// agentTmp is where the heartbeat and metrics files go (the script's /tmp).
var agentTmp = "/tmp"

// runAgentHealth is scripts/agent-health.sh run as a command: with
// checkpoint, heartbeat, metric or rmse and their arguments it runs that
// function (agent_checkpoint "msg", agent_heartbeat, agent_metric KEY VALUE,
// agent_rmse DOC BEFORE AFTER); otherwise it prints the line the script
// prints when it loads. (The functions themselves are for sourcing into an
// agent's shell; the script stays sourceable.)
func runAgentHealth(ctx *checks.Context, args []string) int {
	a := &agentHealth{ctx: ctx, dir: ctx.WorkDir()}
	a.id = os.Getenv("AGENT_ID")
	if a.id == "" {
		if top, code := gitx.Run(a.dir, nil, "rev-parse", "--show-toplevel"); code == 0 {
			a.id = shellBasename(strings.TrimRight(top, "\n"))
		} else {
			a.id = "agent-" + strconv.Itoa(os.Getpid())
		}
	}
	a.progress = os.Getenv("AGENT_PROGRESS_FILE")
	if a.progress == "" {
		a.progress = "agent-progress.jsonl"
	}
	if len(args) > 0 {
		switch args[0] {
		case "checkpoint":
			return a.checkpoint(args[1:])
		case "heartbeat":
			return a.heartbeat()
		case "metric":
			return a.metric(args[1:])
		case "rmse":
			return a.rmse(args[1:])
		}
	}
	fmt.Fprintf(ctx.Stdout, "[agent-health] loaded for agent=%s\n", a.id)
	return 0
}

func agentTS() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

// required is ${N:?message}: the argument, or the shell's complaint and
// exit 1 when it is missing or empty.
func (a *agentHealth) required(args []string, n int, msg string) (string, bool) {
	if n <= len(args) && args[n-1] != "" {
		return args[n-1], true
	}
	if msg == "" {
		msg = "parameter null or not set"
	}
	fmt.Fprintf(a.ctx.Stderr, "agent-health: %d: %s\n", n, msg)
	return "", false
}

// appendTo appends text to the file at p (relative to the working
// directory), reporting whether it could.
func (a *agentHealth) appendTo(p, text string) bool {
	f, err := os.OpenFile(resolve(a.dir, p), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err == nil
}

// checkpoint is agent_checkpoint: stage tracked files, commit them, record
// the commit in the progress file.
func (a *agentHealth) checkpoint(args []string) int {
	msg, ok := a.required(args, 1, `Usage: agent_checkpoint "commit message"`)
	if !ok {
		return 1
	}
	if code := run(a.dir, a.ctx.Stdout, a.ctx.Stderr, "git", "add", "-u"); code != 0 {
		return code
	}
	if run(a.dir, nil, a.ctx.Stderr, "git", "diff", "--cached", "--quiet") == 0 {
		fmt.Fprintln(a.ctx.Stdout, "[agent-health] nothing to commit")
		return 0
	}
	if code := run(a.dir, a.ctx.Stdout, a.ctx.Stderr, "git", "commit", "-m", msg); code != 0 {
		return code
	}
	out, code := gitx.Run(a.dir, a.ctx.Stderr, "rev-parse", "--short", "HEAD")
	if code != 0 {
		return code
	}
	hash := strings.TrimRight(out, "\n")
	fmt.Fprintf(a.ctx.Stdout, "[agent-health] checkpoint: %s %s\n", hash, msg)
	entry := fmt.Sprintf(`{"agent":"%s","ts":"%s","type":"checkpoint","commit":"%s","message":"%s"}`, a.id, agentTS(), hash, msg)
	a.appendTo(a.progress, strings.TrimRight(entry, "\n")+"\n")
	return 0
}

// heartbeat is agent_heartbeat: the time, in /tmp/agent-<id>.heartbeat.
func (a *agentHealth) heartbeat() int {
	p := filepath.Join(agentTmp, "agent-"+a.id+".heartbeat")
	if err := os.WriteFile(p, []byte(agentTS()+"\n"), 0o644); err != nil {
		fmt.Fprintf(a.ctx.Stderr, "agent-health: %s: %v\n", p, errText(err))
		return 1
	}
	return 0
}

// metric is agent_metric: a key and a JSON value, in
// /tmp/agent-<id>.metrics.jsonl and the progress file.
func (a *agentHealth) metric(args []string) int {
	usage := `Usage: agent_metric "key" "value"`
	key, ok := a.required(args, 1, usage)
	if !ok {
		return 1
	}
	value, ok := a.required(args, 2, usage)
	if !ok {
		return 1
	}
	return a.writeMetric(key, value)
}

func (a *agentHealth) writeMetric(key, value string) int {
	p := filepath.Join(agentTmp, "agent-"+a.id+".metrics.jsonl")
	if !a.appendTo(p, fmt.Sprintf(`{"agent":"%s","ts":"%s","key":"%s","value":%s}`+"\n", a.id, agentTS(), key, value)) {
		fmt.Fprintf(a.ctx.Stderr, "agent-health: %s: cannot append\n", p)
		return 1
	}
	a.appendTo(a.progress, fmt.Sprintf(`{"agent":"%s","ts":"%s","type":"metric","key":"%s","value":%s}`+"\n", a.id, agentTS(), key, value))
	return 0
}

// rmse is agent_rmse: a before/after RMSE pair for a document, as a metric
// and a progress entry.
func (a *agentHealth) rmse(args []string) int {
	doc, ok := a.required(args, 1, `Usage: agent_rmse "doc" before after`)
	if !ok {
		return 1
	}
	before, ok := a.required(args, 2, "")
	if !ok {
		return 1
	}
	after, ok := a.required(args, 3, "")
	if !ok {
		return 1
	}
	if code := a.writeMetric("rmse", fmt.Sprintf(`{"doc":"%s","before":%s,"after":%s}`, doc, before, after)); code != 0 {
		return code
	}
	a.appendTo(a.progress, fmt.Sprintf(`{"agent":"%s","ts":"%s","type":"rmse","doc":"%s","rmse_before":%s,"rmse_after":%s}`+"\n", a.id, agentTS(), doc, before, after))
	return 0
}

func errText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
