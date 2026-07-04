package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// doc.go implements `rebar doc <ref>` — one of the two resolvers of
// record for abstract cross-repo `rebar:` refs (the other is
// scripts/rebar-doc.sh; both implement conventions.md §Cross-Repo
// References and docs/v3-beta-plan.md D10 identically).
//
// Grammar:   rebar:<kind>/<name>   (the `rebar:` prefix is optional here)
//
// Kind table (conventions.md §Cross-Repo References):
//
//	practice/<name>   -> practices/<name>.md
//	script/<name>     -> scripts/<name>.sh
//	agents/<name>     -> agents/<name>.md
//	convention[/<s>]  -> conventions.md          (section is informational)
//	charter           -> CHARTER.md
//	doc/<name>        -> <name>.md, else docs/<name>.md
//	feedback/<name>   -> feedback/<name>.md, else feedback/processed/<name>.md
//
// Resolution order:
//
//	1. the current repo (nearest .git walking up from cwd — a
//	   vendored/synced copy)
//	2. $REBAR_ROOT
//	3. a discovered checkout (findRebarRoot covers 2+3: $REBAR_ROOT,
//	   exe-relative, cwd, ~/.rebar, ~/dev/rebar, ~/src/rebar,
//	   ~/code/rebar — a superset of the documented probes, same order)
//	4. otherwise: print the canonical upstream URL + `ask rebar <role>`
//	   hint, exit 4
//
// Exit codes: 0 resolved · 2 malformed ref / unknown kind · 4 unresolved
// locally. On success stdout carries one line: `<source>\t<path>` where
// <source> is local | REBAR_ROOT | checkout (or the file contents with
// --cat). On exit 4 stdout carries `upstream\t<url>` plus the ask hint.
// Stdout lines and exit codes match scripts/rebar-doc.sh exactly.

// rebarUpstreamBase is the canonical upstream prefix for unresolved refs.
const rebarUpstreamBase = "https://github.com/willackerly/rebar/blob/main/"

var docCat bool

var docCmd = &cobra.Command{
	Use:   "doc <ref>",
	Short: "Resolve a rebar:<kind>/<name> cross-repo reference to a local file",
	Long: `Resolve an abstract cross-repo reference (rebar:<kind>/<name>) to a
local file, per conventions.md §Cross-Repo References. The rebar: prefix
is optional.

  Kinds:
    rebar:practice/<name>    practices/<name>.md
    rebar:script/<name>      scripts/<name>.sh
    rebar:agents/<name>      agents/<name>.md
    rebar:convention[/<s>]   conventions.md (section informational)
    rebar:charter            CHARTER.md
    rebar:doc/<name>         <name>.md at repo root, else docs/<name>.md
    rebar:feedback/<name>    feedback/<name>.md, else feedback/processed/<name>.md

  Resolution order: current repo (vendored copy) -> $REBAR_ROOT ->
  discovered checkout (~/.rebar, ~/dev/rebar, ~/src/rebar, ~/code/rebar)
  -> canonical upstream URL + ask hint.

  Output: '<source>\t<path>' on stdout (source: local | REBAR_ROOT |
  checkout), or the file contents with --cat.

  Exit codes: 0 resolved, 2 malformed ref or unknown kind, 4 unresolved
  locally (prints 'upstream\t<url>' and an 'ask rebar <role>' hint).`,
	Example: `  rebar doc rebar:practice/inbox-watch
  rebar doc practice/inbox-watch --cat
  rebar doc convention/cross-repo-references`,
	// Arg-count errors are usage errors -> exit 2, so validate in RunE
	// instead of cobra.ExactArgs (which would exit 1 via main).
	Args: cobra.ArbitraryArgs,
	// Override the root hook: `rebar doc` must work anywhere — including
	// adopter repos and plain directories with no .rebar/ config.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: runDoc,
}

func init() {
	docCmd.Flags().BoolVar(&docCat, "cat", false, "print the resolved file's contents instead of '<source>\\t<path>'")
}

func runDoc(cmd *cobra.Command, args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(os.Stderr, "usage: rebar doc <rebar:kind/name> [--cat]")
		os.Exit(2)
	}

	ref := strings.TrimSpace(args[0])
	ref = strings.TrimPrefix(ref, "rebar:")
	displayRef := "rebar:" + ref

	kind, name := ref, ""
	if i := strings.Index(ref, "/"); i >= 0 {
		kind, name = ref[:i], ref[i+1:]
	}

	candidates, err := docCandidates(kind, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rebar doc: %s: %v\n", displayRef, err)
		os.Exit(2)
	}

	for _, root := range docRoots() {
		for _, rel := range candidates {
			p := filepath.Join(root.dir, filepath.FromSlash(rel))
			fi, statErr := os.Stat(p)
			if statErr != nil || !fi.Mode().IsRegular() {
				continue
			}
			if docCat {
				data, readErr := os.ReadFile(p)
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "rebar doc: reading %s: %v\n", p, readErr)
					os.Exit(1)
				}
				os.Stdout.Write(data)
			} else {
				fmt.Printf("%s\t%s\n", root.source, p)
			}
			return nil
		}
	}

	// Step 4: unresolved locally — canonical upstream URL + ask hint,
	// both on stdout (same lines as scripts/rebar-doc.sh).
	fmt.Printf("upstream\t%s%s\n", rebarUpstreamBase, candidates[0])
	fmt.Println("unresolved locally — ask rebar architect (or the relevant role) for questions")
	os.Exit(4)
	return nil
}

// docCandidates maps a parsed <kind>/<name> to its ordered repo-relative
// candidate paths, per the kind table in conventions.md §Cross-Repo
// References. A nil slice + error means the ref is malformed or the kind
// unknown (exit 2 territory).
func docCandidates(kind, name string) ([]string, error) {
	// needName also enforces the single-segment name rule (same as
	// rebar-doc.sh): a bare name, no '/' — which doubles as the
	// path-traversal guard.
	needName := func(rel ...string) ([]string, error) {
		if name == "" {
			return nil, fmt.Errorf("kind %q requires a name", kind)
		}
		if strings.Contains(name, "/") {
			return nil, fmt.Errorf("name must not contain '/'")
		}
		return rel, nil
	}

	switch kind {
	case "practice":
		return needName("practices/" + name + ".md")
	case "script":
		return needName("scripts/" + name + ".sh")
	case "agents":
		return needName("agents/" + name + ".md")
	case "convention":
		// Optional section is informational only — not validated.
		return []string{"conventions.md"}, nil
	case "charter":
		if name != "" {
			return nil, fmt.Errorf("'charter' takes no name")
		}
		return []string{"CHARTER.md"}, nil
	case "doc":
		return needName(name+".md", "docs/"+name+".md")
	case "feedback":
		return needName("feedback/"+name+".md", "feedback/processed/"+name+".md")
	default:
		return nil, fmt.Errorf("unknown kind %q (expected practice|script|agents|convention|charter|doc|feedback)", kind)
	}
}

// docRoot is one directory a ref may resolve against, tagged with the
// <source> label emitted on stdout.
type docRoot struct {
	source string // local | REBAR_ROOT | checkout
	dir    string // absolute path
}

// docRoots returns the resolution roots in spec order, deduplicated
// (running inside the rebar checkout itself would otherwise yield the
// same directory as both "local" and "checkout").
func docRoots() []docRoot {
	var roots []docRoot
	seen := map[string]bool{}
	add := func(source, dir string) {
		abs, err := filepath.Abs(dir)
		if err != nil || seen[abs] {
			return
		}
		seen[abs] = true
		roots = append(roots, docRoot{source: source, dir: abs})
	}

	// 1. The current repo — nearest .git walking up from cwd (covers
	//    vendored/synced copies in adopter repos).
	if cwd, err := os.Getwd(); err == nil {
		if repo := nearestGitRoot(cwd); repo != "" {
			add("local", repo)
		}
	}

	// 2–3. $REBAR_ROOT, then a discovered checkout. findRebarRoot
	// (init.go) probes in exactly that order and returns $REBAR_ROOT
	// verbatim when it matched.
	if root := findRebarRoot(); root != "" {
		source := "checkout"
		if env := os.Getenv("REBAR_ROOT"); env != "" && env == root {
			source = "REBAR_ROOT"
		}
		add(source, root)
	}

	return roots
}

// nearestGitRoot walks up from dir to the nearest directory containing a
// .git entry (dir or file — worktrees use a file). Empty if none.
func nearestGitRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
