package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Probe trees: synthetic directory trees that exercise what a check or a
// metric looks at, so that "same result on the project" is backed by "same
// result where the difference would show".

// writeProbe writes body to root/rel, creating directories.
func writeProbe(root, rel, body string) error {
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(body), 0o644)
}

// globVariants turns a name glob into concrete names that match it: * as
// a plain word and as version-like strings (so a distinct pattern sees
// several groups), ? as one letter, a bracket as its first member.
func globVariants(g string) []string {
	if !strings.ContainsAny(g, "*?[") {
		return []string{g}
	}
	var out []string
	for _, star := range []string{"p", "a.1.0", "a.2.0"} {
		var b strings.Builder
		for i := 0; i < len(g); i++ {
			switch g[i] {
			case '*':
				b.WriteString(star)
			case '?':
				b.WriteByte('x')
			case '[':
				j := strings.IndexByte(g[i+1:], ']')
				if j < 0 {
					b.WriteByte('[')
					continue
				}
				set := strings.TrimPrefix(g[i+1:i+1+j], "!")
				set = strings.TrimPrefix(set, "^")
				if set != "" {
					b.WriteByte(set[0])
				}
				i += j + 1
			case '\\':
				if i+1 < len(g) {
					i++
					b.WriteByte(g[i])
				}
			default:
				b.WriteByte(g[i])
			}
		}
		out = appendUnique(out, b.String())
	}
	return out
}

// Directories a metric or a scanner commonly leaves out (or not). Not
// .git: the install always skips it, and a nested .git directory inside a
// source tree is too rare to let it veto every find without -path '*/.git/*'.
var baitDirs = []string{"node_modules", "vendor", "dist", "build", "generated", "testdata", "third_party", "mocks", "fixtures", "out", "target", ".hidden", "__tests__"}

// defFromRC reads a metric already declared in .rebarrc.
func defFromRC(rc []byte, key string) *MetricDef {
	f := func(field string) string { v, _ := RCValue(rc, "metric."+key+"."+field); return v }
	d := &MetricDef{Type: f("type"), Paths: strings.Fields(f("paths")), Names: strings.Fields(f("names")),
		ExcludeNames: strings.Fields(f("exclude_names")), ExcludeDirs: strings.Fields(f("exclude_dirs")),
		MinDepth: f("min_depth"), MaxDepth: f("max_depth"), Contains: f("contains"), Distinct: f("distinct")}
	switch f("tracked") {
	case "true", "1", "yes", "on":
		d.Tracked = true
	}
	if len(d.Paths) == 0 {
		d.Paths = []string{"."}
	}
	return d
}

// buildMetricProbe lays out a tree for metric definitions: under every
// metric path, at depths 0 to 2, files (and, for dirs metrics, directories)
// named after every name and exclude glob, a dot-prefixed variant, and the
// same inside the usual excluded directories. Files alternate between a
// line matching the metric's `contains` pattern (taken from the project)
// and one that does not.
func (m *migrator) buildMetricProbe(root string, defs map[string]*MetricDef) error {
	paths := []string{"."}
	bait := append([]string{}, baitDirs...)
	var names, dirNames, samples []string
	for _, d := range defs {
		for _, p := range d.Paths {
			paths = appendUnique(paths, p)
		}
		for _, n := range append(append([]string{}, d.Names...), d.ExcludeNames...) {
			for _, v := range globVariants(n) {
				names = appendUnique(names, v)
				if d.Type == "dirs" {
					dirNames = appendUnique(dirNames, v)
				}
			}
		}
		for _, x := range d.ExcludeDirs {
			bait = appendUnique(bait, x)
		}
		if d.Contains != "" {
			if s := m.sampleLine(d.Contains, d.Paths); s != "" {
				samples = appendUnique(samples, s)
			}
		}
	}
	if len(names) == 0 {
		names = []string{"probe.txt"}
	}
	content := func(i int) string {
		if len(samples) > 0 && i%2 == 0 {
			return samples[(i/2)%len(samples)] + "\n"
		}
		return "nothing to see\n"
	}
	i := 0
	for _, p := range paths {
		for _, sub := range []string{"", "d1", "d1/d2"} {
			dir := filepath.Join(p, sub)
			for _, n := range names {
				for _, f := range []string{n, "." + n} {
					if err := writeProbe(root, filepath.Join(dir, f), content(i)); err != nil {
						return err
					}
					i++
				}
			}
			for _, n := range dirNames {
				if err := os.MkdirAll(filepath.Join(root, dir, n+".d", n), 0o755); err != nil {
					return err
				}
			}
		}
		for _, b := range bait {
			for _, n := range names {
				if err := writeProbe(root, filepath.Join(p, b, n), content(i)); err != nil {
					return err
				}
				i++
			}
			for _, n := range dirNames {
				_ = os.MkdirAll(filepath.Join(root, p, b, n), 0o755)
			}
		}
	}
	return nil
}

// sampleLine is a line from the project that matches re, so probe files
// can carry real content for `contains`.
func (m *migrator) sampleLine(re string, paths []string) string {
	args := []string{"-rhEm1", "--", re}
	for _, p := range paths {
		args = append(args, filepath.Join(m.work, p))
	}
	cmd := exec.Command("grep", args...)
	cmd.Env = scrubbedEnv()
	out, _ := cmd.Output()
	line, _, _ := strings.Cut(string(out), "\n")
	if len(line) > 300 {
		return ""
	}
	return line
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// verifyMetricsOnProbe computes the metrics with the legacy copy and with
// the install (candidate settings) on a metric probe tree, and returns the
// keys whose values differ, with a reason. A key the copy computes nothing
// for there is reported too: unverifiable is not verified.
func (m *migrator) verifyMetricsOnProbe(name string, defs map[string]*MetricDef, candidates []Setting) map[string]string {
	bad := map[string]string{}
	if len(defs) == 0 {
		return bad
	}
	root, err := os.MkdirTemp(m.tmp, "metric-probe-")
	if err == nil {
		err = m.buildMetricProbe(root, defs)
	}
	if err == nil {
		err = copyFile(m.legacyPath(name), filepath.Join(root, "scripts", name))
	}
	if err != nil {
		for k := range defs {
			bad[k] = "could not build a probe tree: " + err.Error()
		}
		return bad
	}
	_ = copyFile(m.legacyPath("_rebar-config.sh"), filepath.Join(root, "scripts", "_rebar-config.sh"))
	for _, f := range []string{"METRICS", "METRICS.md"} {
		_ = copyFile(filepath.Join(m.opts.ProjectRoot, f), filepath.Join(root, f))
	}
	legacy := m.legacyMetricsIn(root, name)
	install := parseMetricLines(m.runInstall(root, name, candidates, "--print").Stdout)
	for k := range defs {
		lv, lok := legacy.Val[k]
		iv := install.Val[k]
		switch {
		case !lok:
			bad[k] = "the copy computes nothing for it on a probe tree, so the translation can't be checked there"
		case lv != iv:
			bad[k] = fmt.Sprintf("differs on a probe tree (copy %s, install %s), so the translation is not equivalent", lv, iv)
		}
	}
	return bad
}

// Scanner probe content beyond the plain probe.EXT files: names and
// directories custom copies commonly filter out, so a lost exclusion shows.
var scannerBaitFiles = []string{"probe_test.go", "probe.pb.go", "probe.min.js", "probe.generated.ts", "probe_test.py", "test_probe.py", "probe.spec.ts", "probe.test.ts", "probe.d.ts", "probe.gen.go", "zz_generated.go"}

var (
	grepVRe     = regexp.MustCompile(`grep\s+-v\s+(?:-[A-Za-z]+\s+)*(?:"([^"]*)"|'([^']*)')`)
	excludeArgs = regexp.MustCompile(`--exclude(?:-dir)?=["']?([^"'\s]+)["']?`)
	literalRe   = regexp.MustCompile(`^[A-Za-z0-9_./:-]+$`)
)

// exclusionBait returns probe paths (relative to a source directory) that
// exercise the filters a copy has and the install lacks: each `grep -v`
// pattern and --exclude/--exclude-dir value not present in the install's
// script. ok is false when a filter can't be turned into a path (then the
// copy can't be verified and stays).
func exclusionBait(legacy, install string) (paths []string, unverifiable []string) {
	have := map[string]bool{}
	for _, mm := range grepVRe.FindAllStringSubmatch(install, -1) {
		have[grepVPattern(mm)] = true
	}
	for _, mm := range excludeArgs.FindAllStringSubmatch(install, -1) {
		have["--x="+mm[1]] = true
	}
	for _, mm := range grepVRe.FindAllStringSubmatch(legacy, -1) {
		pat := grepVPattern(mm)
		if have[pat] {
			continue
		}
		for _, alt := range strings.Split(pat, `\|`) {
			lit := strings.NewReplacer(`\.`, ".", `\/`, "/", `\-`, "-").Replace(strings.TrimSuffix(strings.TrimPrefix(alt, "^"), "$"))
			lit = strings.Trim(lit, ":")
			if lit == "" || !literalRe.MatchString(lit) {
				unverifiable = append(unverifiable, "grep -v "+pat)
				continue
			}
			paths = append(paths, baitPath(lit)...)
		}
	}
	for _, mm := range excludeArgs.FindAllStringSubmatch(legacy, -1) {
		if have["--x="+mm[1]] {
			continue
		}
		lit := mm[1]
		if !literalRe.MatchString(strings.NewReplacer("*", "").Replace(lit)) {
			unverifiable = append(unverifiable, "--exclude "+lit)
			continue
		}
		lit = strings.ReplaceAll(lit, "*", "probe")
		if strings.HasPrefix(mm[0], "--exclude-dir") {
			paths = append(paths, filepath.Join(lit, "probe"))
		} else {
			paths = append(paths, lit)
		}
	}
	return paths, unverifiable
}

// grepVPattern is the pattern grep receives: double-quoted text loses the
// shell's backslash escapes, single-quoted text is literal.
func grepVPattern(mm []string) string {
	if mm[2] != "" || mm[1] == "" {
		return mm[2]
	}
	return strings.NewReplacer(`\\`, `\`, `\"`, `"`, `\$`, `$`, "\\`", "`").Replace(mm[1])
}

// baitPath makes paths that contain a literal filter: as a directory, as a
// file name suffix, and as a whole file name.
func baitPath(lit string) []string {
	if strings.Contains(strings.Trim(lit, "/"), "/") || strings.HasSuffix(lit, "/") {
		return []string{filepath.Join(strings.Trim(lit, "/"), "probe")}
	}
	out := []string{filepath.Join(lit, "probe"), "probe" + lit, lit}
	return out
}

// buildFreshnessProbe writes markdown files with freshness and last-synced
// markers at a spread of ages, in plain, nested, template and vendored
// locations.
func buildFreshnessProbe(root string) error {
	now := time.Now()
	for _, age := range []int{0, 1, 7, 13, 14, 15, 30, 31, 90, 91, 365} {
		date := now.AddDate(0, 0, -age).Format("2006-01-02")
		for _, dir := range []string{".", "docs/deep", "templates", "node_modules/x", ".hidden"} {
			if err := writeProbe(root, filepath.Join(dir, fmt.Sprintf("f%d.md", age)), "# Doc\n<!-- freshness: "+date+" -->\n"); err != nil {
				return err
			}
			if err := writeProbe(root, filepath.Join(dir, fmt.Sprintf("s%d.md", age)), "# Doc\nlast-synced: "+date+"\n"); err != nil {
				return err
			}
		}
	}
	_ = writeProbe(root, "archived.md", "<!-- freshness: archived -->\n<!-- freshness: 2001-01-01 -->\n")
	_ = writeProbe(root, "placeholder.md", "<!-- freshness: YYYY-MM-DD -->\n")
	_ = writeProbe(root, "notes.txt", "freshness: 2001-01-01\n")
	return nil
}
