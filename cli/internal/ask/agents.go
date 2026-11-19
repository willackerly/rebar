package ask

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/willackerly/rebar/cli/internal/jqjson"
)

// agentDirs is the glob "$AGENTS_DIR"/*/ filtered to directories holding
// an AGENT.md file: their names, sorted.
func (a *App) agentDirs() []string {
	entries, err := os.ReadDir(a.agentsDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dir := a.agentsDir + "/" + name
		if isDir(dir) && isFile(dir+"/AGENT.md") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (a *App) requireAgentName(name string) {
	if name == "" {
		a.errf("ask: error: agent name required\nUsage: ask <command> <agent>\n")
		a.exit(4)
	}
}

// canonicalize is _canonicalize_agent: the agent directory matching input,
// ignoring case ("" when none does).
func (a *App) canonicalize(input string) string {
	if !ValidAgentName(input) {
		return ""
	}
	// The directory's own spelling, also on a case-insensitive file system
	// (where the script's [ -d ] fast path kept the spelling as typed).
	names := a.agentDirs()
	for _, name := range names {
		if name == input {
			return name
		}
	}
	lc := lowerASCII(input)
	for _, name := range names {
		if lowerASCII(name) == lc {
			return name
		}
	}
	if ValidAgentName(input) && isDir(a.agentsDir+"/"+input) && isFile(a.agentsDir+"/"+input+"/AGENT.md") {
		return input
	}
	return ""
}

// requireAgentExists is require_agent_exists: it sets canonicalAgent or
// reports the missing agent, with a suggestion and the list, and exits 2.
func (a *App) requireAgentExists(agent string) {
	if !ValidAgentName(agent) {
		a.errf("ask: error: invalid agent name '%s' (must be a directory name under %s)\n", agent, a.agentsDir)
		a.exit(2)
	}
	if canonical := a.canonicalize(agent); canonical != "" {
		a.canonicalAgent = canonical
		if canonical != agent {
			a.log("Resolved agent '%s' → '%s' (case-insensitive)", agent, canonical)
		}
		return
	}
	a.errf("ask: error: agent '%s' not found (no directory at %s/%s)\n\n", agent, a.agentsDir, agent)
	a.suggestAgent(agent)
	a.errf("Available agents:\n")
	a.who(a.Stderr)
	a.exit(2)
}

// levenshtein is the edit distance between s and t (in characters).
func levenshtein(s, t string) int {
	a, b := []rune(s), []rune(t)
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// suggestAgent is _suggest_agent: "Did you mean" the closest agent within
// min(4, half the name's length) edits.
func (a *App) suggestAgent(target string) {
	if target == "" || !isDir(a.agentsDir) {
		return
	}
	lc := lowerASCII(target)
	best, bestDist := "", 999
	for _, name := range a.agentDirs() {
		if d := levenshtein(lc, lowerASCII(name)); d < bestDist {
			best, bestDist = name, d
		}
	}
	maxDist := min(4, utf8.RuneCountInString(target)/2)
	if best != "" && bestDist <= maxDist {
		a.errf("Did you mean: %s?\n\n", best)
	}
}

// agentVoice is the awk regex of _caller_facing_description.
var agentVoice = regexp.MustCompilePOSIX(`^You (are|own|make decisions|represent|are responsible|do not|don't|receive|run|report)[^.!?]*[.!?]([[:space:]]+|$)`)

// callerFacingDescription is _caller_facing_description: the first
// paragraph of AGENT.md (after headings and blank lines, trailing space
// trimmed, lines joined by spaces) without its leading "You are…" sentences.
func callerFacingDescription(agentMD string) string {
	data, err := os.ReadFile(agentMD)
	if err != nil {
		return ""
	}
	var para []string
	for _, line := range splitLines(string(data)) {
		line = strings.TrimRight(line, " \t\r\v\f")
		if line == "" || strings.HasPrefix(line, "#") {
			if para != nil {
				break
			}
			continue
		}
		para = append(para, line)
	}
	text := strings.Join(para, " ")
	for {
		loc := agentVoice.FindStringIndex(text)
		if loc == nil {
			break
		}
		text = strings.TrimLeft(text[loc[1]:], " \t\n\r\v\f")
	}
	return text
}

// splitLines splits text into lines as awk and sed read them (no empty
// last line for a trailing newline).
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// sectionLine is the fallback descriptions of cmd_who: the first "- item"
// of the "## Responsibilities" section (stripped of "- "), else the first
// non-blank line of "## Role".
func sectionLine(agentMD string) string {
	data, err := os.ReadFile(agentMD)
	if err != nil {
		return ""
	}
	lines := splitLines(string(data))
	// sed -n '/^## Responsibilities/,/^##/{s/^- *//p;}' | head -1
	in := false
	for _, l := range lines {
		if !in {
			if strings.HasPrefix(l, "## Responsibilities") {
				in = true
			}
			continue
		}
		if strings.HasPrefix(l, "-") {
			return strings.TrimLeft(l[1:], " ")
		}
		if strings.HasPrefix(l, "##") {
			in = false
		}
	}
	// sed -n '/^## Role/,/^##/{/^## Role/d;/^##/d;/^$/d;p;}' | head -1
	in = false
	for _, l := range lines {
		if !in {
			if strings.HasPrefix(l, "## Role") {
				in = true
			}
			continue
		}
		if strings.HasPrefix(l, "##") {
			in = false
			continue
		}
		if l == "" {
			continue
		}
		return strings.TrimLeft(l, " \t\r\v\f")
	}
	return ""
}

// who is cmd_who, written to w (stdout by default).
func (a *App) who(w ...io.Writer) {
	out := a.Stdout
	if len(w) > 0 {
		out = w[0]
	}
	if !isDir(a.agentsDir) {
		a.errf("ask: no agents directory at %s\n", a.agentsDir)
		a.exit(2)
	}
	fmt.Fprintf(out, "\n%s  %s — Available Agents%s\n\n", a.c.bold, basename(a.pwd), a.c.reset)
	descWidth := min(max(a.termWidth()-18, 30), 100)
	for _, name := range a.agentDirs() {
		md := a.agentsDir + "/" + name + "/AGENT.md"
		colour := a.c.role(name)
		desc := callerFacingDescription(md)
		if desc == "" {
			desc = sectionLine(md)
		}
		if r := []rune(desc); len(r) > descWidth {
			trunc := r[:descWidth]
			cut := len(trunc)
			for i := len(trunc) - 1; i >= 0; i-- {
				if trunc[i] == ' ' {
					cut = i
					break
				}
			}
			if cut > 10 {
				desc = string(r[:cut]) + "…"
			} else {
				desc = string(trunc) + "…"
			}
		}
		if desc != "" {
			fmt.Fprintf(out, "  %s%s%-14s%s %s%s%s\n", colour, a.c.bold, name, a.c.reset, a.c.dim, desc, a.c.reset)
		} else {
			fmt.Fprintf(out, "  %s%s%s%s\n", colour, a.c.bold, name, a.c.reset)
		}
	}
	fmt.Fprintln(out)
}

// where is cmd_where: the agent directory's absolute path.
func (a *App) where(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	dir := a.agentsDir + "/" + a.canonicalAgent
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		if abs, err := filepath.Abs(real); err == nil {
			dir = abs
		}
	}
	a.out("%s\n", dir)
}

// readTrimmed is $(cat file): the contents without trailing newlines.
func readTrimmed(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(data), "\n"), true
}

// alivePID returns the pid in pidfile and whether kill -0 reaches it.
func alivePID(pidfile string) (string, bool) {
	text, ok := readTrimmed(pidfile)
	if !ok {
		return "", false
	}
	pid, err := strconv.Atoi(text)
	if err != nil || pid <= 0 {
		return text, false
	}
	return text, syscall.Kill(pid, 0) == nil
}

// countJSON is `find dir -name '*.json' | wc -l`.
func countJSON(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil {
			if ok, _ := filepath.Match("*.json", d.Name()); ok {
				n++
			}
		}
		return nil
	})
	return n
}

// globJSON is the files of the glob "$dir"/*.json, sorted.
func globJSON(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	var files []string
	for _, m := range matches {
		if strings.HasPrefix(filepath.Base(m), ".") {
			continue
		}
		if isFile(m) {
			files = append(files, dir+"/"+filepath.Base(m))
		}
	}
	sort.Strings(files)
	return files
}

// status is cmd_status.
func (a *App) status(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	dir := a.agentsDir + "/" + name
	pidfile := dir + "/.pid"
	if pid, alive := alivePID(pidfile); alive {
		inbox := countJSON(dir + "/inbox")
		lastActive := "unknown"
		var latest string
		var latestTime time.Time
		for _, f := range globJSON(dir + "/outbox") {
			if info, err := os.Stat(f); err == nil && (latest == "" || info.ModTime().After(latestTime)) {
				latest, latestTime = f, info.ModTime()
			}
		}
		if latest != "" {
			elapsed := time.Now().Unix() - latestTime.Unix()
			lastActive = fmt.Sprintf("%ds ago", elapsed)
		}
		a.out("STATUS: running\nINBOX: %d pending\nPID: %s\nLAST_ACTIVE: %s\n", inbox, pid, lastActive)
		return
	}
	a.out("STATUS: stopped\n")
	if isFile(pidfile) {
		os.Remove(pidfile)
	}
}

// showLog is cmd_log: memory.log.md, through a pager on a terminal.
func (a *App) showLog(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	logfile := a.agentsDir + "/" + a.canonicalAgent + "/memory.log.md"
	if !isFile(logfile) {
		a.out("(no log entries yet)\n")
		return
	}
	if a.stdoutTTY && os.Getenv("PAGER") != "cat" {
		pager := envOr("PAGER", "less")
		if path, err := exec.LookPath(pager); err == nil {
			cmd := exec.Command(path, "-RFX", logfile)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, a.Stdout, a.Stderr
			_ = cmd.Run()
			return
		}
	}
	data, _ := os.ReadFile(logfile)
	a.Stdout.Write(data)
}

// jqField is `jq -r '.key // def' file 2>/dev/null || fallback`.
func jqField(path, key string, def any, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return jqRaw(data, []string{key}, def, fallback)
}

// jqRaw is `jq -r '.k1.k2… // def' 2>/dev/null || fallback` on the JSON
// stream data (def nil: no "// def"): one line per value; where jq fails
// (a parse error, indexing a non-object), the fallback follows the lines
// printed before it, as both commands' output lands in $(…).
func jqRaw(data []byte, keys []string, def any, fallback string) string {
	vals, err := jqjson.ParseStream(data)
	var lines []string
	for _, v := range vals {
		got, ok := jqjson.Path(v, keys...)
		if !ok { // jq: "Cannot index …"; "//" does not catch it
			return strings.Join(append(lines, fallback), "\n")
		}
		if def != nil && !jqjson.Truthy(got) {
			if def == jqEmpty {
				continue
			}
			got = def
		}
		lines = append(lines, jqjson.Raw(got))
	}
	if err != nil {
		lines = append(lines, fallback)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// peek is cmd_peek: the pending inbox messages.
func (a *App) peek(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	inbox := a.agentsDir + "/" + name + "/inbox"
	if !isDir(inbox) {
		a.out("(no inbox)\n")
		a.exit(0)
	}
	count := countJSON(inbox)
	if count == 0 {
		a.out("(inbox empty)\n")
		a.exit(0)
	}
	a.out("%d pending message(s):\n\n", count)
	for _, msg := range globJSON(inbox) {
		id := jqField(msg, "id", "unknown", basename(msg))
		body := jqField(msg, "body", "(no body)", "(unreadable)")
		ts := jqField(msg, "timestamp", "", "")
		a.out("  [%s] %s\n    %s\n\n", ts, id, body)
	}
}

// follow runs `tail -f file`.
func (a *App) follow(file string) int {
	cmd := exec.Command("tail", "-f", file)
	cmd.Stdout, cmd.Stderr = a.Stdout, a.Stderr
	return exitCode(cmd.Run())
}

// tail is cmd_tail: follow the event log.
func (a *App) tail() {
	if !isFile(a.eventsLog) {
		a.errf("ask: no event log at %s (run 'ask init' first)\n", a.eventsLog)
		a.exit(4)
	}
	a.errf("Following %s (Ctrl-C to stop)...\n", a.eventsLog)
	a.exit(a.follow(a.eventsLog))
}

// watch follows an agent's .progress file.
func (a *App) watch(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	progress := a.agentsDir + "/" + name + "/.progress"
	data, err := os.ReadFile(progress)
	if err != nil || !isFile(progress) {
		a.errf("No active progress for %s. Start a write-mode query first:\n  ask -w %s \"<question>\"\n", name, name)
		a.exit(2)
	}
	a.Stdout.Write(data)
	a.out("\n")
	if strings.Contains(string(data), "status: running") {
		a.out("--- following live (Ctrl+C to stop) ---\n")
		a.follow(progress)
	}
	a.exit(0)
}

// up is cmd_up: run the agent's run.sh, else the built-in loop.
func (a *App) up(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	a.requireClaude()
	dir := a.agentsDir + "/" + name
	runsh := dir + "/run.sh"
	if isFile(runsh) {
		a.log("Starting agent '%s' via %s", name, runsh)
		path, _ := filepath.Abs(runsh)
		err := syscall.Exec(path, []string{runsh}, os.Environ())
		a.errf("ask: %s: %v\n", runsh, err)
		a.exit(126)
	}
	a.log("Starting agent '%s' with built-in loop", name)
	loop := &Loop{
		Name: name, Dir: dir, EventsLog: a.eventsLog, Model: a.model,
		Stdout: a.Stdout, Stderr: a.Stderr,
	}
	a.exit(loop.Run())
}

// reset is cmd_reset: forget the agent's session.
func (a *App) reset(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	sessionFile := a.agentsDir + "/" + name + "/.session-id"
	if isFile(sessionFile) {
		old, _ := readTrimmed(sessionFile)
		os.Remove(sessionFile)
		a.out("Session cleared for %s (was: %s)\nNext question will establish a fresh session.\n", name, old)
		return
	}
	a.out("%s has no active session.\n", name)
}

// countSeparators is `grep -c '^---$' file`.
func countSeparators(text string) int {
	n := 0
	for _, l := range splitLines(text) {
		if l == "---" {
			n++
		}
	}
	return n
}

// envInt is ${name:-def} read as a number (def when not one).
func envInt(name string, def int) int {
	if n, ok := atoi(os.Getenv(name)); ok {
		return n
	}
	return def
}

// compact is cmd_compact: summarize all but the last ASK_COMPACT_KEEP
// entries of memory.log.md into memory.md once the log reaches
// ASK_COMPACT_SIZE bytes or ASK_COMPACT_ENTRIES entries.
func (a *App) compact(name string) {
	a.requireAgentName(name)
	a.requireAgentExists(name)
	dir := a.agentsDir + "/" + name
	logFile, memoryFile := dir+"/memory.log.md", dir+"/memory.md"
	data, err := os.ReadFile(logFile)
	if err != nil || !isFile(logFile) {
		a.out("%s has no memory log to compact.\n", name)
		return
	}
	size := len(data)
	entries := countSeparators(string(data))
	sizeThreshold := envInt("ASK_COMPACT_SIZE", 51200)
	entryThreshold := envInt("ASK_COMPACT_ENTRIES", 100)
	keepRecent := envInt("ASK_COMPACT_KEEP", 20)

	if size < sizeThreshold && entries < entryThreshold {
		a.out("%s memory: %d entries, %dKB — no compaction needed.\n", name, entries, size/1024)
		return
	}
	a.out("Compacting %s memory: %d entries, %dKB...\n", name, entries, size/1024)

	keepFrom := max(entries-keepRecent, 1)
	var older, recent []string
	count := 0
	for _, l := range splitLines(string(data)) {
		if l == "---" {
			count++
		}
		if count < keepFrom {
			older = append(older, l)
		} else {
			recent = append(recent, l)
		}
	}
	olderText := strings.TrimRight(strings.Join(older, "\n"), "\n")
	recentText := strings.TrimRight(strings.Join(recent, "\n"), "\n")
	if olderText == "" {
		a.out("Not enough old entries to compact.\n")
		return
	}

	prompt := "Summarize these agent interaction logs into a concise memory document.\n" +
		"Preserve: key decisions, facts learned, important context, open questions, recurring patterns.\n" +
		"Discard: routine Q&A, greetings, obvious/redundant information.\n" +
		"Format: bullet points grouped by topic. Be concise but preserve critical details.\n\n" +
		"LOGS TO SUMMARIZE:\n" + olderText
	summary := ""
	if claude, err := exec.LookPath("claude"); err == nil {
		a.log("Generating summary via claude...")
		cmd := exec.Command(claude, "--print", "--model", a.model)
		cmd.Stdin = strings.NewReader(prompt + "\n")
		out, _ := cmd.Output() // a failing claude's output still counts
		summary = strings.TrimRight(string(out), "\n")
	}
	if summary == "" {
		a.log("Claude unavailable; keeping truncated log entries.")
		head := splitLines(olderText + "\n")
		if len(head) > 50 {
			head = head[:50]
		}
		summary = "## Compacted Memory (" + timestamp() + ")\n\n" + strings.TrimRight(strings.Join(head, "\n"), "\n") +
			"\n\n_[" + strconv.Itoa(entries) + " older entries truncated during compaction]_"
	}

	memory := "# Agent Memory: " + name + "\n\n" +
		"<!-- Compacted at " + timestamp() + ". Source: memory.log.md -->\n" +
		"<!-- Older entries summarized, recent entries in memory.log.md -->\n\n" +
		summary + "\n"
	if err := os.WriteFile(memoryFile, []byte(memory), 0o644); err != nil {
		a.die("cannot write %s: %v", memoryFile, err)
	}
	newLog := "<!-- Compacted at " + timestamp() + ". Prior history summarized in memory.md -->\n" + recentText + "\n"
	if err := os.WriteFile(logFile, []byte(newLog), 0o644); err != nil {
		a.die("cannot write %s: %v", logFile, err)
	}
	a.out("Compacted: %d -> %d entries, %dKB -> %dKB\n", entries, countSeparators(newLog), size/1024, len(newLog)/1024)
	a.out("Summary written to: %s\n", memoryFile)
}

// maybeCompact is _maybe_compact: compact in the background (a detached
// `rebar ask compact`) once the log reaches ASK_COMPACT_SIZE bytes.
func (a *App) maybeCompact(name string) {
	info, err := os.Stat(a.agentsDir + "/" + name + "/memory.log.md")
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	threshold := envInt("ASK_COMPACT_SIZE", 51200)
	if info.Size() < int64(threshold) || a.Exe == "" {
		return
	}
	a.log("Memory log exceeds %d bytes, auto-compacting in background...", threshold)
	cmd := exec.Command(a.Exe, "ask", "compact", name)
	cmd.Env = append(os.Environ(), "ASK_AGENTS_DIR="+a.agentsDir, "ASK_SHARED_DIR="+a.sharedDir, noRegisterEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}

// timestamp is date -u +%Y-%m-%dT%H:%M:%SZ.
func timestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// listAgentCommands is _list_agent_commands, on stderr.
func (a *App) listAgentCommands(agent string) {
	dir := a.agentsDir + "/" + agent + "/commands"
	if !isDir(dir) {
		return
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.sh"))
	sort.Strings(matches)
	var scripts []string
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), ".") && isFile(m) {
			scripts = append(scripts, m)
		}
	}
	if len(scripts) == 0 {
		return
	}
	a.errf("\nAvailable commands for %s:\n", agent)
	for _, f := range scripts {
		name := strings.TrimSuffix(filepath.Base(f), ".sh")
		if name == "default" {
			continue
		}
		desc := ""
		if file, err := os.Open(f); err == nil {
			sc := bufio.NewScanner(file)
			for i := 0; sc.Scan() && i < 2; i++ {
				if i == 1 && strings.HasPrefix(sc.Text(), "#") {
					desc = strings.TrimLeft(sc.Text()[1:], " ")
				}
			}
			file.Close()
		}
		if desc != "" {
			a.errf("  ask %-10s %-12s — %s\n", agent, name, desc)
		} else {
			a.errf("  ask %s %s\n", agent, name)
		}
	}
}

// runCommand runs an agent command script with args on the standard
// streams and returns its exit status as the shell reports it.
func (a *App) runCommand(script string, args []string) int {
	cmd := exec.Command(script, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.Stdin, a.Stdout, a.Stderr
	err := cmd.Run()
	if errors.Is(err, syscall.ENOEXEC) {
		// No #! line: the shell runs such a file as a script itself.
		cmd = exec.Command("bash", append([]string{script}, args...)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = a.Stdin, a.Stdout, a.Stderr
		err = cmd.Run()
	}
	return exitCode(err)
}

// exitCode is the shell's $? for a finished command.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ee.ExitCode()
	}
	if errors.Is(err, os.ErrPermission) {
		return 126
	}
	return 127
}
