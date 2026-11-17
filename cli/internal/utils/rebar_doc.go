package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/willackerly/rebar/cli/internal/config"

	"github.com/willackerly/rebar/cli/internal/checks"
)

// The rebar-doc resolver: scripts/rebar-doc.sh, whose kind table
// (conventions.md §Cross-Repo References) `rebar doc` shares through
// DocCandidates.

// UpstreamBase is the canonical upstream prefix for unresolved refs.
const UpstreamBase = "https://github.com/willackerly/rebar/blob/main"

const rebarDocUsage = `Usage: rebar-doc.sh <ref> [--cat]
       rebar-doc.sh -h|--help

Resolve a rebar:<kind>/<name> cross-repo ref to a local file.
The 'rebar:' prefix is optional.

Kinds:
  practice/<name>          script/<name>   agents/<name>   doc/<name>
  convention[/<section>]   charter         feedback/<name>

Default output is '<source>\t<path>' (source: local|REBAR_ROOT|checkout|upstream).
--cat prints the resolved file's contents instead.

Exit codes: 0 resolved, 2 usage/malformed ref, 4 unresolvable locally
(upstream URL printed).
`

// DocCandidates maps the <kind> and <name> of a ref (written ref, for the
// messages) to its ordered repo-relative candidate paths, the first being
// the canonical upstream one. A non-empty problem is why the ref is
// malformed or its kind unknown, worded as rebar-doc.sh reports it.
func DocCandidates(ref, kind, name string) (candidates []string, problem string) {
	needName := func(rel ...string) ([]string, string) {
		switch {
		case name == "":
			return nil, fmt.Sprintf("malformed ref '%s': kind '%s' requires a name", ref, kind)
		case strings.Contains(name, "/"):
			return nil, fmt.Sprintf("malformed ref '%s': name must not contain '/'", ref)
		case strings.ContainsAny(name, " \t\n\v\f\r"):
			return nil, fmt.Sprintf("malformed ref '%s': name must not contain whitespace", ref)
		case strings.Contains(name, ".."):
			return nil, fmt.Sprintf("malformed ref '%s': name must not contain '..'", ref)
		}
		return rel, ""
	}
	switch kind {
	case "practice":
		return needName("practices/" + name + ".md")
	case "script":
		return needName("scripts/" + name + ".sh")
	case "agents":
		return needName("agents/" + name + ".md")
	case "convention":
		return []string{"conventions.md"}, "" // the section is informational
	case "charter":
		if name != "" {
			return nil, fmt.Sprintf("malformed ref '%s': 'charter' takes no name", ref)
		}
		return []string{"CHARTER.md"}, ""
	case "doc":
		return needName(name+".md", "docs/"+name+".md")
	case "feedback":
		return needName("feedback/"+name+".md", "feedback/processed/"+name+".md")
	}
	return nil, fmt.Sprintf("unknown kind '%s' (expected practice|script|agents|convention|charter|doc|feedback)", kind)
}

// SplitRef splits a ref (rebar: prefix optional) into kind and name, as
// rebar-doc.sh does: the kind up to the first /, the name after it.
func SplitRef(ref string) (kind, name string) {
	r := strings.TrimPrefix(ref, "rebar:")
	kind, name, _ = strings.Cut(r, "/")
	return kind, name
}

// runRebarDoc is scripts/rebar-doc.sh: resolve <ref> against the current
// repo (the nearest directory with a .git above the working directory, else
// the working directory), $REBAR_ROOT, then a rebar checkout at ~/.rebar,
// ~/dev/rebar, ~/src/rebar or ~/code/rebar; print '<source>\t<path>' (or the
// file with --cat), or the upstream URL and exit 4.
func runRebarDoc(ctx *checks.Context, args []string) int {
	ref, cat := "", false
	usageErr := func(msg string) int {
		fmt.Fprintf(ctx.Stderr, "rebar-doc.sh: %s\n", msg)
		io.WriteString(ctx.Stderr, rebarDocUsage)
		return 2
	}
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			io.WriteString(ctx.Stdout, rebarDocUsage)
			return 0
		case a == "--cat":
			cat = true
		case strings.HasPrefix(a, "-"):
			return usageErr("unknown option: " + a)
		default:
			if ref != "" {
				return usageErr("unexpected extra argument: " + a)
			}
			ref = a
		}
	}
	if ref == "" {
		return usageErr("missing <ref>")
	}
	kind, name := SplitRef(ref)
	candidates, problem := DocCandidates(ref, kind, name)
	if problem != "" {
		return usageErr(problem)
	}

	emit := func(source, path string) int {
		if cat {
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "cat: %s: %s\n", path, errText(err))
				return 0 // the script exits 0 after cat, whatever cat did
			}
			ctx.Stdout.Write(data)
		} else {
			fmt.Fprintf(ctx.Stdout, "%s\t%s\n", source, path)
		}
		return 0
	}
	resolveIn := func(root string) (string, bool) {
		for _, c := range candidates {
			p := root + "/" + c
			if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
				return p, true
			}
		}
		return "", false
	}

	if p, ok := resolveIn(currentRepo(ctx.WorkDir())); ok {
		return emit("local", p)
	}
	if root := config.Getenv("REBAR_ROOT"); root != "" {
		if p, ok := resolveIn(root); ok {
			return emit("REBAR_ROOT", p)
		}
	}
	if home := config.Getenv("HOME"); home != "" {
		for _, c := range []string{home + "/.rebar", home + "/dev/rebar", home + "/src/rebar", home + "/code/rebar"} {
			if !isRebarSource(c) {
				continue
			}
			if p, ok := resolveIn(c); ok {
				return emit("checkout", p)
			}
		}
	}
	fmt.Fprintf(ctx.Stdout, "upstream\t%s/%s\n", UpstreamBase, candidates[0])
	fmt.Fprintln(ctx.Stdout, "unresolved locally — ask rebar architect (or the relevant role) for questions")
	return 4
}

// currentRepo is the script's find_current_repo: the nearest of dir and its
// parents holding a .git entry, else dir.
func currentRepo(dir string) string {
	for d := dir; ; {
		if _, err := os.Stat(d + "/.git"); err == nil {
			return d
		}
		if d == "/" {
			break
		}
		d = filepath.Dir(d)
	}
	return dir
}

// isRebarSource is the rebar source repo marker (config.IsRebarTree), as
// cli/cmd/init.go findRebarRoot uses it.
func isRebarSource(dir string) bool { return config.IsRebarTree(dir) }
