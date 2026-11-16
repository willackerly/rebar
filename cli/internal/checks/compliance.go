package checks

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/contracts"
)

// check-compliance: the rebar version file, the .rebarrc tier, the README
// badge and its agreement with both, AGENTS.md's load-bearing sections
// (tier 2+), the federation drift-check (when CONSUMES.md declares
// dependencies), and the contracts' declared maturity weighed against the
// badge. Native port of scripts/check-compliance.sh.
func init() {
	register(&Check{Name: "check-compliance", Run: runCompliance})
}

var (
	semverTag   = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)
	badgeLine   = regexp.MustCompile(`^[[:space:]]*>[[:space:]]*\*\*rebar v`)
	badgeFormat = regexp.MustCompile(`\*\*rebar (v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?)\*\*.*\*\*Tier ([0-9]+): ([A-Z]+)\*\*`)
	// The required AGENTS.md sections: grep -qi pattern, and the
	// description printed for it.
	agentsSections = []struct {
		re   *regexp.Regexp
		desc string
	}{
		{regexp.MustCompile(`(?i)Cold Start|Read Before Coding`), "Cold Start / Read Before Coding — agents must know the reading order"},
		{regexp.MustCompile(`(?i)Contract-Driven|Contract.Driven`), "Contract-Driven Development — the 4 rules that make contracts operational"},
		{regexp.MustCompile(`(?i)Testing Cascade|Testing Expectations|Scout Rule`), "Testing expectations — cascade tiers or scout rule"},
		{regexp.MustCompile(`(?i)TODO Tracking`), "TODO Tracking — two-tag system prevents invisible debt"},
	}
	askMention    = regexp.MustCompile(`(?i)\bask\b|ASK CLI|ask architect|ask product|ask steward|ask englead`)
	consumesEntry = regexp.MustCompile(`^## [A-Za-z0-9_-]+/[A-Za-z0-9_.-]+$`)
	statusLine    = regexp.MustCompile(`^\*{0,2}Status:`)
	statusMarker  = regexp.MustCompile(`^\*{0,2}Status:\*{0,2}`)
)

// tierLabel is the badge level of a tier.
func tierLabel(tier string) string {
	switch tier {
	case "1":
		return "PARTIAL"
	case "2":
		return "ADOPTED"
	case "3":
		return "ENFORCED"
	}
	return "UNKNOWN"
}

// validTier is [[ $t =~ ^[123]$ ]].
func validTier(t string) bool { return t == "1" || t == "2" || t == "3" }

// rcTierValue is the .rebarrc tier as the scripts read it (grep '^tier' |
// head -1 | sed 's/.*=[[:space:]]*//' | tr -d ' '): the first line starting
// with "tier", after its last "=" and the whitespace after it, spaces
// dropped. ok is false without such a line.
func rcTierValue(data []byte) (string, bool) {
	for _, line := range textLines(data) {
		if !strings.HasPrefix(line, "tier") {
			continue
		}
		v := line
		if i := strings.LastIndex(v, "="); i >= 0 {
			v = strings.TrimLeft(v[i+1:], posixSpace)
		}
		return strings.ReplaceAll(v, " ", ""), true
	}
	return "", false
}

func runCompliance(ctx *Context, args []string) int {
	root := ctx.Root
	w := ctx.Stdout
	say := func(lines ...string) {
		for _, l := range lines {
			fmt.Fprint(w, l+"\n")
		}
	}
	errors := 0

	// Check 1: .rebar-version.
	say("=== Rebar compliance check ===", "")
	declaredVersion := ""
	if !isRegular(root + "/.rebar-version") {
		say("FAIL: .rebar-version file not found", "  Create it with: echo 'v3.1.0-rc.2' > .rebar-version")
		errors++
	} else {
		data, _ := os.ReadFile(root + "/.rebar-version")
		declaredVersion = deleteBytes(cmdSubst(data), posixSpace)
		if semverTag.MatchString(declaredVersion) {
			say("OK: .rebar-version = " + declaredVersion)
		} else {
			say("FAIL: .rebar-version contains '" + declaredVersion + "' — expected format: vX.Y.Z or vX.Y.Z-prerelease")
			errors++
		}
	}

	// Check 2: the .rebarrc tier (the project's .rebarrc, not REBAR_RCFILE).
	declaredTier := ""
	if !isRegular(root + "/.rebarrc") {
		say("FAIL: .rebarrc file not found", "  Create it from .rebarrc.template")
		errors++
	} else {
		data, _ := os.ReadFile(root + "/.rebarrc")
		declaredTier, _ = rcTierValue(data)
		if validTier(declaredTier) {
			say("OK: .rebarrc tier = " + declaredTier + " (" + tierLabel(declaredTier) + ")")
		} else {
			say("FAIL: .rebarrc tier is '" + declaredTier + "' — expected 1, 2, or 3")
			errors++
		}
	}

	// Checks 3–5: the README badge.
	if !isRegular(root + "/README.md") {
		say("FAIL: README.md not found")
		errors++
	} else {
		data, _ := os.ReadFile(root + "/README.md")
		lineNum, content := 0, ""
		for i, line := range textLines(data) {
			if badgeLine.MatchString(line) {
				lineNum, content = i+1, line
				break
			}
		}
		switch m := badgeFormat.FindStringSubmatch(content); {
		case lineNum == 0:
			say("FAIL: README.md has no rebar badge",
				"  Add this as the first line after your # Title:",
				"  > **rebar v3.1.0-rc.2** | **Tier 2: ADOPTED**")
			errors++
		case m == nil:
			say(fmt.Sprintf("FAIL: README.md badge is malformed on line %d", lineNum),
				"  Found: "+content,
				"  Expected: > **rebar vX.Y.Z** | **Tier N: LEVEL**",
				"  Where LEVEL is PARTIAL (1), ADOPTED (2), or ENFORCED (3)")
			errors++
		default:
			version, tier, level := m[1], m[3], m[4]
			say(fmt.Sprintf("OK: README.md badge found on line %d", lineNum),
				"    Version: "+version+" | Tier: "+tier+" ("+level+")")
			if lineNum > 5 {
				say(fmt.Sprintf("WARN: Badge is on line %d — should be within the first 3 lines (right after # Title)", lineNum))
			}
			if declaredVersion != "" && version != declaredVersion {
				say("FAIL: Badge says " + version + " but .rebar-version says " + declaredVersion)
				errors++
			}
			if declaredTier != "" && tier != declaredTier {
				say("FAIL: Badge says Tier " + tier + " but .rebarrc says tier = " + declaredTier)
				errors++
			}
			if want := tierLabel(tier); level != want {
				say("FAIL: Tier " + tier + " should be '" + want + "', not '" + level + "'")
				errors++
			}
		}
	}

	// Checks 6–7: AGENTS.md at tier 2+ (checks_enable counts; a tier that
	// is not a number makes the script's [ -ge ] fail, so neither runs).
	agents := root + "/AGENTS.md"
	if tier, ok := TestInt(CheckTier(root, "check-compliance")); ok && tier >= 2 {
		if isRegular(agents) {
			say("", "=== AGENTS.md required sections ===")
			data, _ := os.ReadFile(agents)
			for _, s := range agentsSections {
				if anyLineMatches(s.re, data) {
					say("OK: " + s.desc)
				} else {
					say("FAIL: AGENTS.md missing required section: " + s.desc)
					errors++
				}
			}
			if anyLineMatches(askMention, data) {
				say("OK: AGENTS.md mentions ASK CLI")
			} else {
				say("FAIL: AGENTS.md does not mention ASK CLI",
					"  Agents need to know they can use 'ask <role> \"question\"' for focused queries.",
					"  Add a reference in the Cold Start or Reference section.")
				errors++
			}
		} else {
			say("", "FAIL: AGENTS.md not found (required at Tier 2+)")
			errors++
		}
	}

	// Check 8: drift-check wired when CONSUMES.md declares dependencies.
	if consumes := root + "/CONSUMES.md"; isRegular(consumes) {
		say("", "=== Federation (CONSUMES.md present) ===")
		data, _ := os.ReadFile(consumes)
		entries, fence := 0, false
		for _, line := range textLines(data) {
			if strings.HasPrefix(line, "```") {
				fence = !fence
				continue
			}
			if !fence && consumesEntry.MatchString(line) {
				entries++
			}
		}
		if entries == 0 {
			say("OK: CONSUMES.md present but no entries declared (federation opt-in not yet active)")
		} else {
			say(fmt.Sprintf("OK: CONSUMES.md declares %d cross-repo dependency(ies)", entries))
			if driftCheckWired(root) {
				say("OK: drift-check is wired into CI")
			} else {
				say("FAIL: CONSUMES.md declares dependencies but `rebar contract drift-check` is not wired into CI",
					"  Add to scripts/ci-check.sh (or your CI of choice):",
					"    rebar contract drift-check",
					"  Without this, consumed contracts can silently age out as upstream evolves (CHARTER §1.6).")
				errors++
			}
		}
	}

	// Check 9: contract maturity from the declared Status: fields.
	contractsDir := contracts.Dir(root)
	if archDir := root + "/" + contractsDir; isDirPath(archDir) {
		errors += complianceMaturity(say, archDir, contractsDir, declaredTier)
	}

	say("")
	if errors > 0 {
		say(fmt.Sprintf("FAIL: %d compliance issue(s) found.", errors))
		return 1
	}
	say("OK: Rebar compliance verified.")
	return 0
}

// complianceMaturity is Check 9; it returns the errors it found.
func complianceMaturity(say func(...string), archDir, contractsDir, declaredTier string) int {
	say("", "=== Contract maturity (declared Status:) ===")
	statusTotal, liveTotal, liveDeclared, stubDraft := 0, 0, 0, 0
	var missing []string
	for _, name := range contracts.List(archDir) {
		path := archDir + "/" + name
		if !isRegular(path) || contracts.IsIndex(name) || contracts.IsCompanion(name) {
			continue
		}
		data, _ := os.ReadFile(path)
		raw := ""
		if !hasNULBlock(data) { // GNU grep prints no line of a binary file
			for _, line := range textLines(data) {
				if statusLine.MatchString(line) {
					raw = line
					break
				}
			}
		}
		// sed -E 's/^\*{0,2}Status:\*{0,2}//' | awk '{print $1}' |
		// tr -d '*' | tr '[:upper:]' '[:lower:]'
		status := ""
		if f := awkFields(statusMarker.ReplaceAllString(raw, "")); len(f) > 0 {
			status = asciiLower(strings.ReplaceAll(f[0], "*", ""))
		}
		if status != "" {
			statusTotal++
		}
		switch status {
		case "superseded", "deprecated", "retired":
			continue // out of the live mix
		}
		liveTotal++
		switch status {
		case "":
			missing = append(missing, name)
		case "stub", "draft":
			liveDeclared++
			stubDraft++
		case "in-progress", "active", "verified":
			liveDeclared++
		default:
			src := raw
			if len(src) > 120 {
				src = src[:120] // head -c 120
			}
			say("WARN: "+name+" declares Status: '"+status+"' — not in the maturity vocabulary (stub|draft|in-progress|active|verified); counted as stub-or-draft",
				"  source line: "+src)
			stubDraft++
		}
	}

	switch {
	case statusTotal == 0 && liveTotal > 0:
		say("ADVISORY: no contract declares a Status: field — treating as a pre-v3 repo (no maturity penalty).",
			"  Add '**Status:** <stub|draft|in-progress|active|verified>' to each contract header",
			"  (see "+contractsDir+"/CONTRACT-TEMPLATE.md) so the badge can reflect real maturity.")
	case statusTotal == 0:
		say("OK: no contracts found — maturity weighting not applicable")
	default:
		for _, name := range missing {
			say("WARN: " + name + " has no Status: line — add one (see " + contractsDir + "/CONTRACT-TEMPLATE.md header)")
		}
	}

	if statusTotal == 0 {
		return 0
	}
	if liveTotal == 0 {
		say("OK: no live contracts declare maturity (all terminal) — no maturity weighting")
		return 0
	}
	// Undeclared live contracts count as stub-or-draft (so do, a second
	// time, the ones with a status outside the vocabulary: the script's
	// arithmetic, kept).
	if undeclared := liveTotal - liveDeclared; undeclared > 0 {
		stubDraft += undeclared
	}
	tenths := stubDraft * 1000 / liveTotal
	pct := fmt.Sprintf("%d.%d", tenths/10, tenths%10)
	say(fmt.Sprintf("OK: %d of %d live contract(s) declare maturity — %d stub-or-draft-or-undeclared (%s%%)", liveDeclared, liveTotal, stubDraft, pct))
	if !validTier(declaredTier) {
		say("NOTE: no valid tier declared — maturity computed but badge weighting skipped")
		return 0
	}
	label := "Tier " + declaredTier + ": " + tierLabel(declaredTier)
	switch {
	case liveTotal < 2:
		say(fmt.Sprintf("NOTE: only %d live contract(s) — maturity is advisory at this population", liveTotal),
			"  (a percentage over n=1 quantizes to 0% or 100%; badge not weighted)")
	case stubDraft*100 < liveTotal*33:
		say("OK: maturity supports the declared badge (" + label + ")")
	case stubDraft*100 <= liveTotal*66:
		say("NOTE: "+pct+"% of live contracts are stub-or-draft — badge reads as",
			"  '"+label+" — IN PROGRESS' until the set matures")
	default:
		say("FAIL: " + pct + "% of live contracts are stub-or-draft (>66%) — badge demoted one tier")
		demoted := fmt.Sprint(int(declaredTier[0]-'0') - 1)
		if demoted != "0" {
			say("  Declared: " + label + " — Effective: Tier " + demoted + ": " + tierLabel(demoted))
		} else {
			say("  Declared: " + label + " — Effective: below Tier 1 (adoption not yet real)")
		}
		say("  Reason: a tier claimed on top of a mostly stub/draft contract set overstates adoption.",
			"  Fix: mature the contracts (or lower the badge) until <=66% are stub-or-draft.")
		return 1
	}
	return 0
}

// driftCheckWired reports whether "drift-check" appears in
// scripts/ci-check.sh, scripts/pre-commit.sh, the Makefile or a
// .github/workflows entry (the glob, so not a dot file; a directory is
// searched as grep -r does, not following symlinks below it).
func driftCheckWired(root string) bool {
	candidates := []string{root + "/scripts/ci-check.sh", root + "/scripts/pre-commit.sh", root + "/Makefile"}
	if entries, err := os.ReadDir(root + "/.github/workflows"); err == nil {
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				candidates = append(candidates, root+"/.github/workflows/"+e.Name())
			}
		}
	}
	needle := []byte("drift-check")
	for _, c := range candidates {
		info, err := os.Stat(c) // [ -e ]
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if data, err := os.ReadFile(c); err == nil && bytes.Contains(data, needle) {
				return true
			}
			continue
		}
		// grep -r follows a symlink given as an operand, not below it.
		if real, err := filepath.EvalSymlinks(c); err == nil {
			c = real
		}
		found := false
		filepath.WalkDir(c, func(p string, d fs.DirEntry, err error) error {
			if err != nil || found {
				return nil
			}
			if d.Type().IsRegular() {
				if data, err := os.ReadFile(p); err == nil && bytes.Contains(data, needle) {
					found = true
					return filepath.SkipAll
				}
			}
			return nil
		})
		if found {
			return true
		}
	}
	return false
}

// cmdSubst is $(cat file): the text without its trailing newlines and
// without NUL bytes (bash drops them).
func cmdSubst(data []byte) string {
	return strings.TrimRight(strings.ReplaceAll(string(data), "\x00", ""), "\n")
}

// deleteBytes is tr -d with a set of single-byte characters.
func deleteBytes(s, set string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(set, s[i]) < 0 {
			b = append(b, s[i])
		}
	}
	return string(b)
}

// awkFields are awk's default fields: split on runs of blanks (space, tab,
// newline), leading and trailing blanks ignored.
func awkFields(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' })
}

// isDirPath is [ -d p ].
func isDirPath(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
