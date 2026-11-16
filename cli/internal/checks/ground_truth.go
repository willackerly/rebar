package checks

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// check-ground-truth: compute the metrics declared as metric.<key>.* lines
// in .rebarrc and compare them with the METRICS file (key = value exact,
// key >= value a floor); --print prints the computed values instead.
// Native port of scripts/check-ground-truth.sh.
func init() {
	register(&Check{Name: "check-ground-truth", Run: runGroundTruth})
}

// The script's colours, as echo -e reads them.
const (
	gtRed    = `\033[0;31m`
	gtGreen  = `\033[0;32m`
	gtYellow = `\033[1;33m`
	gtNC     = `\033[0m`
)

func runGroundTruth(ctx *Context, args []string) int {
	printOnly := len(args) > 0 && args[0] == "--print"
	if !printOnly && Skip(ctx, 3, "check-ground-truth") {
		return 0
	}
	root := ctx.Root
	rcFile := config.RCFile(root)
	w := ctx.Stdout
	if printOnly {
		for _, m := range computeMetrics(root, rcFile) {
			fmt.Fprintf(w, "%s=%s\n", m.name, m.value)
		}
		return 0
	}
	metricsFile := root + "/METRICS"
	if !isRegular(metricsFile) && isRegular(root+"/METRICS.md") {
		metricsFile = root + "/METRICS.md"
	}
	strict := false
	switch config.RCValueOr(root, "metrics_strict", "REBAR_METRICS_STRICT", "false") {
	case "true", "1", "yes", "on":
		strict = true
	}

	echo := func(line string) { fmt.Fprint(w, line+"\n") }
	echoE := func(line string) { fmt.Fprint(w, echoEscapes(line)) }
	exit := 0
	echo("=== Ground Truth Verification ===")
	echo("")
	func() { // verify
		if !isRegular(metricsFile) {
			if strict {
				echoE(gtRed + "FAIL" + gtNC + ": No METRICS file found (metrics_strict = true)")
				exit = 1
			} else {
				echoE(gtYellow + "SKIP" + gtNC + ": No METRICS file found")
			}
			echo("  Create a METRICS file with key = value pairs and declare how")
			echo("  to compute each one as metric.<key>.* lines in .rebarrc.")
			return
		}
		computed := computeMetrics(root, rcFile)
		if len(computed) == 0 {
			echo("No metrics declared. Add metric.<key>.* lines to .rebarrc (see this script's header).")
			return
		}
		data, _ := os.ReadFile(metricsFile)
		lines := textLines(data)
		for _, m := range computed {
			key, value := m.name, m.value
			claim := regexp.MustCompilePOSIX(`^[[:space:]]*` + key + `[[:space:]]*>?=`)
			var line string
			for _, l := range lines {
				if claim.MatchString(l) {
					line = l
					break
				}
			}
			_, after, _ := strings.Cut(line, "=")
			documented, ok := xargsEcho(after)
			if !ok {
				// xargs fails on an unterminated quote, and the script with it.
				fmt.Fprintf(ctx.Stderr, "check-ground-truth: unterminated quote in the METRICS value of %s\n", key)
				exit = -1
				return
			}
			floor := regexp.MustCompilePOSIX(`^[[:space:]]*` + key + `[[:space:]]*>=`).MatchString(line)
			switch {
			case documented == "" && strict:
				echoE(gtRed + "MISSING" + gtNC + ": " + key + " = " + value + "  (not in METRICS file; metrics_strict = true)")
				exit = 1
			case documented == "":
				echoE(gtYellow + "NEW" + gtNC + ":   " + key + " = " + value + "  (not in METRICS file)")
			case floor:
				if atLeast(value, documented) {
					echoE(gtGreen + "OK" + gtNC + ":    " + key + " = " + value + "  (floor " + documented + ")")
				} else {
					echoE(gtRed + "DRIFT" + gtNC + ": " + key + " — METRICS floor is " + documented + ", code says " + value)
					exit = 1
				}
			case value == documented:
				echoE(gtGreen + "OK" + gtNC + ":    " + key + " = " + value)
			default:
				echoE(gtRed + "DRIFT" + gtNC + ": " + key + " — METRICS says " + documented + ", code says " + value)
				exit = 1
			}
		}
	}()
	if exit < 0 {
		return 1
	}
	echo("")
	if exit == 0 {
		echoE(gtGreen + "All documented metrics match codebase reality" + gtNC)
	} else {
		echoE(gtRed + "Metric drift detected — update METRICS to match reality" + gtNC)
	}
	return exit
}

var digitsRE = regexp.MustCompile(`^[0-9]+$`)

// atLeast is the script's floor test: both are digit strings and value
// -ge floor. Numbers too large for test(1) fail it.
func atLeast(value, floor string) bool {
	if !digitsRE.MatchString(value) || !digitsRE.MatchString(floor) {
		return false
	}
	v, err1 := strconv.ParseInt(value, 10, 64)
	f, err2 := strconv.ParseInt(floor, 10, 64)
	return err1 == nil && err2 == nil && v >= f
}

// echoEscapes is what bash's `echo -e line` prints: the line and a
// newline, with backslash escapes interpreted; \c ends the output there,
// newline included. (\u and \U, which only bash 4.2+ knows, are left as
// they are.)
func echoEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 == len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'c':
			return b.String()
		case 'e', 'E':
			b.WriteByte(0x1b)
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '\\':
			b.WriteByte('\\')
		case '0':
			n, j := 0, i+1
			for ; j < len(s) && j < i+4 && s[j] >= '0' && s[j] <= '7'; j++ {
				n = n*8 + int(s[j]-'0')
			}
			b.WriteByte(byte(n))
			i = j - 1
		case 'x':
			n, j := 0, i+1
			for ; j < len(s) && j < i+3 && isHex(s[j]); j++ {
				n = n*16 + hexVal(s[j])
			}
			if j == i+1 {
				b.WriteString(`\x`)
			} else {
				b.WriteByte(byte(n))
			}
			i = j - 1
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func hexVal(c byte) int {
	switch {
	case c <= '9':
		return int(c - '0')
	case c >= 'a':
		return int(c-'a') + 10
	}
	return int(c-'A') + 10
}

// metric is a declared metric's computed value.
type metric struct{ name, value string }

var metricLineRE = regexp.MustCompilePOSIX(`^metric\.([A-Za-z0-9_-]+)\.[a-z_]+[[:space:]]*=`)

// computeMetrics computes every metric declared in rcFile, in the order of
// their first lines (compute_metrics).
func computeMetrics(root, rcFile string) []metric {
	data, err := os.ReadFile(rcFile)
	if err != nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, line := range textLines(data) {
		if m := metricLineRE.FindStringSubmatch(line); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	var out []metric
	for _, name := range names {
		out = append(out, metric{name, strconv.Itoa(countMetric(root, data, name))})
	}
	return out
}

// rcField is _rebar_config "metric.<name>.<field>" with no environment
// override: the first line of rc that starts with the key (as grep reads
// it, a dot matching any character) and "=", its value after the "="
// without surrounding whitespace or a "#" comment; def when that is empty.
func rcField(rc []byte, name, field, def string) string {
	key := regexp.QuoteMeta("metric." + name + "." + field)
	re := regexp.MustCompilePOSIX("^" + strings.ReplaceAll(key, `\.`, ".") + "[[:space:]]*=")
	for _, line := range textLines(rc) {
		if !re.MatchString(line) {
			continue
		}
		_, v, _ := strings.Cut(line, "=")
		v = strings.TrimLeft(v, posixSpace)
		if i := strings.IndexByte(v, '#'); i >= 0 {
			v = v[:i]
		}
		if v = strings.TrimRight(v, posixSpace); v != "" {
			return v
		}
		return def
	}
	return def
}

// metricDef is a metric's fields as count_metric reads them.
type metricDef struct {
	dirs                     bool
	paths, names             []string
	excludeNames, excludeDir []string
	min, max                 int // -1: unset
	contains                 *regexp.Regexp
	distinct                 *regexp.Regexp
	containsSet, distinctSet bool
}

// countMetric is count_metric: the number of files (or directories) the
// script's find prints for the metric, or, with distinct, the number of
// distinct values its pattern makes of their names.
func countMetric(root string, rc []byte, name string) int {
	f := func(field, def string) string { return rcField(rc, name, field, def) }
	d := metricDef{
		dirs:         f("type", "files") == "dirs",
		paths:        ifsFields(f("paths", ".")),
		names:        ifsFields(f("names", "")),
		excludeNames: ifsFields(f("exclude_names", "")),
		excludeDir:   append([]string{".git"}, ifsFields(f("exclude_dirs", ""))...),
		min:          -1,
		max:          -1,
	}
	var starts []string
	for _, p := range d.paths {
		if _, err := os.Stat(root + "/" + p); err == nil {
			starts = append(starts, root+"/"+p)
		}
	}
	if len(starts) == 0 {
		return 0
	}
	// find rejects a depth that is not a non-negative decimal integer, and
	// prints nothing.
	for _, depth := range []struct {
		v   string
		dst *int
	}{{f("min_depth", ""), &d.min}, {f("max_depth", ""), &d.max}} {
		if depth.v == "" {
			continue
		}
		n, err := strconv.Atoi(depth.v)
		if err != nil || !digitsRE.MatchString(depth.v) {
			return 0
		}
		*depth.dst = n
	}
	if c := f("contains", ""); c != "" {
		d.containsSet = true
		d.contains, _ = regexp.CompilePOSIX(c) // nil: grep rejects it, nothing matches
	}
	if c := f("distinct", ""); c != "" {
		d.distinctSet = true
		// sed rejects a pattern it cannot compile or with no group for \1.
		if re, err := regexp.CompilePOSIX(c); err == nil && re.NumSubexp() >= 1 {
			d.distinct = re
		}
	}
	var found []string
	for _, s := range starts {
		found = d.walk(s, lastName(s), 0, found)
	}
	if !d.distinctSet {
		return len(found)
	}
	if d.distinct == nil {
		return 0
	}
	values := map[string]bool{}
	for _, p := range found {
		base := p[strings.LastIndexByte(p, '/')+1:]
		if loc := d.distinct.FindStringSubmatchIndex(base); loc != nil {
			group := ""
			if loc[2] >= 0 {
				group = base[loc[2]:loc[3]]
			}
			base = base[:loc[0]] + group + base[loc[1]:]
		}
		values[base] = true
	}
	return len(values)
}

// lastName is the name find's -name tests for a starting point: its last
// component, trailing slashes ignored.
func lastName(p string) string {
	t := strings.TrimRight(p, "/")
	if t == "" {
		return "/"
	}
	return t[strings.LastIndexByte(t, '/')+1:]
}

// walk appends what the script's find prints below p (named name, at
// depth): with -mindepth/-maxdepth, directories named .git or like an
// exclude_dirs glob pruned, entries of the metric's type whose name
// matches names and no exclude_names, and with contains only files with a
// matching line. Symlinks are not followed.
func (d *metricDef) walk(p, name string, depth int, found []string) []string {
	if d.max >= 0 && depth > d.max {
		return found
	}
	info, err := os.Lstat(p)
	if err != nil {
		return found
	}
	isDir := info.IsDir()
	if depth >= d.min {
		if isDir && matchesAny(d.excludeDir, name) {
			return found
		}
		if d.selects(info.Mode().IsRegular(), isDir, name) && d.containsMatch(p, isDir) {
			found = append(found, p)
		}
	}
	if !isDir || (d.max >= 0 && depth >= d.max) {
		return found
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return found
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		found = d.walk(p+"/"+n, n, depth+1, found)
	}
	return found
}

func (d *metricDef) selects(regular, isDir bool, name string) bool {
	if d.dirs != isDir || (!d.dirs && !regular) {
		return false
	}
	if len(d.names) > 0 && !matchesAny(d.names, name) {
		return false
	}
	return !matchesAny(d.excludeNames, name)
}

// containsMatch is `grep -lE -- <contains>` on the path: true without a
// contains pattern; false for a directory, which grep cannot read.
func (d *metricDef) containsMatch(p string, isDir bool) bool {
	if !d.containsSet {
		return true
	}
	if isDir || d.contains == nil {
		return false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	for len(data) > 0 {
		line := data
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line, data = data[:i], data[i+1:]
		} else {
			data = nil
		}
		if d.contains.Match(line) {
			return true
		}
	}
	return false
}

func matchesAny(globs []string, name string) bool {
	for _, g := range globs {
		if sourcescan.Match(g, name) {
			return true
		}
	}
	return false
}
