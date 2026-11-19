package ask

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/willackerly/rebar/cli/internal/checks"
)

// The project registry (~/.config/ask/projects, ASK_REGISTRY) holds one
// "name = path" line per project.

// registryLines returns the registry's lines as `while read` sees them (a
// last line without a newline is not read) and as grep sees them (it is).
func (a *App) registryLines() (read, grep []string, ok bool) {
	data, err := os.ReadFile(a.registry)
	if err != nil {
		return nil, nil, false
	}
	text := string(data)
	if text == "" {
		return nil, nil, true
	}
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
		return lines, lines, true
	}
	return lines[:len(lines)-1], lines, true
}

// nameRE is the script's grep pattern "^${name}[[:space:]]*=" (nil when
// grep would reject it).
func nameRE(name string) *regexp.Regexp {
	return checks.BRE("^" + name + "[[:space:]]*=")
}

// grepFirst returns the first registry line matching name's pattern.
func (a *App) grepFirst(name string) (string, bool) {
	re := nameRE(name)
	if re == nil {
		return "", false
	}
	_, lines, _ := a.registryLines()
	for _, l := range lines {
		if re.MatchString(l) {
			return l, true
		}
	}
	return "", false
}

// cutField2 is `cut -d'=' -f2`.
func cutField2(line string) string {
	fields := strings.Split(line, "=")
	if len(fields) < 2 {
		return line // cut prints a line without the delimiter whole
	}
	return fields[1]
}

// xargsEcho is `echo "$s" | xargs`: the words of s, quotes removed, one
// space apart ("" when a quote is unmatched, as xargs then fails).
func xargsEcho(s string) string {
	var words []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case c == '\'' || c == '"':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				return ""
			}
			cur.WriteString(s[i+1 : i+1+j])
			i += j + 1
			inWord = true
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			inWord = true
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return strings.Join(words, " ")
}

// trimTrailingSpace is sed 's/[[:space:]]*$//'.
func trimTrailingSpace(s string) string {
	return strings.TrimRight(s, " \t\r\v\f")
}

func (a *App) appendRegistry(name string) {
	_ = os.MkdirAll(filepath.Dir(a.registry), 0o755)
	f, err := os.OpenFile(a.registry, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		a.die("cannot write %s: %v", a.registry, err)
	}
	fmt.Fprintf(f, "%s = %s\n", name, a.pwd)
	f.Close()
}

// noRegisterEnv tells the background compaction ASK starts not to
// auto-register its directory: the script ran it as a subshell function.
const noRegisterEnv = "REBAR_ASK_NO_REGISTER"

// autoRegister silently registers the project when ./agents exists.
func (a *App) autoRegister() {
	if !isDir("./agents") || os.Getenv(noRegisterEnv) == "1" {
		return
	}
	name := basename(a.pwd)
	_ = os.MkdirAll(filepath.Dir(a.registry), 0o755)
	if _, found := a.grepFirst(name); !found {
		a.appendRegistry(name)
		a.log("Auto-registered project '%s'", name)
	}
}

// register is cmd_register.
func (a *App) register(name string) {
	if name == "" {
		name = basename(a.pwd)
	}
	_ = os.MkdirAll(filepath.Dir(a.registry), 0o755)
	if line, found := a.grepFirst(name); found {
		if xargsEcho(cutField2(line)) == a.pwd {
			a.out("'%s' already registered at %s\n", name, a.pwd)
			return
		}
		// Drop the old entries. (The script's `grep -v` exited the script
		// silently when no other line was left.)
		re := nameRE(name)
		_, lines, _ := a.registryLines()
		var keep []string
		for _, l := range lines {
			if !re.MatchString(l) {
				keep = append(keep, l)
			}
		}
		text := ""
		if len(keep) > 0 {
			text = strings.Join(keep, "\n") + "\n"
		}
		if err := os.WriteFile(a.registry, []byte(text), 0o644); err != nil {
			a.die("cannot write %s: %v", a.registry, err)
		}
	}
	a.appendRegistry(name)
	a.out("Registered '%s' → %s\n\nNow from any directory:\n", name, a.pwd)
	a.out("  ask %s:architect \"What is the architecture?\"\n", name)
}

// formatProjects is _format_projects (cmd_projects), written to w.
func (a *App) formatProjects(w ...io.Writer) {
	out := a.Stdout
	if len(w) > 0 {
		out = w[0]
	}
	lines, _, ok := a.registryLines()
	if !ok {
		fmt.Fprintln(out, "  (no projects registered — run 'ask register <name>' in a project)")
		return
	}
	fmt.Fprintf(out, "\n%s  Registered Projects%s\n\n", a.c.bold, a.c.reset)
	for _, line := range lines {
		name, path, _ := strings.Cut(line, "=")
		name, path = xargsEcho(name), xargsEcho(path)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		names := agentFiles(path + "/agents")
		fmt.Fprintf(out, "  %s%-14s%s %s%s%s\n", a.c.bold, name, a.c.reset, a.c.dim, path, a.c.reset)
		if len(names) > 0 {
			fmt.Fprintf(out, "  %s%-14s%s %s\n", a.c.dim, "", a.c.reset,
				fmt.Sprintf("%d agents: %s", len(names), strings.Join(names, ", ")))
		}
	}
	fmt.Fprintln(out)
}

// agentFiles is the sorted basenames of the directories holding an entry
// named AGENT.md at most two levels below dir (`find dir -maxdepth 2 -name
// AGENT.md -exec dirname {} \;`), empty when dir is not a directory.
func agentFiles(dir string) []string {
	if !isDir(dir) {
		return nil
	}
	var names []string
	root := strings.Count(filepath.Clean(dir), string(filepath.Separator))
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		depth := strings.Count(filepath.Clean(p), string(filepath.Separator)) - root
		if d.IsDir() && depth >= 2 {
			if d.Name() == "AGENT.md" {
				names = append(names, basename(filepath.Dir(p)))
			}
			return fs.SkipDir
		}
		if d.Name() == "AGENT.md" && depth >= 1 {
			names = append(names, basename(filepath.Dir(p)))
		}
		return nil
	})
	sort.Strings(names)
	return names
}

// lowerASCII is tr '[:upper:]' '[:lower:]'.
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// registeredLocally reports whether the registry names project, ignoring
// case (the local-first rule of _is_remote_query), and returns its line.
func (a *App) registeredLocally(project string) (string, string, bool) {
	lines, _, _ := a.registryLines()
	lc := lowerASCII(project)
	for _, line := range lines {
		registered, _, _ := strings.Cut(line, "=")
		registered = trimTrailingSpace(registered)
		if registered == "" {
			continue
		}
		if lowerASCII(registered) == lc {
			return registered, line, true
		}
	}
	return "", "", false
}

// isRemoteQuery is _is_remote_query: ASK_SERVER is set, spec is
// project:agent with both halves, and the project is not registered here.
func (a *App) isRemoteQuery(spec string) bool {
	if a.server == "" || !strings.Contains(spec, ":") {
		return false
	}
	project, role := splitSpec(spec)
	if project == "" || role == "" {
		return false
	}
	_, _, local := a.registeredLocally(project)
	return !local
}

// resolveProjectAgent is resolve_project_agent: for project:agent it
// points the agents and shared directories at the registered project's and
// returns the agent half; otherwise it returns spec.
func (a *App) resolveProjectAgent(spec string) string {
	a.resolvedProjectDir = ""
	if !strings.Contains(spec, ":") {
		return spec
	}
	project, agent := splitSpec(spec)
	if project == "" || agent == "" {
		a.errf("ask: error: malformed cross-repo spec '%s' — expected '<project>:<agent>'\n", spec)
		a.exit(2)
	}
	if !isFile(a.registry) {
		a.die("no project registry at %s — run 'ask register <name>' in a project first", a.registry)
	}
	path, canonical := "", project
	if line, found := a.grepFirst(project); found {
		path = xargsEcho(cutField2(line))
	}
	if path == "" {
		if registered, line, found := a.registeredLocally(project); found {
			_, rest, _ := strings.Cut(line, "=")
			path, canonical = xargsEcho(rest), registered
			a.log("Resolved project '%s' → '%s' (case-insensitive)", project, canonical)
		}
	}
	if path == "" {
		a.errf("ask: error: project '%s' not found in registry\nRegistered projects:\n", project)
		a.formatProjects(a.Stderr)
		a.exit(2)
	}
	if !isDir(path) {
		a.die("registered path for '%s' no longer exists: %s", canonical, path)
	}
	a.agentsDir = path + "/agents"
	a.sharedDir = path + "/shared"
	a.eventsLog = a.sharedDir + "/messages/events.jsonl"
	a.resolvedProjectDir = path
	return agent
}
