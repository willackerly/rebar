package checks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/contracts"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// compute-registry: render <contracts_dir>/CONTRACT-REGISTRY.md from the
// contract files on disk (default), compare it with the committed file
// (--check), print it (--stdout), or print implementing-file counts and
// zombie/shadow contracts (--health). Native port of
// scripts/compute-registry.sh, whose output it reproduces byte for byte.
func init() {
	register(&Check{Name: "compute-registry", Run: runComputeRegistry})
}

// Defaults of the registry_* .rebarrc keys, as in the script.
const (
	defaultRegistryImplGlobs   = "*.go *.ts *.tsx *.js *.py *.rs *.java *.rb *.jsx *.sh bin/"
	defaultRegistryExcludeDirs = "node_modules vendor dist"
)

// registryRun is one compute-registry run: the settings it read and what it
// found.
type registryRun struct {
	ctx            *Context
	implCounts     bool
	scopePrefixes  []string
	purposeHeading *regexp.Regexp // nil: the heading ERE does not compile
	bytesPurpose   bool
	implGlobs      []string
	excludeDirs    []string
	contractsDir   string
	refPrefix      string
	archDir        string // absolute, as "$PROJECT_ROOT/$CONTRACTS_DIR"
	files          []string
	impl           *implSearch // built on first use
}

func runComputeRegistry(ctx *Context, args []string) int {
	r := newRegistry(ctx)
	mode := "generate"
	if len(args) > 0 && args[0] != "" {
		mode = args[0]
	}
	path := r.archDir + "/CONTRACT-REGISTRY.md"
	w := ctx.Stdout
	switch mode {
	case "--check":
		current, err := os.ReadFile(path)
		if err != nil || !isRegular(path) {
			fmt.Fprintf(w, "FAIL: %s does not exist. Generate it: rebar run compute-registry\n", path)
			return 1
		}
		var fresh bytes.Buffer
		r.generate(&fresh)
		if registryBody(current) == registryBody(fresh.Bytes()) {
			fmt.Fprintln(w, "OK: Registry is current.")
			return 0
		}
		fmt.Fprintln(w, "STALE: Registry needs regeneration: rebar run compute-registry")
		return 1
	case "--stdout":
		r.generate(w)
	case "--health":
		r.healthReport(w)
	default:
		var out bytes.Buffer
		r.generate(&out)
		if err := os.WriteFile(path, out.Bytes(), 0o666); err != nil {
			fmt.Fprintf(ctx.Stderr, "compute-registry: %s: %v\n", path, errText(err))
			return 1
		}
		fmt.Fprintf(w, "Generated: %s\n", path)
	}
	return 0
}

// errText is an error as the shell words it.
func errText(err error) string {
	switch {
	case os.IsNotExist(err):
		return "No such file or directory"
	case os.IsPermission(err):
		return "Permission denied"
	}
	return err.Error()
}

func newRegistry(ctx *Context) *registryRun {
	root := ctx.Root
	r := &registryRun{ctx: ctx}
	switch config.RCValueOr(root, "registry_impl_counts", "REBAR_REGISTRY_IMPL_COUNTS", "false") {
	case "true", "1", "yes", "on":
		r.implCounts = true
	}
	r.scopePrefixes = ifsFields(config.RCValueOr(root, "registry_scope_prefixes", "", ""))
	heading := `^## Purpose`
	if h := config.RCValueOr(root, "registry_purpose_heading", "", ""); h != "" {
		heading = "^## ([0-9]+[.][[:space:]]*)?(" + h + ")"
	}
	r.purposeHeading, _ = regexp.CompilePOSIX(heading)
	r.bytesPurpose = config.RCValueOr(root, "registry_purpose_style", "", "ellipsis") == "bytes"
	r.implGlobs = ifsFields(config.RCValueOr(root, "registry_impl_globs", "", defaultRegistryImplGlobs))
	r.excludeDirs = ifsFields(config.RCValueOr(root, "registry_exclude_dirs", "", defaultRegistryExcludeDirs))
	r.contractsDir = contracts.Dir(root)
	r.refPrefix = contracts.RefPrefix(ContractNamespace(ctx))
	r.archDir = root + "/" + r.contractsDir
	for _, name := range contracts.List(r.archDir) {
		if contracts.IsIndex(name) || contracts.IsCompanion(name) || !isRegular(r.archDir+"/"+name) {
			continue
		}
		r.files = append(r.files, name)
	}
	return r
}

// ifsFields splits s as the shell splits an unquoted expansion with the
// default IFS: on runs of spaces, tabs and newlines.
func ifsFields(s string) []string {
	return strings.FieldsFunc(s, func(c rune) bool { return c == ' ' || c == '\t' || c == '\n' })
}

// registryCategories are the sections of the registry, in order.
var registryCategories = []string{"Services", "Components", "Interfaces", "Protocols", "Other"}

// category files a contract ID by its first letter, after stripping the
// registry_scope_prefixes in order (SDK-S1-FOO is a Service with SDK-).
func (r *registryRun) category(id string) string {
	for _, p := range r.scopePrefixes {
		id = strings.TrimPrefix(id, p)
	}
	switch {
	case strings.HasPrefix(id, "S"):
		return "Services"
	case strings.HasPrefix(id, "C"):
		return "Components"
	case strings.HasPrefix(id, "I"):
		return "Interfaces"
	case strings.HasPrefix(id, "P"):
		return "Protocols"
	}
	return "Other"
}

// generate writes the registry (generate_registry).
func (r *registryRun) generate(w io.Writer) {
	rows := map[string][]string{}
	for _, name := range r.files {
		id, major, minor := contracts.ParseName(name)
		data, _ := os.ReadFile(r.archDir + "/" + name)
		purpose := r.purpose(data)
		if purpose == "" {
			purpose = "—"
		}
		var row string
		if r.implCounts {
			row = fmt.Sprintf("| %s | %s.%s | %s | %d | %s |", id, major, minor, contractStatus(data), r.count(id), purpose)
		} else {
			row = fmt.Sprintf("| %s | %s.%s | %s | %s |", id, major, minor, contractStatus(data), purpose)
		}
		cat := r.category(id)
		rows[cat] = append(rows[cat], row)
	}

	fmt.Fprint(w, "# Contract Registry\n\n"+
		"<!-- AUTO-GENERATED by rebar run compute-registry — do not edit manually -->\n"+
		"<!-- Regenerate: rebar run compute-registry -->\n\n")
	for _, cat := range registryCategories {
		if len(rows[cat]) == 0 {
			continue
		}
		fmt.Fprintf(w, "## %s\n\n", cat)
		if r.implCounts {
			fmt.Fprint(w, "| ID | Version | Status | Impl Files | Purpose |\n|----|---------|--------|------------|---------|\n")
		} else {
			fmt.Fprint(w, "| ID | Version | Status | Purpose |\n|----|---------|--------|---------|\n")
		}
		for _, row := range rows[cat] {
			fmt.Fprintln(w, row)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprint(w, "## Contract Files\n\n<!-- Plain list of filenames for grep-based tooling -->\n")
	for _, name := range r.files {
		fmt.Fprintf(w, "- %s\n", name)
	}
	fmt.Fprintln(w)
	if r.implCounts {
		r.health(w)
	}
}

// healthReport writes --health: the implementing-file count of every
// contract ID, then the Contract Health section (generate_health_report).
func (r *registryRun) healthReport(w io.Writer) {
	fmt.Fprint(w, "## Implementing Files\n\n| ID | Impl Files |\n|----|------------|\n")
	var rows []string
	for _, name := range r.files {
		id, _, _ := contracts.ParseName(name)
		rows = append(rows, fmt.Sprintf("| %s | %d |", id, r.count(id)))
	}
	for _, row := range sortedUnique(rows) {
		fmt.Fprintln(w, row)
	}
	fmt.Fprintln(w)
	r.health(w)
}

// sortedUnique is `LC_ALL=C sort -u`.
func sortedUnique(lines []string) []string {
	s := append([]string(nil), lines...)
	sort.Strings(s)
	out := s[:0]
	for i, l := range s {
		if i == 0 || l != s[i-1] {
			out = append(out, l)
		}
	}
	return out
}

// health writes the Contract Health section (generate_health): zombies,
// contract files without implementing code, and shadow references to
// contracts that have no file. Nothing when there are neither.
func (r *registryRun) health(w io.Writer) {
	var zombies []string
	known := map[string]bool{}
	todoPath := r.ctx.Root + "/TODO.md"
	var todo []byte
	if isRegular(todoPath) {
		todo, _ = os.ReadFile(todoPath)
	}
	for _, name := range r.files {
		id, _, _ := contracts.ParseName(name)
		known[id] = true
		if r.count(id) != 0 {
			continue
		}
		if todo != nil && greps(todo, id) {
			zombies = append(zombies, "- **"+id+"** — tracked in TODO.md")
		} else {
			zombies = append(zombies, "- **"+id+"** — ⚠ untracked, 0 implementing files")
		}
	}
	var shadows []string
	for _, id := range r.search().shadowIDs(r.refPrefix) {
		if !known[id] {
			shadows = append(shadows, "- **"+id+"** — referenced in source, no "+r.contractsDir+"/CONTRACT-"+id+".*.md found")
		}
	}
	if len(zombies) == 0 && len(shadows) == 0 {
		return
	}
	fmt.Fprint(w, "## Contract Health\n\n")
	if len(zombies) > 0 {
		fmt.Fprint(w, "### Zombies — contract files with no implementing code\n\n"+
			"(Deletion candidates unless intentionally pre-impl. Add a TODO.md entry\n"+
			"to suppress this warning while implementation is in flight.)\n\n")
		for _, z := range zombies {
			fmt.Fprintln(w, z)
		}
		fmt.Fprintln(w)
	}
	if len(shadows) > 0 {
		fmt.Fprint(w, "### Shadow refs — code references to nonexistent contracts\n\n"+
			"(Either write the missing contract file or fix the typo in source.)\n\n")
		for _, s := range shadows {
			fmt.Fprintln(w, s)
		}
		fmt.Fprintln(w)
	}
}

// registryBody is a registry's text for --check: its lines without the
// header comments that are not content (the timestamp older versions
// wrote, the regenerate hint, and the AUTO-GENERATED line, which named
// scripts/compute-registry.sh before rebar 4.0), each ending in a newline,
// as `grep -Ev '^<!-- (Generated|Regenerate|AUTO-GENERATED by )'` prints
// them.
func registryBody(data []byte) string {
	var b strings.Builder
	for _, line := range textLines(data) {
		if IsRegistryHeaderComment(line) {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// IsRegistryHeaderComment reports whether line is one of the registry's
// header comments that --check ignores: the timestamp older versions
// wrote, the regenerate hint, and the AUTO-GENERATED line (any generator
// name, so registries written by scripts/compute-registry.sh stay current).
func IsRegistryHeaderComment(line string) bool {
	for _, p := range []string{"<!-- Generated:", "<!-- Regenerate:", "<!-- AUTO-GENERATED by "} {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// textLines splits data into lines as grep and awk read them: on "\n",
// with a last line that has no newline counted, and nothing after a final
// newline.
func textLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	s := strings.TrimSuffix(string(data), "\n")
	return strings.Split(s, "\n")
}

// contractStatus is extract_status: the text after the last colon of the
// first line that starts with **Status:** or Status: (any case), without
// leading whitespace or asterisks, as xargs echoes it; "draft" without one.
//
// A file with a NUL in its first block is binary to grep, which then
// prints no line (GNU grep; BSD grep prints "Binary file … matches", which
// the script took for the status), so it is a draft.
func contractStatus(data []byte) string {
	if hasNULBlock(data) {
		return "draft"
	}
	for _, line := range textLines(data) {
		lower := asciiLower(line)
		if !strings.HasPrefix(lower, "**status:**") && !strings.HasPrefix(lower, "status:") {
			continue
		}
		// sed 's/.*:\s*//': up to the last colon and the whitespace after it
		// (GNU sed's \s; BSD sed reads \s as s).
		v := line[strings.LastIndexByte(line, ':')+1:]
		v = strings.TrimLeft(v, posixSpace)
		v = strings.ReplaceAll(v, "*", "")
		if s, _ := xargsEcho(v); s != "" {
			return s
		}
		return "draft"
	}
	return "draft"
}

// hasNULBlock reports whether grep takes a file for binary: a NUL in its
// first block (sourcescan.BinaryBlock bytes).
func hasNULBlock(data []byte) bool {
	head := data
	if len(head) > sourcescan.BinaryBlock {
		head = head[:sourcescan.BinaryBlock]
	}
	return bytes.IndexByte(head, 0) >= 0
}

// asciiLower lowercases ASCII letters only, as grep -i does in the C locale.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// xargsEcho is what `printf '%s' s | xargs` prints, without the newline:
// the words of s (split on blanks, with quotes and backslashes removed as
// xargs parses them) joined by single spaces. An unterminated quote makes
// xargs fail and print nothing: ok is false. Leading words that GNU echo
// takes for options (-n, -e, -E and combinations) are dropped.
func xargsEcho(s string) (out string, ok bool) {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
			continue
		}
		switch c {
		case ' ', '\t', '\n':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case '\'', '"':
			quote, inWord = c, true
		case '\\':
			if i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
			}
			inWord = true
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if quote != 0 {
		return "", false
	}
	if inWord {
		words = append(words, cur.String())
	}
	for len(words) > 0 && echoOption.MatchString(words[0]) {
		words = words[1:]
	}
	return strings.Join(words, " "), true
}

var echoOption = regexp.MustCompile(`^-[neE]+$`)

// purpose is extract_purpose: the first line that is neither blank nor a
// heading after a purpose heading (registry_purpose_heading, else
// "## Purpose"), with `|` escaped and cut to 77 characters plus "..."
// when longer than 80 (ellipsis style), or its first 80 bytes (bytes).
func (r *registryRun) purpose(data []byte) string {
	if r.purposeHeading == nil {
		return "" // awk rejects the heading pattern
	}
	var purpose string
	found := false
	for _, line := range textLines(data) {
		if r.purposeHeading.MatchString(line) {
			found = true
			continue
		}
		if found && line != "" && line[0] != '#' && strings.Trim(line, " \t") != "" {
			purpose = line
			break
		}
	}
	purpose = strings.ReplaceAll(purpose, "\x00", "") // the shell drops NULs
	if r.bytesPurpose {
		if len(purpose) > 80 {
			purpose = purpose[:80]
		}
		return purpose
	}
	purpose = strings.ReplaceAll(purpose, "|", `\|`)
	// Characters as bash counts them under C.UTF-8: a byte that is not
	// part of a valid sequence counts as one.
	if utf8.RuneCountInString(purpose) > 80 {
		n, i := 0, 0
		for i < len(purpose) && n < 77 {
			_, size := utf8.DecodeRuneInString(purpose[i:])
			i += size
			n++
		}
		purpose = purpose[:i] + "..."
	}
	return purpose
}

// count is count_implementations: the number of searched source files
// with a line matching the reference ERE for id (the reference prefix and
// the ID, whose dots match any character).
func (r *registryRun) count(id string) int {
	return r.search().count(r.refPrefix + id)
}

func (r *registryRun) search() *implSearch {
	if r.impl == nil {
		r.impl = newImplSearch(r)
	}
	return r.impl
}

// implSearch holds the lines with a contract reference marker of every
// file compute-registry searches, so each file is read once however many
// contracts are counted.
type implSearch struct {
	files  map[string][]string // path as listed → marker lines
	counts map[string]int
}

// newImplSearch reads the searched files (search_source). In a git work
// tree they are the tracked regular files that git grep searches for the
// script's pathspecs: the registry_impl_globs, minus the contracts
// directory and the registry_exclude_dirs anywhere. Outside git they are
// what the script's find lists: files named like a glob anywhere, and
// every file below a glob that ends in "/", pruning the contracts
// directory, .git, .claude and the excluded directories by name.
func newImplSearch(r *registryRun) *implSearch {
	s := &implSearch{files: map[string][]string{}, counts: map[string]int{}}
	root := r.ctx.Root
	if insideGitWorkTree(root) {
		for _, p := range r.gitFiles() {
			data, err := os.ReadFile(filepath.Join(root, p))
			if err != nil {
				continue
			}
			// git grep -I: a NUL in the first 8000 bytes makes a file binary.
			head := data
			if len(head) > 8000 {
				head = head[:8000]
			}
			if bytes.IndexByte(head, 0) >= 0 {
				continue
			}
			s.add(p, data)
		}
		return s
	}
	for _, p := range r.findFiles() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if hasNULBlock(data) {
			continue // grep -I
		}
		s.add(p, data)
	}
	return s
}

func (s *implSearch) add(path string, data []byte) {
	if _, ok := s.files[path]; ok {
		return
	}
	var lines []string
	for _, line := range textLines(data) {
		if strings.Contains(line, contracts.Marker) {
			lines = append(lines, line)
		}
	}
	s.files[path] = lines
}

// count returns the number of files with a line that matches the ERE (0
// when it does not compile, as grep then fails).
func (s *implSearch) count(ere string) int {
	if n, ok := s.counts[ere]; ok {
		return n
	}
	n := 0
	if re, err := regexp.CompilePOSIX(ere); err == nil {
		for _, lines := range s.files {
			for _, l := range lines {
				if re.MatchString(l) {
					n++
					break
				}
			}
		}
	}
	s.counts[ere] = n
	return n
}

// shadowIDs returns the contract IDs referenced as <prefix><ID>.<n>.<n> in
// the searched files, sorted and unique, as the script's
// `grep -Eho | sed | sort -u` lists them.
func (s *implSearch) shadowIDs(refPrefix string) []string {
	ref := regexp.MustCompilePOSIX(refPrefix + `[A-Za-z0-9_-]+\.[0-9]+\.[0-9]+`)
	lead := regexp.MustCompilePOSIX("^" + refPrefix)
	version := regexp.MustCompilePOSIX(`\.[0-9]+\.[0-9]+$`)
	var ids []string
	for _, lines := range s.files {
		for _, l := range lines {
			for _, m := range ref.FindAllString(l, -1) {
				m = lead.ReplaceAllString(m, "")
				ids = append(ids, version.ReplaceAllString(m, ""))
			}
		}
	}
	var out []string
	for _, id := range sortedUnique(ids) {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// insideGitWorkTree is `git -C root rev-parse --is-inside-work-tree`
// succeeding.
func insideGitWorkTree(root string) bool {
	return config.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree").Run() == nil
}

// gitFiles lists the files git grep searches with the script's pathspecs:
// tracked regular files (not symlinks or submodules), relative to root.
func (r *registryRun) gitFiles() []string {
	args := []string{"-C", r.ctx.Root, "ls-files", "-z", "-s", "--"}
	args = append(args, r.implGlobs...)
	args = append(args, ":(exclude)"+r.contractsDir+"/")
	for _, d := range r.excludeDirs {
		args = append(args, ":(exclude,glob)**/"+d+"/**")
	}
	out, err := config.Command("git", args...).Output()
	if err != nil {
		return nil
	}
	var files []string
	seen := map[string]bool{}
	for _, rec := range strings.Split(string(out), "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		if !ok || seen[p] {
			continue
		}
		if mode, _, _ := strings.Cut(meta, " "); mode != "100644" && mode != "100755" {
			continue
		}
		seen[p] = true
		files = append(files, p)
	}
	return files
}

// findFiles lists the files the script's find commands print outside git,
// as find prints them (so a file listed twice is one file).
func (r *registryRun) findFiles() []string {
	root := r.ctx.Root
	var names []string
	var dirs []string
	for _, g := range r.implGlobs {
		if strings.HasSuffix(g, "/") {
			dirs = append(dirs, strings.TrimSuffix(g, "/"))
		} else {
			names = append(names, g)
		}
	}
	contractsPath := root + "/" + r.contractsDir
	pruned := func(p, name string) bool {
		if p == contractsPath || name == ".git" || name == ".claude" {
			return true
		}
		for _, d := range r.excludeDirs {
			if sourcescan.Match(d, name) {
				return true
			}
		}
		return false
	}
	var files []string
	if len(names) > 0 {
		files = findWalk(root, pruned, func(name string) bool {
			for _, g := range names {
				if sourcescan.Match(g, name) {
					return true
				}
			}
			return false
		}, files)
	}
	for _, d := range dirs {
		if info, err := os.Stat(root + "/" + d); err == nil && info.IsDir() {
			files = findWalk(root+"/"+d, pruned, func(string) bool { return true }, files)
		}
	}
	return files
}

// findWalk appends what `find start \( <pruned> \) -prune -o -type f
// \( <admit> \) -print` prints: regular files (not followed through
// symlinks) below start whose name admit accepts, pruning entries
// (start included) for which pruned is true.
func findWalk(start string, pruned func(path, name string) bool, admit func(name string) bool, files []string) []string {
	info, err := os.Lstat(start)
	if err != nil || pruned(start, filepath.Base(start)) {
		return files
	}
	if info.Mode().IsRegular() {
		if admit(filepath.Base(start)) {
			files = append(files, start)
		}
		return files
	}
	if !info.IsDir() {
		return files
	}
	entries, err := os.ReadDir(start)
	if err != nil {
		return files
	}
	for _, e := range entries {
		files = findWalk(start+"/"+e.Name(), pruned, admit, files)
	}
	return files
}
