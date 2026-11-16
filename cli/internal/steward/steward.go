// Package steward is the native (Go) implementation of scripts/steward.sh,
// the project health scanner governed by CONTRACT:S1-STEWARD.2.0 in
// architecture/: per-contract lifecycle state derived from CONTRACT:
// references and test files, the enforcement checks' results, role-routed
// action items, written to <contracts_dir>/.state/<id>.<version>.json,
// <contracts_dir>/.state/steward-report.json and STEWARD_REPORT.md.
//
// It is a faithful port: the same arguments, the same files byte for byte
// (as jq writes them), the same output and exit codes. It differs from the
// script where the script depends on the platform or locale:
//   - files, lines and contracts sort in byte order (the script's ls, glob
//     and sort -u follow LC_COLLATE; they agree in the C locale);
//   - a binary file holding a reference contributes nothing, as with GNU
//     grep 3.5+ (BSD grep prints "Binary file … matches", which the script
//     then took for a file name);
//   - a directory named like a contract file is not a contract (ls would
//     list its contents, which the script scanned as contract files);
//   - it needs no jq (the contract's "missing jq → exit 2" cannot happen).
package steward

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/contracts"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
	"github.com/willackerly/rebar/cli/internal/suite"
)

func init() {
	checks.Register(&checks.Check{Name: "steward", Run: Run})
}

// implIncludes are the script's grep --include globs for implementing files.
var implIncludes = []string{
	"*.go", "*.ts", "*.js", "*.py", "*.rs", "*.java", "*.rb", "*.c",
	"*.cpp", "*.h", "*.cs", "*.swift", "*.kt", "*.sh", "*.yaml", "*.yml",
	"*.toml", "*.json", "*.vue", "*.svelte", "*.jsx", "*.tsx",
}

// enforcementChecks are the script's run_enforcement checks: the report key,
// the script and its arguments, and the label of the markdown table.
var enforcementChecks = []struct {
	key, script string
	args        []string
	label       string
}{
	{"contract_headers", "check-contract-headers.sh", nil, "Contract Headers"},
	{"contract_refs", "check-contract-refs.sh", nil, "Contract References"},
	{"todo_tracking", "check-todos.sh", nil, "TODO Tracking"},
	{"doc_freshness", "check-freshness.sh", nil, "Doc Freshness"},
	{"registry", "compute-registry.sh", []string{"--check"}, "Registry Consistency"},
	{"ground_truth", "check-ground-truth.sh", nil, "Ground Truth"},
	{"compliance", "check-compliance.sh", nil, "Rebar Compliance"},
}

// scan is one steward run.
type scan struct {
	ctx *checks.Context
	// root is PROJECT_ROOT as the script spells it (REBAR_PROJECT_ROOT as
	// given), stateDir and archDir are built from it by concatenation.
	root, archDir, stateDir string
	refPrefix               string
	archRE                  *regexp.Regexp
	generatedAt             string
	refLines                []refLine // loaded on first use
	loaded                  bool
}

// refLine is a source line holding the reference marker, as grep -rn
// prints it (path:line:text).
type refLine struct {
	path, content, full string
}

// contract is the scan of one contract file (scan_contract).
type contract struct {
	id, version, lifecycle string
	sections               [5]bool // interfaces, behavioral, errors, tests, implementing
	completeness           string
	impl, tests            []string
	discoveries            []discovery
}

type discovery struct{ typ, desc string }

// Run is scripts/steward.sh with args: a full scan writing the reports
// (default), the aggregate JSON (--json), the one-line summary (--summary),
// or one contract's state (--check <id>).
func Run(ctx *checks.Context, args []string) int {
	root := ctx.Root
	dir := contracts.Dir(root)
	s := &scan{
		ctx:         ctx,
		root:        root,
		archDir:     root + "/" + dir,
		stateDir:    root + "/" + dir + "/.state",
		refPrefix:   contracts.RefPrefix(checks.ContractNamespace(ctx)),
		generatedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	s.archRE = checks.BRE("^" + s.archDir)
	if err := os.MkdirAll(s.path(s.stateDir), 0o755); err != nil {
		fmt.Fprintf(ctx.Stderr, "mkdir: %s: %v\n", s.stateDir, errText(err))
		return 1
	}
	mode, checkID := "full", ""
	if len(args) > 0 && args[0] != "" {
		mode = args[0]
	}
	if len(args) > 1 {
		checkID = args[1]
	}
	if mode == "--check" {
		return s.checkOne(checkID)
	}

	var all []*contract
	for _, f := range s.contractFiles() {
		c := s.scanContract(f)
		all = append(all, c)
	}
	enf, passing, total := s.enforcement()
	actions := actionItems(all, passing, total)
	report := s.aggregate(all, enf, passing, total, actions)
	text := jqPretty(report)
	if err := os.WriteFile(s.path(s.stateDir+"/steward-report.json"), []byte(text), 0o644); err != nil {
		fmt.Fprintln(ctx.Stderr, err)
		return 1
	}

	out := ctx.Stdout
	sum := summarize(report)
	switch mode {
	case "--json":
		io.WriteString(out, text)
	case "--summary":
		fmt.Fprintf(out, "Steward: %d contracts (%dd/%da/%dt/%dip), %d discoveries, %d/%d enforcement passing\n",
			sum.total, sum.draft, sum.active, sum.testing, sum.implPresent, sum.discoveries, sum.passing, sum.enfTotal)
	default:
		if err := os.WriteFile(s.path(root+"/STEWARD_REPORT.md"), []byte(s.markdown(report, all, enf, sum)), 0o644); err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return 1
		}
		fmt.Fprintln(out, "Steward scan complete.")
		fmt.Fprintf(out, "  JSON:     %s/steward-report.json\n", s.stateDir)
		fmt.Fprintf(out, "  Markdown: %s/STEWARD_REPORT.md\n", root)
		fmt.Fprintln(out)
		fmt.Fprintf(out, "  %d contracts, %d discoveries, %d/%d enforcement passing\n", sum.total, sum.discoveries, sum.passing, sum.enfTotal)
	}
	return 0
}

// path resolves a path the script would use against its working
// directory (REBAR_PROJECT_ROOT may be relative).
func (s *scan) path(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(s.ctx.WorkDir(), p)
}

func errText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// contractGlob is "$ARCH_DIR"/CONTRACT-<prefix>*.md as ls lists it: the
// matching regular files, in byte order, as full paths.
func (s *scan) contractGlob(prefix string) []string {
	var files []string
	for _, name := range contracts.List(s.path(s.archDir)) {
		if !strings.HasPrefix(name, "CONTRACT-"+prefix) {
			continue
		}
		if info, err := os.Stat(filepath.Join(s.path(s.archDir), name)); err != nil || info.IsDir() {
			continue
		}
		files = append(files, s.archDir+"/"+name)
	}
	return files
}

// contractFiles are the full scan's contracts: `ls CONTRACT-*.md | grep -v
// TEMPLATE | grep -v REGISTRY`, which tests the whole path.
func (s *scan) contractFiles() []string {
	var files []string
	for _, f := range s.contractGlob("") {
		if !strings.Contains(f, "TEMPLATE") && !strings.Contains(f, "REGISTRY") {
			files = append(files, f)
		}
	}
	return files
}

// checkOne is --check <id>: scan the first contract file starting with
// CONTRACT-<id>, print its state file's path, then every state file
// <id>.*.json through `jq .`; exit 1 when none matches.
func (s *scan) checkOne(id string) int {
	if id == "" {
		fmt.Fprintln(s.ctx.Stderr, "Usage: steward.sh --check <contract-id>")
		return 1
	}
	files := s.contractGlob(id)
	if len(files) == 0 {
		fmt.Fprintf(s.ctx.Stderr, "Contract not found: %s\n", id)
		fmt.Fprintln(s.ctx.Stderr, "Valid IDs:")
		for _, v := range s.validIDs() {
			fmt.Fprintf(s.ctx.Stderr, "  %s\n", v)
		}
		return 1
	}
	c := s.scanContract(files[0])
	fmt.Fprintln(s.ctx.Stdout, s.stateFile(c))
	matches, _ := filepath.Glob(filepath.Join(s.path(s.stateDir), globEscape(id)+".*.json"))
	sort.Strings(matches)
	if len(matches) == 0 {
		return 1 // cat fails on the unmatched glob; pipefail and set -e exit 1
	}
	var data []byte
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		data = append(data, b...)
	}
	vals, err := parseJSONStream(data)
	for _, v := range vals {
		io.WriteString(s.ctx.Stdout, jqPretty(v))
	}
	if err != nil {
		fmt.Fprintf(s.ctx.Stderr, "jq: error: %v\n", err)
		return 2
	}
	return 0
}

// globEscape quotes the glob characters of a literal path part.
func globEscape(s string) string {
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(`*?[\`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// versionSuffix is sed's `\.[0-9][0-9.]*\.md$`.
var versionSuffix = regexp.MustCompile(`\.[0-9][0-9.]*\.md$`)

// skipIDs is the listing's `grep -v -E 'TEMPLATE|REGISTRY|GAPS|\.impl$'`.
var skipIDs = regexp.MustCompile(`TEMPLATE|REGISTRY|GAPS|\.impl$`)

// validIDs is the "Valid IDs:" listing: contract names without CONTRACT-
// and their version suffix, sorted, unique.
func (s *scan) validIDs() []string {
	seen := map[string]bool{}
	var ids []string
	for _, f := range s.contractGlob("") {
		id := versionSuffix.ReplaceAllString(filepath.Base(f)[len("CONTRACT-"):], "")
		if skipIDs.MatchString(id) || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

var (
	versionMajorMinor = regexp.MustCompile(`^(.+)\.([0-9]+)\.([0-9]+)$`)
	versionMajor      = regexp.MustCompile(`^(.+)\.([0-9]+)$`)
)

// parseContractFile is parse_contract_file: the ID and version
// (MAJOR.MINOR, MAJOR → MAJOR.0, none → 1.0) of CONTRACT-<stem>.md.
func parseContractFile(path string) (id, version string) {
	stem := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "CONTRACT-"), ".md")
	if m := versionMajorMinor.FindStringSubmatch(stem); m != nil {
		return m[1], m[2] + "." + m[3]
	}
	if m := versionMajor.FindStringSubmatch(stem); m != nil {
		return m[1], m[2] + ".0"
	}
	return stem, "1.0"
}

// sectionPatterns are the spec gate's `grep -qi` patterns.
var sectionPatterns = [5]string{"## interfaces", "## behavioral", "## error", "## test", "## implementing"}

func (s *scan) stateFile(c *contract) string {
	return s.stateDir + "/" + c.id + "." + c.version + ".json"
}

// scanContract is scan_contract: it writes the contract's state file.
func (s *scan) scanContract(file string) *contract {
	id, version := parseContractFile(file)
	c := &contract{id: id, version: version}
	// grep -qi: a binary file matches too (exit 0, no output).
	body, _ := os.ReadFile(s.path(file))
	lower := asciiLower(string(body))
	complete := true
	for i, p := range sectionPatterns {
		c.sections[i] = strings.Contains(lower, p)
		complete = complete && c.sections[i]
	}
	c.completeness = "fail"
	if complete {
		c.completeness = "pass"
	}

	c.impl = s.implementingFiles(id)
	for _, f := range c.impl {
		base := filepath.Base(f)
		if strings.Contains(base, "_test.") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
			c.tests = append(c.tests, f)
		}
	}
	c.lifecycle = "draft"
	if complete {
		switch {
		case len(c.impl) == 0:
			c.lifecycle = "active"
		case len(c.tests) == 0:
			c.lifecycle = "testing"
		default:
			c.lifecycle = "impl-present"
		}
	}
	c.discoveries = s.discoveries(id)
	_ = os.WriteFile(s.path(s.stateFile(c)), []byte(jqPretty(c.json())), 0o644)
	return c
}

// loadRefLines walks the project once for the script's per-contract
// `grep -rEn "<ref><id>" "$PROJECT_ROOT" --include=…`, keeping the lines
// that hold the reference marker (every pattern starts with it). It skips
// .git and node_modules directories, whose lines the script drops anyway.
func (s *scan) loadRefLines() {
	s.loaded = true
	files := sourcescan.Filter{Include: implIncludes, ExcludeDirs: []string{".git", "node_modules"}}.Files(s.ctx.WorkDir(), []string{s.root})
	for _, f := range files {
		data, err := os.ReadFile(f.Path)
		if err != nil || !bytes.Contains(data, []byte(contracts.Marker)) {
			continue
		}
		data = sourcescan.TextPart(data)
		n := 0
		for len(data) > 0 {
			n++
			var line []byte
			if i := bytes.IndexByte(data, '\n'); i >= 0 {
				line, data = data[:i], data[i+1:]
			} else {
				line, data = data, nil
			}
			if bytes.Contains(line, []byte(contracts.Marker)) {
				s.refLines = append(s.refLines, refLine{path: f.Display, content: string(line), full: f.Display + ":" + strconv.Itoa(n) + ":" + string(line)})
			}
		}
	}
}

// implementingFiles are the files referencing contract id, as the script
// finds them: matching lines not under the contracts directory (a BRE
// prefix of the whole line) and without /.git/ or /node_modules/, sorted
// and made unique, then the path before the first colon of each, first
// occurrence kept.
func (s *scan) implementingFiles(id string) []string {
	if !s.loaded {
		s.loadRefLines()
	}
	re, err := regexp.Compile(s.refPrefix + id)
	if err != nil {
		return nil // grep -E rejects the pattern: no lines
	}
	var lines []string
	for _, l := range s.refLines {
		if !re.MatchString(l.content) {
			continue
		}
		if (s.archRE != nil && s.archRE.MatchString(l.full)) || strings.Contains(l.full, "/.git/") || strings.Contains(l.full, "/node_modules/") {
			continue
		}
		lines = append(lines, l.full)
	}
	sort.Strings(lines)
	var files []string
	seen := map[string]bool{}
	for i, l := range lines {
		if i > 0 && l == lines[i-1] {
			continue
		}
		p, _, _ := strings.Cut(l, ":")
		if !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}
	return files
}

// discoveryTag is `grep -iE '\*\*(BUG|DISCOVERY|DRIFT|DISPUTE)\*\*'`,
// applied to ASCII-lowercased text (grep -i in the C locale).
var discoveryTag = regexp.MustCompile(`\*\*(bug|discovery|drift|dispute)\*\*`)

// leadingNumber is sed's `^[0-9]*:`.
var leadingNumber = regexp.MustCompile(`^[0-9]*:`)

// discoveries are the TODO.md lines mentioning id (a BRE, `grep -n`) and
// tagged **BUG**, **DISCOVERY**, **DRIFT** or **DISPUTE**: the first tag,
// lowercased, and the line without its leading whitespace.
func (s *scan) discoveries(id string) []discovery {
	data, err := os.ReadFile(s.path(s.root + "/TODO.md"))
	if err != nil {
		return nil
	}
	if info, err := os.Stat(s.path(s.root + "/TODO.md")); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	re := checks.BRE(id)
	if re == nil {
		return nil
	}
	var found []discovery
	for i, text := range textLines(sourcescan.TextPart(data)) {
		if !re.MatchString(text) {
			continue
		}
		line := strconv.Itoa(i+1) + ":" + text
		m := discoveryTag.FindString(asciiLower(line))
		if m == "" {
			continue
		}
		desc := strings.TrimLeft(leadingNumber.ReplaceAllString(line, ""), " \t\n\v\f\r")
		found = append(found, discovery{typ: strings.Trim(m, "*"), desc: jqString(desc)})
	}
	return found
}

// textLines splits data into lines as grep reads them.
func textLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// asciiLower lowercases ASCII letters only, as grep -i does in the C locale.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func strs(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = jqString(s)
	}
	return out
}

// json is the contract's state file object.
func (c *contract) json() object {
	discs := make([]any, len(c.discoveries))
	for i, d := range c.discoveries {
		discs[i] = object{{"type", d.typ}, {"description", d.desc}}
	}
	return object{
		{"contract_id", jqString(c.id)},
		{"version", c.version},
		{"lifecycle", c.lifecycle},
		{"spec_gate", object{
			{"has_interfaces", c.sections[0]},
			{"has_behavioral", c.sections[1]},
			{"has_errors", c.sections[2]},
			{"has_tests", c.sections[3]},
			{"has_implementing", c.sections[4]},
			{"completeness", c.completeness},
		}},
		{"impl_gate", object{
			{"implementing_files", strs(c.impl)},
			{"test_files", strs(c.tests)},
			{"implementing_count", len(c.impl)},
			{"test_count", len(c.tests)},
		}},
		{"discoveries", discs},
	}
}

// enforcement is run_enforcement: each check's result (pass, fail, or
// skip when there is nothing to run), with its output discarded.
func (s *scan) enforcement() (object, int, int) {
	var results object
	passing := 0
	for _, e := range enforcementChecks {
		result := "skip"
		if code, found := suite.RunQuiet(s.ctx, e.script, e.args); found {
			result = "fail"
			if code == 0 {
				result = "pass"
				passing++
			}
		}
		results = append(results, field{e.key, result})
	}
	return results, passing, len(enforcementChecks)
}

// actions are the role-routed action items.
type actions struct{ architect, englead, product, dev []string }

var sectionNames = [5]string{"Interfaces", "Behavioral", "Errors", "Tests", "Implementing Files"}

// actionItems is generate_action_items.
func actionItems(all []*contract, passing, total int) actions {
	var a actions
	for _, c := range all {
		cid := jqString(c.id)
		if c.lifecycle == "draft" {
			var missing []string
			for i, ok := range c.sections {
				if !ok {
					missing = append(missing, sectionNames[i])
				}
			}
			// "${missing[*]}" joins with the first character of IFS=', '.
			a.architect = append(a.architect, cid+" is DRAFT — missing sections: "+strings.Join(missing, ","))
		}
		if n := c.countType("dispute"); n > 0 {
			a.architect = append(a.architect, fmt.Sprintf("%s has %d DISPUTE discovery(ies) — needs resolution", cid, n))
		}
		if c.lifecycle == "testing" {
			a.englead = append(a.englead, cid+" is TESTING — needs test files for verification")
		}
		if n := c.countType("discovery"); n > 0 {
			a.product = append(a.product, fmt.Sprintf("%s has %d DISCOVERY item(s) — needs triage", cid, n))
		}
		if c.lifecycle == "active" {
			a.dev = append(a.dev, cid+" is ACTIVE — needs implementing files")
		}
		if c.lifecycle == "testing" {
			a.dev = append(a.dev, cid+" is TESTING — needs test files")
		}
	}
	if passing < total {
		a.englead = append(a.englead, fmt.Sprintf("%d enforcement check(s) failing", total-passing))
	}
	return a
}

func (c *contract) countType(t string) int {
	n := 0
	for _, d := range c.discoveries {
		if d.typ == t {
			n++
		}
	}
	return n
}

func anyStrings(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// aggregate is build_aggregate: the steward-report.json object.
func (s *scan) aggregate(all []*contract, enf object, passing, total int, a actions) object {
	counts := map[string]int{}
	discs := 0
	list := make([]any, len(all))
	for i, c := range all {
		counts[c.lifecycle]++
		discs += len(c.discoveries)
		list[i] = c.json()
	}
	return object{
		{"generated_at", s.generatedAt},
		{"summary", object{
			{"contracts", object{
				{"total", len(all)},
				{"draft", counts["draft"]},
				{"active", counts["active"]},
				{"testing", counts["testing"]},
				{"impl_present", counts["impl-present"]},
			}},
			{"open_discoveries", discs},
			{"enforcement", object{{"passing", passing}, {"total", total}}},
		}},
		{"contracts", list},
		{"action_items", object{
			{"architect", anyStrings(a.architect)},
			{"englead", anyStrings(a.englead)},
			{"product", anyStrings(a.product)},
			{"dev", anyStrings(a.dev)},
		}},
		{"enforcement", enf},
	}
}

// summary holds the report's counts.
type summary struct {
	total, draft, active, testing, implPresent, discoveries, passing, enfTotal int
}

func summarize(report object) summary {
	sum := report.get("summary").(object)
	c := sum.get("contracts").(object)
	e := sum.get("enforcement").(object)
	return summary{
		total: c.get("total").(int), draft: c.get("draft").(int), active: c.get("active").(int),
		testing: c.get("testing").(int), implPresent: c.get("impl_present").(int),
		discoveries: sum.get("open_discoveries").(int),
		passing:     e.get("passing").(int), enfTotal: e.get("total").(int),
	}
}

// markdown is generate_markdown: STEWARD_REPORT.md.
func (s *scan) markdown(report object, all []*contract, enf object, sum summary) string {
	var b strings.Builder
	w := func(lines ...string) {
		for _, l := range lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	w("# Steward Report", "",
		"<!-- Generated by scripts/steward.sh — do not edit manually -->",
		"<!-- freshness: "+s.generatedAt+" -->", "",
		"## Summary", "",
		"| Metric | Value |",
		"|--------|-------|",
		fmt.Sprintf("| Contracts | %d total (%d draft, %d active, %d testing, %d impl-present) |", sum.total, sum.draft, sum.active, sum.testing, sum.implPresent),
		fmt.Sprintf("| Open Discoveries | %d |", sum.discoveries),
		fmt.Sprintf("| Enforcement | %d/%d passing |", sum.passing, sum.enfTotal),
		"",
		"## Contract Status", "",
		"| Contract | Version | Lifecycle | Spec Gate | Impl Files | Test Files | Discoveries |",
		"|----------|---------|-----------|-----------|------------|------------|-------------|")
	if len(all) == 0 {
		w("| _(none)_ | | | | | | |")
	}
	for _, c := range all {
		w(fmt.Sprintf("| %s | %s | %s | %s | %d | %d | %d |", jqString(c.id), c.version, c.lifecycle, c.completeness, len(c.impl), len(c.tests), len(c.discoveries)))
	}
	w("", "## Action Items", "")
	items := report.get("action_items").(object)
	for _, role := range []struct{ key, title string }{{"architect", "Architect"}, {"englead", "Engineering Lead"}, {"product", "Product"}, {"dev", "Developer"}} {
		w("### " + role.title)
		list := items.get(role.key).([]any)
		if len(list) == 0 {
			w("- _(no items)_")
		}
		for _, item := range list {
			w("- " + item.(string))
		}
		w("")
	}
	w("## Open Discoveries", "",
		"_(See TODO.md Discoveries section for details)_", "",
		"| Type | Contract | Description |",
		"|------|----------|-------------|")
	if sum.discoveries == 0 {
		w("| _(none)_ | | |")
	}
	for _, c := range all {
		for _, d := range c.discoveries {
			w("| " + d.typ + " | " + jqString(c.id) + " | " + d.desc + " |")
		}
	}
	w("", "## Enforcement Results", "",
		"| Check | Result |",
		"|-------|--------|")
	for _, e := range enforcementChecks {
		result, _ := enf.get(e.key).(string)
		if result == "" {
			result = "_(not run)_"
		}
		w("| " + e.label + " | " + result + " |")
	}
	w("", "---", "", "_Generated at "+s.generatedAt+" by `scripts/steward.sh`_")
	return b.String()
}
