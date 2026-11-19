package checks

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/gitx"
)

// check-decay-patterns: flag soft-hardening patterns (silenced failures,
// inverted assertions, keep-in-sync comments, magic-string gating,
// plausible test-only env names, demo-spec bypasses) in spec, test, test
// config and workflow files. Native port of scripts/check-decay-patterns.sh.
func init() {
	register(&Check{Name: "check-decay-patterns", Run: runDecayPatterns})
}

// A decayPattern is one of the script's scans: grep -nEi regex.
type decayPattern struct {
	id      string
	re      *regexp.Regexp
	message string
	// files picks the files of the scan set it scans (nil: all).
	files func(root string, scan []string) []string
}

var decayPatterns = []decayPattern{
	{
		id:      "P1-silenced-failure",
		re:      regexp.MustCompile(`(?i)(testInfo\.fail|test\.fail|it\.fail|describe\.fail|expect\.fail)\(`),
		message: "Pattern 1 (silenced failure): use a real failing assertion + CI continue-on-error instead — silencing the test makes the bug invisible to anyone glancing at the dashboard.",
	},
	{
		id:      "P2-inverted-assertion",
		re:      regexp.MustCompile(`(?i)\.(toBe\(null\)|toBeNull\(\)|toEqual\(\[\]\)|not\.toContain)`),
		message: "Pattern 2 (inverted assertion): assertion passes when the thing being tested is broken — a future engineer 'fixing' this test will silently elide the signal. Restate positively + use expected-fail at the workflow level. Or pair with a negative-control test that stages the violation.",
		files: func(_ string, scan []string) []string {
			var sec []string
			for _, f := range scan {
				if globMatch("*security*", f) || globMatch("*audit*", f) || globMatch("*regression*", f) || globMatch("*detector*", f) {
					sec = append(sec, f)
				}
			}
			return sec
		},
	},
	{
		id:      "P4-keep-in-sync",
		re:      regexp.MustCompile(`(?i)(keep[[:space:]]+(this|in|them)[[:space:]]+(in[[:space:]]+)?sync|update[[:space:]]+when[[:space:]]+upgrading|mirrors[[:space:]]+the[[:space:]]+upstream)`),
		message: "Pattern 4 (silent drift on upgrade): a comment saying 'keep in sync' is a wish. Read the source of truth at runtime and throw on parse failure, instead of hand-copying.",
	},
	{
		id:      "P5-magic-string-gating",
		re:      regexp.MustCompile(`(?i)testInfo\.project\.name[[:space:]]*[!=]==?[[:space:]]*['"]`),
		message: "Pattern 5 (magic-string gating): renaming the project silently breaks the gate, no compile error. Use testInfo.project.metadata.<flag> instead — moving metadata with the project definition makes the coupling explicit.",
	},
	{
		id:      "P7-plausible-env-name",
		re:      regexp.MustCompile(`(?i)(VITE_ALLOW_ALL|ALLOW_ALL_HOSTS|DISABLE_AUTH|SKIP_VERIFY|BYPASS_(AUTH|TLS|CORS))`),
		message: "Pattern 7 (too-plausible-to-refuse): this env var name reads like normal infrastructure config — a deploy script copy-paste won't be flagged. Make the name long and obviously test-only (e.g., FOO_PLAYWRIGHT_REAL_CHROME_LOOSENING) and require a second test-only env var to activate.",
	},
	{
		id:      "P8-demo-bypass",
		re:      regexp.MustCompile(`(?i)(loginAs[[:space:]]*\(|installSessionCookie|userManager\.storeUser|sessionStorage\.setItem|localStorage\.setItem|addInitScript|addCookies[[:space:]]*\(|page\.route[[:space:]]*\(|route\.fulfill|page\.evaluate|request\.(post|get|put|delete)[[:space:]]*\()`),
		message: "Pattern 8 (demo-spec bypass): a demo-claiming or uaks-declared spec must use only what a user at a keyboard could do — no state injection, no network mocks, no API-direct user actions. Move the shortcut to a lower-fidelity spec (and name the helper STUB_/INJECT_/BYPASS_), or drop the demo claim. See practices/test-fidelity.md.",
		files:   demoFiles,
	},
}

// uaksDeclaration opts a file into P8 (grep -qE, case-sensitive).
var uaksDeclaration = regexp.MustCompile(`(//|#|--)[[:space:]]*fidelity:[[:space:]]*uaks`)

// decayScanGlobs are the case patterns of the files the script scans.
var decayScanGlobs = []string{
	"*.spec.ts", "*.spec.tsx", "*.spec.js", "*.spec.jsx", "*.spec.mjs",
	"*.test.ts", "*.test.tsx", "*.test.js", "*.test.jsx", "*.test.mjs",
	"*_test.go", "*_spec.rb", "*_test.py", "*test_*.py",
	"*playwright*.config.*", "*vitest*.config.*", "*jest*.config.*",
	".github/workflows/*.yml", ".github/workflows/*.yaml",
}

// demoFiles are the files P8 scans: demo-claiming specs by name, and any
// spec that declares `fidelity: uaks`; never configs or workflows.
func demoFiles(root string, scan []string) []string {
	var demo []string
	for _, f := range scan {
		if globMatch("*.config.*", f) || globMatch(".github/workflows/*", f) {
			continue
		}
		base := f[strings.LastIndexByte(f, '/')+1:]
		switch {
		case globMatch("demo-*", base), globMatch("ui-demo-*", base), globMatch("*uaks*", base), globMatch("*user-at-keyboard*", base):
			demo = append(demo, f)
		default:
			if data, err := os.ReadFile(rootPath(root, f)); err == nil && anyLineMatches(uaksDeclaration, data) {
				demo = append(demo, f)
			}
		}
	}
	return demo
}

func runDecayPatterns(ctx *Context, args []string) int {
	mode := "staged"
	warnOnly := false
	var explicit []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--all":
			mode = "all"
		case "--paths":
			mode = "explicit"
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				i++
				explicit = append(explicit, args[i])
			}
		case "--warn":
			warnOnly = true
		case "-h", "--help":
			fmt.Fprint(ctx.Stdout, scriptHelp["check-decay-patterns"])
			return 0
		default:
			fmt.Fprintf(ctx.Stderr, "check-decay-patterns: unknown arg '%s'\n", a)
			return 2
		}
	}
	root := ctx.Root // the script cds to the project root
	w := ctx.Stdout

	var files []string
	switch mode {
	case "staged":
		out, _ := gitx.Run(root, nil, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
		files = gitx.Lines(out)
		if len(files) == 0 {
			// Working-tree changes, for runs outside a hook.
			out, _ = gitx.Run(root, nil, "diff", "--name-only", "--diff-filter=ACMR")
			files = gitx.Lines(out)
		}
	case "all":
		out, _ := gitx.Run(root, ctx.Stderr, "ls-files")
		files = gitx.Lines(out)
	case "explicit":
		files = explicit
	}
	if len(files) == 0 {
		fmt.Fprintln(w, "check-decay-patterns: no files to scan.")
		return 0
	}

	var scan []string
	for _, f := range files {
		if !isRegular(rootPath(root, f)) {
			continue
		}
		for _, g := range decayScanGlobs {
			if globMatch(g, f) {
				scan = append(scan, f)
				break
			}
		}
	}
	if len(scan) == 0 {
		fmt.Fprintln(w, "check-decay-patterns: no spec/config/workflow files in scan set.")
		return 0
	}

	var allow []byte
	allowFile := root + "/.rebar/decay-patterns-allow.txt"
	allowed := func(key string) bool {
		if !isRegular(allowFile) {
			return false
		}
		if allow == nil {
			allow, _ = os.ReadFile(allowFile)
		}
		for _, line := range textLines(allow) {
			if line == key {
				return true
			}
		}
		return false
	}

	var findings []string
	for _, p := range decayPatterns {
		targets := scan
		if p.files != nil {
			if targets = p.files(root, scan); len(targets) == 0 {
				continue
			}
		}
		for _, f := range targets {
			data, err := os.ReadFile(rootPath(root, f))
			if err != nil || hasNULBlock(data) {
				// A binary file: GNU grep prints no lines for it.
				continue
			}
			for n, line := range textLines(data) {
				if !p.re.MatchString(line) {
					continue
				}
				key := fmt.Sprintf("%s:%d:%s", f, n+1, p.id)
				if !allowed(key) {
					findings = append(findings, key+":"+p.message)
				}
			}
		}
	}

	if len(findings) == 0 {
		fmt.Fprintln(w, "check-decay-patterns: OK — no soft-hardening patterns detected in scan set.")
		return 0
	}
	fmt.Fprintf(w, "check-decay-patterns: %d finding(s) in scan set\n\n", len(findings))
	fmt.Fprint(w, "Each line: <file>:<line>:<pattern-id>:<guidance>\n\n")
	for _, f := range findings {
		fmt.Fprintln(w, f)
	}
	fmt.Fprint(w, "\n",
		"If a finding is intentional + reviewed, allowlist by adding\n",
		"  <file>:<line>:<pattern-id>\n",
		"to .rebar/decay-patterns-allow.txt (one per line, # for comments).\n",
		"\n",
		"References: feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md\n",
		"            feedback/processed/2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md (P8)\n",
		"            practices/test-fidelity.md (fidelity ladder + gates)\n")
	if warnOnly {
		return 0
	}
	return 1
}

// rootPath is a path the script uses after cd-ing to root.
func rootPath(root, p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return root + "/" + p
}

// anyLineMatches is grep -q re on a file's text.
func anyLineMatches(re *regexp.Regexp, data []byte) bool {
	for _, line := range textLines(data) {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// globMatch is a bash case pattern made of literal text and *, which
// matches any string (slashes included).
func globMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	last := parts[len(parts)-1]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(s, part)
		if i < 0 {
			return false
		}
		s = s[i+len(part):]
	}
	return len(s) >= len(last) && strings.HasSuffix(s, last)
}
