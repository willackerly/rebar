package ask

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/willackerly/rebar/cli/internal/jqjson"
)

// ask is cmd_ask: answer through the running agent's inbox, else invoke
// claude directly.
func (a *App) ask(agent, question string) {
	if agent == "featurerequest" {
		// Write-capable role (gated intake, CHARTER §2.9 / §3).
		a.writeMode = true
		a.log("Auto-enabling write mode for write-capable role: %s", agent)
	}
	a.requireAgentExists(agent)
	dir := a.agentsDir + "/" + agent
	_ = os.MkdirAll(dir+"/inbox", 0o755)
	_ = os.MkdirAll(dir+"/outbox", 0o755)
	if _, alive := alivePID(dir + "/.pid"); alive {
		a.askViaInbox(agent, question)
	} else {
		a.askDirect(agent, question)
	}
}

func (a *App) requireClaude() {
	if _, err := exec.LookPath("claude"); err != nil {
		a.die("claude CLI is required but not found. See: https://docs.anthropic.com/en/docs/claude-code")
	}
}

// msgID is ask-<local date>-<time>-<pid>-<$RANDOM>.
func msgID() string {
	return fmt.Sprintf("ask-%s-%d-%d", time.Now().Format("20060102-150405"), os.Getpid(), rand.Intn(32768))
}

func newSessionID() string { return strings.ToLower(uuid.NewString()) }

func writeLine(path, line string) { _ = os.WriteFile(path, []byte(line+"\n"), 0o644) }

// askDirect is _ask_direct: one claude invocation that resumes the
// agent's session (.session-id) or starts one with the full context, with
// one retry on a fresh session when claude no longer knows the old one.
func (a *App) askDirect(agent, question string) {
	dir := a.agentsDir + "/" + agent
	id := msgID()
	a.requireClaude()
	a.log("Direct invocation for agent '%s'", agent)

	sessionFile := dir + "/.session-id"
	hasSession := false
	sessionID := ""
	if isFile(sessionFile) {
		sessionID, _ = readTrimmed(sessionFile)
		hasSession = true
		a.log("Resuming session %s", sessionID)
	} else {
		sessionID = newSessionID()
		writeLine(sessionFile, sessionID)
		a.log("Created new session %s", sessionID)
	}
	a.dbg("session=%s has_session=%t", sessionID, hasSession)

	ts := timestamp()
	appendEvent(a.eventsLog, event(id, "human", agent, "question", ts, question))
	a.log("Invoking claude...")

	if a.resolvedProjectDir != "" {
		a.log("Changing to project dir: %s", a.resolvedProjectDir)
		if err := os.Chdir(a.resolvedProjectDir); err != nil {
			a.errf("ask: cd: %s: %v\n", a.resolvedProjectDir, err)
			a.exit(1)
		}
		a.pwd = cleanLogical(a.pwd, a.resolvedProjectDir)
		os.Setenv("PWD", a.pwd)
	}

	stderrFile := dir + "/.last-stderr"
	var raw string
	for attempt := 1; ; attempt++ {
		prompt := a.prompt(agent, dir, question, hasSession)
		args := []string{"--print", "--model", a.model}
		if hasSession {
			args = append(args, "--resume", sessionID)
		} else {
			args = append(args, "--session-id", sessionID)
		}
		if a.writeMode {
			args = append(args, "--allowedTools", "Read Grep Glob Edit Write Bash", "--permission-mode", "acceptEdits")
			a.log("Write mode enabled — agent can edit/create files")
			args = append(args, "--output-format", "stream-json", "--verbose")
		} else {
			args = append(args, "--output-format", "json")
		}
		a.dbg("claude %s <prompt> (attempt %d)", strings.Join(args, " "), attempt)
		args = append(args, prompt)

		var ok bool
		if a.writeMode {
			a.initProgress(agent, question)
			raw, ok = a.runClaudeStreaming(agent, args, stderrFile)
			if ok {
				a.finishProgress(agent, "done")
			} else {
				a.finishProgress(agent, "error")
			}
		} else {
			raw, ok = runClaude(args, stderrFile)
		}
		if ok {
			break
		}
		stderrText, _ := os.ReadFile(stderrFile)
		if attempt < 2 && hasSession && strings.Contains(string(stderrText), "No conversation found with session ID") {
			a.errf("[%s] session expired, restarting fresh\n", agent)
			a.log("Stale session %s — rotating and replaying with full context", sessionID)
			os.Remove(sessionFile)
			sessionID = newSessionID()
			writeLine(sessionFile, sessionID)
			hasSession = false
			continue
		}
		a.errf("ask: error: claude invocation failed\n")
		if isFile(stderrFile) {
			a.Stderr.Write(stderrText)
		}
		a.exit(1)
	}

	if newID := jqRaw([]byte(raw), []string{"session_id"}, jqEmpty, ""); newID != "" {
		writeLine(sessionFile, newID)
		a.log("Session saved: %s", newID)
	}
	if a.debug {
		a.dbg("%s", tokenUsage(raw))
		newID := jqRaw([]byte(raw), []string{"session_id"}, jqEmpty, "")
		if newID == "" {
			newID = "none"
		}
		a.dbg("session_id returned: %s", newID)
	}

	a.checkContextLimit(agent, raw)
	response := responseText(raw)
	appendEvent(a.eventsLog, event(id, agent, "human", "answer", timestamp(), response))
	first, _, _ := strings.Cut(response, "\n")
	_ = os.MkdirAll(dir, 0o755)
	appendMemoryLog(dir, "---\n**"+ts+"** Q: "+question+"\nA: "+first+"\n")
	a.maybeCompact(agent)
	a.formatResponse(agent, response, "")
	a.exit(0)
}

// cleanLogical is the logical working directory after `cd target` from
// pwd.
func cleanLogical(pwd, target string) string {
	if strings.HasPrefix(target, "/") {
		return cleanPath(target)
	}
	return cleanPath(pwd + "/" + target)
}

func cleanPath(p string) string {
	var parts []string
	for _, c := range strings.Split(p, "/") {
		switch c {
		case "", ".":
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, c)
		}
	}
	return "/" + strings.Join(parts, "/")
}

// prompt builds the question for claude: a follow-up in a resumed
// session, else the agent's full context and the answer format.
func (a *App) prompt(agent, dir, question string, hasSession bool) string {
	if hasSession {
		hint := ""
		if a.writeMode {
			hint = "\n\nYou have write access. Use Edit, Write, and Bash tools to make changes directly."
		}
		if a.verbose {
			return question + hint + "\n\nRespond with: a concise answer paragraph, then a blank line, then\n" +
				"1-3 sentences of reasoning, then a blank line, then relevant file\n" +
				"paths as a bulleted list. No labels."
		}
		return question + hint + "\n\nRespond with a single concise paragraph — the answer itself, nothing else."
	}
	context := ""
	if isFile(dir + "/AGENT.md") {
		context, _ = readTrimmed(dir + "/AGENT.md")
	}
	if isFile(dir + "/memory.md") {
		memory, _ := readTrimmed(dir + "/memory.md")
		context += "\n\n--- Current Memory ---\n" + memory
	}
	tools := "You have full access to the project filesystem. Use Read, Grep, and Glob\n" +
		"tools to look up files, search code, and verify claims before answering.\n" +
		"Do not guess — look it up."
	if a.writeMode {
		tools = "You have full access to the project filesystem including write access.\n" +
			"Use Read, Grep, and Glob to research. Use Edit and Write to modify or\n" +
			"create files. Use Bash for shell commands (tests, builds, git).\n" +
			"Do not guess — look it up, then act on it."
	}
	head := "You are the " + agent + " agent.\n\nYour role and context:\n" + context + "\n\n" + tools +
		"\n\nQuestion: " + question + "\n\n"
	if a.verbose {
		return head + `Respond in this exact format (plain text, no labels or prefixes):

<one concise paragraph answering the question directly>

<blank line>
<1-3 sentences of reasoning — how you arrived at this answer>

<blank line>
- <relevant file path or document>
- <another ref if applicable>

Rules:
- First paragraph is the answer — direct, no preamble, no label
- Second block is reasoning — only if it adds value
- Third block is file references — only list files that exist and matter
- If you cannot answer, say so directly and explain why`
	}
	return head + `Rules:
- Respond with a single concise paragraph — the answer itself, nothing else
- No labels, no preamble, no "ANSWER:" prefix
- No rationale or references unless the question explicitly asks for them
- Be direct and specific`
}

// runClaude runs claude with args, its stderr to stderrFile, and returns
// its output without trailing newlines and whether it succeeded.
func runClaude(args []string, stderrFile string) (string, bool) {
	errf, err := os.Create(stderrFile)
	if err != nil {
		return "", false
	}
	defer errf.Close()
	cmd := exec.Command("claude", args...)
	cmd.Stderr = errf
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err == nil
}

// runClaudeStreaming is the write-mode pipeline `claude … | _stream_progress`:
// claude's stream-json events become progress lines, and the last result
// event is the output.
func (a *App) runClaudeStreaming(agent string, args []string, stderrFile string) (string, bool) {
	errf, err := os.Create(stderrFile)
	if err != nil {
		return "", false
	}
	defer errf.Close()
	cmd := exec.Command("claude", args...)
	cmd.Stderr = errf
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", false
	}
	if err := cmd.Start(); err != nil {
		return "", false
	}
	result := a.streamProgress(agent, pipe)
	err = cmd.Wait()
	return result, err == nil
}

// jqEmpty, as jqRaw's default, is jq's `// empty`: no line for a null or
// false value.
var jqEmpty = struct{ empty bool }{true}

// lastContent is .message.content[-1] of a stream event (ok false where jq
// errors).
func lastContent(v any) (any, bool) {
	c, ok := jqjson.Path(v, "message", "content")
	if !ok {
		return nil, false
	}
	switch x := c.(type) {
	case nil:
		return nil, true
	case []any:
		if len(x) == 0 {
			return nil, true
		}
		return x[len(x)-1], true
	}
	return nil, false
}

// firstTruthy is jq's `a // b // … // empty` over paths below v: the first
// truthy one, nil when none or where jq fails (callers end in || true).
func firstTruthy(v any, paths ...[]string) any {
	for _, p := range paths {
		got, ok := jqjson.Path(v, p...)
		if !ok {
			return nil
		}
		if jqjson.Truthy(got) {
			return got
		}
	}
	return nil
}

// streamProgress is _stream_progress: it appends a line per tool call and
// text block to .progress and returns the last result line.
func (a *App) streamProgress(agent string, r io.Reader) string {
	progress := a.agentsDir + "/" + agent + "/.progress"
	last := ""
	step := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		vals, _ := jqjson.ParseStream([]byte(line + "\n"))
		if len(vals) == 0 {
			continue
		}
		v := vals[0]
		typ, _ := firstTruthy(v, []string{"type"}).(string)
		switch typ {
		case "assistant":
			block, ok := lastContent(v)
			if !ok {
				continue
			}
			kind := ""
			if t := firstTruthy(block, []string{"type"}); t != nil {
				kind = jqjson.Raw(t)
			}
			switch kind {
			case "tool_use":
				step++
				name := "?"
				if n := firstTruthy(block, []string{"name"}); n != nil {
					name = jqjson.Raw(n)
				}
				input := ""
				if in := firstTruthy(block, []string{"input", "file_path"}, []string{"input", "path"},
					[]string{"input", "pattern"}, []string{"input", "command"}); in != nil {
					input = jqjson.Raw(in)
				}
				if utf8.RuneCountInString(input) > 80 {
					input = string([]rune(input)[:77]) + "..."
				}
				appendFile(progress, fmt.Sprintf("[%d] %s %s\n", step, name, input))
				a.dbg("progress: [%d] %s %s", step, name, input)
			case "text":
				if t := firstTruthy(block, []string{"text"}); t != nil {
					snippet, _, _ := strings.Cut(jqjson.Raw(t), "\n")
					if r := []rune(snippet); len(r) > 120 {
						snippet = string(r[:120])
					}
					if snippet != "" {
						appendFile(progress, "  ... "+snippet+"\n")
					}
				}
			}
		case "result":
			last = line
		}
	}
	return last
}

func (a *App) initProgress(agent, question string) {
	progress := a.agentsDir + "/" + agent + "/.progress"
	_ = os.WriteFile(progress, []byte("# ask "+agent+" — "+timestamp()+"\n# Q: "+question+"\n# status: running\n---\n"), 0o644)
	a.log("Progress file: %s (use 'ask watch %s' to follow)", progress, agent)
}

func (a *App) finishProgress(agent, status string) {
	appendFile(a.agentsDir+"/"+agent+"/.progress", "---\n# status: "+status+" — "+timestamp()+"\n")
}

// firstUsage is [.modelUsage[]] | first for one value (ok false where jq
// errors).
func firstUsage(v any) (any, bool) {
	mu, ok := jqjson.Path(v, "modelUsage")
	if !ok {
		return nil, false
	}
	switch x := mu.(type) {
	case jqjson.Object:
		if len(x) == 0 {
			return nil, true
		}
		return x[0].Val, true
	case []any:
		if len(x) == 0 {
			return nil, true
		}
		return x[0], true
	}
	return nil, false // null or a scalar: "Cannot iterate"
}

// jqjsonNumber is how jqjson holds a number.
type jqjsonNumber = json.Number

// number is a JSON number's value.
func number(v any) (float64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(string(n), 64)
	return f, err == nil
}

// interp is a value in jq string interpolation.
func interp(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return jqjson.Compact(v)
}

// tokenUsage is the debug line of token counts.
func tokenUsage(raw string) string {
	vals, err := jqjson.ParseStream([]byte(raw))
	if err != nil || len(vals) == 0 {
		return "tokens: unavailable"
	}
	var lines []string
	for _, v := range vals {
		u, ok := firstUsage(v)
		if !ok {
			return strings.Join(append(lines, "tokens: unavailable"), "\n")
		}
		get := func(k string) string {
			x, ok := jqjson.Path(u, k)
			if !ok {
				return "null"
			}
			return interp(x)
		}
		if _, isObj := u.(jqjson.Object); u != nil && !isObj {
			return strings.Join(append(lines, "tokens: unavailable"), "\n")
		}
		lines = append(lines, fmt.Sprintf("tokens: in=%s out=%s cached=%s window=%s",
			get("inputTokens"), get("outputTokens"), get("cacheReadInputTokens"), get("contextWindow")))
	}
	return strings.Join(lines, "\n")
}

// contextPercent is the context window share the response's first model
// used, floored ("" when unknown).
func contextPercent(raw string) string {
	vals, _ := jqjson.ParseStream([]byte(raw))
	var out []string
	for _, v := range vals {
		u, ok := firstUsage(v)
		if !ok {
			break
		}
		obj, isObj := u.(jqjson.Object)
		if !isObj {
			continue // null: .contextWindow is null, so empty
		}
		cwv, _ := obj.Get("contextWindow")
		cw, isNum := number(cwv)
		if !jqjson.Truthy(cwv) || !isNum || cw <= 0 {
			continue
		}
		sum, any := 0.0, false
		bad := false
		for _, k := range []string{"inputTokens", "cacheReadInputTokens", "cacheCreationInputTokens"} {
			x, _ := obj.Get(k)
			if x == nil {
				continue
			}
			n, ok := number(x)
			if !ok {
				bad = true
				break
			}
			sum += n
			any = true
		}
		if bad || !any {
			break
		}
		out = append(out, strconv.FormatFloat(math.Floor(sum/cw*100), 'f', -1, 64))
	}
	return strings.Join(out, "\n")
}

// checkContextLimit is _check_context_limit: forget the session once its
// context reaches ASK_CONTEXT_LIMIT percent.
func (a *App) checkContextLimit(agent, raw string) {
	pct := contextPercent(raw)
	if pct == "" {
		a.log("Context usage: unknown")
		return
	}
	a.log("Context usage: %s%% (limit: %s%%)", pct, a.contextLimit)
	used, ok1 := atoi(pct)
	limit, ok2 := atoi(a.contextLimit)
	if ok1 && ok2 && used >= limit {
		os.Remove(a.agentsDir + "/" + agent + "/.session-id")
		a.errf("[%s] context at %s%% — session auto-cleared\n", agent, pct)
	}
}

// responseText is jq -r '.result // .content // .' of the raw response,
// or the raw response where it is not JSON.
func responseText(raw string) string {
	vals, err := jqjson.ParseStream([]byte(raw))
	var lines []string
	for _, v := range vals {
		got := v
		if _, isObj := v.(jqjson.Object); !isObj && v != nil {
			err = errors.New("cannot index") // jq fails; the raw text is echoed
			break
		}
		if r := firstTruthy(v, []string{"result"}, []string{"content"}); r != nil {
			got = r
		}
		lines = append(lines, jqjson.Raw(got))
	}
	if err != nil {
		lines = append(lines, raw)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// askViaInbox is _ask_via_inbox: leave the question in the running
// agent's inbox and wait up to ASK_TIMEOUT seconds for its answer.
func (a *App) askViaInbox(agent, question string) {
	dir := a.agentsDir + "/" + agent
	id := msgID()
	a.log("Inbox delivery for agent '%s', msg %s", agent, id)
	ts := timestamp()
	body := jqjson.String(jqjson.RepairUTF8(question + "\n"))
	msg := "{\n  \"id\": \"" + id + "\",\n  \"from\": \"human\",\n  \"type\": \"question\",\n  \"timestamp\": \"" + ts + "\",\n  \"body\": " + body + "\n}\n"
	_ = os.WriteFile(dir+"/inbox/"+id+".json", []byte(msg), 0o644)
	appendEvent(a.eventsLog, event(id, "human", agent, "question", ts, question))

	outboxFile := dir + "/outbox/" + id + ".json"
	a.log("Waiting for response at %s (timeout: %ss)", outboxFile, a.timeout)
	limit, limited := atoi(a.timeout)
	for elapsed := 0; !isFile(outboxFile); {
		time.Sleep(time.Second)
		elapsed++
		if limited && elapsed >= limit {
			a.errf("ask: timeout waiting for agent '%s' to respond (%ss)\n", agent, a.timeout)
			a.exit(3)
		}
	}
	data, _ := os.ReadFile(outboxFile)
	answer := strings.TrimRight(string(data), "\n")
	if hasAnswer([]byte(answer + "\n")) {
		a.formatResponse(agent, jqRaw([]byte(answer+"\n"), []string{"answer"}, nil, ""), "")
	} else {
		a.formatResponse(agent, answer, "")
	}
	os.Remove(outboxFile)
	a.exit(0)
}

// hasAnswer is `jq -e '.answer'`: every value an object (or null) and the
// last one's answer truthy.
func hasAnswer(data []byte) bool {
	vals, err := jqjson.ParseStream(data)
	if err != nil || len(vals) == 0 {
		return false
	}
	var last any
	for _, v := range vals {
		got, ok := jqjson.Path(v, "answer")
		if !ok {
			return false
		}
		last = got
	}
	return jqjson.Truthy(last)
}
