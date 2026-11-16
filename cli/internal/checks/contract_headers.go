package checks

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/contracts"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// check-contract-headers: every source file names the contract it
// implements (a CONTRACT: or Architecture: line in its first 15 lines).
// Tier 2. Native port of scripts/check-contract-headers.sh.
func init() {
	register(&Check{Name: "check-contract-headers", Run: runContractHeaders})
}

// headerLines is how much of a file the header may be in.
const headerLines = 15

func runContractHeaders(ctx *Context, args []string) int {
	if Skip(ctx, 2, "check-contract-headers") {
		return 0
	}
	sourceDirs := config.RCValueOr(ctx.Root, "source_dirs", "", sourcescan.DefaultSourceDirs)
	extensions := config.RCValueOr(ctx.Root, "source_extensions", "CONTRACT_EXTENSIONS", ".go .ts .tsx .js .jsx .py .rs")

	dirs := args
	if len(dirs) == 0 {
		for _, d := range strings.Fields(sourceDirs) {
			if ctx.isDir(d) {
				dirs = append(dirs, d)
			}
		}
	}
	w := ctx.Stdout
	if len(dirs) == 0 {
		fmt.Fprintln(w, "No source directories found. Pass directories as arguments.")
		return 0
	}

	var names []string
	for _, ext := range strings.Fields(extensions) {
		names = append(names, "*."+strings.TrimPrefix(ext, "."))
	}
	missing, total := 0, 0
	for _, file := range findFiles(ctx, dirs, names) {
		if headerExempt(file.Display) {
			continue
		}
		total++
		if !hasContractHeader(file.Path) {
			fmt.Fprintf(w, "MISSING: %s\n", file.Display)
			missing++
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Scanned %d source files, %d missing contract headers.\n", total, missing)
	if missing == 0 {
		return 0
	}
	p := contracts.Marker
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Every source file must declare which contract it implements:")
	fmt.Fprintf(w, "  // %sC1-BLOBSTORE.2.1\n", p)
	fmt.Fprintf(w, "  // Architecture: %sS2-API-GATEWAY.1.0\n", p)
	if ns := ContractNamespace(ctx); ns != "" {
		fmt.Fprintf(w, "  // %s%s:C1-BLOBSTORE.2.1   (namespaced form, contract_namespace = %s)\n", p, ns, ns)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "See architecture/README.md for the full convention.")
	return 1
}

// findFiles lists what `find <operands> \( -name <names[0]> -o ... \) -type f`
// lists, operands in the order given and the files below each in lexical
// order (find lists directory order). Symbolic links, operands included,
// are neither listed nor followed (find -P); missing operands are skipped.
// Display is the path find prints: the operand, then the path below it
// (one slash between them).
func findFiles(ctx *Context, operands, names []string) []sourcescan.File {
	matches := func(name string) bool {
		for _, n := range names {
			if sourcescan.Match(n, name) {
				return true
			}
		}
		return false
	}
	var files []sourcescan.File
	for _, op := range operands {
		root := ctx.path(op)
		info, err := os.Lstat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if info.Mode().IsRegular() && matches(filepath.Base(root)) {
				files = append(files, sourcescan.File{Display: op, Path: root})
			}
			continue
		}
		prefix := strings.TrimRight(op, "/")
		if prefix == "" {
			prefix = "/" // find / prints /a, not //a
		}
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == root {
				if err != nil && d != nil && d.IsDir() && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || !matches(d.Name()) {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			display := prefix + "/" + filepath.ToSlash(rel)
			if prefix == "/" {
				display = "/" + filepath.ToSlash(rel)
			}
			files = append(files, sourcescan.File{Display: display, Path: p})
			return nil
		})
	}
	return files
}

// headerExempt reports whether the script skips a found file: tests,
// vendored and build output, generated code (matched on the whole path, as
// the script's case patterns are).
func headerExempt(p string) bool {
	for _, suffix := range []string{"_test.go", ".test.ts", ".test.tsx", ".test.js", ".spec.ts", ".spec.tsx", ".spec.js", ".pb.go", ".pb.ts"} {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	for _, part := range []string{"/vendor/", "/node_modules/", "/dist/", "/build/", "/.git/", "_generated", ".gen."} {
		if strings.Contains(p, part) {
			return true
		}
	}
	return false
}

// hasContractHeader reports whether the first headerLines lines of the file
// hold a contract reference marker or "Architecture:". An unreadable file
// has none.
func hasContractHeader(file string) bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	head := data
	for i, n := 0, 0; i < len(data); i++ {
		if data[i] == '\n' {
			if n++; n == headerLines {
				head = data[:i]
				break
			}
		}
	}
	return bytes.Contains(head, []byte(contracts.Marker)) || bytes.Contains(head, []byte("Architecture:"))
}
