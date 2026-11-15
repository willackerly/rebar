// Package migrate turns an adopter's legacy copies of rebar scripts into
// .rebarrc settings (`rebar migrate`).
//
// Every decision is made by running things, not by reading the copies'
// prose: a copy may be deleted only when the install's script, with the
// proposed settings, behaves the same on the project (and, for the checks
// that scan source trees, on a probe tree that exercises their scope).
// A copy nothing could verify is kept and reported.
package migrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Options configure a migration.
type Options struct {
	ProjectRoot string
	RebarRoot   string
	// RegenerateRegistry drops registry_purpose_style = bytes when only the
	// old byte cut kept the registry identical, and regenerates
	// CONTRACT-REGISTRY.md on apply (a one-time registry diff).
	RegenerateRegistry bool
	Timeout            time.Duration // per script run
}

const defaultTimeout = 3 * time.Minute

// Verdict is what happens to a legacy copy.
type Verdict int

const (
	Keep    Verdict = iota
	Delete          // verified; the install's script replaces it
	Replace         // verified; becomes the template's thin wrapper / README
)

func (v Verdict) String() string {
	return [...]string{"keep", "delete", "replace"}[v]
}

// ScriptPlan is the decision for one legacy copy.
type ScriptPlan struct {
	Name    string // file name under scripts/
	Verdict Verdict
	Reason  string
	Details []string
	// ReplaceWith is the install file a Replace copies in.
	ReplaceWith string
	// Check is how the copy is exercised as a check (nil: not a check).
	Check []string
	// MinTier is the tier from which the install's script runs (1 = always).
	MinTier int
	// LegacyGated is true when the copy has its own tier gate.
	LegacyGated bool
	// Enabled is true when checks_enable keeps the install's script running
	// below its tier.
	Enabled bool
	// Before is the copy's check result before migration.
	Before *runResult
}

// MetricsEdit is a change to a METRICS line that deleting copies requires
// (a deleted .sh file no longer counts toward `shell_scripts`).
type MetricsEdit struct {
	Key, Op  string // Op is "=" or ">="
	From, To string
}

// Plan is the full proposal.
type Plan struct {
	Project, Install string
	Tier             int
	RC, NewRC        []byte
	Added            []Setting
	Scripts          []*ScriptPlan
	MetricsFile      string
	MetricsEdits     []MetricsEdit
	RegenRegistry    bool
	Notes            []string
}

// Deletions returns the copies a plan deletes or replaces.
func (p *Plan) Deletions() []*ScriptPlan {
	var out []*ScriptPlan
	for _, s := range p.Scripts {
		if s.Verdict != Keep {
			out = append(out, s)
		}
	}
	return out
}

// Empty reports whether the plan changes nothing.
func (p *Plan) Empty() bool {
	return len(p.Added) == 0 && len(p.Deletions()) == 0 && len(p.MetricsEdits) == 0 && !p.RegenRegistry
}

// Checks that only read the project and can run on any repo, with the
// arguments that exercise them fully.
var readOnlyChecks = map[string][]string{
	"check-todos.sh":             {},
	"check-contract-refs.sh":     {},
	"check-contract-headers.sh":  {},
	"check-freshness.sh":         {},
	"check-jtbd-presence.sh":     {},
	"check-prefix-uniqueness.sh": {},
	"check-registry.sh":          {},
	"check-doc-refs.sh":          {},
	"check-compliance.sh":        {},
	"check-decay-patterns.sh":    {"--all"},
	"check-fix-commit.sh":        {},
	"check-bypass-flags.sh":      {},
	"refresh-context.sh":         {},
}

// Settings the install reads in place of a check's hard-coded values, and
// the probe-tree content that exercises the check's scan scope.
var settingKeys = map[string][]string{
	"check-todos.sh":            {"source_dirs", "source_extensions"},
	"check-contract-refs.sh":    {"source_extensions", "contracts_dir"},
	"check-contract-headers.sh": {"source_dirs", "source_extensions"},
	"check-freshness.sh":        {"freshness_max_days"},
}

var probeContent = map[string]string{
	"check-todos.sh":            "x = 1 // TODO" + ": rebar migrate probe\n",
	"check-contract-refs.sh":    "// CONTRACT" + ":ZZPROBE-MIGRATE.9.9\n",
	"check-contract-headers.sh": "x = 1\n",
}

// Entry points the template now ships as thin wrappers.
var wrapperNames = map[string]bool{"ci-check.sh": true, "pre-commit.sh": true}

// Defaults the install uses when a key is unset (scripts/*.sh).
var installDefaults = map[string]string{
	"source_dirs":        "src internal cmd client packages lib app",
	"source_extensions":  "go ts tsx js py rs jsx",
	"contracts_dir":      "architecture",
	"freshness_max_days": "14",
}

type migrator struct {
	opts     Options
	tmp      string
	rc       []byte
	accepted []Setting
	plan     *Plan
	// probe trees by check name, with the legacy copy's result on each
	trees map[string]*probeTreeRun
	// install runs by root, script, arguments and candidate .rebarrc
	runs map[string]runResult
	mu   sync.Mutex
}

type probeTreeRun struct {
	root   string
	legacy runResult
	err    error
}

// BuildPlan inspects the project and decides what to migrate. It runs the
// legacy copies and the install's scripts but changes nothing in the project.
func BuildPlan(opts Options) (*Plan, error) {
	if opts.Timeout == 0 {
		opts.Timeout = defaultTimeout
	}
	proj, err := filepath.Abs(opts.ProjectRoot)
	if err != nil {
		return nil, err
	}
	opts.ProjectRoot = proj
	if opts.RebarRoot == "" {
		return nil, fmt.Errorf("rebar install not found (set REBAR_ROOT to your rebar checkout)")
	}
	if same(proj, opts.RebarRoot) {
		return nil, fmt.Errorf("%s is the rebar install itself; nothing to migrate", proj)
	}
	tmp, err := os.MkdirTemp("", "rebar-migrate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	rc, _ := os.ReadFile(filepath.Join(proj, ".rebarrc"))
	m := &migrator{opts: opts, tmp: tmp, rc: rc, trees: map[string]*probeTreeRun{}, runs: map[string]runResult{}}
	m.plan = &Plan{Project: proj, Install: opts.RebarRoot, RC: rc, Tier: tierOf(rc)}

	names := m.legacyCopies()
	order := func(n string) int {
		switch {
		case n == "check-ground-truth.sh":
			return 0
		case n == "compute-registry.sh":
			return 1
		case settingKeys[n] != nil:
			return 2
		case readOnlyChecks[n] != nil:
			return 3
		case n == "_rebar-config.sh":
			return 6
		case wrapperNames[n] || n == "cold-start-checks.sh":
			return 5
		default:
			return 4
		}
	}
	sort.SliceStable(names, func(i, j int) bool { return order(names[i]) < order(names[j]) })

	for _, n := range names {
		var sp *ScriptPlan
		switch {
		case n == "check-ground-truth.sh":
			sp = m.groundTruth()
		case n == "compute-registry.sh":
			sp = m.registry()
		case readOnlyChecks[n] != nil:
			sp = m.check(n)
		case n == "_rebar-config.sh":
			sp = m.configLib()
		default:
			sp = m.byProvenance(n)
		}
		sp.MinTier, sp.LegacyGated = m.gates(n)
		m.plan.Scripts = append(m.plan.Scripts, sp)
	}

	m.reverify()
	m.keepDependencies()
	m.tierGates()
	m.plan.NewRC, m.plan.Added = MergeRC(rc, m.accepted)
	m.metricsEdits()
	m.notes()
	return m.plan, nil
}

func same(a, b string) bool {
	ra, e1 := filepath.EvalSymlinks(a)
	rb, e2 := filepath.EvalSymlinks(b)
	if e1 != nil || e2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

func tierOf(rc []byte) int {
	if env := os.Getenv("REBAR_TIER"); env != "" {
		if t, err := strconv.Atoi(env); err == nil && t >= 1 && t <= 3 {
			return t
		}
	}
	if v, ok := RCValue(rc, "tier"); ok {
		if t, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && t >= 1 && t <= 3 {
			return t
		}
	}
	return 3
}

const thinWrapperMarker = "# rebar-thin-wrapper"

func isThinWrapper(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == thinWrapperMarker {
			return true
		}
	}
	return false
}

// legacyCopies lists the project's scripts/ files the per-script override
// would run instead of the install's (same rule as cli/cmd/scripts.go and
// _rebar_script), plus the shared _rebar-config.sh and the old scripts README.
func (m *migrator) legacyCopies() []string {
	entries, err := os.ReadDir(filepath.Join(m.opts.ProjectRoot, "scripts"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !(strings.HasSuffix(n, ".sh") || n == "README.md") {
			continue
		}
		local := filepath.Join(m.opts.ProjectRoot, "scripts", n)
		info, err := os.Lstat(local)
		if err != nil || !info.Mode().IsRegular() {
			continue // symlinks into an install are not copies
		}
		data, err := os.ReadFile(local)
		if err != nil || isThinWrapper(data) {
			continue
		}
		if n == "README.md" {
			current, _ := os.ReadFile(filepath.Join(m.opts.RebarRoot, "templates", "project-bootstrap", "scripts", n))
			if !bytes.Equal(data, current) && releasedBlobs(m.opts.RebarRoot, n)[gitBlobHash(data)] {
				out = append(out, n)
			}
			continue
		}
		if isFile(filepath.Join(m.opts.RebarRoot, "scripts", n)) {
			out = append(out, n)
		}
	}
	return out
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func (m *migrator) legacyPath(name string) string {
	return filepath.Join(m.opts.ProjectRoot, "scripts", name)
}

func (m *migrator) installPath(name string) string {
	return filepath.Join(m.opts.RebarRoot, "scripts", name)
}

var skipRe = regexp.MustCompile(`_rebar_skip\s+([123])`)

// gates returns the install script's minimum tier and whether the legacy
// copy has a tier gate of its own.
func (m *migrator) gates(name string) (int, bool) {
	minTier := 1
	if data, err := os.ReadFile(m.installPath(name)); err == nil {
		if mm := skipRe.FindStringSubmatch(string(data)); mm != nil {
			minTier, _ = strconv.Atoi(mm[1])
		}
	}
	legacy, _ := os.ReadFile(m.legacyPath(name))
	return minTier, skipRe.Match(legacy)
}

// runLegacy runs the project's copy as the per-script override would.
func (m *migrator) runLegacy(name string, args ...string) runResult {
	return run(m.opts.Timeout, m.opts.ProjectRoot,
		scrubbedEnv("REBAR_PROJECT_ROOT="+m.opts.ProjectRoot, "REBAR_TIER=3"),
		append([]string{m.legacyPath(name)}, args...)...)
}

// runInstall runs the install's script against root with candidate settings.
// Safe to call concurrently.
func (m *migrator) runInstall(root, name string, extra []Setting, args ...string) runResult {
	m.mu.Lock()
	all := append(append([]Setting{}, m.accepted...), extra...)
	m.mu.Unlock()
	rc, _ := MergeRC(m.rc, all)
	key := root + "\x00" + name + "\x00" + strings.Join(args, "\x00") + "\x00" + string(rc)
	m.mu.Lock()
	r, ok := m.runs[key]
	m.mu.Unlock()
	if ok {
		return r
	}
	rcPath := filepath.Join(m.tmp, "rebarrc-"+gitBlobHash(rc))
	if err := os.WriteFile(rcPath, rc, 0o644); err != nil {
		return runResult{Err: err, Code: -1}
	}
	r = run(m.opts.Timeout, root,
		scrubbedEnv("REBAR_PROJECT_ROOT="+root, "REBAR_TIER=3", "REBAR_RCFILE="+rcPath),
		append([]string{m.installPath(name)}, args...)...)
	m.mu.Lock()
	m.runs[key] = r
	m.mu.Unlock()
	return r
}

// ---------------------------------------------------------------- ground truth

func (m *migrator) groundTruth() *ScriptPlan {
	const name = "check-ground-truth.sh"
	sp := &ScriptPlan{Name: name, Check: []string{}}
	src, _ := os.ReadFile(m.legacyPath(name))
	before := m.runLegacy(name)
	sp.Before = &before

	legacy := m.legacyMetrics(name)
	if len(legacy.Keys) == 0 {
		if out := parseCheckOutput(before.Stdout); len(out.Keys) > 0 {
			legacy = out
		}
	}

	translations := map[string]Translation{}
	for _, tr := range NewTranslator(string(src)).Translate() {
		translations[tr.Key] = tr
	}

	var candidates []Setting
	type pending struct {
		key, why string
		settings []Setting
		notes    []string
	}
	var keys []pending
	for _, k := range legacy.Keys {
		p := pending{key: k}
		switch tr, ok := translations[k]; {
		case HasMetric(m.rc, k):
			p.why = "already declared in .rebarrc"
		case !ok:
			p.why = "untranslatable: could not find how compute_metrics() computes it"
		case tr.Err != nil:
			p.why = "untranslatable: " + tr.Err.Error()
		default:
			p.settings = tr.Def.Settings(k, "scripts/"+name)
			p.notes = tr.Def.Notes
			candidates = append(candidates, p.settings...)
		}
		keys = append(keys, p)
	}

	installVals := parseMetricLines(m.runInstall(m.opts.ProjectRoot, name, candidates, "--print").Stdout)

	var accepted []Setting
	var kept []string
	for _, p := range keys {
		lv := legacy.Val[p.key]
		iv, has := installVals.Val[p.key]
		switch {
		case p.settings == nil && p.why != "already declared in .rebarrc":
			kept = append(kept, fmt.Sprintf("%s = %s — %s", p.key, lv, p.why))
		case !has:
			kept = append(kept, fmt.Sprintf("%s = %s — the install computes nothing for it", p.key, lv))
		case iv != lv:
			kept = append(kept, fmt.Sprintf("%s = %s — the install computes %s from %s", p.key, lv, iv, describeSettings(p.settings, p.why)))
		default:
			accepted = append(accepted, p.settings...)
			line := fmt.Sprintf("%s = %s (same value from the install)", p.key, lv)
			if p.settings == nil {
				line = fmt.Sprintf("%s = %s (%s; same value)", p.key, lv, p.why)
			}
			for _, n := range p.notes {
				line += "; note: " + n
			}
			sp.Details = append(sp.Details, "migrated  "+line)
		}
	}
	m.accepted = append(m.accepted, accepted...)

	var extra []string
	for _, k := range installVals.Keys {
		if _, ok := legacy.Val[k]; !ok {
			extra = append(extra, k)
		}
	}

	switch {
	case before.Err != nil:
		sp.Reason = "the legacy copy did not run: " + before.Err.Error()
	case len(legacy.Keys) == 0 && len(translations) > 0:
		sp.Reason = "could not read the legacy copy's metric values"
	case len(kept) > 0:
		sp.Reason = fmt.Sprintf("%d of %d metrics stay with the legacy copy", len(kept), len(legacy.Keys))
		for _, k := range kept {
			sp.Details = append(sp.Details, "kept      "+k)
		}
	case len(extra) > 0:
		sp.Reason = "the install would also check " + strings.Join(extra, ", ") + " (declared in .rebarrc, not computed by the copy)"
	default:
		after := m.runInstall(m.opts.ProjectRoot, name, nil)
		if after.Code != before.Code {
			sp.Reason = fmt.Sprintf("same values, but the check exits %d with the install and %d with the copy", after.Code, before.Code)
			break
		}
		sp.Verdict = Delete
		if len(legacy.Keys) == 0 {
			sp.Reason = "computes no metrics, like the install without metric.* settings"
		} else {
			sp.Reason = fmt.Sprintf("all %d metrics have the same value from the install", len(legacy.Keys))
		}
	}
	return sp
}

func describeSettings(s []Setting, why string) string {
	if len(s) == 0 {
		return why
	}
	var parts []string
	for _, x := range s {
		parts = append(parts, strings.TrimPrefix(x.Key, "metric.")+"="+x.Value)
	}
	return strings.Join(parts, ", ")
}

// legacyMetrics reads the values compute_metrics() emits.
func (m *migrator) legacyMetrics(name string) metricValues {
	probe := filepath.Join(m.opts.ProjectRoot, "scripts", ".rebar-migrate-probe")
	r := run(m.opts.Timeout, m.opts.ProjectRoot,
		scrubbedEnv("REBAR_PROJECT_ROOT="+m.opts.ProjectRoot, "REBAR_TIER=3", "REBAR_MIGRATE_LEGACY="+m.legacyPath(name)),
		"-c", legacyHarness, probe)
	return parseMetricLines(r.Stdout)
}

// ---------------------------------------------------------------- registry

type regResult struct{ stdout, health bool }

func (m *migrator) registry() *ScriptPlan {
	const name = "compute-registry.sh"
	sp := &ScriptPlan{Name: name, Check: []string{"--check"}}
	src, _ := os.ReadFile(m.legacyPath(name))
	before := m.runLegacy(name, "--check")
	sp.Before = &before

	want := m.runLegacy(name, "--stdout")
	if want.Err != nil || want.Code != 0 || want.Stdout == "" {
		sp.Reason = "the legacy copy's --stdout did not produce a registry"
		return sp
	}
	hasHealth := strings.Contains(string(src), "--health)")
	var wantHealth runResult
	if hasHealth {
		wantHealth = m.runLegacy(name, "--health")
	}

	try := func(s []Setting) regResult {
		var r regResult
		var h runResult
		var wg sync.WaitGroup
		if hasHealth {
			wg.Add(1)
			go func() {
				defer wg.Done()
				h = m.runInstall(m.opts.ProjectRoot, name, s, "--health")
			}()
		}
		got := m.runInstall(m.opts.ProjectRoot, name, s, "--stdout")
		wg.Wait()
		r.stdout = got.Err == nil && got.Code == 0 && got.Stdout == want.Stdout
		r.health = hasHealth && h.Code == wantHealth.Code && h.Stdout == wantHealth.Stdout
		return r
	}

	cands := append(registryCandidates(string(src), m.rc), Setting{Key: "registry_purpose_style", Value: "bytes"})
	if _, set := RCValue(m.rc, "registry_purpose_style"); set {
		cands = cands[:len(cands)-1]
	}
	chosen, res, ok := searchSettings(cands, try)
	if !ok {
		got := m.runInstall(m.opts.ProjectRoot, name, cands, "--stdout")
		sp.Reason = "no combination of settings reproduces the legacy registry byte for byte"
		sp.Details = append(sp.Details, "candidates: "+describeSettings(cands, "none"), firstDiff(want.Stdout, got.Stdout))
		return sp
	}
	for i := range chosen {
		chosen[i].Source = "scripts/" + name
	}

	usedBytes := false
	for _, s := range chosen {
		if s.Key == "registry_purpose_style" && s.Value == "bytes" {
			usedBytes = true
		}
	}
	sp.Verdict = Delete
	sp.Reason = "the install's --stdout is byte-identical to the copy's"
	if len(chosen) > 0 {
		sp.Reason += " with " + describeSettings(chosen, "")
	}
	if usedBytes {
		if m.opts.RegenerateRegistry {
			var rest []Setting
			for _, s := range chosen {
				if s.Key != "registry_purpose_style" {
					rest = append(rest, s)
				}
			}
			chosen = rest
			m.plan.RegenRegistry = true
			sp.Details = append(sp.Details, "--regenerate-registry: registry_purpose_style = bytes left out; CONTRACT-REGISTRY.md is regenerated with the ellipsis purpose style (a one-time registry diff)")
		} else {
			sp.Details = append(sp.Details, "registry_purpose_style = bytes was needed: the copy cuts purposes at 80 bytes. Kept so the registry does not change; rerun with --regenerate-registry to switch to the ellipsis style instead")
		}
	}
	if hasHealth && !res.health {
		h := m.runInstall(m.opts.ProjectRoot, name, chosen, "--health")
		sp.Details = append(sp.Details, "--health (advisory, not committed) still differs: "+firstDiff(wantHealth.Stdout, h.Stdout))
	}
	if before.Code != 0 {
		sp.Details = append(sp.Details, "note: the committed registry was already stale before migration (legacy --check failed)")
	}
	m.accepted = append(m.accepted, chosen...)
	return sp
}

// searchSettings finds the smallest subset of cands for which try reports
// the same behavior (r.stdout), trying subsets in order of size, so a
// candidate that is not needed, or that breaks the output, is never kept.
// For the registry, a remaining candidate that also makes --health equal
// (r.health) without changing --stdout is then added. More than 8 candidates fall back to dropping one at a time
// from the full set.
func searchSettings(cands []Setting, try func([]Setting) regResult) ([]Setting, regResult, bool) {
	var chosen []Setting
	var res regResult
	found := false
	if len(cands) <= 8 {
		n := len(cands)
		for size := 0; size <= n && !found; size++ {
			for mask := 0; mask < 1<<n; mask++ {
				if bitCount(mask) != size {
					continue
				}
				var sub []Setting
				for i := 0; i < n; i++ {
					if mask&(1<<i) != 0 {
						sub = append(sub, cands[i])
					}
				}
				if r := try(sub); r.stdout {
					chosen, res, found = sub, r, true
					break
				}
			}
		}
	} else if r := try(cands); r.stdout {
		chosen, res, found = cands, r, true
		for i := 0; i < len(chosen); {
			trial := append(append([]Setting{}, chosen[:i]...), chosen[i+1:]...)
			if r := try(trial); r.stdout {
				chosen, res = trial, r
				continue
			}
			i++
		}
	}
	if !found {
		return nil, res, false
	}
	if !res.health {
		for _, c := range cands {
			if _, in := acceptedValue(chosen, c.Key); in {
				continue
			}
			trial := append(append([]Setting{}, chosen...), c)
			if r := try(trial); r.stdout && r.health {
				chosen, res = trial, r
				break
			}
		}
	}
	return chosen, res, true
}

func bitCount(x int) int {
	n := 0
	for ; x > 0; x &= x - 1 {
		n++
	}
	return n
}

var (
	scopePrefixRe = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*#([A-Za-z0-9_]+-)\}`)
	headingRe     = regexp.MustCompile(`\^## ([^/'"]*)`)
	headingAltRe  = regexp.MustCompile(`\(([^()]*[A-Za-z][^()]*)\)`)
	pathspecRe    = regexp.MustCompile(`'(\*\.[A-Za-z0-9]+|[A-Za-z0-9_-]+/)'`)
	includeRe     = regexp.MustCompile(`--include=["']?\*\.([A-Za-z0-9]+)["']?`)
	archDirRe     = regexp.MustCompile(`ARCH_DIR="?\$(?:\{)?(?:PROJECT_ROOT|REPO_ROOT|ROOT)(?:\})?/([A-Za-z0-9_./-]+)"?`)
	archArgRe     = regexp.MustCompile(`ARCH_DIR="?\$\{1:-([A-Za-z0-9_./-]+)\}"?`)
)

// registryCandidates derives registry settings from a legacy
// compute-registry.sh: the prefixes its classifier strips, its purpose
// heading, the pathspecs it searches. Each is only a candidate; the search
// in registry() keeps the ones the output needs.
func registryCandidates(src string, rc []byte) []Setting {
	var out []Setting
	add := func(k, v string) {
		if _, set := RCValue(rc, k); !set && v != "" {
			out = append(out, Setting{Key: k, Value: v})
		}
	}
	if m := archDirRe.FindStringSubmatch(src); m != nil && strings.TrimSuffix(m[1], "/") != "architecture" {
		add("contracts_dir", strings.TrimSuffix(m[1], "/"))
	}
	// Prefixes come from the classifier only (other functions strip
	// CONTRACT- from file names the same way).
	classify := src
	for name, body := range parseFunctions(src) {
		if strings.Contains(strings.ToLower(name), "classif") {
			classify = body
		}
	}
	var prefixes []string
	for _, m := range scopePrefixRe.FindAllStringSubmatch(classify, -1) {
		prefixes = appendUnique(prefixes, m[1])
	}
	add("registry_scope_prefixes", strings.Join(prefixes, " "))
	if m := headingRe.FindStringSubmatch(src); m != nil {
		h := ""
		alts := headingAltRe.FindAllStringSubmatch(m[1], -1)
		if len(alts) > 0 {
			h = alts[len(alts)-1][1]
		} else {
			h = strings.TrimSpace(m[1])
		}
		if h != "" && h != "Purpose" && !strings.ContainsAny(h, "[]\\") {
			add("registry_purpose_heading", h)
		}
	}
	add("registry_impl_counts", "false")
	var globs []string
	for _, m := range pathspecRe.FindAllStringSubmatch(src, -1) {
		if m[1] != "architecture/" {
			globs = appendUnique(globs, m[1])
		}
	}
	const defaultGlobs = "*.go *.ts *.tsx *.js *.py *.rs *.java *.rb *.jsx *.sh bin/"
	if g := strings.Join(globs, " "); len(globs) > 0 && g != defaultGlobs {
		add("registry_impl_globs", g)
	}
	return out
}

// ---------------------------------------------------------------- checks

// check decides a read-only check by running both versions on the project,
// and on a probe tree for the checks that scan source files.
func (m *migrator) check(name string) *ScriptPlan {
	args := readOnlyChecks[name]
	sp := &ScriptPlan{Name: name, Check: args}
	before := m.runLegacy(name, args...)
	sp.Before = &before
	if before.Err != nil {
		sp.Reason = "the legacy copy did not run: " + before.Err.Error()
		return sp
	}
	src, _ := os.ReadFile(m.legacyPath(name))
	cands := m.settingCandidates(name, string(src))

	probe := func(s []Setting) (bool, string) {
		ok, how := compareRuns(before, m.runInstall(m.opts.ProjectRoot, name, s, args...))
		if !ok {
			return false, how
		}
		if content, scans := probeContent[name]; scans {
			fok, fhow := m.probeTree(name, content, s, cands)
			if !fok {
				return false, "on a probe tree: " + fhow
			}
		}
		return true, how
	}

	ok, how := probe(nil)
	var chosen []Setting
	if !ok && len(cands) > 0 {
		_, fhow := probe(cands)
		how = fhow + " (with " + describeSettings(cands, "") + ")"
		var found bool
		chosen, _, found = searchSettings(cands, func(s []Setting) regResult {
			r, _ := probe(s)
			return regResult{stdout: r, health: true}
		})
		if found {
			ok, how = probe(chosen)
		}
	}
	if !ok {
		sp.Reason = "behaves differently from the install's: " + how
		return sp
	}
	for i := range chosen {
		chosen[i].Source = "scripts/" + name
	}
	m.accepted = append(m.accepted, chosen...)
	sp.Verdict = Delete
	sp.Reason = how
	if len(chosen) > 0 {
		sp.Reason += " with " + describeSettings(chosen, "")
	}
	if _, scans := probeContent[name]; scans {
		sp.Reason += "; same findings on a probe tree"
	}
	return sp
}

var (
	loopDirsRe   = regexp.MustCompile(`for\s+\w+\s+in\s+([^;\n]+?)\s*;\s*do\s*\n?\s*\[\s+-d\b`)
	dirsArrayRe  = regexp.MustCompile(`(?m)^\s*DIRS=\(([^)"$]+)\)`)
	defaultVarRe = regexp.MustCompile(`(?m)^\s*(SOURCE_DIRS|SOURCE_EXTENSIONS)="([^"$]+)"`)
	maxAgeRe     = regexp.MustCompile(`(?i)(?:max[_a-z]*|days)="?\$\{1:-([0-9]+)\}"?`)
)

// settingCandidates extracts the values a legacy check hard-codes for the
// settings the install reads instead.
func (m *migrator) settingCandidates(name, src string) []Setting {
	var out []Setting
	for _, k := range settingKeys[name] {
		if _, set := RCValue(m.rc, k); set {
			continue
		}
		if _, ok := acceptedValue(m.accepted, k); ok {
			continue
		}
		v := ""
		switch k {
		case "source_dirs":
			if mm := loopDirsRe.FindStringSubmatch(src); mm != nil {
				v = strings.Join(strings.Fields(mm[1]), " ")
			} else if mm := dirsArrayRe.FindStringSubmatch(src); mm != nil {
				v = strings.Join(strings.Fields(mm[1]), " ")
			} else {
				for _, mm := range defaultVarRe.FindAllStringSubmatch(src, -1) {
					if mm[1] == "SOURCE_DIRS" {
						v = mm[2]
					}
				}
			}
		case "source_extensions":
			var exts []string
			for _, mm := range includeRe.FindAllStringSubmatch(src, -1) {
				exts = appendUnique(exts, mm[1])
			}
			v = strings.Join(exts, " ")
			if v == "" {
				for _, mm := range defaultVarRe.FindAllStringSubmatch(src, -1) {
					if mm[1] == "SOURCE_EXTENSIONS" {
						v = mm[2]
					}
				}
			}
		case "contracts_dir":
			if mm := archArgRe.FindStringSubmatch(src); mm != nil {
				v = strings.TrimSuffix(mm[1], "/")
			}
		case "freshness_max_days":
			if mm := maxAgeRe.FindStringSubmatch(src); mm != nil {
				v = mm[1]
			}
		}
		if v != "" && v != installDefaults[k] {
			out = append(out, Setting{Key: k, Value: v})
		}
	}
	return out
}

func acceptedValue(s []Setting, key string) (string, bool) {
	for _, x := range s {
		if x.Key == key {
			return x.Value, true
		}
	}
	return "", false
}

// probeTree runs both versions of a scanning check on a scratch tree with a
// probe file of every candidate extension in every candidate directory
// (and in node_modules/, vendor/, dist/ and a nested directory below each),
// so a difference in scan scope shows even when the project is clean.
func (m *migrator) probeTree(name, content string, s, cands []Setting) (bool, string) {
	t, ok := m.trees[name]
	if !ok {
		t = m.buildProbeTree(name, content, cands)
		m.trees[name] = t
	}
	if t.err != nil {
		return false, t.err.Error()
	}
	return compareRuns(t.legacy, m.runInstall(t.root, name, s))
}

// buildProbeTree lays out the probe tree for a check once (its directories
// and extensions cover every candidate setting) and runs the legacy copy on it.
func (m *migrator) buildProbeTree(name, content string, cands []Setting) *probeTreeRun {
	t := &probeTreeRun{}
	root, err := os.MkdirTemp(m.tmp, "probe-")
	if err != nil {
		t.err = err
		return t
	}
	t.root = root

	dirs := strings.Fields(installDefaults["source_dirs"])
	exts := strings.Fields(installDefaults["source_extensions"])
	exts = append(exts, "sh", "java", "rb", "c", "h", "kt", "swift", "cs", "php", "mjs", "vue")
	configured := []string{"go", "ts"} // plus every extension a setting names
	if entries, err := os.ReadDir(m.opts.ProjectRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != ".git" {
				dirs = appendUnique(dirs, e.Name())
			}
		}
	}
	for _, list := range [][]Setting{cands, m.accepted} {
		for _, c := range list {
			switch c.Key {
			case "source_dirs":
				for _, d := range strings.Fields(c.Value) {
					dirs = appendUnique(dirs, d)
				}
			case "source_extensions":
				for _, e := range strings.Fields(c.Value) {
					exts = appendUnique(exts, strings.TrimPrefix(e, "."))
					configured = appendUnique(configured, strings.TrimPrefix(e, "."))
				}
			}
		}
	}
	if v, ok := RCValue(m.rc, "source_dirs"); ok {
		for _, d := range strings.Fields(v) {
			dirs = appendUnique(dirs, d)
		}
	}
	if v, ok := RCValue(m.rc, "source_extensions"); ok {
		for _, e := range strings.Fields(v) {
			exts = appendUnique(exts, strings.TrimPrefix(e, "."))
			configured = appendUnique(configured, strings.TrimPrefix(e, "."))
		}
	}
	for _, d := range append([]string{"."}, dirs...) {
		if d == "scripts" || strings.Contains(d, "..") || strings.HasPrefix(d, "/") {
			continue // scripts/ holds the copies under test
		}
		for _, sub := range []string{"", "deep", "node_modules", "vendor", "dist"} {
			subExts := exts
			if sub != "" {
				// Below the top level, two types are enough to see whether
				// the directory itself is scanned.
				subExts = configured
			}
			for _, e := range subExts {
				p := filepath.Join(root, d, sub, "probe."+e)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.err = err
					return t
				}
				if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
					t.err = err
					return t
				}
			}
		}
	}
	// The copy runs from the probe tree's scripts/, with the project's
	// _rebar-config.sh and .rebarrc, as it would at home.
	if err := copyFile(m.legacyPath(name), filepath.Join(root, "scripts", name)); err != nil {
		t.err = err
		return t
	}
	_ = copyFile(m.legacyPath("_rebar-config.sh"), filepath.Join(root, "scripts", "_rebar-config.sh"))
	if len(m.rc) > 0 {
		_ = os.WriteFile(filepath.Join(root, ".rebarrc"), m.rc, 0o644)
	}
	for _, c := range []string{installDefaults["contracts_dir"], firstNonEmpty(rcValue(m.rc, "contracts_dir"), "architecture")} {
		_ = os.MkdirAll(filepath.Join(root, c), 0o755)
	}
	t.legacy = run(m.opts.Timeout, root, scrubbedEnv("REBAR_PROJECT_ROOT="+root, "REBAR_TIER=3"),
		filepath.Join(root, "scripts", name))
	return t
}

func rcValue(rc []byte, k string) string {
	v, _ := RCValue(rc, k)
	return v
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

// ---------------------------------------------------------------- the rest

// byProvenance decides the scripts that cannot be probed safely (they write
// state, watch forever, or need arguments): a copy byte-identical to a file
// rebar has shipped is unmodified, and the install's current version is its
// successor. Anything else is customized and stays.
func (m *migrator) byProvenance(name string) *ScriptPlan {
	sp := &ScriptPlan{Name: name}
	data, err := os.ReadFile(m.legacyPath(name))
	if err != nil {
		sp.Reason = err.Error()
		return sp
	}
	blobs := releasedBlobs(m.opts.RebarRoot, name)
	if len(blobs) == 0 {
		sp.Reason = "not probed (it writes state or needs arguments), and the install has no git history to compare it with"
		return sp
	}
	if !blobs[gitBlobHash(data)] {
		sp.Reason = "customized: matches no version rebar has shipped, and it is not safe to probe (writes state, runs long or needs arguments); review it by hand"
		return sp
	}
	switch {
	case wrapperNames[name] || name == "README.md":
		sp.Verdict = Replace
		sp.ReplaceWith = filepath.Join(m.opts.RebarRoot, "templates", "project-bootstrap", "scripts", name)
		if !isFile(sp.ReplaceWith) {
			sp.Verdict, sp.Reason = Keep, "unmodified, but the install has no template "+name+" to replace it with"
			return sp
		}
		sp.Reason = "unmodified rebar copy; replaced by the template's " + name
		if wrapperNames[name] {
			sp.Reason = "unmodified rebar copy; replaced by the thin wrapper that calls `rebar check` (keeps CI and git-hook paths working)"
		}
	default:
		sp.Verdict = Delete
		sp.Reason = "unmodified rebar copy (byte-identical to a released version); the install's version replaces it"
	}
	return sp
}

// configLib decides _rebar-config.sh. Only project copies source it; the
// install's scripts source their own. It goes when it is unmodified, or
// when it reports the same tier and contracts directory as the install's.
func (m *migrator) configLib() *ScriptPlan {
	const name = "_rebar-config.sh"
	sp := &ScriptPlan{Name: name}
	data, _ := os.ReadFile(m.legacyPath(name))
	if releasedBlobs(m.opts.RebarRoot, name)[gitBlobHash(data)] {
		sp.Verdict = Delete
		sp.Reason = "unmodified rebar copy; only legacy copies source it"
		return sp
	}
	q := `source "$1" >/dev/null 2>&1; _rebar_tier 2>/dev/null; type _rebar_contracts_dir >/dev/null 2>&1 && _rebar_contracts_dir`
	env := scrubbedEnv("REBAR_PROJECT_ROOT=" + m.opts.ProjectRoot)
	a := run(m.opts.Timeout, m.opts.ProjectRoot, env, "-c", q, "x", m.legacyPath(name))
	b := run(m.opts.Timeout, m.opts.ProjectRoot, env, "-c", q, "x", m.installPath(name))
	al := strings.Fields(a.Stdout)
	bl := strings.Fields(b.Stdout)
	if len(al) == 1 && len(bl) == 2 && bl[1] == "architecture" {
		al = append(al, "architecture") // an old copy without contracts_dir
	}
	if a.Err == nil && strings.Join(al, " ") == strings.Join(bl, " ") {
		sp.Verdict = Delete
		sp.Reason = "reports the same tier and contracts directory as the install's; only legacy copies source it"
		return sp
	}
	sp.Reason = fmt.Sprintf("customized: reports tier/contracts dir %q, the install's %q", strings.Join(al, " "), strings.Join(bl, " "))
	return sp
}

// reverify reruns every probe-verified check with the final settings: a
// setting accepted for one check (source_dirs for check-todos) must not
// change another (check-contract-headers reads it too).
func (m *migrator) reverify() {
	for _, sp := range m.plan.Scripts {
		if sp.Verdict != Delete || readOnlyChecks[sp.Name] == nil || sp.Before == nil {
			continue
		}
		ok, how := compareRuns(*sp.Before, m.runInstall(m.opts.ProjectRoot, sp.Name, nil, readOnlyChecks[sp.Name]...))
		if !ok {
			sp.Verdict = Keep
			sp.Reason = "differs once all proposed settings apply (another check's setting changes it): " + how
		}
	}
}

// keepDependencies keeps a copy that a kept copy runs or sources: the
// install suite still picks the kept one, and it must not break.
func (m *migrator) keepDependencies() {
	for changed := true; changed; {
		changed = false
		for _, kept := range m.plan.Scripts {
			if kept.Verdict != Keep {
				continue
			}
			data, _ := os.ReadFile(m.legacyPath(kept.Name))
			for _, dep := range m.plan.Scripts {
				if dep.Verdict == Delete && dep.Name != kept.Name && strings.Contains(string(data), dep.Name) {
					dep.Verdict = Keep
					dep.Reason = "used by the kept scripts/" + kept.Name + " (was: " + dep.Reason + ")"
					changed = true
				}
			}
		}
	}
}

// metricsEdits works out which METRICS lines the deletions change. The
// deleted files are laid out alone in a scratch tree and counted with the
// final metric settings; a metric's new value is its current value minus
// that count.
func (m *migrator) metricsEdits() {
	mf := filepath.Join(m.opts.ProjectRoot, "METRICS")
	if !isFile(mf) {
		mf = filepath.Join(m.opts.ProjectRoot, "METRICS.md")
	}
	data, err := os.ReadFile(mf)
	if err != nil {
		return
	}
	m.plan.MetricsFile = mf

	var deleted []string
	for _, sp := range m.plan.Scripts {
		if sp.Verdict == Delete {
			deleted = append(deleted, sp.Name)
		}
	}
	if len(deleted) == 0 {
		return
	}
	gt := m.groundTruthChecker()
	if gt == "" {
		return
	}
	now := parseMetricLines(m.runInstall(m.opts.ProjectRoot, gt, nil, "--print").Stdout)
	scratch, err := os.MkdirTemp(m.tmp, "deleted-")
	if err != nil {
		return
	}
	for _, n := range deleted {
		_ = copyFile(m.legacyPath(n), filepath.Join(scratch, "scripts", n))
	}
	gone := parseMetricLines(m.runInstall(scratch, gt, nil, "--print").Stdout)
	for _, k := range now.Keys {
		g, _ := strconv.Atoi(gone.Val[k])
		if g == 0 {
			continue
		}
		cur, err := strconv.Atoi(now.Val[k])
		if err != nil {
			continue
		}
		if v, _ := RCValue(m.plan.NewRC, "metric."+k+".distinct"); v != "" {
			m.plan.Notes = append(m.plan.Notes, fmt.Sprintf("METRICS %s counts distinct names and the deleted copies match it; check its value after --apply", k))
			continue
		}
		after := cur - g
		op, doc, ok := metricsLine(string(data), k)
		if !ok {
			continue
		}
		d, err := strconv.Atoi(doc)
		if err != nil {
			continue
		}
		switch {
		case op == "=" && d == cur:
			m.plan.MetricsEdits = append(m.plan.MetricsEdits, MetricsEdit{Key: k, Op: op, From: doc, To: strconv.Itoa(after)})
		case op == ">=" && d <= cur && d > after:
			m.plan.MetricsEdits = append(m.plan.MetricsEdits, MetricsEdit{Key: k, Op: op, From: doc, To: strconv.Itoa(after)})
		}
	}
}

// groundTruthChecker is the check-ground-truth.sh that computes values
// after migration: the install's, once it is the one that runs.
func (m *migrator) groundTruthChecker() string {
	for _, sp := range m.plan.Scripts {
		if sp.Name == "check-ground-truth.sh" && sp.Verdict == Keep {
			return "" // the copy keeps computing; its own definitions apply
		}
	}
	return "check-ground-truth.sh"
}

var metricsLineRe = regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*(>=|=)\s*(\S+)`)

func metricsLine(data, key string) (op, val string, ok bool) {
	for _, line := range strings.Split(data, "\n") {
		if mm := metricsLineRe.FindStringSubmatch(line); mm != nil && mm[1] == key {
			return mm[2], mm[3], true
		}
	}
	return "", "", false
}

// applyMetricsEdits rewrites the value on each edited METRICS line.
func applyMetricsEdits(data string, edits []MetricsEdit) string {
	lines := strings.Split(data, "\n")
	for _, e := range edits {
		for i, line := range lines {
			mm := metricsLineRe.FindStringSubmatchIndex(line)
			if mm == nil || line[mm[2]:mm[3]] != e.Key {
				continue
			}
			lines[i] = line[:mm[6]] + e.To + line[mm[7]:]
			break
		}
	}
	return strings.Join(lines, "\n")
}

// tierGates keeps migrated checks running where the copies ran. A copy
// without a tier gate ran at any tier; the install's script may only run
// from tier 2 or 3. migrate never changes the tier: it proposes
// `checks_enable` for those checks (or says what to add when the project
// already sets checks_enable).
func (m *migrator) tierGates() {
	p := m.plan
	enabled := map[string]bool{}
	existing, has := RCValue(m.rc, "checks_enable")
	for _, n := range strings.Fields(existing + " " + os.Getenv("REBAR_CHECKS_ENABLE")) {
		enabled[strings.TrimSuffix(n, ".sh")] = true
	}
	var need []string
	for _, sp := range p.Scripts {
		name := strings.TrimSuffix(sp.Name, ".sh")
		if sp.Verdict != Delete || sp.Check == nil || sp.MinTier <= p.Tier || sp.LegacyGated {
			continue
		}
		if enabled[name] {
			sp.Enabled = true
			continue
		}
		need = append(need, name)
	}
	if len(need) == 0 {
		return
	}
	list := strings.Join(need, " ")
	if has {
		p.Notes = append(p.Notes, fmt.Sprintf(
			"tier: the project is tier %d; the install runs %s only from a higher tier, where the copies ran at any tier. "+
				"Add them to the existing checks_enable line to keep them running (rebar migrate does not edit existing lines or the tier).", p.Tier, list))
		return
	}
	for _, sp := range p.Scripts {
		for _, n := range need {
			if strings.TrimSuffix(sp.Name, ".sh") == n {
				sp.Enabled = true
			}
		}
	}
	m.accepted = append(m.accepted, Setting{Key: "checks_enable", Value: list, Source: "tier gates: the copies ran at any tier"})
	p.Notes = append(p.Notes, fmt.Sprintf(
		"tier: the project is tier %d and the install runs %s only from a higher tier, where the copies ran at any tier. "+
			"rebar migrate leaves the tier alone and proposes `checks_enable = %s` so they keep running; drop that line to let the tier decide.",
		p.Tier, list, list))
}

// notes adds the findings that need a person: tier gates that would turn
// a migrated check into a SKIP, other files that still call deleted copies,
// and the integrity manifest.
func (m *migrator) notes() {
	p := m.plan
	gone := map[string]bool{}
	var replaced []string
	for _, sp := range p.Scripts {
		switch sp.Verdict {
		case Delete:
			gone[sp.Name] = true
		case Replace:
			replaced = append(replaced, sp.Name)
		}
	}
	if len(gone) > 0 {
		for _, ref := range m.references(gone, replaced) {
			p.Notes = append(p.Notes, "still refers to a deleted copy (use `rebar run <name>` or `rebar check`): "+ref)
		}
		manifest := filepath.Join(p.Project, ".rebar", "integrity.json")
		if data, err := os.ReadFile(manifest); err == nil {
			for n := range gone {
				if strings.Contains(string(data), "scripts/"+n) {
					p.Notes = append(p.Notes, "integrity: .rebar/integrity.json tracks the deleted copies; record the deletions with `rebar commit` (git rm alone leaves MISSING entries)")
					break
				}
			}
		}
	}
}

// references finds project files (outside .git and the deleted copies)
// that mention a deleted copy by path.
func (m *migrator) references(gone map[string]bool, replaced []string) []string {
	skipFiles := map[string]bool{}
	for _, n := range replaced {
		skipFiles["scripts/"+n] = true
	}
	var out []string
	skipDirs := map[string]bool{".git": true, "node_modules": true, "vendor": true, "dist": true, ".rebar": true}
	_ = filepath.WalkDir(m.opts.ProjectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(m.opts.ProjectRoot, path)
		if skipFiles[rel] || (strings.HasPrefix(rel, "scripts/") && gone[strings.TrimPrefix(rel, "scripts/")]) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 512*1024 {
			return nil
		}
		ext := filepath.Ext(path)
		switch ext {
		case ".sh", ".bash", ".yml", ".yaml", ".json", ".toml", ".mk", "":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			for n := range gone {
				if n == "_rebar-config.sh" && !strings.Contains(line, "_rebar-config.sh") {
					continue
				}
				if strings.Contains(line, "scripts/"+n) || (strings.HasPrefix(rel, "scripts/") && strings.Contains(line, "/"+n)) {
					out = append(out, fmt.Sprintf("%s:%d (%s)", rel, i+1, n))
				}
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}
