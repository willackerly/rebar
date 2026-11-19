package checks

import (
	"fmt"
	"strings"

	"github.com/willackerly/rebar/cli/internal/contracts"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// check-contract-refs: every contract reference in source names a contract
// file that exists. With a contract namespace, references qualified with
// this repo's namespace are checked too and other namespaces are counted.
// Native port of scripts/check-contract-refs.sh.
func init() {
	register(&Check{Name: "check-contract-refs", Run: runContractRefs})
}

func runContractRefs(ctx *Context, args []string) int {
	scope := sourcescan.ScopeFor(ctx.Root)
	archDir := contracts.Dir(ctx.Root)
	namespace := ContractNamespace(ctx)
	if len(args) > 0 && args[0] != "" {
		archDir = args[0]
	}
	w := ctx.Stdout
	if !ctx.isDir(archDir) {
		fmt.Fprintf(w, "Architecture directory '%s' not found.\n", archDir)
		return 1
	}

	search := contracts.HasRef
	if namespace != "" {
		search = contracts.HasQualifiedRef
	}
	filter := sourcescan.Filter{Include: scope.ExtensionGlobs(), ExcludeDirs: scope.ExcludeDirs}
	broken, total, cross := 0, 0, 0
	for _, l := range sourcescan.Search(filter.Files(ctx.WorkDir(), []string{"."}), search) {
		// The script cuts its grep line (path:line:text) at colons.
		line := l.String()
		fields := strings.SplitN(line, ":", 3)
		file, lineno := fields[0], fields[1]
		if namespace != "" {
			var n int
			line, n = contracts.Localize(line, namespace)
			cross += n
		}
		ref := contracts.FirstRef(line)
		if ref == "" {
			continue
		}
		total++
		expected := archDir + "/CONTRACT-" + ref + ".md"
		if !ctx.isFile(expected) {
			fmt.Fprintf(w, "BROKEN: %s:%s references %s%s\n", file, lineno, contracts.Marker, ref)
			fmt.Fprintf(w, "        Expected: %s\n", expected)
			broken++
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Checked %d contract references, %d broken.\n", total, broken)
	if namespace != "" {
		fmt.Fprintf(w, "Namespace %s: %d cross-repo reference(s) to other namespaces not checked.\n", namespace, cross)
	}
	if broken > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Fix by either:")
		fmt.Fprintf(w, "  1. Creating the missing contract in %s/\n", archDir)
		fmt.Fprintln(w, "  2. Updating the code reference to the correct contract version")
		return 1
	}
	return 0
}
