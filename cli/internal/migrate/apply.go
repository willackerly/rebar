package migrate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Render prints the plan: proposed .rebarrc lines, files deleted or
// replaced, files kept with reasons, METRICS edits, and notes.
func Render(w io.Writer, p *Plan, applied bool) {
	verb := map[bool]string{false: "would", true: ""}
	fmt.Fprintf(w, "rebar migrate — %s\n", p.Project)
	if p.Install != "" {
		fmt.Fprintf(w, "compared with: rebar's built-in checks (rebar tree: %s)   tier: %d\n\n", p.Install, p.Tier)
	} else {
		fmt.Fprintf(w, "compared with: rebar's built-in checks   tier: %d\n\n", p.Tier)
	}
	if len(p.Scripts) == 0 {
		fmt.Fprintln(w, "No legacy script copies: the project already runs on rebar's built-in checks.")
	}

	if len(p.Added) > 0 {
		fmt.Fprintf(w, ".rebarrc additions (existing lines are kept as they are):\n")
		block, _ := MergeRC(nil, p.Added)
		for _, line := range strings.Split(strings.TrimRight(string(block), "\n"), "\n") {
			fmt.Fprintf(w, "  + %s\n", line)
		}
		fmt.Fprintln(w)
	} else if len(p.Scripts) > 0 {
		fmt.Fprintf(w, ".rebarrc: nothing to add\n\n")
	}

	if d := p.Deletions(); len(d) > 0 {
		fmt.Fprintf(w, "Verified, %s:\n", strings.TrimSpace(verb[applied]+" remove or replace"))
		for _, s := range d {
			label := s.Verdict.String()
			if s.Shim {
				label = "wrap"
			}
			fmt.Fprintf(w, "  %-8s scripts/%s — %s\n", label, s.Name, s.Reason)
			for _, det := range s.Details {
				fmt.Fprintf(w, "             %s\n", det)
			}
		}
		fmt.Fprintln(w)
	}
	var kept []*ScriptPlan
	for _, s := range p.Scripts {
		if s.Verdict == Keep {
			kept = append(kept, s)
		}
	}
	if len(kept) > 0 {
		fmt.Fprintf(w, "Kept (not verified equivalent; the project's copy keeps running):\n")
		for _, s := range kept {
			fmt.Fprintf(w, "  keep     scripts/%s — %s\n", s.Name, s.Reason)
			for _, det := range s.Details {
				fmt.Fprintf(w, "             %s\n", det)
			}
		}
		fmt.Fprintln(w)
	}
	if len(p.MetricsEdits) > 0 {
		fmt.Fprintf(w, "%s (the deleted copies are counted by these metrics):\n", filepath.Base(p.MetricsFile))
		for _, e := range p.MetricsEdits {
			fmt.Fprintf(w, "  %s %s %s → %s\n", e.Key, e.Op, e.From, e.To)
		}
		fmt.Fprintln(w)
	}
	if p.MCP != nil {
		fmt.Fprintf(w, ".mcp.json: %s to rebar's native MCP server (bin/ask-mcp-server only hands off to it):\n",
			map[bool]string{false: "would point Claude Code", true: "pointed Claude Code"}[applied])
		for _, e := range p.MCP.Servers {
			fmt.Fprintf(w, "  %s: %s\n  %s→ %s\n", e.Name, e.From, strings.Repeat(" ", len(e.Name)), e.To)
		}
		fmt.Fprintln(w)
	}
	if p.RegenRegistry {
		why := "with the ellipsis purpose style (--regenerate-registry)"
		if p.RegenReason != "" {
			why = "because " + p.RegenReason
		}
		fmt.Fprintf(w, "CONTRACT-REGISTRY.md: %s %s\n\n", map[bool]string{false: "would be regenerated", true: "regenerated"}[applied], why)
	}
	for _, n := range p.Notes {
		fmt.Fprintf(w, "note: %s\n", n)
	}
	if len(p.Notes) > 0 {
		fmt.Fprintln(w)
	}
}

// CheckResult is a check run after apply.
type CheckResult struct {
	Name       string
	Ran        string // "rebar" (the built-in check), "legacy copy" or "wrapper …"
	Tier       int
	BeforeCode int
	AfterCode  int
	Output     string
}

// Regressed reports a check that passed before and fails now.
func (c CheckResult) Regressed() bool { return c.BeforeCode == 0 && c.AfterCode != 0 }

// ApplyOptions are the knobs only --apply has.
type ApplyOptions struct {
	// AllowDirty lets apply run on a work tree with uncommitted changes or
	// untracked copies (whose deletion git cannot undo).
	AllowDirty bool
	// RebarBin is the rebar CLI the wrappers' smoke run puts on PATH ("":
	// whatever `rebar` PATH has, else no smoke run).
	RebarBin string
}

// ApplyResult is what apply did.
type ApplyResult struct {
	Checks     []CheckResult
	RolledBack bool
}

// backup holds a file's state before apply, for rollback.
type backup struct {
	path    string
	data    []byte
	mode    os.FileMode
	existed bool
}

// Apply writes .rebarrc, removes or replaces the verified copies, edits
// METRICS, optionally regenerates the registry, then reruns every check
// that had a legacy copy (and each wrapper left for callers). If a check
// that passed before fails, everything apply changed is put back.
//
// It refuses to start on a dirty work tree (unless AllowDirty), when a file
// changed since the plan was made, or when a target is a symlink or
// resolves outside the project.
func Apply(p *Plan, opts Options, ao ApplyOptions) (*ApplyResult, error) {
	if opts.Timeout == 0 {
		opts.Timeout = defaultTimeout
	}
	defer Cleanup()
	if err := p.preflight(ao); err != nil {
		return nil, err
	}

	var backups []backup
	save := func(path string) error {
		b := backup{path: path}
		if info, err := os.Lstat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			b.data, b.mode, b.existed = data, info.Mode().Perm(), true
		}
		backups = append(backups, b)
		return nil
	}
	rollback := func() {
		for i := len(backups) - 1; i >= 0; i-- {
			b := backups[i]
			if b.existed {
				_ = writeAtomic(b.path, b.data, b.mode)
			} else {
				_ = os.Remove(b.path)
			}
		}
	}
	fail := func(err error) (*ApplyResult, error) {
		rollback()
		return nil, fmt.Errorf("%v (nothing was changed: apply rolled back)", err)
	}

	rcPath := filepath.Join(p.Project, ".rebarrc")
	if !bytes.Equal(p.RC, p.NewRC) {
		if err := save(rcPath); err != nil {
			return fail(err)
		}
		if err := writeAtomic(rcPath, p.NewRC, 0o644); err != nil {
			return fail(err)
		}
	}
	for _, s := range p.Deletions() {
		path := filepath.Join(p.Project, "scripts", s.Name)
		if err := save(path); err != nil {
			return fail(err)
		}
		switch s.Verdict {
		case Delete:
			if err := os.Remove(path); err != nil {
				return fail(err)
			}
		case Replace:
			data := s.ReplaceContent
			if data == nil {
				return fail(fmt.Errorf("no replacement content for scripts/%s", s.Name))
			}
			mode := os.FileMode(0o644)
			if strings.HasSuffix(s.Name, ".sh") {
				mode = 0o755
			}
			if err := writeAtomic(path, data, mode); err != nil {
				return fail(err)
			}
		}
	}
	if len(p.MetricsEdits) > 0 {
		if err := save(p.MetricsFile); err != nil {
			return fail(err)
		}
		if err := writeAtomic(p.MetricsFile, []byte(applyMetricsEdits(string(p.MetricsData), p.MetricsEdits)), 0o644); err != nil {
			return fail(err)
		}
	}
	if p.MCP != nil {
		if err := save(p.MCP.File); err != nil {
			return fail(err)
		}
		mode := os.FileMode(0o644)
		if info, err := os.Stat(p.MCP.File); err == nil {
			mode = info.Mode().Perm()
		}
		if err := writeAtomic(p.MCP.File, p.MCP.New, mode); err != nil {
			return fail(err)
		}
	}
	env := scrubbedEnv("REBAR_PROJECT_ROOT=" + p.Project)
	if p.RegenRegistry {
		reg := filepath.Join(p.Project, firstNonEmpty(rcValue(p.NewRC, "contracts_dir"), "architecture"), "CONTRACT-REGISTRY.md")
		if err := save(reg); err != nil {
			return fail(err)
		}
		r := runNative(p.Project, "", env, "compute-registry.sh")
		if !r.ok() {
			return fail(fmt.Errorf("regenerating the registry failed: %s%s", r.Stdout, r.Stderr))
		}
	}

	res := &ApplyResult{}
	for _, s := range p.Scripts {
		if s.Check == nil || s.Before == nil || s.Writes {
			continue
		}
		tier := p.Tier
		if s.MinTier > tier && !s.Enabled {
			tier = s.MinTier
		}
		tierEnv := append(append([]string{}, env...), fmt.Sprintf("REBAR_TIER=%d", tier))
		var r runResult
		ran := "rebar"
		if s.Verdict == Keep {
			ran = "legacy copy"
			r = run(opts.Timeout, p.Project, tierEnv, append([]string{filepath.Join(p.Project, "scripts", s.Name)}, s.Check...)...)
		} else {
			r = runNative(p.Project, "", tierEnv, s.Name, s.Check...)
		}
		res.Checks = append(res.Checks, CheckResult{Name: s.Name, Ran: ran, Tier: tier, BeforeCode: s.Before.Code, AfterCode: codeOf(r), Output: r.Stdout + r.Stderr})
		if s.Shim {
			// The wrapper the callers will run, the way they run it.
			if wenv, ok := wrapperEnv(ao.RebarBin, p.Install, tierEnv); ok {
				w := run(opts.Timeout, p.Project, wenv, append([]string{filepath.Join(p.Project, "scripts", s.Name)}, s.Check...)...)
				res.Checks = append(res.Checks, CheckResult{Name: s.Name, Ran: "wrapper scripts/" + s.Name, Tier: tier, BeforeCode: s.Before.Code, AfterCode: codeOf(w), Output: w.Stdout + w.Stderr})
			}
		}
	}
	for _, c := range res.Checks {
		if c.Regressed() {
			rollback()
			res.RolledBack = true
			break
		}
	}
	return res, nil
}

func codeOf(r runResult) int {
	if r.Err != nil {
		return -1
	}
	return r.Code
}

// wrapperEnv puts a `rebar` (bin, or the one on PATH) first on PATH and
// points it at this install, for running a thin wrapper as a caller would.
func wrapperEnv(bin, install string, env []string) ([]string, bool) {
	if bin == "" {
		p, err := exec.LookPath("rebar")
		if err != nil {
			return nil, false
		}
		bin = p
	}
	dir, err := os.MkdirTemp("", "rebar-migrate-bin-")
	if err != nil {
		return nil, false
	}
	if err := os.Symlink(bin, filepath.Join(dir, "rebar")); err != nil {
		os.RemoveAll(dir)
		return nil, false
	}
	defer func() { cleanupDirs = append(cleanupDirs, dir) }()
	var out []string
	if install != "" {
		out = append(out, "REBAR_ROOT="+install)
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			out = append(out, "PATH="+dir+string(os.PathListSeparator)+strings.TrimPrefix(kv, "PATH="))
			continue
		}
		if strings.HasPrefix(kv, "REBAR_PROJECT_ROOT=") || strings.HasPrefix(kv, "REBAR_TIER=") {
			continue // the wrapper finds the project itself, and its tier in .rebarrc
		}
		out = append(out, kv)
	}
	return out, true
}

// cleanupDirs are removed by Cleanup.
var cleanupDirs []string

// Cleanup removes the temporary directories apply created (Apply calls it).
func Cleanup() {
	for _, d := range cleanupDirs {
		os.RemoveAll(d)
	}
	cleanupDirs = nil
}

// preflight refuses to apply when the plan is stale, a target is unsafe,
// or (without AllowDirty) the work tree has changes git could not restore.
func (p *Plan) preflight(ao ApplyOptions) error {
	if !sameBytes(filepath.Join(p.Project, ".rebarrc"), nilIfEmpty(p.RC)) {
		return fmt.Errorf(".rebarrc changed since the plan was made; rerun rebar migrate")
	}
	if p.MetricsFile != "" && !sameBytes(p.MetricsFile, p.MetricsData) {
		return fmt.Errorf("%s changed since the plan was made; rerun rebar migrate", filepath.Base(p.MetricsFile))
	}
	targets := []string{filepath.Join(p.Project, ".rebarrc")}
	if p.MetricsFile != "" {
		targets = append(targets, p.MetricsFile)
	}
	if p.MCP != nil {
		if !sameBytes(p.MCP.File, p.MCP.Data) {
			return fmt.Errorf(".mcp.json changed since the plan was made; rerun rebar migrate")
		}
		targets = append(targets, p.MCP.File)
	}
	for _, s := range p.Deletions() {
		path := filepath.Join(p.Project, "scripts", s.Name)
		data, err := os.ReadFile(path)
		if err != nil || gitBlobHash(data) != s.Hash {
			return fmt.Errorf("scripts/%s changed since the plan was made; rerun rebar migrate", s.Name)
		}
		targets = append(targets, path)
	}
	for _, t := range targets {
		if err := safeTarget(p.Project, p.Install, t); err != nil {
			return err
		}
	}
	if ao.AllowDirty {
		return nil
	}
	if _, err := gitOut(p.Project, "rev-parse", "--verify", "HEAD"); err != nil {
		return fmt.Errorf("%s is not a git work tree with commits, so deleted copies could not be restored; commit first or pass --allow-dirty", p.Project)
	}
	out, err := gitOut(p.Project, "status", "--porcelain", "--untracked-files=all", "--", ".")
	if err != nil {
		return err
	}
	if lines := strings.TrimSpace(out); lines != "" {
		first := strings.Split(lines, "\n")
		if len(first) > 5 {
			first = append(first[:5], "…")
		}
		return fmt.Errorf("the work tree has uncommitted or untracked files; commit or stash them first so every change apply makes can be reviewed and undone with git (or pass --allow-dirty):\n  %s", strings.Join(first, "\n  "))
	}
	return nil
}

func nilIfEmpty(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}

// safeTarget refuses a path apply would write or delete unless it is a
// regular file (or absent) whose real directory is inside the project and
// not inside the rebar install.
func safeTarget(project, install, path string) error {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to touch %s: not a regular file (symlink?)", path)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("refusing to touch %s: %v", path, err)
	}
	if !within(project, dir) {
		return fmt.Errorf("refusing to touch %s: it resolves to %s, outside the project", path, dir)
	}
	if install == "" {
		return nil
	}
	if inst, err := filepath.EvalSymlinks(install); err == nil && within(inst, dir) {
		return fmt.Errorf("refusing to touch %s: it resolves into the rebar install", path)
	}
	return nil
}

// writeAtomic replaces path with data via a temp file and rename in the
// same directory, so a crash never leaves it half written, and a symlink
// at path is never followed.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write through symlink %s", path)
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".migrate-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// RenderChecks prints post-apply check results; it returns true when a
// check that passed before fails now.
func RenderChecks(w io.Writer, results []CheckResult, projectTier int) bool {
	if len(results) == 0 {
		return false
	}
	regressed := false
	fmt.Fprintln(w, "Checks after migration:")
	for _, r := range results {
		status := "PASS"
		if r.AfterCode != 0 {
			status = "FAIL"
		}
		before := "pass"
		if r.BeforeCode != 0 {
			before = "fail"
		}
		extra := ""
		if r.Tier > projectTier {
			extra = fmt.Sprintf(" (run at tier %d; at the project's tier %d the install skips it)", r.Tier, projectTier)
		}
		fmt.Fprintf(w, "  %s  %s via %s, before: %s%s\n", status, strings.TrimSuffix(r.Name, ".sh"), r.Ran, before, extra)
		if r.Regressed() {
			regressed = true
			for _, line := range lastLines(r.Output, 12) {
				fmt.Fprintf(w, "        %s\n", line)
			}
		}
	}
	return regressed
}

func lastLines(s string, n int) []string {
	lines := strings.Split(strings.TrimRight(ansiRe.ReplaceAllString(s, ""), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
