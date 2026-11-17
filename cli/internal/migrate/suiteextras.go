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

var (
	ifOpenRe  = regexp.MustCompile(`^\s*if\b`)
	fiCloseRe = regexp.MustCompile(`^\s*fi\b`)
	leadInRe  = regexp.MustCompile(`^\s*(?:echo\b.*|#.*|)$`)
)

// suiteExtras finds the project scripts (not rebar's) that the suite copy
// src calls, in order, as checks_extra entries (scripts/<name>), and
// whether src without those calls is a version rebar released (blobs). The
// calls are taken out as their lines, or as the if … fi blocks holding
// them, with or without the echo / comment / blank lines that lead in.
func (m *migrator) suiteExtras(src string, blobs map[string]bool) (extras []string, released bool) {
	lines := strings.Split(src, "\n")
	code := codeLines(src, ".sh")
	var refLines []int
	for i, l := range code {
		hit := false
		for _, mm := range scriptRefRe.FindAllStringSubmatch(l, -1) {
			name := mm[1]
			if isRebarScript(m.opts.RebarRoot, name) || !isExecutable(filepath.Join(m.opts.ProjectRoot, "scripts", name)) {
				continue
			}
			hit = true
			extras = appendUnique(extras, "scripts/"+name)
		}
		if hit {
			refLines = append(refLines, i)
		}
	}
	if len(extras) == 0 {
		return nil, false
	}
	for _, block := range []bool{false, true} {
		for _, leadIn := range []bool{false, true} {
			drop := map[int]bool{}
			for _, i := range refLines {
				start, end := i, i
				if block {
					start, end = enclosingIf(lines, i)
				}
				if leadIn {
					for start > 0 && leadInRe.MatchString(lines[start-1]) && !drop[start-1] {
						start--
					}
				}
				for k := start; k <= end; k++ {
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
				return extras, true
			}
		}
	}
	return extras, false
}

// enclosingIf is the outermost if … fi block around line i (i, i when there
// is none).
func enclosingIf(lines []string, i int) (int, int) {
	start, end := i, i
	for s := i; s >= 0; s-- {
		if !ifOpenRe.MatchString(lines[s]) {
			continue
		}
		depth := 0
		for e := s; e < len(lines); e++ {
			if ifOpenRe.MatchString(lines[e]) {
				depth++
			}
			if fiCloseRe.MatchString(lines[e]) {
				depth--
				if depth == 0 {
					if e >= i {
						start, end = s, e
					}
					break
				}
			}
		}
	}
	return start, end
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
	extras, released := m.suiteExtras(string(data), releasedBlobs(m.opts.RebarRoot, name))
	switch {
	case len(extras) == 0:
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
	before := run(m.opts.Timeout, m.work, scrubbedEnv("REBAR_PROJECT_ROOT="+m.work, "REBAR_TIER=3"), m.workPath(name))
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
	m.accepted = append(m.accepted, extra...)
	sp.Verdict = Replace
	sp.ReplaceWith = "templates/project-bootstrap/scripts/" + name
	sp.ReplaceContent = templateScript(name)
	sp.Reason = fmt.Sprintf("a released %s plus calls to project scripts (%s): rebar's suite runs them as %s and exits %d like the copy; replaced by the thin wrapper that calls `rebar check`",
		name, strings.Join(extras, ", "), key, before.Code)
	return sp
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
