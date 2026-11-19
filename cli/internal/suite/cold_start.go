package suite

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/contracts"
	"github.com/willackerly/rebar/cli/internal/gitx"
)

func init() {
	checks.Register(&checks.Check{Name: "cold-start-checks", Run: runColdStart})
}

// coldStartChecks are the checks the hook summarizes, in order.
var coldStartChecks = []string{"check-contract-refs", "check-todos", "check-freshness", "check-ground-truth"}

// runColdStart is scripts/cold-start-checks.sh, the SessionStart hook: one
// line per check, a count of the contracts' declared Status: fields, an
// unpushed-commits note, all inside <rebar-cold-start>; always exit 0.
func runColdStart(ctx *checks.Context, _ []string) int {
	out := ctx.Stdout
	start := time.Now().Unix()
	archDir := contracts.Dir(ctx.Root)

	echo(out, "<rebar-cold-start>")
	if info, err := os.Stat(ctx.Root); err != nil || !info.IsDir() {
		echo(out, "cold-start-checks: cannot cd to "+ctx.Root+" — no checks run.")
		echo(out, "</rebar-cold-start>")
		return 0
	}
	echo(out, "rebar cold-start health — advisory (this hook always exits 0)")
	// The script sends these warnings to stdout (2>&1).
	warnOnce(&checks.Context{Root: ctx.Root, Stdout: out, Stderr: out})

	failCount, runCount := 0, 0
	for _, name := range coldStartChecks {
		s := resolve(ctx, name+".sh", true)
		if s.native == nil && !isRegular(s.path) {
			fmt.Fprintf(out, "[MISS] %s — %s not installed\n", name, s.path)
			continue
		}
		runCount++
		output, status := s.output(ctx, true)
		summary := summarize(output)
		label := "PASS"
		switch {
		case status != 0:
			label = "FAIL"
			failCount++
		case strings.HasPrefix(summary, "SKIP:"):
			label = "SKIP"
		}
		if summary == "" {
			summary = "(no output, exit " + strconv.Itoa(status) + ")"
		}
		fmt.Fprintf(out, "[%s] %s — %s\n", label, name, summary)
	}

	echo(out, maturity(ctx.Root, archDir))
	unpushed(ctx, out)

	elapsed := time.Now().Unix() - start
	if failCount > 0 {
		fmt.Fprintf(out, "%d of %d checks failing (%ds) — rerun the named check (`rebar run <name>`) for full output.\n", failCount, runCount, elapsed)
	} else {
		fmt.Fprintf(out, "all %d checks passing (%ds).\n", runCount, elapsed)
	}
	echo(out, "</rebar-cold-start>")
	return 0
}

var (
	ansiColour   = regexp.MustCompile("\x1b\\[[0-9;]*m")
	checkedLine  = regexp.MustCompile(`Checked [0-9]`)
	verdictLine  = regexp.MustCompile(`^(FAIL|OK|SKIP|DRIFT):`)
	keywordLine  = regexp.MustCompile(`(?i)drift|match|No metrics`)
	statusHeader = regexp.MustCompile(`^\*{0,2}Status:`)
)

// summarize picks the one line that best summarizes a check's output, as
// the script's summarize does: colours stripped, the last "Checked N" line,
// else the first verdict line, else the last line mentioning drift, a match
// or no metrics, else the first non-blank line.
func summarize(output string) string {
	lines := strings.Split(ansiColour.ReplaceAllString(output, ""), "\n")
	last := func(re *regexp.Regexp) string {
		for i := len(lines) - 1; i >= 0; i-- {
			if re.MatchString(lines[i]) {
				return lines[i]
			}
		}
		return ""
	}
	first := func(match func(string) bool) string {
		for _, l := range lines {
			if match(l) {
				return l
			}
		}
		return ""
	}
	if l := last(checkedLine); l != "" {
		return l
	}
	if l := first(verdictLine.MatchString); l != "" {
		return l
	}
	if l := last(keywordLine); l != "" {
		return l
	}
	return first(func(l string) bool { return strings.Trim(l, " \t\n\v\f\r") != "" })
}

// maturity is the script's maturity line: the declared Status: of each
// CONTRACT-*.md in the contracts directory (not recursive; templates, the
// registry, gaps and companions skipped), counted by vocabulary word.
func maturity(root, archDir string) string {
	dir := archDir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	entries, _ := os.ReadDir(dir)
	words := []string{"stub", "draft", "in-progress", "active", "verified", "superseded"}
	counts := map[string]int{}
	other, total, withStatus := 0, 0, 0
	for _, e := range entries {
		if ok, _ := path.Match("CONTRACT-*.md", e.Name()); !ok {
			continue
		}
		f := archDir + "/" + e.Name()
		if _, err := os.Stat(filepath.Join(dir, e.Name())); err != nil {
			continue
		}
		if strings.Contains(f, "TEMPLATE") || strings.Contains(f, "REGISTRY") || strings.Contains(f, "GAPS") || strings.HasSuffix(f, ".impl.md") {
			continue
		}
		total++
		v := declaredStatus(filepath.Join(dir, e.Name()))
		if v == "" {
			continue
		}
		withStatus++
		known := false
		for _, w := range words {
			if v == w {
				counts[w]++
				known = true
			}
		}
		if !known {
			other++
		}
	}
	switch {
	case total == 0:
		return "maturity: no contracts yet"
	case withStatus == 0:
		return "maturity: pre-v3 (no Status fields)"
	}
	var parts []string
	for _, w := range words {
		if counts[w] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[w], w))
		}
	}
	if other > 0 {
		parts = append(parts, fmt.Sprintf("%d unrecognized", other))
	}
	return fmt.Sprintf("maturity: %s (%d of %d contracts declare Status)", strings.Join(parts, ", "), withStatus, total)
}

// declaredStatus is the script's status_val: the first word after the
// first "Status:" header line (**Status:** or Status:), lowercased.
func declaredStatus(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !statusHeader.MatchString(line) {
			continue
		}
		line = strings.TrimPrefix(line, "**Status:**")
		line = strings.TrimPrefix(line, "Status:")
		fields := strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '\t' })
		if len(fields) == 0 {
			return ""
		}
		return asciiLower(fields[0])
	}
	return ""
}

// asciiLower is tr '[:upper:]' '[:lower:]' in the C locale: ASCII letters
// only.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// unpushed prints the reflexive-push note: a warning above 5 commits ahead
// of the upstream, a reminder for 1–5, nothing without an upstream.
func unpushed(ctx *checks.Context, out io.Writer) {
	if _, code := gitx.Run(ctx.Root, nil, "rev-parse", "--abbrev-ref", "@{u}"); code != 0 {
		return
	}
	count, code := gitx.Run(ctx.Root, nil, "rev-list", "--count", "@{u}..HEAD")
	if code != 0 {
		count += "0\n"
	}
	count = strings.TrimRight(count, "\n")
	branch, code := gitx.Run(ctx.Root, nil, "rev-parse", "--abbrev-ref", "HEAD")
	if code != 0 {
		branch += "?\n"
	}
	branch = strings.TrimRight(branch, "\n")
	n, ok := checks.TestInt(count)
	switch {
	case ok && n > 5:
		fmt.Fprintf(out, "[WARN] %s unpushed commit(s) on %s — push now; durability lives at origin, not on this disk\n", count, branch)
	case ok && n > 0:
		fmt.Fprintf(out, "unpushed: %s commit(s) on %s — push when settled (reflexive-push rule)\n", count, branch)
	}
}
