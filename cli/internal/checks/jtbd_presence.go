package checks

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/willackerly/rebar/cli/internal/contracts"
)

// check-jtbd-presence: the latest version of every contract has non-empty
// "Why this exists", "Who needs this" and "Scenarios" sections. Tier 2
// warns, tier 3 blocks. Native port of scripts/check-jtbd-presence.sh.
func init() {
	register(&Check{Name: "check-jtbd-presence", Run: runJTBDPresence})
}

// jtbdSections are the required headings, in the order the script reports
// them missing.
var jtbdSections = []string{"Why this exists", "Who needs this", "Scenarios"}

func runJTBDPresence(ctx *Context, args []string) int {
	w := ctx.Stdout
	if os.Getenv("SKIP_JTBD") == "1" {
		fmt.Fprintln(w, "SKIP: check-jtbd-presence (SKIP_JTBD=1)")
		return 0
	}
	if Skip(ctx, 2, "check-jtbd-presence") {
		return 0
	}
	tier := CheckTier(ctx.Root, "check-jtbd-presence")
	dir := contracts.Dir(ctx.Root)
	archDir := ctx.Root + "/" + dir
	if info, err := os.Stat(archDir); err != nil || !info.IsDir() {
		fmt.Fprintf(w, "SKIP: no %s/ directory found.\n", dir)
		return 0
	}

	latest := latestContracts(archDir)
	if latest == nil {
		fmt.Fprintf(w, "OK: no contracts found in %s/ — nothing to check.\n", dir)
		return 0
	}

	checked, superseded, failed := 0, 0, 0
	for _, name := range latest {
		file := archDir + "/" + name
		if !isRegular(file) {
			continue
		}
		data, _ := os.ReadFile(file)
		if bytes.Contains(data, []byte("SUPERSEDED BY:")) {
			superseded++
			continue
		}
		checked++
		missing := ""
		for _, s := range jtbdSections {
			if !sectionPresent(data, "## "+s) {
				missing += " '" + s + "'"
			}
		}
		if missing != "" {
			fmt.Fprintf(w, "MISSING: %s/%s —%s\n", dir, name, missing)
			failed++
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Checked %d latest-version contract(s), %d superseded skipped, %d with missing JTBD sections.\n", checked, superseded, failed)
	if failed > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Every contract needs non-empty 'Why this exists', 'Who needs this', and")
		fmt.Fprintln(w, "'Scenarios' sections. An interface description is not a Job To Be Done —")
		fmt.Fprintf(w, "see %s/CONTRACT-TEMPLATE.md for the required framing.\n", dir)
		if n, ok := TestInt(tier); ok && n >= 3 {
			return 1
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "WARN: advisory at Tier %s; blocking at Tier 3.\n", tier)
		return 0
	}
	fmt.Fprintln(w, "OK: all checked contracts carry the JTBD sections.")
	return 0
}

// latestContracts returns the file name of the latest version of each
// contract in dir, sorted (nil when there are no contracts): templates,
// registry files, the gap tracker and companions aside, the file whose
// (major, minor) is highest per ID. Ties, as in the script's sort, go to
// the file whose index line (ID, major, minor and name, tab-separated)
// sorts last.
func latestContracts(dir string) []string {
	type entry struct{ id, major, minor, name string }
	var entries []entry
	for _, name := range contracts.List(dir) {
		if !isRegular(dir+"/"+name) || contracts.IsCompanion(name) || strings.HasPrefix(name, "CONTRACT-REGISTRY") ||
			name == "CONTRACT-TEMPLATE.md" || name == "CONTRACT-SEAM-TEMPLATE.md" || name == "CONTRACT-GAPS.md" {
			continue
		}
		id, major, minor := contracts.ParseName(name)
		entries = append(entries, entry{id, major, minor, name})
	}
	if len(entries) == 0 {
		return nil
	}
	latest := map[string]entry{}
	for _, e := range entries {
		cur, ok := latest[e.id]
		if !ok {
			latest[e.id] = e
			continue
		}
		c := compareNumeric(e.major, cur.major)
		if c == 0 {
			c = compareNumeric(e.minor, cur.minor)
		}
		if c == 0 {
			c = strings.Compare(e.id+"\t"+e.major+"\t"+e.minor+"\t"+e.name, cur.id+"\t"+cur.major+"\t"+cur.minor+"\t"+cur.name)
		}
		if c > 0 {
			latest[e.id] = e
		}
	}
	names := make([]string, 0, len(latest))
	for _, e := range latest {
		names = append(names, e.name)
	}
	slices.Sort(names)
	return names
}

// compareNumeric compares two digit strings by value, as sort -n does.
func compareNumeric(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

var (
	headingRE    = regexp.MustCompile(`^##?[[:space:]]`)
	commentEndRE = regexp.MustCompile(`^.*-->`)
	commentRE    = regexp.MustCompile(`<!--.*-->`)
	commentOpen  = regexp.MustCompile(`<!--.*$`)
	spaceRE      = regexp.MustCompile(`[[:space:]]`)
)

// sectionPresent reports whether data has a line starting with heading
// whose section (up to the next "# " or "## " heading) holds something
// other than whitespace and HTML comments. Subsections count as content.
// Mirrors the script's section_present awk program, which only reads the
// first such heading's section.
func sectionPresent(data []byte, heading string) bool {
	text := string(data)
	if text == "" {
		return false
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	in, inComment := false, false
	for _, line := range lines {
		if !in {
			in = strings.HasPrefix(line, heading)
			continue
		}
		if headingRE.MatchString(line) {
			break
		}
		if inComment {
			if !commentEndRE.MatchString(line) {
				continue
			}
			line = commentEndRE.ReplaceAllString(line, "")
			inComment = false
		}
		line = commentRE.ReplaceAllString(line, "")
		if strings.Contains(line, "<!--") {
			line = commentOpen.ReplaceAllString(line, "")
			inComment = true
		}
		if spaceRE.ReplaceAllString(line, "") != "" {
			return true
		}
	}
	return false
}
