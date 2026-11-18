package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/gitx"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// refreshContext is one run of scripts/refresh-context.sh.
type refreshContext struct {
	out              io.Writer
	repo             string
	red, green, yell string
	nc               string
}

func (r *refreshContext) say(s string)  { fmt.Fprintln(r.out, s) }
func (r *refreshContext) warn(s string) { fmt.Fprintln(r.out, r.yell+"WARNING:"+r.nc+" "+s) }
func (r *refreshContext) ok(s string)   { fmt.Fprintln(r.out, r.green+"OK:"+r.nc+" "+s) }
func (r *refreshContext) err(s string)  { fmt.Fprintln(r.out, r.red+"ERROR:"+r.nc+" "+s) }

// lastSyncedRE is `grep -oE 'last-synced: [0-9]{4}-[0-9]{2}-[0-9]{2}'`.
var lastSyncedRE = regexp.MustCompile(`last-synced: [0-9]{4}-[0-9]{2}-[0-9]{2}`)

// lastSynced is the script's $(grep -oE '<lastSyncedRE>' FILE | grep -oE
// '<date>'): every match's date, one per line ("" for none).
func lastSynced(data []byte) string {
	var dates []string
	for _, line := range splitLines(data) {
		for _, m := range lastSyncedRE.FindAllString(line, -1) {
			dates = append(dates, strings.TrimPrefix(m, "last-synced: "))
		}
	}
	return strings.Join(dates, "\n")
}

// untrackedTODOExts are the script's grep --include globs.
var untrackedTODOExts = []string{"*.ts", "*.tsx", "*.js", "*.jsx", "*.go", "*.py", "*.rs"}

// runRefreshContext is scripts/refresh-context.sh: QUICKCONTEXT.md's
// freshness and the commits since its last-synced date (--quick stops
// there), then TODO.md, untracked TODO comments, worktrees and, with
// --test-baseline as the first or second argument, the test suite.
//
// Where the script depends on the platform the port takes GNU's side: a
// last-synced date that does not exist (2026-02-30), or several
// last-synced dates, give no staleness line (BSD date rolled the first
// over, or read the first and ignored the rest); a binary source file is
// not an untracked TODO (BSD grep listed "Binary file … matches").
//
// Outside a git work tree, or in one without commits, the git sections
// print a "(skipped: …)" note instead, as the script does since v3.1.1. It
// does not print bash's complaint when a count is "0<newline>0" (grep -c … || echo
// 0 with no match), and compares such a count as false, as bash does.
func runRefreshContext(ctx *checks.Context, args []string) int {
	r := &refreshContext{out: ctx.Stdout, repo: toplevel(ctx.WorkDir())}
	if checks.UseColour(ctx.Stdout) {
		r.red, r.green, r.yell, r.nc = "\033[0;31m", "\033[0;32m", "\033[1;33m", "\033[0m"
	}
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	quick := arg(0) == "--quick"

	r.say("=== QUICKCONTEXT Freshness Check ===")
	r.say("")
	// The script's HAS_GIT / HAS_COMMITS: the git sections need a work tree
	// with at least one commit.
	_, inTree := gitx.Run(r.repo, nil, "rev-parse", "--is-inside-work-tree")
	_, hasHead := gitx.Run(r.repo, nil, "rev-parse", "--verify", "-q", "HEAD")
	hasGit, hasCommits := inTree == 0, inTree == 0 && hasHead == 0
	if info, err := os.Stat(resolve(r.repo, "QUICKCONTEXT.md")); err != nil || !info.Mode().IsRegular() {
		r.err("QUICKCONTEXT.md not found in repo root")
		return 1
	}
	qc, _ := os.ReadFile(resolve(r.repo, "QUICKCONTEXT.md"))
	synced := lastSynced(qc)
	if synced == "" {
		r.warn("No last-synced date found in QUICKCONTEXT.md")
		r.warn("Cannot determine freshness — treat all claims as suspect")
	} else {
		r.say("Last synced: " + synced)
		if day, ok := syncDay(synced); ok && day.Unix() > 0 {
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
			days := (today.Unix() - day.Unix() + 43200) / 86400
			switch {
			case days > 7:
				r.warn(fmt.Sprintf("QUICKCONTEXT is %d days stale — treat ALL claims as suspect", days))
			case days > 3:
				r.warn(fmt.Sprintf("QUICKCONTEXT is %d days old — verify critical claims", days))
			default:
				r.ok(fmt.Sprintf("QUICKCONTEXT is %d days old", days))
			}
		}

		r.say("")
		r.say("Commits since last sync:")
		switch {
		case !hasGit:
			r.say("  (skipped: not a git repository)")
		case !hasCommits:
			r.say("  (skipped: no commits yet)")
		default:
			// From the start of the last-synced day: git fills a date-only
			// --since with the current time of day, which dropped that
			// day's later commits.
			since := "--since=" + synced + " 00:00"
			log, _ := gitx.Run(r.repo, nil, "log", since, "--oneline")
			count := strings.Count(log, "\n")
			if log != "" && !strings.HasSuffix(log, "\n") {
				count++
			}
			run(r.repo, ctx.Stdout, nil, "git", "log", "-n", "20", since, "--oneline")
			if count > 20 {
				r.say(fmt.Sprintf("  ... and %d more", count-20))
			}
			switch {
			case count > 10:
				r.warn(fmt.Sprintf("%d commits since last sync — QUICKCONTEXT is likely stale", count))
			case count > 0:
				r.say(fmt.Sprintf("  (%d commits — review against QUICKCONTEXT claims)", count))
			default:
				r.ok("No commits since last sync")
			}
		}
	}
	if quick {
		return 0
	}

	r.say("")
	r.say("=== TODO Freshness ===")
	todoPath := resolve(r.repo, "TODO.md")
	if info, err := os.Stat(todoPath); err == nil && info.Mode().IsRegular() {
		todo, _ := os.ReadFile(todoPath)
		todoSynced := lastSynced(todo)
		if todoSynced == "" {
			todoSynced = "unknown"
		}
		r.say("TODO.md last synced: " + todoSynced)
		open := grepCount(todo, "- [ ]")
		done := grepCount(todo, "- [x]")
		r.say("Open items: " + open + " | Completed items: " + done)
		if d, okD := checks.TestInt(done); okD {
			if o, okO := checks.TestInt(open); okO && d > o {
				r.warn("More completed than open items — consider archiving completed items")
			}
		}
	}

	r.say("")
	r.say("=== Untracked TODOs in Code ===")
	if lines := untrackedTODOs(r.repo); len(lines) > 0 {
		r.warn("Untracked TODO comments found:")
		r.say(strings.Join(lines, "\n"))
	} else {
		r.ok("No untracked TODO comments")
	}

	r.say("")
	r.say("=== Worktree State ===")
	if !hasGit {
		r.say("(skipped: not a git repository)")
	} else {
		worktrees, _ := gitx.Run(r.repo, nil, "worktree", "list")
		worktrees = strings.TrimRight(worktrees, "\n")
		r.say(worktrees)
		if n := strings.Count(worktrees, "\n") + 1; n > 1 {
			r.warn(fmt.Sprintf("%d worktree(s) exist — verify they're active or clean them up", n-1))
			r.say("  To clean: git worktree prune")
		} else {
			r.ok("No extra worktrees")
		}
	}

	if arg(0) == "--test-baseline" || arg(1) == "--test-baseline" {
		r.say("")
		r.say("=== Test Health Baseline ===")
		r.testBaseline()
	}

	r.say("")
	r.say("=== Summary ===")
	r.say("Run 'rebar context session-start' for full context loading")
	r.say("Run 'rebar context' for the Cold Start Quad")
	r.say("Run 'scripts/refresh-context.sh --test-baseline' to verify tests pass")
	return 0
}

// syncDay is the last-synced date as GNU date -d reads it: false for
// several dates or one that does not exist.
func syncDay(s string) (time.Time, bool) {
	if strings.Contains(s, "\n") {
		return time.Time{}, false
	}
	y, errY := strconv.Atoi(s[0:4])
	m, errM := strconv.Atoi(s[5:7])
	d, errD := strconv.Atoi(s[8:10])
	if errY != nil || errM != nil || errD != nil {
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
	if t.Year() != y || int(t.Month()) != m || t.Day() != d {
		return time.Time{}, false
	}
	return t, true
}

// grepCount is $(grep -c '^<prefix>' TODO.md 2>/dev/null || echo 0): the
// count of lines starting with prefix, "0\n0" when there are none (grep -c
// prints 0 and fails, so echo adds another).
func grepCount(data []byte, prefix string) string {
	n := 0
	for _, line := range splitLines(data) {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	if n == 0 {
		return "0\n0"
	}
	return strconv.Itoa(n)
}

// untrackedTODOs is the script's `grep -rn "TODO:" --include=… src/ lib/
// internal/ packages/ cmd/ | grep -v TRACKED-TASK | grep -v node_modules |
// head -10`, files listed in lexical order.
func untrackedTODOs(repo string) []string {
	files := sourcescan.Filter{Include: untrackedTODOExts}.Files(repo, []string{"src/", "lib/", "internal/", "packages/", "cmd/"})
	var found []string
	for _, l := range sourcescan.Search(files, func(text string) bool { return strings.Contains(text, "TODO:") }) {
		s := l.String()
		if strings.Contains(s, "TRACKED-TASK") || strings.Contains(s, "node_modules") {
			continue
		}
		found = append(found, s)
		if len(found) == 10 {
			break
		}
	}
	return found
}

// testBaseline is the --test-baseline section: the detected test command,
// run with its output through `tail -20`.
func (r *refreshContext) testBaseline() {
	isFile := func(name string) bool {
		info, err := os.Stat(resolve(r.repo, name))
		return err == nil && info.Mode().IsRegular()
	}
	have := func(cmd string) bool {
		_, err := exec.LookPath(cmd)
		return err == nil
	}
	var testCmd []string
	switch {
	case isFile("package.json"):
		if have("pnpm") && isFile("pnpm-lock.yaml") {
			testCmd = []string{"pnpm", "test"}
		} else if have("npm") {
			testCmd = []string{"npm", "test"}
		}
	case isFile("go.mod"):
		testCmd = []string{"go", "test", "./..."}
	case isFile("pyproject.toml") || isFile("setup.py"):
		testCmd = []string{"pytest"}
	}
	if testCmd == nil {
		r.warn("No test runner detected — skipping test baseline")
		r.say("  Supported: pnpm/npm (package.json), go (go.mod), pytest (pyproject.toml)")
		return
	}
	r.say("Running: " + strings.Join(testCmd, " "))
	var out bytes.Buffer
	code := run(r.repo, &out, &out, testCmd[0], testCmd[1:]...)
	if code == 127 {
		fmt.Fprintf(&out, "refresh-context: %s: command not found\n", testCmd[0])
	}
	io.WriteString(r.out, tail(out.String(), 20))
	if code == 0 {
		r.ok("Test suite passed")
	} else {
		r.err("Test suite has failures — fix before starting work")
		r.say("  This saves hours of discovering pre-existing failures mid-session")
	}
}

// tail is `tail -n` of s: its last n lines (a last line without a newline
// counts, and is printed without one).
func tail(s string, n int) string {
	if s == "" {
		return ""
	}
	end := len(s)
	if strings.HasSuffix(s, "\n") {
		end--
	}
	i := end
	for k := 0; k < n; k++ {
		j := strings.LastIndexByte(s[:i], '\n')
		if j < 0 {
			return s
		}
		i = j
	}
	return s[i+1:]
}
