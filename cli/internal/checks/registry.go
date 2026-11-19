package checks

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/contracts"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// check-registry (deprecated in favour of compute-registry --check): every
// contract file is listed in CONTRACT-REGISTRY.md, and every contract has
// implementing code or is tracked in TODO.md.
// Native port of scripts/check-registry.sh.
func init() {
	register(&Check{Name: "check-registry", Run: runRegistry})
}

// registrySourceGlobs are the files check-registry.sh searches for
// references (not the configured source_extensions); everything under bin/
// is searched too.
var registrySourceGlobs = []string{"*.go", "*.ts", "*.tsx", "*.js", "*.jsx", "*.py", "*.rs", "*.mjs", "*.cjs", "*.sh"}

func runRegistry(ctx *Context, args []string) int {
	archDir := contracts.Dir(ctx.Root)
	refPrefix := contracts.RefPrefix(ContractNamespace(ctx))
	excludeDirs := sourcescan.ScopeFor(ctx.Root).ExcludeDirs
	if len(args) > 0 && args[0] != "" {
		archDir = args[0]
	}
	archDir = contracts.NormDir(archDir)
	registry := archDir + "/CONTRACT-REGISTRY.md"
	w := ctx.Stdout
	if !ctx.isDir(archDir) {
		fmt.Fprintf(w, "Architecture directory '%s' not found.\n", archDir)
		return 1
	}

	var files []string
	for _, name := range contracts.List(ctx.path(archDir)) {
		if contracts.IsIndex(name) || !ctx.isFile(archDir+"/"+name) {
			continue
		}
		files = append(files, name)
	}

	errors := 0
	fmt.Fprintln(w, "=== Contract files vs registry ===")
	if ctx.isFile(registry) {
		reg, _ := os.ReadFile(ctx.path(registry))
		for _, name := range files {
			if !greps(reg, name) {
				fmt.Fprintf(w, "NOT IN REGISTRY: %s/%s\n", archDir, name)
				errors++
			}
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Orphaned contracts (no implementing code) ===")
	var refs []string // candidate reference lines, unless none is needed
	if len(files) > 0 {
		refs = registryRefLines(ctx, excludeDirs, "./"+archDir+"/")
	}
	var todo []byte
	if ctx.isFile("TODO.md") {
		todo, _ = os.ReadFile(ctx.path("TODO.md"))
	}
	for _, name := range files {
		id := strings.TrimPrefix(strings.TrimSuffix(name, ".md"), "CONTRACT-")
		if referenced(refs, refPrefix+id) {
			continue
		}
		if todo != nil && greps(todo, id) {
			fmt.Fprintf(w, "ORPHAN (tracked in TODO.md): %s/%s\n", archDir, name)
			continue
		}
		fmt.Fprintf(w, "ORPHAN (untracked): %s/%s — 0 implementing files\n", archDir, name)
		fmt.Fprintf(w, "  Either add implementing code with %s%s header,\n", contracts.Marker, id)
		fmt.Fprintln(w, "  or track in TODO.md if implementation is planned.")
		errors++
	}

	fmt.Fprintln(w)
	if errors > 0 {
		fmt.Fprintf(w, "FAIL: %d inconsistencies found.\n", errors)
		return 1
	}
	fmt.Fprintln(w, "OK: Registry and contracts are consistent.")
	return 0
}

// registryRefLines returns the grep lines (path:line:text) holding the
// reference marker in the files check-registry.sh searches: source files
// below ".", and every file below ./bin, except those whose line starts
// with skipPrefix (the contracts directory, by path prefix).
func registryRefLines(ctx *Context, excludeDirs []string, skipPrefix string) []string {
	has := func(text string) bool { return strings.Contains(text, contracts.Marker) }
	files := sourcescan.Filter{Include: registrySourceGlobs, ExcludeDirs: excludeDirs}.Files(ctx.WorkDir(), []string{"."})
	if ctx.isDir("bin") {
		files = append(files, sourcescan.Filter{ExcludeDirs: excludeDirs}.Files(ctx.WorkDir(), []string{"./bin"})...)
	}
	var lines []string
	for _, l := range sourcescan.Search(files, has) {
		if s := l.String(); !strings.HasPrefix(s, skipPrefix) {
			lines = append(lines, l.Text)
		}
	}
	return lines
}

// referenced reports whether a line matches the ERE the script builds from
// the reference prefix and a contract ID (whose dots match any character,
// as in the script). An ID that does not compile matches nothing, as grep
// then fails.
func referenced(lines []string, ere string) bool {
	re, err := regexp.CompilePOSIX(ere)
	if err != nil {
		return false
	}
	for _, l := range lines {
		if re.MatchString(l) {
			return true
		}
	}
	return false
}

// greps is the script's `grep -q <name> <file>`: name is a basic regular
// expression, in which a contract file name's dots match any character.
// Other characters are taken literally (contract names do not use BRE
// syntax).
func greps(data []byte, name string) bool {
	expr := strings.ReplaceAll(regexp.QuoteMeta(name), `\.`, ".")
	re, err := regexp.Compile(expr)
	return err == nil && re.Match(data)
}
