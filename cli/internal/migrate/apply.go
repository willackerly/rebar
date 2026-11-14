package migrate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Render prints the plan: proposed .rebarrc lines, files deleted or
// replaced, files kept with reasons, METRICS edits, and notes.
func Render(w io.Writer, p *Plan, applied bool) {
	verb := map[bool]string{false: "would", true: ""}
	fmt.Fprintf(w, "rebar migrate — %s\n", p.Project)
	fmt.Fprintf(w, "install: %s   tier: %d\n\n", p.Install, p.Tier)
	if len(p.Scripts) == 0 {
		fmt.Fprintln(w, "No legacy script copies: the project already runs on the install.")
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
			fmt.Fprintf(w, "  %-8s scripts/%s — %s\n", s.Verdict, s.Name, s.Reason)
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
	if p.RegenRegistry {
		fmt.Fprintf(w, "CONTRACT-REGISTRY.md: %s be regenerated with the ellipsis purpose style\n\n", strings.TrimSpace(verb[applied]))
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
	Ran        string // "install" or "legacy copy"
	Tier       int
	BeforeCode int
	AfterCode  int
	Output     string
}

// Regressed reports a check that passed before and fails now.
func (c CheckResult) Regressed() bool { return c.BeforeCode == 0 && c.AfterCode != 0 }

// Apply writes .rebarrc, removes or replaces the verified copies, edits
// METRICS, optionally regenerates the registry, then reruns every check
// that had a legacy copy and returns the results.
func Apply(p *Plan, opts Options) ([]CheckResult, error) {
	if opts.Timeout == 0 {
		opts.Timeout = defaultTimeout
	}
	rcPath := filepath.Join(p.Project, ".rebarrc")
	if !bytes.Equal(p.RC, p.NewRC) {
		if err := os.WriteFile(rcPath, p.NewRC, 0o644); err != nil {
			return nil, err
		}
	}
	for _, s := range p.Deletions() {
		path := filepath.Join(p.Project, "scripts", s.Name)
		if !inside(p.Project, path) {
			return nil, fmt.Errorf("refusing to touch %s: outside the project", path)
		}
		switch s.Verdict {
		case Delete:
			if err := os.Remove(path); err != nil {
				return nil, err
			}
		case Replace:
			data, err := os.ReadFile(s.ReplaceWith)
			if err != nil {
				return nil, err
			}
			mode := os.FileMode(0o644)
			if strings.HasSuffix(s.Name, ".sh") {
				mode = 0o755
			}
			if err := os.WriteFile(path, data, mode); err != nil {
				return nil, err
			}
			_ = os.Chmod(path, mode)
		}
	}
	if len(p.MetricsEdits) > 0 {
		data, err := os.ReadFile(p.MetricsFile)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(p.MetricsFile, []byte(applyMetricsEdits(string(data), p.MetricsEdits)), 0o644); err != nil {
			return nil, err
		}
	}
	env := scrubbedEnv("REBAR_PROJECT_ROOT=" + p.Project)
	if p.RegenRegistry {
		r := run(opts.Timeout, p.Project, env, filepath.Join(p.Install, "scripts", "compute-registry.sh"))
		if !r.ok() {
			return nil, fmt.Errorf("regenerating the registry failed: %s%s", r.Stdout, r.Stderr)
		}
	}

	var results []CheckResult
	for _, s := range p.Scripts {
		if s.Check == nil || s.Before == nil {
			continue
		}
		tier := p.Tier
		if s.MinTier > tier {
			tier = s.MinTier
		}
		script := filepath.Join(p.Install, "scripts", s.Name)
		ran := "install"
		if s.Verdict == Keep {
			script, ran = filepath.Join(p.Project, "scripts", s.Name), "legacy copy"
		}
		r := run(opts.Timeout, p.Project, append(env, fmt.Sprintf("REBAR_TIER=%d", tier)), append([]string{script}, s.Check...)...)
		code := r.Code
		if r.Err != nil {
			code = -1
		}
		results = append(results, CheckResult{Name: s.Name, Ran: ran, Tier: tier, BeforeCode: s.Before.Code, AfterCode: code, Output: r.Stdout + r.Stderr})
	}
	return results, nil
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../") && !filepath.IsAbs(rel)
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
