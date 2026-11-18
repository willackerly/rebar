package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/suite"
)

// Suite copies that only add the project's own checks: a scripts/ci-check.sh
// or scripts/pre-commit.sh that is a released rebar version plus calls to
// project scripts rebar does not ship becomes the thin wrapper, with those
// scripts in .rebarrc checks_extra / checks_extra_pre_commit (run by rebar's
// suites after their own checks), once rebar's suite with them exits as the
// copy does on the project.

// suiteExtraKeys maps a suite script to its checks_extra key.
var suiteExtraKeys = map[string]string{"ci-check.sh": suite.ExtraKey, "pre-commit.sh": suite.ExtraPreCommitKey}

// scriptRefRe finds a scripts/<name> reference in a suite: through the
// suite's own directory variable, a root variable, or a relative path.
var scriptRefRe = regexp.MustCompile(`(?:\$\{?(?:SCRIPT_DIR|BASE|HERE|DIR)\}?|\$\{?(?:REPO_ROOT|ROOT|PROJECT_ROOT|REBAR_PROJECT_ROOT)\}?/scripts|(?:^|[\s"'=(])\.?/?scripts)/([A-Za-z0-9_.-]+)`)

// The plain fatal call forms a suite copy may use for a project check that
// becomes a checks_extra entry: anything else (`|| true`, `|| echo`, an
// `if ! check; then warn; fi`, extra statements on the line, arguments) runs
// the check with other consequences than rebar's suites give an extra.
const refWord = `"?(?:\$\{?(?:SCRIPT_DIR|BASE|HERE|DIR)\}?|\$\{?(?:REPO_ROOT|ROOT|PROJECT_ROOT|REBAR_PROJECT_ROOT)\}?/scripts|\.?/?scripts)/([A-Za-z0-9_.-]+)"?`

var (
	// run_check "Label" SKIP_VAR <script>: ci-check's runner counts a
	// failure. v3.1 copies name the script bare (resolved in scripts/).
	runCheckCallRe = regexp.MustCompile(`^\s*run_check\s+(?:"[^"]*"|'[^']*'|[^\s"']+)\s+[A-Za-z_][A-Za-z0-9_]*\s+(?:` + refWord + `|"?([A-Za-z0-9_.-]+)"?)\s*$`)
	// <script> || failed=$((failed + 1)): pre-commit's counter.
	counterCallRe = regexp.MustCompile(`^\s*` + refWord + `\s*\|\|\s*(?:failed=\$\(\(\s*failed\s*\+\s*1\s*\)\)|\(\(\s*failed\s*\+\+\s*\)\)|failed=1)\s*;?\s*$`)
	// if [ -x <script> ]; then: the only if allowed around a call, holding
	// just that call.
	guardRe = regexp.MustCompile(`^\s*if\s+(?:\[\s+-[xf]\s+` + refWord + `\s+\]|test\s+-[xf]\s+` + refWord + `)\s*;\s*then\s*$`)
	// leadInRe: lines that may lead into a call or sit beside it in its
	// guard: blank, comment, or an echo of literal text.
	leadInRe = regexp.MustCompile("^\\s*(?:#.*|echo(?:\\s+-[neE]+)?(?:\\s+(?:\"(?:[^\"\\\\`$]|\\\\.|\\$[A-Za-z_{])*\"|'[^']*'|[A-Za-z0-9_.,:!/-]+))*\\s*|)$")

	blockOpenRe  = regexp.MustCompile(`^\s*(?:if|for|while|until|select|case)\b|^\s*(?:function\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\(\)\s*\{?\s*$|^\s*\{\s*$`)
	blockCloseRe = regexp.MustCompile(`^\s*(?:fi|done|esac|\})(?:\s|;|$)`)
	oneLineRe    = regexp.MustCompile(`(?:;\s*|\s)(?:fi|done|esac)(?:\s|;|<|>|\||$)|;\s*\}\s*$`)
)

// plainCall is the script a suite line runs as a plain fatal call ("" when
// the line is something else).
func plainCall(line string) string {
	if mm := runCheckCallRe.FindStringSubmatch(line); mm != nil {
		if mm[1] != "" {
			return mm[1]
		}
		return mm[2]
	}
	if mm := counterCallRe.FindStringSubmatch(line); mm != nil {
		return mm[1]
	}
	return ""
}

// guardFor is the script an `if [ -x <script> ]; then` line tests.
func guardFor(line string) string {
	if mm := guardRe.FindStringSubmatch(line); mm != nil {
		if mm[1] != "" {
			return mm[1]
		}
		return mm[2]
	}
	return ""
}

// blocks maps each line to the start of the innermost block (if, loop,
// case, function, brace group) around it (-1 at top level), and each
// block start to its end.
func blocks(code []string) (inner []int, end map[int]int) {
	inner, end = make([]int, len(code)), map[int]int{}
	var stack []int
	for i, l := range code {
		if blockCloseRe.MatchString(l) && len(stack) > 0 {
			end[stack[len(stack)-1]] = i
			stack = stack[:len(stack)-1]
		}
		inner[i] = -1
		if len(stack) > 0 {
			inner[i] = stack[len(stack)-1]
		}
		if blockOpenRe.MatchString(l) && !oneLineRe.MatchString(l) {
			stack = append(stack, i)
		}
	}
	return inner, end
}

// projectRefs are the project scripts (not rebar's, executable in the
// project) a suite code line names.
func (m *migrator) projectRefs(l string) []string {
	var names []string
	add := func(name string) {
		if name != "" && !isRebarScript(name) && isExecutable(filepath.Join(m.opts.ProjectRoot, "scripts", name)) {
			names = appendUnique(names, name)
		}
	}
	for _, mm := range scriptRefRe.FindAllStringSubmatch(l, -1) {
		add(mm[1])
	}
	if mm := runCheckCallRe.FindStringSubmatch(l); mm != nil {
		add(mm[2])
	}
	return names
}

// suiteExtras finds the project scripts (not rebar's) that the suite copy
// src calls, in order, as checks_extra entries (scripts/<name>), and
// whether src without those calls is a version rebar released (blobs).
// Each call must be a plain fatal call (plainCall) at the top level of the
// script, or alone in an `if [ -x <same script> ]` guard: why is set,
// and released false, when one is not. The calls are taken out as their
// lines (or guard blocks), with or without the echo / comment / blank
// lines that lead in.
func (m *migrator) suiteExtras(src string, blobs map[string]bool) (extras []string, released bool, why string) {
	lines := strings.Split(src, "\n")
	code := codeLines(src, ".sh")
	inner, end := blocks(code)
	type span struct{ start, end int }
	var calls []span
	for i, l := range code {
		refs := m.projectRefs(l)
		if len(refs) == 0 {
			continue
		}
		for _, name := range refs {
			extras = appendUnique(extras, "scripts/"+name)
		}
		if why != "" || (len(refs) == 1 && guardFor(l) == refs[0]) {
			continue // a guard is checked with the call it holds
		}
		line := strings.TrimSpace(lines[i])
		name := plainCall(l)
		switch {
		case name == "" || len(refs) > 1 || name != refs[0]:
			why = fmt.Sprintf("line %d (`%s`) is not a plain fatal call (`run_check \"Label\" SKIP_VAR <script>` or `<script> || failed=$((failed + 1))`), so a checks_extra entry would not behave the same", i+1, line)
			continue
		case inner[i] < 0:
			calls = append(calls, span{i, i})
			continue
		}
		g := inner[i]
		e, closed := end[g]
		ok := closed && guardFor(code[g]) == name
		for k := g + 1; ok && k < e; k++ {
			ok = k == i || leadInRe.MatchString(lines[k])
		}
		if !ok {
			why = fmt.Sprintf("line %d (`%s`) runs inside `%s` (line %d), which checks_extra cannot express (it runs the check unconditionally)", i+1, line, strings.TrimSpace(lines[g]), g+1)
			continue
		}
		calls = append(calls, span{g, e})
	}
	if len(extras) == 0 || why != "" {
		return extras, false, why
	}
	for _, leadIn := range []bool{false, true} {
		drop := map[int]bool{}
		for _, c := range calls {
			start := c.start
			if leadIn {
				for start > 0 && leadInRe.MatchString(lines[start-1]) && !drop[start-1] {
					start--
				}
			}
			for k := start; k <= c.end; k++ {
				drop[k] = true
			}
		}
		var kept []string
		for k, l := range lines {
			if !drop[k] {
				kept = append(kept, l)
			}
		}
		if blobs[gitBlobHash([]byte(strings.Join(kept, "\n")))] {
			return extras, true, ""
		}
	}
	return extras, false, ""
}

func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// suiteCopy decides a customized ci-check.sh / pre-commit.sh copy (sp is
// byProvenance's verdict): one that only adds calls to project scripts is
// replaced by the thin wrapper, its calls moving to checks_extra(_pre_commit),
// when rebar's suite with them exits as the copy does on the project.
func (m *migrator) suiteCopy(name string, sp *ScriptPlan) *ScriptPlan {
	key := suiteExtraKeys[name]
	if sp.Verdict != Keep || key == "" {
		return sp
	}
	data, err := os.ReadFile(m.legacyPath(name))
	if err != nil {
		return sp
	}
	extras, released, why := m.suiteExtras(string(data), releasedBlobs(m.opts.RebarRoot, name))
	switch {
	case len(extras) == 0:
		return sp
	case why != "":
		sp.Reason = fmt.Sprintf("customized: calls project scripts (%s), but %s; review it by hand (move the calls to .rebarrc %s)", strings.Join(extras, ", "), why, key)
		return sp
	case !released:
		sp.Reason = fmt.Sprintf("customized: calls project scripts (%s), but differs from every released %s in more than those calls; review it by hand (move the calls to .rebarrc %s)", strings.Join(extras, ", "), name, key)
		return sp
	}
	value := strings.Join(extras, " ")
	if cur, set := RCValue(m.rc, key); set {
		if strings.Join(strings.Fields(cur), " ") != value {
			sp.Reason = fmt.Sprintf("calls project scripts (%s), but .rebarrc already sets %s = %s; make it list them and run migrate again", strings.Join(extras, ", "), key, cur)
			return sp
		}
	}
	setting := Setting{Key: key, Value: value, Source: "scripts/" + name}
	// A suite runs the steward, which writes its report: that is not a
	// side effect of the copy (rebar's suite writes the same), so the run
	// does not count against it as runLegacy would.
	before := run(m.opts.Timeout, m.work, m.env("REBAR_PROJECT_ROOT="+m.work, "REBAR_TIER=3"), m.workPath(name))
	m.noteWrites("")
	sp.Before, sp.Check = &before, []string{}
	if before.Err != nil {
		sp.Reason = "the legacy copy did not run: " + before.Err.Error()
		return sp
	}
	extra := []Setting{setting}
	if _, set := RCValue(m.rc, key); set {
		extra = nil
	}
	after := m.runInstall(m.work, name, extra)
	if after.Err != nil || after.Code != before.Code {
		sp.Reason = fmt.Sprintf("calls project scripts (%s) rebar's suite would run as %s, but with them it exits %d where the copy exits %d: %s",
			strings.Join(extras, ", "), key, after.Code, before.Code, firstDiffAs(summaryLines(before.Stdout), summaryLines(after.Stdout), "copy", "rebar"))
		return sp
	}
	if diff, ok := m.forcedFailure(name, extras, extra); !ok {
		sp.Reason = fmt.Sprintf("calls project scripts (%s), but when they fail %s (with them as %s); the copy does not treat their failure as rebar's suite would, review it by hand", strings.Join(extras, ", "), diff, key)
		return sp
	}
	m.accepted = append(m.accepted, extra...)
	sp.Verdict = Replace
	sp.ReplaceWith = "templates/project-bootstrap/scripts/" + name
	sp.ReplaceContent = templateScript(name)
	sp.Reason = fmt.Sprintf("a released %s plus calls to project scripts (%s): rebar's suite runs them as %s and exits %d like the copy; replaced by the thin wrapper that calls `rebar check`",
		name, strings.Join(extras, ", "), key, before.Code)
	return sp
}

// forcedFailure reruns the suite copy and rebar's suite (with extra) with
// every extra script replaced by one that fails: the copy must fail as
// rebar's suite does, so a call whose failure the copy ignores (or only
// warns about) is not turned into a fatal checks_extra entry. diff says how
// the exits differ when it does not.
func (m *migrator) forcedFailure(name string, extras []string, extra []Setting) (diff string, ok bool) {
	defer m.noteWrites("") // restores the scripts
	for _, e := range extras {
		p := filepath.Join(m.work, filepath.FromSlash(e))
		if err := os.WriteFile(p, []byte("#!/bin/sh\necho \"rebar migrate: forced failure of "+e+"\"\nexit 1\n"), 0o755); err != nil {
			return "the scripts could not be replaced (" + err.Error() + ")", false
		}
	}
	before := run(m.opts.Timeout, m.work, m.env("REBAR_PROJECT_ROOT="+m.work, "REBAR_TIER=3"), m.workPath(name))
	m.mu.Lock()
	all := append(append([]Setting{}, m.accepted...), extra...)
	m.mu.Unlock()
	rc, _ := MergeRC(m.rc, all)
	rcPath, err := m.writeRC(rc)
	if err != nil {
		return "rebar's suite could not run (" + err.Error() + ")", false
	}
	after := runNative(m.work, rcPath, m.env("REBAR_PROJECT_ROOT="+m.work, "REBAR_TIER=3"), name)
	switch {
	case before.Err != nil:
		return "the copy did not run (" + before.Err.Error() + ")", false
	case after.Err != nil:
		return "rebar's suite did not run (" + after.Err.Error() + ")", false
	case after.Code == 0 || before.Code != after.Code:
		return fmt.Sprintf("the copy exits %d where rebar's suite exits %d", before.Code, after.Code), false
	}
	return "", true
}

// summaryLines keeps a suite's PASS / FAIL / SKIP lines, for messages.
func summaryLines(out string) string {
	var keep []string
	for _, l := range strings.Split(ansiRe.ReplaceAllString(out, ""), "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "PASS ") || strings.HasPrefix(t, "FAIL ") || strings.HasPrefix(t, "FAIL:") {
			keep = append(keep, t)
		}
	}
	return strings.Join(keep, "\n")
}
