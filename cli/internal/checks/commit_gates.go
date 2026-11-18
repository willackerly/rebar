package checks

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/gitx"
)

// check-fix-commit (Gate G) and check-bypass-flags (Gate I) read commit
// messages, HEAD's, a commit-msg file's or those of a range of commits, and
// flag the ones that break the gate. They share everything but the rule.
// Native ports of scripts/check-fix-commit.sh and scripts/check-bypass-flags.sh.
func init() {
	register(&Check{Name: "check-fix-commit", Run: fixCommitGate.run})
	register(&Check{Name: "check-bypass-flags", Run: bypassFlagsGate.run})
}

// A commitGate is one of the commit-message gates.
type commitGate struct {
	name string
	// check prints a violation for the message and reports false, or
	// reports true when the message passes.
	check func(w io.Writer, subject, body, label string) bool
	ok    string // the line printed when nothing is in violation
}

var fixCommitGate = &commitGate{
	name:  "check-fix-commit",
	check: checkFixCommit,
	ok:    "check-fix-commit: OK — Gate G satisfied (no fix:/regression: commits without Reproduced on:)",
}

var bypassFlagsGate = &commitGate{
	name:  "check-bypass-flags",
	check: checkBypassFlags,
	ok:    "check-bypass-flags: OK — Gate I satisfied (no bypass mentions without Bypass tickets:)",
}

// The gates' patterns, as grep -E reads them line by line. [[:space:]] is
// POSIX's (it includes \v, which Go's \s does not), so \S is spelled
// [^[:space:]].
var (
	fixQualifies     = regexp.MustCompile(`^(fix|regression)(\([^)]*\))?:[[:space:]]`)
	fixSatisfied     = regexp.MustCompile(`^Reproduced[[:space:]]on:[[:space:]]*[^[:space:]]`)
	bypassMention    = regexp.MustCompile(`(--skip-(stress|tests|all|e2e|smoke|ci|build|fix|preflight)|--no-verify|--no-gpg-sign|--force[[:space:]]|--force$|SKIP_TESTS=|SKIP_E2E=|SKIP_STRESS=|BYPASS_=)`)
	bypassSatisfied  = regexp.MustCompile(`^Bypass[[:space:]]tickets:[[:space:]]*[^[:space:]]`)
	bypassMetaOptOut = regexp.MustCompile(`^Bypass-flags-meta:[[:space:]]*[^[:space:]]`)
)

// echoLines are the lines grep reads from `echo "$s"`.
func echoLines(s string) []string { return strings.Split(s, "\n") }

// grepQ is `echo "$s" | grep -qE re`.
func grepQ(re *regexp.Regexp, s string) bool {
	for _, line := range echoLines(s) {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func checkFixCommit(w io.Writer, subject, body, label string) bool {
	if !grepQ(fixQualifies, subject) || grepQ(fixSatisfied, body) {
		return true
	}
	fmt.Fprint(w, "\n",
		"check-fix-commit: VIOLATION ("+label+")\n",
		"  subject:  "+subject+"\n",
		"  Gate G requires a line of the form:\n",
		"    Reproduced on: <SHA / deploy URL / log excerpt / screenshot path>\n",
		"\n",
		"  In the body. Without it, the fix is for a phantom — the user's\n",
		"  symptom hasn't been observed independently before fixing began.\n",
		"  Source: practices/regression-fix-protocol.md §Gate G\n")
	return false
}

func checkBypassFlags(w io.Writer, subject, body, label string) bool {
	if !grepQ(bypassMention, body) || grepQ(bypassMetaOptOut, body) || grepQ(bypassSatisfied, body) {
		return true
	}
	// $(echo "$body" | grep -oE … | head -3 | tr '\n' ' '): the first three
	// matches, each followed by a space.
	var matched strings.Builder
	n := 0
	for _, line := range echoLines(body) {
		for _, m := range bypassMention.FindAllString(line, -1) {
			if n < 3 {
				matched.WriteString(m + " ")
			}
			n++
		}
	}
	fmt.Fprint(w, "\n",
		"check-bypass-flags: VIOLATION ("+label+")\n",
		"  subject:  "+subject+"\n",
		"\n",
		"  Body mentions a test-bypass flag (matched: "+matched.String()+")\n",
		"  Gate I requires a line of the form:\n",
		"    Bypass tickets: TICKET-1, TICKET-2 (one ticket per broken test)\n",
		"\n",
		"  And a justification per ticket. Without this, bypassing a test\n",
		"  ships an unowned regression. Source: practices/regression-fix-protocol.md §Gate I\n")
	return false
}

// run is the scripts' main: HEAD's message by default, a commit-msg file
// given as an argument, or every commit of --range <rev-list args>;
// --warn reports without failing. git runs where the script would, in the
// working directory.
func (g *commitGate) run(ctx *Context, args []string) int {
	w := ctx.Stdout
	warnOnly := false
	mode, arg := "head", ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--warn":
			warnOnly = true
		case a == "--range":
			if i+1 >= len(args) {
				// The script dies here on `set -u` ($2: unbound variable).
				fmt.Fprintf(ctx.Stderr, "%s: --range needs a revision range (<sha>..<sha>)\n", g.name)
				return 1
			}
			mode, arg = "range", args[i+1]
			i++
		case a == "-h" || a == "--help":
			fmt.Fprint(w, scriptHelp[g.name])
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(ctx.Stderr, "%s: unknown flag '%s'\n", g.name, a)
			return 2
		default:
			mode, arg = "file", a
		}
	}

	dir := ctx.WorkDir()
	violations := 0
	switch mode {
	case "head":
		subject, body := gitx.Message(dir, nil, "HEAD")
		if subject == "" {
			fmt.Fprintf(w, "%s: no commit found at HEAD\n", g.name)
			return 0
		}
		if !g.check(w, subject, body, "HEAD") {
			violations++
		}
	case "file":
		if !ctx.isFile(arg) {
			fmt.Fprintf(ctx.Stderr, "%s: file not found: %s\n", g.name, arg)
			return 2
		}
		data, _ := os.ReadFile(ctx.path(arg))
		// The subject is the first line that is neither a comment nor
		// empty (grep -vE '^#|^$' | head -1).
		subject := ""
		for _, line := range textLines(data) {
			if line != "" && !strings.HasPrefix(line, "#") {
				subject = line
				break
			}
		}
		body := strings.TrimRight(string(data), "\n")
		if !g.check(w, subject, body, arg) {
			violations++
		}
	case "range":
		out, _ := gitx.Run(dir, nil, "rev-list", arg)
		for _, sha := range gitx.Lines(out) {
			if sha == "" {
				continue
			}
			subject, body := gitx.Message(dir, ctx.Stderr, sha)
			if !g.check(w, subject, body, sha) {
				violations++
			}
		}
	}

	if violations == 0 {
		fmt.Fprintln(w, g.ok)
		return 0
	}
	fmt.Fprintf(w, "\n%s: %d violation(s)\n", g.name, violations)
	fmt.Fprintf(w, "Wire as commit-msg hook:  printf '#!/bin/sh\\nexec rebar run %s \"$1\"\\n' > .git/hooks/commit-msg && chmod +x .git/hooks/commit-msg\n", g.name)
	if warnOnly {
		return 0
	}
	return 1
}
