package checks

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/gitx"
)

// check-doc-refs: every file a tracked *.md links to ([text](target)) is
// tracked in git too. Native port of scripts/check-doc-refs.sh.
func init() {
	register(&Check{Name: "check-doc-refs", Run: runDocRefs})
}

var (
	// The script's awk: a link, then the parts stripped to leave its target.
	mdLink      = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	linkTitle   = regexp.MustCompile(`[[:space:]]+"[^"]*"$`)
	dotDotSlash = regexp.MustCompile(`[^/]+/\.\./`)
)

// A docRef is a broken link: the file, line number and target.
type docRef struct{ src, line, target string }

func runDocRefs(ctx *Context, args []string) int {
	mode := "text"
	if len(args) > 0 {
		switch args[0] {
		case "--quiet":
			mode = "quiet"
		case "--json":
			mode = "json"
		case "-h", "--help":
			fmt.Fprint(ctx.Stdout, scriptHelp["check-doc-refs"])
			return 0
		}
	}
	if !gitx.Available() {
		fmt.Fprintln(ctx.Stderr, "check-doc-refs: git not found")
		return 2
	}
	root := ctx.Root

	allow := map[string]bool{}
	if data, err := os.ReadFile(root + "/.rebar/doc-refs-allow.txt"); err == nil {
		for _, line := range textLines(data) {
			if line != "" && !strings.HasPrefix(line, "#") {
				allow[line] = true
			}
		}
	}
	trackedOut, _ := gitx.Run(root, ctx.Stderr, "ls-files")
	tracked := gitx.Lines(trackedOut)
	trackedSet := make(map[string]bool, len(tracked))
	for _, p := range tracked {
		trackedSet[p] = true
	}
	var dirCache map[string]*regexp.Regexp

	var broken []docRef
	sources, _ := gitx.Run(root, ctx.Stderr, "ls-files", "*.md")
	for _, src := range gitx.Lines(sources) {
		// The script cds to the project root.
		if !isRegular(root + "/" + src) {
			continue
		}
		data, err := os.ReadFile(root + "/" + src)
		if err != nil || generatedDoc(data) {
			continue
		}
		if strings.HasPrefix(src, ".claude/worktrees/") || strings.HasPrefix(src, "feedback/processed/") || strings.HasPrefix(src, "templates/") {
			continue
		}
		srcDir := shellDirname(src)
		for n, line := range textLines(data) {
			for _, target := range linkTargets(line) {
				resolved, ok := resolveDocRef(srcDir, target)
				if !ok {
					continue
				}
				if len(allow) > 0 && allow[resolved] {
					continue
				}
				if dir, isDir := strings.CutSuffix(resolved, "/"); isDir {
					// grep -q "^${dir}/" over the tracked files: the path is
					// a basic regular expression.
					if dirCache == nil {
						dirCache = map[string]*regexp.Regexp{}
					}
					re, seen := dirCache[dir]
					if !seen {
						re = breRegexp("^" + dir + "/")
						dirCache[dir] = re
					}
					if re != nil && anyMatch(re, tracked) {
						continue
					}
				} else if trackedSet[resolved] {
					continue
				}
				broken = append(broken, docRef{src, fmt.Sprint(n + 1), target})
			}
		}
	}

	w := ctx.Stdout
	switch mode {
	case "json":
		fmt.Fprintf(w, `{"checked":"markdown-link-targets","broken":%d,"findings":[`, len(broken))
		for i, f := range broken {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			// The script reads its findings back with IFS=: read -r src
			// lineno target, so a colon in the file name moves the fields.
			src, line, target := ifsColonFields(f.src + ":" + f.line + ":" + f.target)
			fmt.Fprintf(w, `{"file":"%s","line":%s,"target":"%s"}`, jsonMinimal(src), line, jsonMinimal(target))
		}
		fmt.Fprint(w, "]}\n")
	case "quiet":
		for _, f := range broken {
			fmt.Fprintf(w, "%s:%s:%s\n", f.src, f.line, f.target)
		}
	default:
		if len(broken) == 0 {
			fmt.Fprintln(w, "check-doc-refs: OK — no broken markdown links to untracked files")
			break
		}
		fmt.Fprintf(w, "check-doc-refs: %d broken ref(s) — referenced file is not tracked in git\n\n", len(broken))
		for _, f := range broken {
			fmt.Fprintf(w, "%s:%s:%s\n", f.src, f.line, f.target)
		}
		fmt.Fprint(w, "\nEach line: <source.md>:<lineno>:<target>\n")
		fmt.Fprint(w, "Either `git add` the target, fix the link, or allowlist it via .rebar/doc-refs-allow.txt\n")
	}
	if len(broken) > 0 {
		return 1
	}
	return 0
}

// generatedDoc is `head -3 | grep -qE 'AUTO-GENERATED|<!-- generated'`.
func generatedDoc(data []byte) bool {
	head := data
	for i, n := 0, 0; i < len(data); i++ {
		if data[i] == '\n' {
			if n++; n == 3 {
				head = data[:i]
				break
			}
		}
	}
	return bytes.Contains(head, []byte("AUTO-GENERATED")) || bytes.Contains(head, []byte("<!-- generated"))
}

// linkTargets are the targets the script's awk prints for one line, as
// `IFS=$'\t' read -r lineno target` reads them back (surrounding tabs
// dropped): each [text](target) in turn, without a trailing ` "title"`.
func linkTargets(line string) []string {
	var targets []string
	for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
		t := m[1]
		if loc := linkTitle.FindStringIndex(t); loc != nil {
			t = t[:loc[0]]
		}
		targets = append(targets, strings.Trim(t, "\t"))
	}
	return targets
}

// resolveDocRef is the script's filter and path resolution for a link
// target in a file in srcDir: false for targets it skips (URLs, anchors,
// ~ paths, {placeholders}, bare words), else the path relative to the
// project root, ./ and ../ segments folded as its sed commands fold them.
func resolveDocRef(srcDir, target string) (string, bool) {
	if target == "" {
		return "", false
	}
	for _, scheme := range []string{"http://", "https://", "mailto:", "tel:", "ftp://", "sftp://", "#", "~"} {
		if strings.HasPrefix(target, scheme) {
			return "", false
		}
	}
	if i := strings.IndexByte(target, '{'); i >= 0 && strings.IndexByte(target[i+1:], '}') >= 0 {
		return "", false
	}
	if !strings.ContainsAny(target, "/.") {
		return "", false
	}
	p, _, _ := strings.Cut(target, "#")
	if p == "" {
		return "", false
	}
	p, _, _ = strings.Cut(p, "?")
	var resolved string
	switch {
	case strings.HasPrefix(p, "/"):
		resolved = p[1:]
	case srcDir == ".":
		resolved = p
	default:
		resolved = srcDir + "/" + p
	}
	resolved = strings.ReplaceAll(resolved, "/./", "/")
	resolved = strings.TrimPrefix(resolved, "./")
	for strings.Contains(resolved, "/../") {
		loc := dotDotSlash.FindStringIndex(resolved)
		if loc == nil {
			// The script loops forever here (a//../b, /../b): its sed
			// cannot fold the segment. Leave it as it is.
			break
		}
		resolved = resolved[:loc[0]] + resolved[loc[1]:]
	}
	return resolved, true
}

// shellDirname is dirname(1) for a relative path without trailing slashes.
func shellDirname(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return "."
	}
	if d := strings.TrimRight(p[:i], "/"); d != "" {
		return d
	}
	return "/"
}

func anyMatch(re *regexp.Regexp, lines []string) bool {
	for _, l := range lines {
		if re.MatchString(l) {
			return true
		}
	}
	return false
}

// ifsColonFields is `IFS=: read -r a b c` on line: a and b up to the
// first and second colon, c the rest, losing a single trailing colon when
// it is c's only one.
func ifsColonFields(line string) (a, b, c string) {
	a, rest, ok := strings.Cut(line, ":")
	if !ok {
		return a, "", ""
	}
	b, c, _ = strings.Cut(rest, ":")
	if strings.Count(c, ":") == 1 && strings.HasSuffix(c, ":") {
		c = strings.TrimSuffix(c, ":")
	}
	return a, b, c
}

// jsonMinimal is the script's escaping: backslashes and double quotes.
func jsonMinimal(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
