package ask

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// initRoles are the agents `ask init` creates.
var initRoles = []string{"architect", "product", "englead", "steward", "merger", "featurerequest"}

// initProject is cmd_init: shared/ directories, the role agents and the
// .gitignore entries for ASK's runtime files.
func (a *App) initProject() {
	a.out("Initializing ASK Scoped Knowledge...\n\n")
	name := detectProjectName(a.pwd)
	a.out("Project: %s\n", name)
	context := detectProjectContext()
	a.out("\nDetected project files:\n")
	for _, l := range strings.Split(context, "\n") {
		a.out("  %s\n", l)
	}
	a.out("\n")

	for _, d := range []string{"messages", "artifacts", "decisions"} {
		_ = os.MkdirAll(a.sharedDir+"/"+d, 0o755)
	}
	if f, err := os.OpenFile(a.eventsLog, os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
		f.Close()
	}

	var created []string
	for _, role := range initRoles {
		dir := a.agentsDir + "/" + role
		if isFile(dir + "/AGENT.md") {
			_ = os.MkdirAll(dir+"/inbox", 0o755)
			_ = os.MkdirAll(dir+"/outbox", 0o755)
			a.out("  exists: %s\n", role)
		} else {
			createAgent(dir, role, name, context)
			created = append(created, role)
			a.out("  created: %s\n", role)
		}
	}
	a.ensureGitignore()
	a.out("\n")
	if len(created) > 0 {
		a.out("Created agents: %s\n", strings.Join(created, " "))
	} else {
		a.out("All agents already exist.\n")
	}
	a.out("\nReady. Try:\n  ask who\n  ask architect \"What is the architecture of this project?\"\n")
}

// detectProjectName is _detect_project_name: README.md's first heading,
// else package.json's name, else the directory's name.
func detectProjectName(pwd string) string {
	if data, err := os.ReadFile("README.md"); err == nil {
		for _, line := range splitLines(string(data)) {
			if strings.HasPrefix(line, "#") {
				heading := strings.TrimLeft(line, "#")
				heading = strings.TrimLeft(heading, " ")
				heading = strings.ReplaceAll(heading, "\r", "")
				if heading != "" {
					return heading
				}
				break
			}
		}
	}
	if data, err := os.ReadFile("package.json"); err == nil && isFile("package.json") {
		if name := jqRaw(data, []string{"name"}, jqEmpty, ""); name != "" {
			return name
		}
	}
	return basename(pwd)
}

// contractsDir is the script's reading of .rebarrc contracts_dir:
// REBAR_CONTRACTS_DIR, else the first contracts_dir line (comment, trailing
// space and a trailing slash removed), else architecture.
func contractsDir() string {
	if d := os.Getenv("REBAR_CONTRACTS_DIR"); d != "" {
		return d
	}
	data, err := os.ReadFile(".rebarrc")
	if err != nil {
		return "architecture"
	}
	for _, line := range splitLines(string(data)) {
		rest, ok := strings.CutPrefix(line, "contracts_dir")
		if !ok {
			continue
		}
		rest = strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		v := strings.TrimLeft(rest[1:], " \t")
		if i := strings.Index(v, "#"); i >= 0 {
			// sed 's/[[:space:]]*#.*$//'
			v = strings.TrimRight(v[:i], " \t")
		}
		v = strings.TrimRight(v, " \t")
		v = strings.TrimSuffix(v, "/")
		if v == "" {
			return "architecture"
		}
		return v
	}
	return "architecture"
}

// detectProjectContext is _detect_project_context: a "- `path`" line for
// each well-known file and directory present.
func detectProjectContext() string {
	var lines []string
	add := func(cond bool, line string) {
		if cond {
			lines = append(lines, line)
		}
	}
	for _, f := range []string{"README.md", "QUICKCONTEXT.md", "TODO.md", "AGENTS.md", "CLAUDE.md"} {
		add(isFile(f), "- `"+f+"`")
	}
	add(isFile("DESIGN.md"), "- `DESIGN.md` (contract design philosophy)")
	add(isFile("methodology.md"), "- `methodology.md` (contract methodology)")
	if cdir := contractsDir(); isDir(cdir) {
		matches, _ := filepath.Glob(cdir + "/CONTRACT-*.md")
		n := 0
		for _, m := range matches {
			if !strings.HasPrefix(filepath.Base(m), ".") {
				n++
			}
		}
		lines = append(lines, "- `"+cdir+"/` ("+strconv.Itoa(n)+" contracts)")
	}
	add(isDir("product"), "- `product/` (BDD features, personas)")
	add(isDir("docs"), "- `docs/`")
	add(isDir("tests"), "- `tests/`")
	add(isDir("test") && !isDir("tests"), "- `test/`")
	add(isDir("src"), "- `src/`")
	add(isDir("packages"), "- `packages/` (monorepo)")
	add(isDir("internal"), "- `internal/`")
	add(isFile("package.json"), "- `package.json` (Node.js)")
	add(isFile("go.mod"), "- `go.mod` (Go)")
	add(isFile("Cargo.toml"), "- `Cargo.toml` (Rust)")
	add(isFile("pyproject.toml"), "- `pyproject.toml` (Python)")
	add(isFile("METRICS"), "- `METRICS` (ground truth metrics)")
	add(isDir("docs/specifications"), "- `docs/specifications/` (API specs)")
	return strings.Join(lines, "\n")
}

// createAgent is _create_agent: the agent directory and its AGENT.md.
func createAgent(dir, name, project, context string) {
	_ = os.MkdirAll(dir+"/inbox", 0o755)
	_ = os.MkdirAll(dir+"/outbox", 0o755)
	for _, f := range []string{"memory.md", "memory.log.md"} {
		if fh, err := os.OpenFile(dir+"/"+f, os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
			fh.Close()
		}
	}
	var role, responsibilities, reading string
	switch name {
	case "architect":
		role = "You are the architect agent for " + project + ". You own system\ndesign, contracts, and technical architecture."
		responsibilities = "- Answer questions about system architecture and design patterns\n" +
			"- Evaluate design trade-offs and component boundaries\n" +
			"- Reference and maintain contracts in architecture/\n" +
			"- Advise on interfaces, dependencies, and technical direction\n" +
			"- Review changes for architectural conformance"
		reading = "architecture/ (contracts), README.md, DESIGN.md"
	case "product":
		role = "You are the product agent for " + project + ". You represent\nuser needs and own requirements, priorities, and feature decisions."
		responsibilities = "- Answer questions about product requirements and priorities\n" +
			"- Clarify user stories and acceptance criteria\n" +
			"- Make feature trade-off decisions from the user's perspective\n" +
			"- Maintain BDD scenarios and personas in product/\n" +
			"- Prioritize backlog items in TODO.md"
		reading = "product/ (personas, features), README.md, TODO.md"
	case "englead":
		role = "You are the engineering lead agent for " + project + ". You own\nimplementation coordination, code quality, and task management."
		responsibilities = "- Break down architectural decisions into implementation tasks\n" +
			"- Coordinate implementation work and manage priorities\n" +
			"- Review code quality, test coverage, and technical debt\n" +
			"- Manage TODO.md and QUICKCONTEXT.md progress tracking\n" +
			"- Run and interpret the testing cascade (T0-T5)"
		reading = "QUICKCONTEXT.md, TODO.md, AGENTS.md, tests/"
	case "steward":
		role = "You are the steward agent for " + project + ". You are the project's\n" +
			"automated quality scanner — you report facts about project health, contract\n" +
			"lifecycle, and quality gates. You don't prescribe solutions."
		responsibilities = "- Answer questions about project health and contract status\n" +
			"- Read architecture/.state/steward-report.json as primary context\n" +
			"- Report open discoveries, enforcement failures, and stale contracts\n" +
			"- Suggest running `rebar run steward` if the report is stale (>24h old)\n" +
			"- Surface per-role action items from the steward report"
		reading = "architecture/.state/ (steward reports), TODO.md (discoveries), `rebar run steward`"
	case "merger":
		role = "You are the merger agent for " + project + ". You are a coordination\n" +
			"specialist that handles branch integration, cherry-picking, and conflict\n" +
			"resolution. You are an actor agent — you perform actions on the repository."
		responsibilities = "- Merge worktree branches into the target branch\n" +
			"- Cherry-pick commits with conflict resolution\n" +
			"- Produce detailed summaries of all merge decisions\n" +
			"- Flag ambiguous conflicts for human review\n" +
			"- Run verification tests after integration"
		reading = "agents/subagent-prompts/merge-coordinator.md, git log, AGENTS.md"
	default:
		role = "You are the " + name + " agent for " + project + "."
		responsibilities = "- (define responsibilities for this role)"
		reading = "README.md"
	}
	display := name
	if display != "" {
		display = strings.ToUpper(display[:1]) + display[1:]
	}
	md := "# Agent: " + display + "\n\n## Role\n" + role + "\n\n## Responsibilities\n" + responsibilities +
		"\n\n## Project Files\n" + context + `

## Context Loading
When answering questions, read relevant project files — do not guess
from memory alone. Use Read, Grep, and Glob tools to look things up.

Priority for this role: ` + reading + `

General reading order:
1. This file (AGENT.md) + memory.md (your distilled state)
2. README.md (project orientation)
3. QUICKCONTEXT.md (current state — verify against git log)
4. Files relevant to the specific question

## Permissions
- Read: all project files
- Write: (scope appropriate to role)
- Ask: any agent
`
	_ = os.WriteFile(dir+"/AGENT.md", []byte(md), 0o644)
}

// gitignoreEntries are the ASK runtime files `ask init` keeps out of git.
var gitignoreEntries = []string{
	"# ASK runtime artifacts",
	"agents/*/inbox/",
	"agents/*/outbox/",
	"agents/*/.pid",
	"agents/*/.last-stderr",
	"agents/*/.progress",
	"agents/*/.session-id",
	"agents/*/memory.log.md",
	"shared/messages/events.jsonl",
}

// ensureGitignore is _ensure_gitignore.
func (a *App) ensureGitignore() {
	data, err := os.ReadFile(".gitignore")
	if err != nil || !isFile(".gitignore") {
		_ = os.WriteFile(".gitignore", []byte(strings.Join(gitignoreEntries, "\n")+"\n"), 0o644)
		a.out("  created .gitignore with ASK runtime entries\n")
		return
	}
	text := string(data)
	added := 0
	for _, e := range gitignoreEntries {
		if !strings.Contains(text, e) {
			// echo "$entry" >> .gitignore: a missing final newline joins lines.
			appendFile(".gitignore", e+"\n")
			text += e + "\n"
			added++
		}
	}
	if added > 0 {
		a.out("  added %d entries to .gitignore\n", added)
	}
}

// serve runs the enterprise server, ask-server: Serve (the native server,
// set by the CLI).
func (a *App) serve(args []string) {
	if Serve == nil {
		a.die("ask serve: no server in this build (run 'rebar ask serve')")
	}
	a.exit(Serve(args))
}

// Serve is the native `ask serve`.
var Serve func(args []string) int
