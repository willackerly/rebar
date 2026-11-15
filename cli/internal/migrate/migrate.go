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
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	// Progress, if set, gets a line per step of a long run.
	Progress io.Writer
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
	// Shim is set when a Replace writes a thin wrapper (ReplaceContent)
	// because other project files call the copy.
	Shim           bool
	ReplaceContent []byte
	// Hash is the copy's git blob hash when planned; apply refuses to
	// touch a copy that changed since.
	Hash string
	// Upstream is set for an unmodified copy whose output differs from the
	// install's because rebar changed (not re-verified).
	Upstream bool
	// Writes is set when running the copy changed files (it is kept, and
	// not run again after apply).
	Writes bool
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
	MetricsData      []byte // METRICS as read when planning
	MetricsEdits     []MetricsEdit
	RegenRegistry    bool
	RegenReason      string
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
	"check-todos.sh":            {"source_dirs", "source_extensions", "source_exclude_dirs"},
	"check-contract-refs.sh":    {"source_extensions", "contracts_dir", "source_exclude_dirs"},
	"check-contract-headers.sh": {"source_dirs", "source_extensions"},
	"check-freshness.sh":        {"freshness_max_days"},
}

var probeContent = map[string]string{
	"check-todos.sh":            "x = 1 // TODO" + ": rebar migrate probe\n",
	"check-contract-refs.sh":    "// CONTRACT" + ":ZZPROBE-MIGRATE.9.9\n",
	"check-contract-headers.sh": "x = 1\n",
	"check-freshness.sh":        "", // markdown with dated markers instead
}

// Entry points the template now ships as thin wrappers.
var wrapperNames = map[string]bool{"ci-check.sh": true, "pre-commit.sh": true}

// Defaults the install uses when a key is unset (scripts/*.sh).
var installDefaults = map[string]string{
	"source_dirs":         "src internal cmd client packages lib app",
	"source_extensions":   "go ts tsx js py rs jsx",
	"contracts_dir":       "architecture",
	"freshness_max_days":  "14",
	"source_exclude_dirs": "node_modules vendor dist",
}

type migrator struct {
	opts Options
	tmp  string
	ws   *workspace
	work string // the project inside the scratch workspace
	// writers maps a copy to the files its run created or changed
	writers map[string][]string
	// refs maps a copy to the project files that call it (file:line)
	refs     map[string][]reference
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
	if opts.RebarRoot == "" {
		return nil, fmt.Errorf("rebar install not found (set REBAR_ROOT to your rebar checkout)")
	}
	proj, install, err := guardProject(opts.ProjectRoot, opts.RebarRoot)
	if err != nil {
		return nil, err
	}
	opts.ProjectRoot, opts.RebarRoot = proj, install
	tmp, err := os.MkdirTemp("", "rebar-migrate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	// An interrupted run leaves no scratch copy behind.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		if _, ok := <-sigs; ok {
			os.RemoveAll(tmp)
			os.Exit(130)
		}
	}()
	defer func() { signal.Stop(sigs); close(sigs) }()

	rc, _ := os.ReadFile(filepath.Join(proj, ".rebarrc"))
	m := &migrator{opts: opts, tmp: tmp, rc: rc, trees: map[string]*probeTreeRun{}, runs: map[string]runResult{}, writers: map[string][]string{}}
	m.plan = &Plan{Project: proj, Install: install, RC: rc, Tier: tierOf(rc)}
	m.progress("copying the project to a scratch workspace")
	if m.ws, err = newWorkspace(proj, tmp); err != nil {
		return nil, err
	}
	m.work = m.ws.root

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

	m.refs = m.findReferences(names)
	for _, n := range names {
		m.progress("checking scripts/" + n)
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
		if data, err := os.ReadFile(m.legacyPath(n)); err == nil {
			sp.Hash = gitBlobHash(data)
		}
		m.plan.Scripts = append(m.plan.Scripts, sp)
	}

	m.progress("rechecking with the final settings")
	m.reverify()
	for _, sp := range m.plan.Scripts {
		if w := m.writers[sp.Name]; len(w) > 0 && sp.Verdict != Keep {
			sp.Verdict = Keep
			sp.Reason = "writes files when run (" + strings.Join(uniqueStrings(w), ", ") + "); a copy with side effects is not equivalent to the install's read-only check"
		}
		if len(m.writers[sp.Name]) > 0 {
			sp.Writes = true
		}
	}
	m.keepDependencies()
	m.shimCallers()
	m.tierGates()
	m.plan.NewRC, m.plan.Added = MergeRC(rc, m.accepted)
	m.metricsEdits()
	m.registryAfterDeletions()
	m.notes()
	return m.plan, nil
}

func (m *migrator) progress(msg string) {
	if m.opts.Progress != nil {
		fmt.Fprintf(m.opts.Progress, "rebar migrate: %s\n", msg)
	}
}

func uniqueStrings(in []string) []string {
	var out []string
	for _, s := range in {
		out = appendUnique(out, s)
	}
	return out
}

// within reports whether p is base or inside it (both already resolved).
func within(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// IsRebarSourceDir mirrors cli/cmd's check: a rebar source tree has the
// bootstrap template and the steward.
func IsRebarSourceDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "templates", "project-bootstrap"))
	return err == nil && info.IsDir() && isFile(filepath.Join(dir, "scripts", "steward.sh"))
}

// guardProject resolves the project and install paths and refuses the
// layouts where migrating would delete rebar's own scripts or files outside
// the project: the install itself, any rebar source tree or clone (shared
// root commit), a project inside the install, and a scripts/ that resolves
// (through a symlink) outside the project or into the install.
func guardProject(project, install string) (string, string, error) {
	proj, err := filepath.EvalSymlinks(project)
	if err != nil {
		return "", "", err
	}
	if proj, err = filepath.Abs(proj); err != nil {
		return "", "", err
	}
	inst, err := filepath.EvalSymlinks(install)
	if err != nil {
		return "", "", fmt.Errorf("rebar install %s: %v", install, err)
	}
	switch {
	case proj == inst:
		return "", "", fmt.Errorf("%s is the rebar install itself; nothing to migrate", proj)
	case IsRebarSourceDir(proj):
		return "", "", fmt.Errorf("%s is a rebar source tree; its scripts are rebar's own, not legacy copies", proj)
	case within(inst, proj):
		return "", "", fmt.Errorf("%s is inside the rebar install %s", proj, inst)
	}
	if roots := rootCommits(proj); len(roots) > 0 {
		for r := range rootCommits(inst) {
			if roots[r] {
				return "", "", fmt.Errorf("%s shares history with the rebar install (a rebar clone?); refusing to treat rebar's scripts as legacy copies", proj)
			}
		}
	}
	scripts := filepath.Join(proj, "scripts")
	if _, err := os.Lstat(scripts); err == nil {
		real, err := filepath.EvalSymlinks(scripts)
		if err != nil {
			return "", "", fmt.Errorf("scripts/: %v", err)
		}
		if !within(proj, real) {
			return "", "", fmt.Errorf("scripts/ resolves to %s, outside the project; refusing to delete files there", real)
		}
		if within(inst, real) {
			return "", "", fmt.Errorf("scripts/ resolves into the rebar install (%s); refusing", real)
		}
	}
	return proj, inst, nil
}

func rootCommits(dir string) map[string]bool {
	out := map[string]bool{}
	s, err := gitOut(dir, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return out
	}
	for _, l := range strings.Fields(s) {
		out[l] = true
	}
	return out
}

func same(a, b string) bool {
	ra, e1 := filepath.EvalSymlinks(a)
	rb, e2 := filepath.EvalSymlinks(b)
	if e1 != nil || e2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// tierOf is the project's tier from .rebarrc alone: an exported REBAR_TIER
// in the shell migrate runs from must not change what gets proposed.
func tierOf(rc []byte) int {
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
// It runs in the scratch workspace; a run that changes the workspace marks
// the copy as a writer (kept: a copy with side effects is not equivalent to
// a read-only check), and the workspace is restored.
func (m *migrator) runLegacy(name string, args ...string) runResult {
	r := run(m.opts.Timeout, m.work,
		scrubbedEnv("REBAR_PROJECT_ROOT="+m.work, "REBAR_TIER=3"),
		append([]string{m.workPath(name)}, args...)...)
	m.noteWrites(name)
	return r
}

// noteWrites records files a run changed in the workspace and restores it.
func (m *migrator) noteWrites(name string) {
	if changed := m.ws.changed(m.ws.snapshot()); len(changed) > 0 {
		if name != "" {
			m.writers[name] = append(m.writers[name], changed...)
		}
		_ = m.ws.restore()
	}
}

// workPath is the copy inside the scratch workspace.
func (m *migrator) workPath(name string) string {
	return filepath.Join(m.work, "scripts", name)
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
	if root == m.work && name != "compute-registry.sh" {
		// compute-registry runs concurrently (--stdout, --health) and
		// never writes in those modes; others are checked and undone.
		m.mu.Lock()
		m.noteWrites("")
		m.mu.Unlock()
	}
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

	installVals := parseMetricLines(m.runInstall(m.work, name, candidates, "--print").Stdout)

	// Equal on the project is one tree; check the same definitions on a
	// probe tree built to show where a translation differs (depth, hidden
	// files, excluded directories, content matches, distinct groups).
	defs := map[string]*MetricDef{}
	for _, p := range keys {
		if lv, ok := legacy.Val[p.key]; ok && installVals.Val[p.key] == lv {
			switch {
			case p.settings != nil:
				defs[p.key] = translations[p.key].Def
			case p.why == "already declared in .rebarrc":
				defs[p.key] = defFromRC(m.rc, p.key)
			}
		}
	}
	m.progress("checking the translated metrics on a probe tree")
	probeBad := m.verifyMetricsOnProbe(name, defs, candidates)

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
		case probeBad[p.key] != "":
			kept = append(kept, fmt.Sprintf("%s = %s — same value on the project, but %s", p.key, lv, probeBad[p.key]))
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
		after := m.runInstall(m.work, name, nil)
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
	return m.legacyMetricsIn(m.work, name)
}

// legacyMetricsIn runs the harness on the copy at root/scripts/name.
func (m *migrator) legacyMetricsIn(root, name string) metricValues {
	probe := filepath.Join(root, "scripts", ".rebar-migrate-probe")
	r := run(m.opts.Timeout, root,
		scrubbedEnv("REBAR_PROJECT_ROOT="+root, "REBAR_TIER=3", "REBAR_MIGRATE_LEGACY="+filepath.Join(root, "scripts", name)),
		"-c", legacyHarness, probe)
	if root == m.work {
		m.noteWrites(name)
	}
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
				h = m.runInstall(m.work, name, s, "--health")
			}()
		}
		got := m.runInstall(m.work, name, s, "--stdout")
		wg.Wait()
		r.stdout = got.Err == nil && got.Code == 0 && registryBody(got.Stdout) == registryBody(want.Stdout)
		r.health = hasHealth && h.Code == wantHealth.Code && h.Stdout == wantHealth.Stdout
		return r
	}

	cands := append(registryCandidates(string(src), m.rc), Setting{Key: "registry_purpose_style", Value: "bytes"})
	if _, set := RCValue(m.rc, "registry_purpose_style"); set {
		cands = cands[:len(cands)-1]
	}
	chosen, res, ok := searchSettings(cands, try)
	if !ok {
		got := m.runInstall(m.work, name, cands, "--stdout")
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
	// Implementing-file counts are off by default since 3.1 (they churn and
	// mis-merge under parallel PRs). A copy that printed them would need
	// registry_impl_counts = true to reproduce its bytes; follow the new
	// default instead and regenerate, unless the project opted in already
	// (then the key is set and never proposed).
	for i, s := range chosen {
		if s.Key == "registry_impl_counts" && s.Value == "true" {
			chosen = append(chosen[:i:i], chosen[i+1:]...)
			m.plan.RegenRegistry = true
			sp.Details = append(sp.Details, "the copy's registry has implementing-file counts; since 3.1 they are off by default, so CONTRACT-REGISTRY.md is regenerated without them (a one-time registry diff). To keep them, set registry_impl_counts = true in .rebarrc and run migrate again")
			break
		}
	}
	if hasHealth && !res.health {
		h := m.runInstall(m.work, name, chosen, "--health")
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
		// A value with # can't be written (it would start a comment).
		if _, set := RCValue(rc, k); !set && v != "" && ValidValue(v) {
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
	// Counts are off by default since 3.1; a copy that printed them needs them on.
	add("registry_impl_counts", "true")
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
	installSrc, _ := os.ReadFile(m.installPath(name))
	_, scans := probeContent[name]
	if scans {
		if _, unver := exclusionBait(string(src), string(installSrc)); len(unver) > 0 {
			sp.Reason = "has filters migrate can't exercise on a probe tree (" + strings.Join(unver, "; ") + "); review by hand"
			return sp
		}
	}

	probe := func(s []Setting) (bool, string) {
		ok, how := compareRuns(before, m.runInstall(m.work, name, s, args...))
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
	pristine := releasedBlobs(m.opts.RebarRoot, name)[gitBlobHash(src)]
	release := ""
	if !ok && !pristine && scans {
		release = m.settingsOnly(name, string(src), before, cands)
	}
	if !ok && (pristine || release != "") {
		// A released version (byte for byte, or once its settings are
		// applied): there is no customization to keep, and the difference
		// is what upgrading rebar changes. The post-apply rerun still rolls
		// everything back if it turns a pass into a failure.
		sp.Verdict, sp.Upstream = Delete, true
		if pristine {
			sp.Reason = "unmodified rebar copy; the install's version differs only by upstream changes (" + how + ")"
		} else {
			for i := range cands {
				cands[i].Source = "scripts/" + name
			}
			m.accepted = append(m.accepted, cands...)
			sp.Reason = "customized only in settings: behaves like rebar's released version " + release[:12]
			if len(cands) > 0 {
				sp.Reason += " with " + describeSettings(cands, "")
			}
			sp.Reason += "; the install's version differs only by upstream changes"
		}
		sp.Details = append(sp.Details, m.upstreamImpact(name, string(src), before, args)...)
		return sp
	}
	if !ok {
		sp.Reason = "behaves differently from the install's: " + how
		if scans {
			if hint := m.substringHint(name, string(src), cands); hint != "" {
				sp.Details = append(sp.Details, hint)
			}
		}
		return sp
	}
	if !scans && !pristine {
		// Equal output on one tree says little about what a customization
		// does elsewhere, and this check has no probe tree.
		sp.Reason = "customized (matches no released version) and migrate can compare this check only on the project, which can't show what the customization changes; review by hand"
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
	if scans {
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
		v := rawSetting(name, k, src)
		if v != "" && !sameSetting(k, v, installDefaults[k]) && ValidValue(v) {
			out = append(out, Setting{Key: k, Value: v})
		}
	}
	return out
}

// sameSetting compares setting values; list-valued exclusions as sets.
func sameSetting(k, a, b string) bool {
	if k != "source_exclude_dirs" {
		return a == b
	}
	fa, fb := strings.Fields(a), strings.Fields(b)
	if len(fa) != len(fb) {
		return false
	}
	set := map[string]bool{}
	for _, x := range fa {
		set[x] = true
	}
	for _, x := range fb {
		if !set[x] {
			return false
		}
	}
	return true
}

// rawSetting extracts the value a legacy copy hard-codes for a setting the
// install reads from .rebarrc ("" when it has none).
func rawSetting(name, k, src string) string {
	switch k {
	case "source_dirs":
		if mm := loopDirsRe.FindStringSubmatch(src); mm != nil {
			return strings.Join(strings.Fields(mm[1]), " ")
		}
		if mm := dirsArrayRe.FindStringSubmatch(src); mm != nil {
			return strings.Join(strings.Fields(mm[1]), " ")
		}
		for _, mm := range defaultVarRe.FindAllStringSubmatch(src, -1) {
			if mm[1] == "SOURCE_DIRS" {
				return mm[2]
			}
		}
	case "source_extensions":
		var exts []string
		for _, mm := range includeRe.FindAllStringSubmatch(src, -1) {
			exts = appendUnique(exts, mm[1])
		}
		if len(exts) > 0 {
			return strings.Join(exts, " ")
		}
		for _, mm := range defaultVarRe.FindAllStringSubmatch(src, -1) {
			if mm[1] == "SOURCE_EXTENSIONS" {
				return mm[2]
			}
		}
	case "contracts_dir":
		if mm := archArgRe.FindStringSubmatch(src); mm != nil {
			return strings.TrimSuffix(mm[1], "/")
		}
	case "freshness_max_days":
		if mm := maxAgeRe.FindStringSubmatch(src); mm != nil {
			return mm[1]
		}
	case "source_exclude_dirs":
		return strings.Join(excludedDirNames(name, src), " ")
	}
	return ""
}

// excludedDirNames reads the directories a copy leaves out: --exclude-dir
// values, and the plain-word alternatives of its `grep -v` line filters
// (`grep -v "node_modules\|vendor\|dist"`), which old copies used to mean
// directory names. File names (TODO.md), the script's own name and the
// TRACKED-TASK marker are not directories; .git and .claude are always
// excluded by the install.
func excludedDirNames(name, src string) []string {
	self := strings.TrimSuffix(name, ".sh")
	var out []string
	add := func(d string) {
		d = strings.Trim(d, "/")
		if d == "" || d == ".git" || d == ".claude" || d == self || strings.Contains(d, "TRACKED-TASK") {
			return
		}
		out = appendUnique(out, d)
	}
	for _, mm := range excludeArgs.FindAllStringSubmatch(src, -1) {
		if strings.HasPrefix(mm[0], "--exclude-dir") {
			add(mm[1])
		}
	}
	for _, mm := range grepVRe.FindAllStringSubmatch(src, -1) {
		for _, alt := range strings.Split(grepVPattern(mm), `\|`) {
			alt = strings.ReplaceAll(alt, `\.`, ".")
			if dirWordRe.MatchString(alt) {
				add(alt)
			}
		}
	}
	return out
}

var dirWordRe = regexp.MustCompile(`^\.?[A-Za-z0-9_-]+/?$`)

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
	data, err := os.ReadFile(m.legacyPath(name))
	if err != nil {
		return &probeTreeRun{err: err}
	}
	return m.buildProbeTreeWith(name, content, cands, data)
}

// buildProbeTreeWith lays out the same tree as buildProbeTree and runs
// script (another version of the check) on it as the copy would run.
func (m *migrator) buildProbeTreeWith(name, content string, cands []Setting, script []byte) *probeTreeRun {
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
	if name == "check-freshness.sh" {
		if err := buildFreshnessProbe(root); err != nil {
			t.err = err
			return t
		}
	} else {
		src, _ := os.ReadFile(m.legacyPath(name))
		installSrc, _ := os.ReadFile(m.installPath(name))
		extra, _ := exclusionBait(string(src), string(installSrc))
		// Every top-level directory gets probe files (scope); the bait
		// subdirectories and names (exclusions) go only where a setting or
		// the defaults say sources live, which is enough to see a filter
		// and keeps the tree small for copies that loop per finding.
		scanned := map[string]bool{".": true}
		for _, d := range strings.Fields(installDefaults["source_dirs"]) {
			scanned[d] = true
		}
		for _, list := range [][]Setting{cands, m.accepted} {
			for _, c := range list {
				if c.Key == "source_dirs" {
					for _, d := range strings.Fields(c.Value) {
						scanned[d] = true
					}
				}
			}
		}
		for _, d := range strings.Fields(rcValue(m.rc, "source_dirs")) {
			scanned[d] = true
		}
		for _, d := range append([]string{"."}, dirs...) {
			if d == "scripts" || strings.Contains(d, "..") || strings.HasPrefix(d, "/") {
				continue // scripts/ holds the copies under test
			}
			subs := []string{"", "deep"}
			if scanned[d] {
				subs = append(subs, baitDirs...)
			}
			var files []string
			for _, sub := range subs {
				subExts := exts
				if sub != "" {
					// Below the top level, the configured types are enough
					// to see whether the directory itself is scanned.
					subExts = configured
				}
				for _, e := range subExts {
					files = append(files, filepath.Join(d, sub, "probe."+e))
				}
			}
			// Names custom copies tend to filter out, and the copy's own
			// filters that the install lacks.
			bait := []string{}
			if scanned[d] {
				bait = append(append(bait, scannerBaitFiles...), extra...)
			}
			for _, f := range bait {
				if filepath.Ext(f) != "" && !strings.HasSuffix(f, "/probe") {
					files = append(files, filepath.Join(d, f))
					continue
				}
				for _, e := range configured {
					files = append(files, filepath.Join(d, f+"."+e))
				}
			}
			for _, f := range files {
				if err := writeProbe(root, f, content); err != nil {
					// Two bait forms of one filter can collide (a file and
					// a directory of the same name); one of them is enough.
					if info, serr := os.Stat(filepath.Join(root, f)); serr == nil && info.IsDir() {
						continue
					}
					if errors.Is(err, syscall.ENOTDIR) {
						continue
					}
					t.err = err
					return t
				}
			}
		}
		for _, f := range []string{"TODO.md", "CHANGELOG.md", "docs/notes.md"} {
			_ = writeProbe(root, f, content)
		}
	}
	// The copy runs from the probe tree's scripts/, with the project's
	// _rebar-config.sh and .rebarrc, as it would at home.
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.err = err
		return t
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", name), script, 0o755); err != nil {
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
	env := scrubbedEnv("REBAR_PROJECT_ROOT=" + m.work)
	a := run(m.opts.Timeout, m.work, env, "-c", q, "x", m.workPath(name))
	m.noteWrites(name)
	b := run(m.opts.Timeout, m.work, env, "-c", q, "x", m.installPath(name))
	m.noteWrites("")
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
		if sp.Verdict != Delete || readOnlyChecks[sp.Name] == nil || sp.Before == nil || sp.Upstream {
			continue
		}
		ok, how := compareRuns(*sp.Before, m.runInstall(m.work, sp.Name, nil, readOnlyChecks[sp.Name]...))
		if content, scans := probeContent[sp.Name]; ok && scans {
			if fok, fhow := m.probeTree(sp.Name, content, nil, nil); !fok {
				ok, how = false, "on a probe tree: "+fhow
			}
		}
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
			code := strings.Join(codeLines(string(data), ".sh"), "\n")
			for _, dep := range m.plan.Scripts {
				if dep.Verdict == Delete && dep.Name != kept.Name && strings.Contains(code, dep.Name) {
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
	m.plan.MetricsFile, m.plan.MetricsData = mf, data

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
	now := parseMetricLines(m.runInstall(m.work, gt, nil, "--print").Stdout)
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
	for _, n := range strings.Fields(existing) {
		enabled[strings.TrimSuffix(n, ".sh")] = true
	}
	var need []string
	for _, sp := range p.Scripts {
		name := strings.TrimSuffix(sp.Name, ".sh")
		if sp.Verdict == Keep || (sp.Verdict == Replace && !sp.Shim) || sp.Check == nil || sp.MinTier <= p.Tier || sp.LegacyGated {
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

// notes adds the findings that need a person: callers kept working by a
// wrapper, CI that now needs the rebar CLI, and the integrity manifest.
func (m *migrator) notes() {
	p := m.plan
	touched := map[string]bool{}
	for _, sp := range p.Scripts {
		if sp.Verdict == Keep {
			continue
		}
		touched[sp.Name] = true
		if sp.Shim {
			var where []string
			for _, r := range m.refs[sp.Name] {
				where = append(where, fmt.Sprintf("%s:%d", r.File, r.Line))
			}
			p.Notes = append(p.Notes, fmt.Sprintf("callers: scripts/%s becomes a thin wrapper (`rebar run %s`) instead of being deleted, because %s call it; point them at `rebar run %s` and delete the wrapper when convenient",
				sp.Name, strings.TrimSuffix(sp.Name, ".sh"), strings.Join(where, ", "), strings.TrimSuffix(sp.Name, ".sh")))
		}
		if sp.Verdict == Replace && strings.HasSuffix(sp.Name, ".sh") {
			for _, r := range m.refs[sp.Name] {
				if r.CI {
					p.Notes = append(p.Notes, fmt.Sprintf("CI: %s:%d runs scripts/%s, which now calls the `rebar` CLI; CI must install rebar (setup-rebar.sh, or a checkout with REBAR_ROOT and its bin/ on PATH)", r.File, r.Line, sp.Name))
				}
			}
		}
	}
	for _, n := range m.ciViaCallers() {
		p.Notes = append(p.Notes, n)
	}
	for _, sp := range p.Scripts {
		if sp.Verdict != Delete {
			continue
		}
		for _, r := range m.refs[sp.Name] {
			p.Notes = append(p.Notes, fmt.Sprintf("still refers to a deleted copy: %s:%d (%s)", r.File, r.Line, sp.Name))
		}
	}
	if len(touched) > 0 {
		if data, err := os.ReadFile(filepath.Join(p.Project, ".rebar", "integrity.json")); err == nil {
			for n := range touched {
				if strings.Contains(string(data), "scripts/"+n) {
					p.Notes = append(p.Notes, "integrity: .rebar/integrity.json tracks these copies; after --apply, stage the changes (`git add -A`) and run `rebar commit`, which rehashes the changed files and prunes the entries of deleted ones")
					break
				}
			}
		}
	}
	if env := SteeringEnv(); len(env) > 0 {
		p.Notes = append(p.Notes, "environment: "+strings.Join(env, ", ")+" set in this shell; migrate ignored them (decisions use .rebarrc only), but `rebar check` will honor them")
	}
}

// reference is a project file line that calls a legacy copy.
type reference struct {
	File    string
	Line    int
	Sourced bool // `source X` / `. X`: the caller wants the copy's functions
	CI      bool // a CI configuration file
}

var sourceRe = regexp.MustCompile(`(^|[;&|({]\s*|\s)(source|\.)\s+\S*`)

func isCIFile(rel string) bool {
	return strings.HasPrefix(rel, ".github/workflows/") || strings.HasPrefix(rel, ".circleci/") ||
		strings.HasPrefix(rel, ".buildkite/") || rel == ".gitlab-ci.yml" || rel == "Jenkinsfile" ||
		rel == "azure-pipelines.yml" || rel == "bitbucket-pipelines.yml" || rel == ".travis.yml" ||
		rel == ".pre-commit-config.yaml"
}

// findReferences maps each legacy copy to the project files (other than
// the copies themselves) that mention it by path: shell, YAML, JSON, TOML,
// Makefiles and extensionless scripts.
func (m *migrator) findReferences(names []string) map[string][]reference {
	out := map[string][]reference{}
	legacy := map[string]bool{}
	for _, n := range names {
		legacy["scripts/"+n] = true
	}
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
		if legacy[rel] {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 512*1024 {
			return nil
		}
		switch filepath.Ext(path) {
		case ".sh", ".bash", ".yml", ".yaml", ".json", ".toml", ".mk", "":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		for i, line := range codeLines(string(data), filepath.Ext(path)) {
			for _, n := range names {
				if !(strings.Contains(line, "scripts/"+n) || (strings.HasPrefix(rel, "scripts/") && strings.Contains(line, "/"+n))) {
					continue
				}
				sourced := false
				if mm := sourceRe.FindAllStringIndex(line, -1); mm != nil {
					for _, ix := range mm {
						if strings.Contains(line[ix[0]:ix[1]], n) {
							sourced = true
						}
					}
				}
				out[n] = append(out[n], reference{File: rel, Line: i + 1, Sourced: sourced, CI: isCIFile(rel)})
			}
		}
		return nil
	})
	return out
}

// shimCallers turns a verified copy that other project files call into a
// thin wrapper, so those callers keep working: CI scripts, pre-commit
// configs, tests. A copy that is sourced for its functions can only be
// replaced when the install provides them (check-ground-truth's
// compute_metrics, via --print); otherwise it stays.
func (m *migrator) shimCallers() {
	for _, sp := range m.plan.Scripts {
		if sp.Verdict != Delete || len(m.refs[sp.Name]) == 0 {
			continue
		}
		sourced := ""
		for _, r := range m.refs[sp.Name] {
			if r.Sourced {
				sourced = fmt.Sprintf("%s:%d", r.File, r.Line)
			}
		}
		if sp.Name == "_rebar-config.sh" || (sourced != "" && sp.Name != "check-ground-truth.sh") {
			if sourced == "" {
				sourced = fmt.Sprintf("%s:%d", m.refs[sp.Name][0].File, m.refs[sp.Name][0].Line)
			}
			sp.Verdict = Keep
			sp.Reason = "used by " + sourced + " for its functions, which the install does not provide (was: " + sp.Reason + ")"
			continue
		}
		sp.Verdict = Replace
		sp.Shim = true
		sp.ReplaceContent = shimScript(strings.TrimSuffix(sp.Name, ".sh"))
		sp.Reason += "; becomes a thin wrapper because other files call it"
	}
}

// shimScript is the wrapper left for a migrated copy that has callers.
func shimScript(name string) []byte {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString(thinWrapperMarker + "\n")
	fmt.Fprintf(&b, "# scripts/%s.sh — left by `rebar migrate` so existing callers keep working.\n", name)
	fmt.Fprintf(&b, "# The check runs from the rebar install with this project's .rebarrc;\n")
	fmt.Fprintf(&b, "# prefer `rebar run %s`, and delete this file once nothing calls it.\n", name)
	b.WriteString("_rebar_shim_root=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")/..\" && pwd)\"\n")
	if name == "check-ground-truth" {
		b.WriteString("# Sourced for compute_metrics(): the install computes the declared metrics.\n")
		b.WriteString("compute_metrics() { REBAR_PROJECT_ROOT=\"$_rebar_shim_root\" rebar run check-ground-truth --print; }\n")
		b.WriteString("if [ \"${BASH_SOURCE[0]}\" != \"$0\" ]; then return 0; fi\n")
	}
	// The copy checked the project it sits in, wherever it was called from;
	// REBAR_PROJECT_ROOT tells the CLI the same (`rebar run` then runs the
	// install's script there).
	fmt.Fprintf(&b, "REBAR_PROJECT_ROOT=\"$_rebar_shim_root\" exec rebar run %s \"$@\"\n", name)
	return []byte(b.String())
}

// ciViaCallers finds CI files that run a project script which calls a
// wrapper (analyze: .github/workflows/rebar.yml → scripts/rebar-ci.sh →
// scripts/check-todos.sh): CI then needs the rebar CLI too.
func (m *migrator) ciViaCallers() []string {
	callers := map[string][]string{} // caller file → wrapper names
	for _, sp := range m.plan.Scripts {
		if !sp.Shim {
			continue
		}
		for _, r := range m.refs[sp.Name] {
			if !r.CI {
				callers[r.File] = appendUnique(callers[r.File], sp.Name)
			}
		}
	}
	if len(callers) == 0 {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(m.opts.ProjectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(m.opts.ProjectRoot, path)
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isCIFile(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			for caller, names := range callers {
				if strings.Contains(line, caller) {
					out = append(out, fmt.Sprintf("CI: %s:%d runs %s, which calls the wrapper(s) %s; CI must install rebar (setup-rebar.sh, or a checkout with REBAR_ROOT and its bin/ on PATH)",
						rel, i+1, caller, "scripts/"+strings.Join(names, ", scripts/")))
				}
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// codeLines returns src's lines with what can't run a script blanked:
// comments (shell, YAML, TOML, Makefile `#`), and echo / printf lines,
// which only mention a name. Line numbers are kept. JSON is returned as is.
func codeLines(src, ext string) []string {
	lines := strings.Split(src, "\n")
	if ext == ".json" {
		return lines
	}
	for i, l := range lines {
		l = stripComment(l)
		if f := strings.Fields(l); len(f) > 0 && (f[0] == "echo" || f[0] == "printf" || f[0] == "warn" || f[0] == "info") {
			l = ""
		}
		lines[i] = l
	}
	return lines
}

// stripComment drops a `#` comment that starts a line or follows a blank,
// outside quotes.
func stripComment(l string) string {
	var q byte
	for i := 0; i < len(l); i++ {
		c := l[i]
		switch {
		case q != 0:
			if c == '\\' && q == '"' {
				i++
			} else if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
		case c == '\\':
			i++
		case c == '#' && (i == 0 || l[i-1] == ' ' || l[i-1] == '\t'):
			return l[:i]
		}
	}
	return l
}

// registryAfterDeletions checks whether removing the copies changes the
// registry: a copy can carry a CONTRACT: header (old steward.sh names the
// steward contract) that the registry counts or lists as a shadow
// contract. The deletions are made in the scratch workspace and the
// install's registry check rerun; if it now reports STALE where it passed,
// apply regenerates the registry and says why, instead of reporting a
// regression it caused.
func (m *migrator) registryAfterDeletions() {
	if m.plan.RegenRegistry || len(m.plan.Deletions()) == 0 {
		return
	}
	reg := func(arg string) runResult {
		return run(m.opts.Timeout, m.work,
			scrubbedEnv("REBAR_PROJECT_ROOT="+m.work, "REBAR_TIER=3", "REBAR_RCFILE="+m.rcFile(nil)),
			m.installPath("compute-registry.sh"), arg)
	}
	before := reg("--check")
	if before.Err != nil || before.Code != 0 {
		return // not current now: not a change migrate has to explain
	}
	for _, sp := range m.plan.Deletions() {
		p := m.workPath(sp.Name)
		switch {
		case sp.Verdict == Delete:
			_ = os.Remove(p)
		case sp.ReplaceContent != nil:
			_ = os.WriteFile(p, sp.ReplaceContent, 0o755)
		default:
			_ = copyFile(sp.ReplaceWith, p)
		}
	}
	after := reg("--check")
	diff := ""
	if after.Err == nil && after.Code != 0 {
		got := reg("--stdout")
		cur, _ := os.ReadFile(filepath.Join(m.work, firstNonEmpty(rcValue(m.plan.NewRC, "contracts_dir"), "architecture"), "CONTRACT-REGISTRY.md"))
		diff = firstDiffAs(string(cur), got.Stdout, "committed", "regenerated")
		m.plan.RegenRegistry = true
		m.plan.RegenReason = "removing the copies changes it (a copy carried a CONTRACT: header the registry counts or lists): " + diff
	}
	_ = m.ws.restore()
}

// rcFile writes the candidate .rebarrc (the project's plus accepted and
// extra settings) and returns its path.
func (m *migrator) rcFile(extra []Setting) string {
	rc, _ := MergeRC(m.rc, append(append([]Setting{}, m.accepted...), extra...))
	p := filepath.Join(m.tmp, "rebarrc-"+gitBlobHash(rc))
	_ = os.WriteFile(p, rc, 0o644)
	return p
}

// settingsOnly reports whether a scanner copy is a released version with
// only its settings changed: some released version of the script, with the
// copy's directories, extensions and contracts dir written into it, gives
// the same findings as the copy on the project and on the probe tree. It
// returns that version's blob id ("" when none does).
func (m *migrator) settingsOnly(name, src string, before runResult, cands []Setting) string {
	vals := map[string]string{}
	for _, k := range []string{"source_dirs", "source_extensions", "contracts_dir", "freshness_max_days"} {
		if v := rawSetting(name, k, src); v != "" {
			vals[k] = v
		}
	}
	content := probeContent[name]
	legacyTree, ok := m.trees[name]
	if !ok {
		legacyTree = m.buildProbeTree(name, content, cands)
		m.trees[name] = legacyTree
	}
	if legacyTree.err != nil {
		return ""
	}
	for _, h := range sortedKeys(releasedBlobs(m.opts.RebarRoot, name)) {
		rel := releasedSource(m.opts.RebarRoot, h)
		if rel == nil {
			continue
		}
		// The version runs from outside the tree it checks (working
		// directory and REBAR_PROJECT_ROOT set to it), so both runs scan
		// the same files, the copy's own text included.
		dir, err := os.MkdirTemp(m.tmp, "release-")
		if err != nil {
			return ""
		}
		ref := filepath.Join(dir, "scripts", name)
		if err := os.MkdirAll(filepath.Dir(ref), 0o755); err != nil {
			return ""
		}
		if err := os.WriteFile(ref, []byte(applySettings(string(rel), vals)), 0o755); err != nil {
			return ""
		}
		args := readOnlyChecks[name]
		r := run(m.opts.Timeout, m.work, scrubbedEnv("REBAR_PROJECT_ROOT="+m.work, "REBAR_TIER=3"), append([]string{ref}, args...)...)
		m.noteWrites("")
		if same, _ := compareRuns(before, r); !same {
			continue
		}
		pr := run(m.opts.Timeout, legacyTree.root, scrubbedEnv("REBAR_PROJECT_ROOT="+legacyTree.root, "REBAR_TIER=3"), append([]string{ref}, args...)...)
		if same, _ := compareRuns(legacyTree.legacy, pr); same {
			return h
		}
	}
	return ""
}

// applySettings writes setting values into an old version of a check: its
// directory loop or SOURCE_DIRS default, its --include list or
// SOURCE_EXTENSIONS default, its ARCH_DIR default, its max-age default.
func applySettings(src string, vals map[string]string) string {
	if v, ok := vals["source_dirs"]; ok {
		if loc := loopDirsRe.FindStringSubmatchIndex(src); loc != nil {
			src = src[:loc[2]] + v + src[loc[3]:]
		}
		src = regexp.MustCompile(`(?m)^(\s*SOURCE_DIRS=)"[^"$]*"`).ReplaceAllString(src, `${1}"`+v+`"`)
	}
	if v, ok := vals["source_extensions"]; ok {
		var inc []string
		for _, e := range strings.Fields(v) {
			inc = append(inc, `--include="*.`+strings.TrimPrefix(e, ".")+`"`)
		}
		src = includeRunRe.ReplaceAllStringFunc(src, func(run string) string {
			// Keep the line continuation the run ended with, if any.
			tail := " "
			if m := continuationRe.FindString(run); m != "" {
				tail = " " + m
			}
			return strings.Join(inc, " ") + tail
		})
		src = regexp.MustCompile(`(?m)^(\s*SOURCE_EXTENSIONS=)"[^"$]*"`).ReplaceAllString(src, `${1}"`+v+`"`)
	}
	if v, ok := vals["contracts_dir"]; ok {
		src = regexp.MustCompile(`(ARCH_DIR="?\$\{1:-)[A-Za-z0-9_./-]+(\})`).ReplaceAllString(src, "${1}"+v+"${2}")
	}
	if v, ok := vals["freshness_max_days"]; ok {
		src = regexp.MustCompile(`(?i)((?:max[_a-z]*|days)="?\$\{1:-)[0-9]+(\})`).ReplaceAllString(src, "${1}"+v+"${2}")
	}
	return src
}

// includeRunRe matches a run of --include="*.ext" arguments, across line
// continuations.
var continuationRe = regexp.MustCompile(`\\\n[ \t]*$`)

var includeRunRe = regexp.MustCompile(`(?:--include=["']?\*\.[A-Za-z0-9]+["']?[ \t]*(?:\\\n[ \t]*)?)+`)

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// upstreamImpact lists what the install (with the accepted settings)
// reports on the project that the copy did not, so the user sees the
// effect of deleting the copy before --apply.
func (m *migrator) upstreamImpact(name, src string, before runResult, args []string) []string {
	after := m.runInstall(m.work, name, nil, args...)
	extra := extraFindings(before.Stdout, after.Stdout)
	why := "upstream change"
	if lits := filterLiterals(src); len(lits) > 0 {
		why = "upstream change (#19): directories are excluded by name, not by substring, and TODO.md / the script itself by file name; add directory names to source_exclude_dirs to exclude more"
	}
	if len(extra) == 0 {
		same := "same findings"
		if facts(before.Stdout) != facts(after.Stdout) {
			same += "; " + firstDiffAs(before.Stdout, after.Stdout, "copy", "install")
		}
		return []string{"impact on this project: none (" + same + "); on a probe tree the install also reports what the copy's filters dropped — " + why}
	}
	shown := extra
	if len(shown) > 10 {
		shown = append(append([]string{}, shown[:10]...), fmt.Sprintf("… %d more", len(extra)-10))
	}
	return []string{fmt.Sprintf("impact on this project: the install reports %d finding(s) the copy did not (%s): %s",
		len(extra), why, strings.Join(shown, ", "))}
}

// extraFindings lists file references in b that a lacks.
func extraFindings(a, b string) []string {
	have := map[string]bool{}
	for _, p := range pathRe.FindAllString(ansiRe.ReplaceAllString(a, ""), -1) {
		have[p] = true
	}
	var out []string
	for _, p := range pathRe.FindAllString(ansiRe.ReplaceAllString(b, ""), -1) {
		if !have[p] {
			out = appendUnique(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// filterLiterals are the plain-text alternatives of a copy's grep -v
// line filters.
func filterLiterals(src string) []string {
	var out []string
	for _, mm := range grepVRe.FindAllStringSubmatch(src, -1) {
		for _, alt := range strings.Split(grepVPattern(mm), `\|`) {
			alt = strings.ReplaceAll(strings.ReplaceAll(alt, `\.`, "."), "\\", "")
			if alt != "" && !strings.Contains(alt, "TRACKED-TASK") {
				out = appendUnique(out, alt)
			}
		}
	}
	return out
}

// substringHint explains a kept scanner copy whose findings on the probe
// tree are a strict subset of the install's, with every extra one dropped
// by one of the copy's substring filters: the difference is the #19 fix,
// and the rest is for a person to judge.
func (m *migrator) substringHint(name, src string, cands []Setting) string {
	t, ok := m.trees[name]
	if !ok || t.err != nil {
		return ""
	}
	inst := m.runInstall(t.root, name, cands)
	if missing := extraFindings(inst.Stdout, t.legacy.Stdout); len(missing) > 0 {
		return "" // the copy finds things the install does not: a real difference
	}
	extra := extraFindings(t.legacy.Stdout, inst.Stdout)
	lits := filterLiterals(src)
	if len(extra) == 0 || len(lits) == 0 {
		return ""
	}
	var lines []string
	for _, l := range strings.Split(ansiRe.ReplaceAllString(inst.Stdout, ""), "\n") {
		for _, p := range extra {
			if strings.Contains(l, p) {
				lines = append(lines, l)
				break
			}
		}
	}
	for _, l := range lines {
		hit := false
		for _, lit := range lits {
			if strings.Contains(l, lit) {
				hit = true
				break
			}
		}
		if !hit {
			return ""
		}
	}
	ex := extra[0]
	return fmt.Sprintf("on a probe tree the install finds everything the copy does plus %d finding(s) the copy's substring filters (%s) drop, e.g. %s — rebar now excludes directories by name (#19). If that is the copy's only customization beyond settings, set source_dirs / source_extensions / source_exclude_dirs and delete it",
		len(extra), strings.Join(lits, ", "), ex)
}

// registryBody drops the registry's header comments that aren't content —
// the timestamp older versions wrote and the regenerate hint (which named
// ./scripts/compute-registry.sh before 3.1) — as compute-registry.sh --check
// does, so a copy that differs only there still reproduces the registry.
func registryBody(s string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		if strings.HasPrefix(line, "<!-- Generated:") || strings.HasPrefix(line, "<!-- Regenerate:") {
			continue
		}
		b.WriteString(line)
	}
	return b.String()
}
