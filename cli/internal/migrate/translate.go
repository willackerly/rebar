package migrate

import (
	"fmt"
	"regexp"
	"strings"
)

// MetricDef is a declarative metric, as the `metric.<key>.*` fields of
// .rebarrc describe it (see check-ground-truth.sh).
type MetricDef struct {
	Type         string // "files" or "dirs"
	Paths        []string
	Names        []string
	ExcludeNames []string
	ExcludeDirs  []string
	MinDepth     string
	MaxDepth     string
	Contains     string
	Distinct     string
	// Notes describe where the translation is looser than the original,
	// for the report. The value check still has to pass.
	Notes []string
}

// Settings renders d as metric.<key>.* lines, leaving out defaults.
func (d MetricDef) Settings(key, source string) []Setting {
	var out []Setting
	add := func(field, v string) {
		if v != "" {
			out = append(out, Setting{Key: "metric." + key + "." + field, Value: v, Source: source})
		}
	}
	if d.Type == "dirs" {
		add("type", "dirs")
	}
	if !(len(d.Paths) == 1 && d.Paths[0] == ".") {
		add("paths", strings.Join(d.Paths, " "))
	}
	add("min_depth", d.MinDepth)
	add("max_depth", d.MaxDepth)
	add("names", strings.Join(d.Names, " "))
	add("exclude_names", strings.Join(d.ExcludeNames, " "))
	add("exclude_dirs", strings.Join(d.ExcludeDirs, " "))
	add("contains", d.Contains)
	add("distinct", d.Distinct)
	return out
}

// valid checks that every field can be written to .rebarrc unchanged: list
// items are space-separated, and `#` would start a comment.
func (d MetricDef) valid() error {
	lists := map[string][]string{"paths": d.Paths, "names": d.Names, "exclude_names": d.ExcludeNames, "exclude_dirs": d.ExcludeDirs}
	for field, items := range lists {
		for _, it := range items {
			if it == "" || strings.ContainsAny(it, " \t#\n") {
				return fmt.Errorf("%s value %q cannot be written to .rebarrc", field, it)
			}
		}
	}
	for field, v := range map[string]string{"contains": d.Contains, "distinct": d.Distinct} {
		if !ValidValue(v) {
			return fmt.Errorf("%s value %q contains '#', which .rebarrc reads as a comment", field, v)
		}
	}
	return nil
}

// Translation is the outcome for one metric of a legacy script.
type Translation struct {
	Key string
	Def *MetricDef // nil when Err is set
	Err error
}

// Translator reads a legacy check-ground-truth.sh.
type Translator struct {
	src   string
	funcs map[string]string
	vars  map[string]string // root variables → rootMarker
	// cdElsewhere is set when compute_metrics changes into a directory other
	// than the root: relative paths then do not mean root-relative.
	cdElsewhere bool
}

// NewTranslator parses the functions and root variables of a script.
func NewTranslator(src string) *Translator {
	t := &Translator{src: src, funcs: parseFunctions(src), vars: rootVars(src)}
	return t
}

var (
	funcStartRe = regexp.MustCompile(`^(\s*)(?:function\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{(.*)$`)
	funcEndRe   = regexp.MustCompile(`^\s*\}\s*(?:#.*)?$`)
	assignRe    = regexp.MustCompile(`^\s*(?:export\s+|readonly\s+|local\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
)

// parseFunctions returns the body of each `name() {` function, ending at
// the first `}` line at the same indentation (or on the same line).
func parseFunctions(src string) map[string]string {
	funcs := map[string]string{}
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		m := funcStartRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		indent, name, rest := m[1], m[2], strings.TrimSpace(m[3])
		if strings.HasSuffix(rest, "}") {
			// One-liner: f() { cmd; }
			body := strings.TrimSpace(strings.TrimSuffix(rest, "}"))
			funcs[name] = strings.TrimSuffix(body, ";")
			continue
		}
		var body []string
		if rest != "" {
			body = append(body, rest)
		}
		for j := i + 1; j < len(lines); j++ {
			if funcEndRe.MatchString(lines[j]) && leadingWS(lines[j]) == indent {
				funcs[name] = strings.Join(body, "\n")
				i = j
				break
			}
			body = append(body, lines[j])
		}
	}
	return funcs
}

func leadingWS(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " \t"))]
}

// rootVars finds the variables that hold the project root: the usual names,
// plus any assignment computed from `..` of the script directory, git's
// top level or REBAR_PROJECT_ROOT. A script-directory variable maps to
// <root>/scripts so `$SCRIPT_DIR/../x` resolves.
func rootVars(src string) map[string]string {
	vars := map[string]string{"REBAR_PROJECT_ROOT": rootMarker}
	for _, line := range strings.Split(src, "\n") {
		m := assignRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, val := m[1], m[2]
		switch {
		case strings.Contains(val, "REBAR_PROJECT_ROOT") || strings.Contains(val, "show-toplevel"):
			vars[name] = rootMarker
		case strings.Contains(val, "..") && (strings.Contains(val, "dirname") || strings.Contains(val, "pwd")):
			vars[name] = rootMarker
		case strings.Contains(val, "dirname") && strings.Contains(val, "pwd"):
			vars[name] = rootMarker + "/scripts"
		}
	}
	return vars
}

// logicalLines joins continuation lines and lines inside open quotes or
// $( … ), drops comments and blank lines, and trims each line.
func logicalLines(body string) []string {
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if cur.Len() == 0 && strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.HasSuffix(line, "\\") {
			cur.WriteString(strings.TrimSuffix(line, "\\"))
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(line)
		if !balanced(cur.String()) {
			cur.WriteString("\n")
			continue
		}
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

// balanced reports whether s closes every quote and parenthesis it opens.
func balanced(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return false
			}
			i += j + 1
		case '"':
			closed := false
			for i++; i < len(s); i++ {
				if s[i] == '\\' {
					i++
					continue
				}
				if s[i] == '$' && i+1 < len(s) && s[i+1] == '(' {
					depth++
					i++
					continue
				}
				if s[i] == ')' && depth > 0 {
					depth--
					continue
				}
				if s[i] == '"' {
					closed = true
					break
				}
			}
			if !closed {
				return false
			}
		case '(':
			depth++
		case ')':
			depth--
		case '#':
			// A comment runs to the end of the line.
			if i == 0 || s[i-1] == ' ' || s[i-1] == '\t' {
				if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
					i += j
				} else {
					i = len(s)
				}
			}
		}
	}
	return depth <= 0
}

var (
	echoMetricRe = regexp.MustCompile(`^echo\s+(?:-e\s+)?["']?([A-Za-z0-9_.-]+)=(.*?)["']?\s*$`)
	varRefRe     = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)
	cdRe         = regexp.MustCompile(`^cd\s+(.+?)(\s*\|\|.*)?$`)
)

// Translate returns a translation for each metric compute_metrics() emits,
// in the order it emits them. A script without compute_metrics() yields none.
func (t *Translator) Translate() []Translation {
	body, ok := t.funcs["compute_metrics"]
	if !ok {
		return nil
	}
	assigned := map[string]string{} // var → command it was assigned from
	var out []Translation
	for _, line := range logicalLines(body) {
		if m := cdRe.FindStringSubmatch(line); m != nil {
			toks, err := lex(m[1], t.vars)
			if err == nil && len(toks) == 1 && toks[0].kind == tWord && toks[0].text == rootMarker {
				continue
			}
			t.cdElsewhere = true
			continue
		}
		if m := echoMetricRe.FindStringSubmatch(line); m != nil {
			key, val := m[1], strings.TrimSpace(m[2])
			def, err := t.translateValue(val, assigned)
			if err == nil {
				err = def.valid()
			}
			if err != nil {
				out = append(out, Translation{Key: key, Err: err})
			} else {
				out = append(out, Translation{Key: key, Def: def})
			}
			continue
		}
		if m := assignRe.FindStringSubmatch(line); m != nil {
			v := strings.Trim(strings.TrimSpace(m[2]), `"`)
			if strings.HasPrefix(v, "$(") {
				assigned[m[1]] = v
			}
		}
	}
	return out
}

// translateValue handles the value part of `echo "key=VALUE"`: $(cmd), or a
// variable assigned from $(cmd) earlier in compute_metrics.
func (t *Translator) translateValue(val string, assigned map[string]string) (*MetricDef, error) {
	if m := varRefRe.FindStringSubmatch(val); m != nil {
		v, ok := assigned[m[1]]
		if !ok {
			return nil, fmt.Errorf("value comes from $%s, which is not assigned from a command in compute_metrics", m[1])
		}
		val = v
	}
	if !strings.HasPrefix(val, "$(") || matchParen(val, 1) != len(val)-1 {
		return nil, fmt.Errorf("value %q is not a single $(…) command", val)
	}
	return t.translateCommand(val[2:len(val)-1], t.vars, 0)
}

// translateCommand translates a counting command: a helper call or a pipeline.
func (t *Translator) translateCommand(cmd string, vars map[string]string, depth int) (*MetricDef, error) {
	if depth > 3 {
		return nil, fmt.Errorf("helper functions nest too deeply")
	}
	toks, err := lex(cmd, vars)
	if err != nil {
		return nil, err
	}
	if len(toks) > 0 && toks[0].kind == tWord {
		if body, ok := t.funcs[toks[0].text]; ok {
			args, err := words(toks[1:])
			if err != nil {
				return nil, fmt.Errorf("helper %s: %v", toks[0].text, err)
			}
			bound := map[string]string{}
			for k, v := range vars {
				bound[k] = v
			}
			for i, a := range args {
				bound[fmt.Sprint(i+1)] = a
			}
			def, err := t.translateFunction(body, bound, depth+1)
			if err != nil {
				return nil, fmt.Errorf("helper %s: %v", toks[0].text, err)
			}
			return def, nil
		}
	}
	return t.translatePipeline(toks)
}

var (
	countLoopRe = regexp.MustCompile(`^(?:local\s+)?(\w+)=0; for (\w+) in ([^;]+); do (.+); done; echo "?\$\{?(\w+)\}?"?$`)
	loopTestRe  = regexp.MustCompile(`^(?:\[\[?\s+-([df])\s+(\S+)\s+\]\]?\s*&&\s*(.+)|if\s+\[\[?\s+-([df])\s+(\S+)\s+\]\]?;?\s*then\s+(.+?);?\s*fi)$`)
	localDeclRe = regexp.MustCompile(`^local\s+[A-Za-z_][A-Za-z0-9_ ]*$`)
	doSemiRe    = regexp.MustCompile(`\bdo;\s*`)
	thenSemiRe  = regexp.MustCompile(`\bthen;\s*`)
	semiFiRe    = regexp.MustCompile(`;\s*fi\b`)
	envPrefixRe = regexp.MustCompile(`^[A-Z_]+=`)
	incrementRe = regexp.MustCompile(`^(?:(\w+)=\$\(\(\s*(\w+)\s*\+\s*1\s*\)\)|\(\(\s*(\w+)\s*(?:\+\+|\+=\s*1)\s*\)\)|let\s+"?(\w+)\+\+"?)$`)
)

// translateFunction translates a helper's body: one counting pipeline, or a
// loop that counts which of a fixed list of files or directories exist.
func (t *Translator) translateFunction(body string, vars map[string]string, depth int) (*MetricDef, error) {
	var lines []string
	for _, l := range logicalLines(body) {
		l = strings.TrimSuffix(strings.TrimSpace(l), ";")
		if l == "" || localDeclRe.MatchString(l) {
			continue
		}
		lines = append(lines, l)
	}
	if len(lines) == 1 {
		return t.translateCommand(lines[0], vars, depth)
	}
	joined := strings.Join(lines, "; ")
	joined = doSemiRe.ReplaceAllString(joined, "do ")
	joined = thenSemiRe.ReplaceAllString(joined, "then ")
	joined = semiFiRe.ReplaceAllString(joined, " fi")
	if m := countLoopRe.FindStringSubmatch(joined); m != nil && m[1] == m[5] {
		return t.translateExistsLoop(m[1], m[2], m[3], m[4], vars)
	}
	return nil, fmt.Errorf("body is not a single counting pipeline or an existence-count loop")
}

// translateExistsLoop handles
//
//	count=0; for x in a b c; do [ -d "$ROOT/dir/$x" ] && count=$((count + 1)); done; echo "$count"
//
// as type dirs (or files for -f), paths dir, depth 1, names a b c.
func (t *Translator) translateExistsLoop(counter, loopVar, list, test string, vars map[string]string) (*MetricDef, error) {
	m := loopTestRe.FindStringSubmatch(strings.TrimSpace(test))
	if m == nil {
		return nil, fmt.Errorf("loop body %q is not `[ -d|-f PATH ] && count=$((count + 1))`", test)
	}
	kind, pathWord, incr := m[1], m[2], m[3]
	if kind == "" {
		kind, pathWord, incr = m[4], m[5], m[6]
	}
	im := incrementRe.FindStringSubmatch(strings.TrimSpace(incr))
	if im == nil {
		return nil, fmt.Errorf("loop does not just increment %s", counter)
	}
	for _, v := range im[1:] {
		if v != "" && v != counter {
			return nil, fmt.Errorf("loop increments %s, not %s", v, counter)
		}
	}
	listToks, err := lex(list, vars)
	if err != nil {
		return nil, err
	}
	items, err := words(listToks)
	if err != nil {
		return nil, err
	}
	dir := ""
	var names []string
	seen := map[string]bool{}
	for _, it := range items {
		if strings.ContainsAny(it, "*?[") {
			return nil, fmt.Errorf("loop item %q is a glob", it)
		}
		if seen[it] {
			return nil, fmt.Errorf("loop item %q is listed twice", it)
		}
		seen[it] = true
		b := map[string]string{}
		for k, v := range vars {
			b[k] = v
		}
		b[loopVar] = it
		pt, err := lex(pathWord, b)
		if err != nil {
			return nil, err
		}
		ws, err := words(pt)
		if err != nil || len(ws) != 1 {
			return nil, fmt.Errorf("loop path %q is not a single word", pathWord)
		}
		rel, err := t.rel(ws[0])
		if err != nil {
			return nil, err
		}
		d, name := splitDir(rel)
		if name == "" || name == "." {
			return nil, fmt.Errorf("loop path %q has no file name", pathWord)
		}
		if dir == "" {
			dir = d
		} else if d != dir {
			return nil, fmt.Errorf("loop paths are in different directories (%s, %s)", dir, d)
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("empty loop list")
	}
	def := &MetricDef{Type: "files", Paths: []string{dir}, MaxDepth: "1", Names: names}
	if kind == "d" {
		def.Type = "dirs"
		def.MinDepth = "1"
	}
	return def, nil
}

func splitDir(rel string) (string, string) {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return ".", rel
	}
	return rel[:i], rel[i+1:]
}

func (t *Translator) rel(p string) (string, error) {
	if t.cdElsewhere && !strings.HasPrefix(p, rootMarker) {
		return "", fmt.Errorf("relative path %q after a cd away from the root", p)
	}
	return relPath(p)
}

// stream is what flows between pipeline stages.
type stream int

const (
	streamPaths stream = iota
	streamNames        // basenames
)

// translatePipeline handles `SOURCE [| filters] | wc -l [| tr -d ' ']`.
func (t *Translator) translatePipeline(toks []token) (*MetricDef, error) {
	stages, err := splitPipeline(toks)
	if err != nil {
		return nil, err
	}
	sw := make([][]string, len(stages))
	for i, st := range stages {
		w, err := words(st)
		if err != nil {
			return nil, err
		}
		// LC_ALL=C sort -u: an environment prefix doesn't change counting.
		for len(w) > 1 && envPrefixRe.MatchString(w[0]) {
			w = w[1:]
		}
		sw[i] = w
	}

	// The tail: wc -l, optionally trimmed.
	last := len(sw) - 1
	for last > 0 && isTrim(sw[last]) {
		last--
	}
	if last < 1 || !(len(sw[last]) == 2 && sw[last][0] == "wc" && sw[last][1] == "-l") {
		return nil, fmt.Errorf("pipeline does not end in `wc -l`")
	}

	var def *MetricDef
	var out stream
	switch sw[0][0] {
	case "find":
		def, out, err = t.translateFind(sw[0][1:])
	case "grep":
		def, err = t.translateGrepR(sw[0][1:])
	case "ls":
		def, err = t.translateLs(sw[0][1:])
	default:
		return nil, fmt.Errorf("unsupported command %q (find, grep -rl and ls are understood)", sw[0][0])
	}
	if err != nil {
		return nil, err
	}

	// Filters between the source and wc -l.
	sedRE := ""
	dedupe := false
	for _, w := range sw[1:last] {
		switch {
		case isBasename(w):
			if out == streamNames {
				return nil, fmt.Errorf("basename taken twice")
			}
			out = streamNames
		case len(w) >= 1 && w[0] == "sed":
			if out != streamNames || sedRE != "" || dedupe {
				return nil, fmt.Errorf("unsupported sed stage %q", strings.Join(w, " "))
			}
			re, err := sedCapture(w[1:])
			if err != nil {
				return nil, err
			}
			sedRE = re
		case isSortUnique(w):
			dedupe = true
		case len(w) == 1 && (w[0] == "sort" || w[0] == "cat"):
		default:
			return nil, fmt.Errorf("unsupported pipeline stage %q", strings.Join(w, " "))
		}
	}
	if out == streamNames && dedupe {
		def.Distinct = sedRE
		if def.Distinct == "" {
			def.Distinct = "^(.*)$"
		}
	}
	// Without dedupe the count is one per path whatever the names become;
	// with dedupe over full paths it is a no-op (find prints a path once).
	return def, nil
}

// isTrim matches the stages that only strip wc's padding: tr -d ' ',
// tr -d '[:space:]', xargs, awk '{print $1}'.
func isTrim(w []string) bool {
	switch {
	case len(w) == 1 && w[0] == "xargs":
		return true
	case len(w) == 2 && w[0] == "awk" && strings.Join(strings.Fields(w[1]), " ") == "{print $1}":
		return true
	case len(w) == 3 && w[0] == "tr" && w[1] == "-d":
		set := w[2]
		for _, cls := range []string{"[:space:]", "[:blank:]", `\n`, `\t`} {
			set = strings.ReplaceAll(set, cls, "")
		}
		return strings.Trim(set, " \t\n") == ""
	}
	return false
}

// isBasename matches the stages that reduce paths to file names.
func isBasename(w []string) bool {
	switch strings.Join(w, " ") {
	case "xargs basename", "xargs -n1 basename", "xargs -n 1 basename", "xargs -I{} basename {}",
		"sed s|.*/||", "sed s#.*/##", "sed s,.*/,,", "awk -F/ {print $NF}", "awk -F / {print $NF}":
		return true
	}
	return false
}

func isSortUnique(w []string) bool {
	s := strings.Join(w, " ")
	return s == "sort -u" || s == "uniq" || s == "sort | uniq"
}

// sedCapture accepts `sed [-E|-r] 's/RE/\1/'` and returns RE as an ERE.
func sedCapture(args []string) (string, error) {
	ere := false
	for len(args) > 1 {
		switch args[0] {
		case "-E", "-r":
			ere = true
		case "-e":
		default:
			return "", fmt.Errorf("unsupported sed option %q", args[0])
		}
		args = args[1:]
	}
	if len(args) != 1 || len(args[0]) < 4 || args[0][0] != 's' {
		return "", fmt.Errorf("sed stage is not a single s command")
	}
	expr := args[0]
	delim := expr[1]
	parts := splitSed(expr[2:], delim)
	if len(parts) != 3 || parts[1] != `\1` || (parts[2] != "" && parts[2] != "g") {
		return "", fmt.Errorf("sed stage is not s%cRE%c\\1%c", delim, delim, delim)
	}
	re := parts[0]
	if !ere {
		re = breToERE(re)
	}
	if !strings.Contains(re, "(") {
		return "", fmt.Errorf("sed expression has no capture group")
	}
	return re, nil
}

// splitSed splits the body of an s command on unescaped delim.
func splitSed(s string, delim byte) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			if s[i+1] == delim {
				cur.WriteByte(delim)
			} else {
				cur.WriteByte('\\')
				cur.WriteByte(s[i+1])
			}
			i++
			continue
		}
		if s[i] == delim {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	return append(parts, cur.String())
}

// translateFind translates find arguments.
func (t *Translator) translateFind(args []string) (*MetricDef, stream, error) {
	def := &MetricDef{}
	i := 0
	for i < len(args) && !strings.HasPrefix(args[i], "-") && args[i] != "(" && args[i] != "!" {
		p, err := t.rel(args[i])
		if err != nil {
			return nil, 0, err
		}
		def.Paths = append(def.Paths, p)
		i++
	}
	if len(def.Paths) == 0 {
		def.Paths = []string{"."}
	}
	p := &findParser{args: args[i:]}
	expr, err := p.parse()
	if err != nil {
		return nil, 0, err
	}
	out, err := def.fromFindExpr(expr)
	if err != nil {
		return nil, 0, err
	}
	if def.Type == "" {
		def.Type = "files"
		def.Notes = append(def.Notes, "find had no -type; counted as files (a directory matching the names would have counted before)")
	}
	return def, out, nil
}

// find expression tree.
type fnode struct {
	op   string // "and", "or", "not", or a primary like "-name"
	kids []*fnode
	args []string
}

type findParser struct {
	args []string
	pos  int
}

func (p *findParser) peek() string {
	if p.pos < len(p.args) {
		return p.args[p.pos]
	}
	return ""
}

func (p *findParser) parse() (*fnode, error) {
	if len(p.args) == 0 {
		return &fnode{op: "and"}, nil
	}
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.args) {
		return nil, fmt.Errorf("unexpected find argument %q", p.peek())
	}
	return n, nil
}

func (p *findParser) parseOr() (*fnode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	kids := []*fnode{left}
	for p.peek() == "-o" || p.peek() == "-or" {
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		kids = append(kids, right)
	}
	if len(kids) == 1 {
		return left, nil
	}
	return &fnode{op: "or", kids: kids}, nil
}

func (p *findParser) parseAnd() (*fnode, error) {
	var kids []*fnode
	for {
		switch p.peek() {
		case "", "-o", "-or", ")":
			if len(kids) == 0 {
				return nil, fmt.Errorf("empty find expression")
			}
			if len(kids) == 1 {
				return kids[0], nil
			}
			return &fnode{op: "and", kids: kids}, nil
		case "-a", "-and":
			p.pos++
			continue
		}
		n, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		kids = append(kids, n)
	}
}

func (p *findParser) parseUnary() (*fnode, error) {
	switch p.peek() {
	case "!", "-not":
		p.pos++
		k, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &fnode{op: "not", kids: []*fnode{k}}, nil
	case "(":
		p.pos++
		n, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("unbalanced ( in find expression")
		}
		p.pos++
		return n, nil
	}
	return p.parsePrimary()
}

func (p *findParser) parsePrimary() (*fnode, error) {
	tok := p.peek()
	p.pos++
	takeArg := func() (string, error) {
		if p.pos >= len(p.args) {
			return "", fmt.Errorf("%s needs an argument", tok)
		}
		a := p.args[p.pos]
		p.pos++
		return a, nil
	}
	switch tok {
	case "-name", "-path", "-wholename", "-type", "-maxdepth", "-mindepth", "-printf", "-iname", "-ipath", "-regex":
		a, err := takeArg()
		if err != nil {
			return nil, err
		}
		return &fnode{op: tok, args: []string{a}}, nil
	case "-print", "-prune", "-print0", "-true":
		return &fnode{op: tok}, nil
	case "-exec", "-execdir":
		var a []string
		for p.pos < len(p.args) {
			x := p.args[p.pos]
			p.pos++
			if x == ";" || x == "+" {
				return &fnode{op: tok, args: append(a, x)}, nil
			}
			a = append(a, x)
		}
		return nil, fmt.Errorf("unterminated %s", tok)
	}
	return nil, fmt.Errorf("unsupported find primary %q", tok)
}

// fromFindExpr maps a find expression onto d and returns what the find prints.
func (d *MetricDef) fromFindExpr(e *fnode) (stream, error) {
	// -maxdepth / -mindepth are global options wherever they appear.
	e = d.takeDepths(e)

	// NAME -prune -o MAIN: the pruned names are excluded directories.
	if e.op == "or" && len(e.kids) == 2 && containsPrune(e.kids[0]) {
		if err := d.prunedDirs(e.kids[0]); err != nil {
			return 0, err
		}
		e = e.kids[1]
	}

	// A bare top-level `-name a -o -name b` (the old template's example).
	if e.op == "or" {
		names, ok := namesOf(e)
		if !ok {
			return 0, fmt.Errorf("find -o combines tests other than -name; not expressible")
		}
		d.Names = names
		return streamPaths, nil
	}

	terms := []*fnode{e}
	if e.op == "and" {
		terms = e.kids
	}
	out := streamPaths
	for idx, n := range terms {
		lastTerm := idx == len(terms)-1
		switch n.op {
		case "-type":
			if d.Type != "" {
				return 0, fmt.Errorf("more than one -type")
			}
			switch n.args[0] {
			case "f":
				d.Type = "files"
			case "d":
				d.Type = "dirs"
			default:
				return 0, fmt.Errorf("-type %s is not expressible (files or dirs only)", n.args[0])
			}
		case "-name":
			if d.Names != nil {
				return 0, fmt.Errorf("more than one positive -name test")
			}
			d.Names = []string{n.args[0]}
		case "or":
			names, ok := namesOf(n)
			if !ok || d.Names != nil {
				return 0, fmt.Errorf("an ( … -o … ) group other than one list of -name tests")
			}
			d.Names = names
		case "not":
			k := n.kids[0]
			switch k.op {
			case "-name":
				d.ExcludeNames = append(d.ExcludeNames, k.args[0])
			case "-path", "-wholename":
				dir, ok := pathDir(k.args[0])
				if !ok {
					return 0, fmt.Errorf("! -path %q is not of the form '*/DIR/*'", k.args[0])
				}
				if dir != ".git" {
					d.ExcludeDirs = appendUnique(d.ExcludeDirs, dir)
				}
			default:
				return 0, fmt.Errorf("negated %s is not expressible", k.op)
			}
		case "-print", "-true":
			if n.op == "-print" && !lastTerm {
				return 0, fmt.Errorf("-print before the last test")
			}
		case "-printf":
			if n.args[0] != `%f\n` || !lastTerm {
				return 0, fmt.Errorf("-printf %q is not expressible", n.args[0])
			}
			out = streamNames
		case "-exec", "-execdir":
			a := n.args
			switch {
			case len(a) == 3 && a[0] == "basename" && a[1] == "{}" && a[2] == ";" && lastTerm:
				out = streamNames
			case len(a) >= 3 && a[0] == "grep":
				re, prints, err := grepContains(a)
				if err != nil {
					return 0, err
				}
				if d.Contains != "" {
					return 0, fmt.Errorf("more than one grep")
				}
				if prints && !lastTerm {
					return 0, fmt.Errorf("grep -l before the last test")
				}
				d.Contains = re
			default:
				return 0, fmt.Errorf("-exec %s is not expressible", strings.Join(a, " "))
			}
		default:
			return 0, fmt.Errorf("find %s is not expressible", n.op)
		}
	}
	return out, nil
}

func (d *MetricDef) takeDepths(e *fnode) *fnode {
	switch e.op {
	case "-maxdepth":
		d.MaxDepth = e.args[0]
		return &fnode{op: "-true"}
	case "-mindepth":
		d.MinDepth = e.args[0]
		return &fnode{op: "-true"}
	case "and", "or":
		var kids []*fnode
		for _, k := range e.kids {
			k = d.takeDepths(k)
			if k.op == "-true" && e.op == "and" {
				continue
			}
			kids = append(kids, k)
		}
		if len(kids) == 0 {
			return &fnode{op: "-true"}
		}
		if len(kids) == 1 {
			return kids[0]
		}
		return &fnode{op: e.op, kids: kids}
	}
	return e
}

func containsPrune(e *fnode) bool {
	if e.op == "-prune" {
		return true
	}
	for _, k := range e.kids {
		if containsPrune(k) {
			return true
		}
	}
	return false
}

// prunedDirs reads the left side of `… -prune -o`: [-type d] (-name X |
// ( -name X -o -name Y … ) | -path '*/X') -prune.
func (d *MetricDef) prunedDirs(e *fnode) error {
	if e.op != "and" || len(e.kids) < 2 || e.kids[len(e.kids)-1].op != "-prune" {
		return fmt.Errorf("unsupported -prune form")
	}
	for _, k := range e.kids[:len(e.kids)-1] {
		switch {
		case k.op == "-type" && k.args[0] == "d":
		case k.op == "-name":
			d.addExcludeDir(k.args[0])
		case k.op == "-path" || k.op == "-wholename":
			name := strings.TrimPrefix(k.args[0], "*/")
			if name == k.args[0] || strings.ContainsAny(name, "/*?[") {
				return fmt.Errorf("-path %q -prune is not of the form '*/DIR'", k.args[0])
			}
			d.addExcludeDir(name)
		case k.op == "or":
			for _, kk := range k.kids {
				if kk.op != "-name" {
					return fmt.Errorf("unsupported -prune group")
				}
				d.addExcludeDir(kk.args[0])
			}
		default:
			return fmt.Errorf("unsupported test %s before -prune", k.op)
		}
	}
	return nil
}

func (d *MetricDef) addExcludeDir(name string) {
	if name != ".git" {
		d.ExcludeDirs = appendUnique(d.ExcludeDirs, name)
	}
}

func namesOf(e *fnode) ([]string, bool) {
	if e.op == "-name" {
		return []string{e.args[0]}, true
	}
	if e.op != "or" {
		return nil, false
	}
	var names []string
	for _, k := range e.kids {
		n, ok := namesOf(k)
		if !ok {
			return nil, false
		}
		names = append(names, n...)
	}
	return names, true
}

// pathDir reads the directory name out of a `*/DIR/*` path pattern.
func pathDir(p string) (string, bool) {
	if !strings.HasPrefix(p, "*/") || !strings.HasSuffix(p, "/*") || len(p) <= 4 {
		return "", false
	}
	dir := p[2 : len(p)-2]
	if strings.ContainsAny(dir, "/*?[") {
		return "", false
	}
	return dir, true
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// grepContains reads `grep [-l|-q] [-E|-F] [-s] [--] PATTERN {} (+|;)` and
// returns the pattern as an ERE, and whether grep prints the file name.
func grepContains(a []string) (string, bool, error) {
	if a[len(a)-2] != "{}" {
		return "", false, fmt.Errorf("grep in -exec does not end with {}")
	}
	flags, pat, err := grepFlags(a[1 : len(a)-2])
	if err != nil {
		return "", false, err
	}
	if len(pat) != 1 {
		return "", false, fmt.Errorf("grep in -exec needs exactly one pattern")
	}
	if !flags["l"] && !flags["q"] {
		return "", false, fmt.Errorf("grep in -exec without -l or -q counts lines, not files")
	}
	if flags["q"] && a[len(a)-1] == "+" {
		return "", false, fmt.Errorf("grep -q with {} + tests a batch, not a file")
	}
	return grepERE(flags, pat[0]), flags["l"], nil
}

// grepFlags splits grep arguments into single-letter flags and operands.
func grepFlags(args []string) (map[string]bool, []string, error) {
	flags := map[string]bool{}
	var operands []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "--include=") || strings.HasPrefix(a, "--exclude-dir=") {
			flags[a] = true
			continue
		}
		if a == "-e" && i+1 < len(args) {
			operands = append(operands, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 && !strings.HasPrefix(a, "--") {
			for _, c := range a[1:] {
				if !strings.ContainsRune("lqEFsrRI", c) {
					return nil, nil, fmt.Errorf("unsupported grep flag -%c", c)
				}
				flags[string(c)] = true
			}
			continue
		}
		if strings.HasPrefix(a, "--") {
			return nil, nil, fmt.Errorf("unsupported grep option %s", a)
		}
		operands = append(operands, a)
	}
	if flags["I"] {
		return nil, nil, fmt.Errorf("grep -I (skip binary files) is not expressible")
	}
	return flags, operands, nil
}

func grepERE(flags map[string]bool, pat string) string {
	switch {
	case flags["F"]:
		return fixedToERE(pat)
	case flags["E"]:
		return pat
	default:
		return breToERE(pat)
	}
}

// translateGrepR reads `grep -rl [-E] [--include=G]… [--exclude-dir=D]… PATTERN PATH…`.
func (t *Translator) translateGrepR(args []string) (*MetricDef, error) {
	flags, ops, err := grepFlags(args)
	if err != nil {
		return nil, err
	}
	if !(flags["r"] || flags["R"]) || !flags["l"] {
		return nil, fmt.Errorf("only `grep -rl PATTERN PATH…` is understood")
	}
	if len(ops) < 2 {
		return nil, fmt.Errorf("grep -rl needs a pattern and a path")
	}
	def := &MetricDef{Type: "files", Contains: grepERE(flags, ops[0])}
	for _, p := range ops[1:] {
		rel, err := t.rel(p)
		if err != nil {
			return nil, err
		}
		def.Paths = append(def.Paths, rel)
	}
	// --include and --exclude-dir in the order written.
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--include="); ok {
			def.Names = append(def.Names, v)
		}
		if v, ok := strings.CutPrefix(a, "--exclude-dir="); ok {
			def.addExcludeDir(v)
		}
	}
	return def, nil
}

// translateLs reads `ls [-1] [-d] DIR/GLOB…` with every GLOB in one DIR.
func (t *Translator) translateLs(args []string) (*MetricDef, error) {
	def := &MetricDef{Type: "files", MaxDepth: "1"}
	dirsOnly := false
	dir := ""
	for _, a := range args {
		switch a {
		case "-1":
			continue
		case "-d":
			continue
		}
		if strings.HasPrefix(a, "-") {
			return nil, fmt.Errorf("unsupported ls flag %s", a)
		}
		if strings.HasSuffix(a, "/") {
			dirsOnly = true
			a = strings.TrimSuffix(a, "/")
		}
		pfx := ""
		if strings.HasPrefix(a, rootMarker) {
			pfx, a = rootMarker, strings.TrimPrefix(a, rootMarker)
		}
		i := strings.LastIndex(a, "/")
		d, glob := ".", a
		if i >= 0 {
			d, glob = a[:i], a[i+1:]
		}
		rel, err := t.rel(pfx + d)
		if err != nil {
			return nil, err
		}
		if dir != "" && rel != dir {
			return nil, fmt.Errorf("ls of globs in different directories")
		}
		dir = rel
		def.Names = append(def.Names, glob)
	}
	if dir == "" {
		return nil, fmt.Errorf("ls without a path")
	}
	def.Paths = []string{dir}
	if dirsOnly {
		def.Type = "dirs"
		def.MinDepth = "1"
	} else {
		def.Notes = append(def.Notes, "ls GLOB counted as files; a directory matching the glob would have listed its contents before")
	}
	return def, nil
}
