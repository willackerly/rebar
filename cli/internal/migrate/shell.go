package migrate

import (
	"fmt"
	"regexp"
	"strings"
)

// A deliberately small shell reader: enough to take apart the one-line
// counting pipelines legacy check-ground-truth.sh copies are made of
// (`find … | wc -l | tr -d ' '`). Anything it does not understand is an
// error, and the caller reports the metric as not translatable rather than
// guessing.

type tokKind int

const (
	tWord  tokKind = iota
	tPipe          // |
	tOp            // && || ; & ( ) and anything else that is not a word
	tRedir         // a redirection, with its target in text (2>/dev/null)
)

type token struct {
	kind tokKind
	text string // the word after quote removal, variables substituted
	// unknown lists variables the word uses that have no binding; a word
	// with any is not translatable.
	unknown []string
	subst   bool // contains $(…), `…`, $((…)) or $'…'
}

// rootMarker stands for the project root in a word: a root variable
// ($REPO_ROOT, …) is replaced by it, and relPath strips it.
const rootMarker = "\x00ROOT\x00"

// lex splits a shell command into tokens. vars binds variable names to the
// text they expand to (root variables map to rootMarker; positional
// parameters of a helper call map to the call's arguments).
func lex(src string, vars map[string]string) ([]token, error) {
	var toks []token
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			i++
			continue
		case c == '\\' && i+1 < n && src[i+1] == '\n':
			i += 2
			continue
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case c == '|':
			if i+1 < n && src[i+1] == '|' {
				toks = append(toks, token{kind: tOp, text: "||"})
				i += 2
			} else {
				toks = append(toks, token{kind: tPipe, text: "|"})
				i++
			}
			continue
		case c == '&':
			if i+1 < n && src[i+1] == '&' {
				toks = append(toks, token{kind: tOp, text: "&&"})
				i += 2
			} else if i+1 < n && src[i+1] == '>' {
				return nil, fmt.Errorf("unsupported redirection &>")
			} else {
				toks = append(toks, token{kind: tOp, text: "&"})
				i++
			}
			continue
		case c == ';' || c == '(' || c == ')':
			toks = append(toks, token{kind: tOp, text: string(c)})
			i++
			continue
		}

		// Redirection: [n]>target, [n]>>target, [n]<target, n>&m.
		if m := redirRe.FindStringSubmatch(src[i:]); m != nil {
			i += len(m[0])
			target := m[2]
			if target == "" {
				for i < n && (src[i] == ' ' || src[i] == '\t') {
					i++
				}
				w, next, err := lexWord(src, i, vars)
				if err != nil {
					return nil, err
				}
				i = next
				target = w.text
			}
			toks = append(toks, token{kind: tRedir, text: m[1] + target})
			continue
		}

		w, next, err := lexWord(src, i, vars)
		if err != nil {
			return nil, err
		}
		i = next
		toks = append(toks, w)
	}
	return toks, nil
}

var redirRe = regexp.MustCompile(`^([0-9]*(?:>>|>|<)&?)([0-9]+\b|-)?`)

// lexWord reads one word starting at i and returns it and the index after it.
func lexWord(src string, i int, vars map[string]string) (token, int, error) {
	var b strings.Builder
	w := token{kind: tWord}
	n := len(src)
	for i < n {
		c := src[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '|' || c == '&' || c == ';' || c == '(' || c == ')' || c == '<' || c == '>' {
			break
		}
		switch c {
		case '\\':
			if i+1 < n {
				if src[i+1] != '\n' {
					b.WriteByte(src[i+1])
				}
				i += 2
			} else {
				i++
			}
		case '\'':
			j := strings.IndexByte(src[i+1:], '\'')
			if j < 0 {
				return w, i, fmt.Errorf("unterminated single quote")
			}
			b.WriteString(src[i+1 : i+1+j])
			i += j + 2
		case '"':
			i++
			for {
				if i >= n {
					return w, i, fmt.Errorf("unterminated double quote")
				}
				d := src[i]
				if d == '"' {
					i++
					break
				}
				if d == '\\' && i+1 < n && strings.IndexByte("$`\"\\\n", src[i+1]) >= 0 {
					if src[i+1] != '\n' {
						b.WriteByte(src[i+1])
					}
					i += 2
					continue
				}
				if d == '$' || d == '`' {
					next := expandDollar(src, i, vars, &b, &w)
					i = next
					continue
				}
				b.WriteByte(d)
				i++
			}
		case '$', '`':
			i = expandDollar(src, i, vars, &b, &w)
		default:
			b.WriteByte(c)
			i++
		}
	}
	w.text = b.String()
	return w, i, nil
}

var (
	varBraceRe = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*|[0-9])\}`)
	varPlainRe = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*|[0-9@*#?])`)
)

// expandDollar handles a $ or ` at src[i]: a bound variable expands, an
// unbound one is recorded, and command or arithmetic substitution marks the
// word untranslatable (its extent is skipped by paren counting).
func expandDollar(src string, i int, vars map[string]string, b *strings.Builder, w *token) int {
	rest := src[i:]
	if strings.HasPrefix(rest, "`") {
		w.subst = true
		j := strings.IndexByte(rest[1:], '`')
		if j < 0 {
			return len(src)
		}
		return i + j + 2
	}
	if strings.HasPrefix(rest, "$(") || strings.HasPrefix(rest, "$'") {
		w.subst = true
		if strings.HasPrefix(rest, "$'") {
			j := strings.IndexByte(rest[2:], '\'')
			if j < 0 {
				return len(src)
			}
			return i + j + 3
		}
		end := matchParen(src, i+1)
		if end < 0 {
			return len(src)
		}
		return end + 1
	}
	var name string
	var size int
	if m := varBraceRe.FindStringSubmatch(rest); m != nil {
		name, size = m[1], len(m[0])
	} else if m := varPlainRe.FindStringSubmatch(rest); m != nil {
		name, size = m[1], len(m[0])
	} else if strings.HasPrefix(rest, "${") {
		// ${VAR:-default} and friends.
		w.subst = true
		j := strings.IndexByte(rest, '}')
		if j < 0 {
			return len(src)
		}
		return i + j + 1
	} else {
		b.WriteByte('$')
		return i + 1
	}
	if v, ok := vars[name]; ok {
		b.WriteString(v)
	} else {
		w.unknown = append(w.unknown, name)
		b.WriteString("${" + name + "}")
	}
	return i + size
}

// matchParen returns the index of the `)` closing the `(` at src[open],
// skipping quoted text, or -1.
func matchParen(src string, open int) int {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case '\'':
			j := strings.IndexByte(src[i+1:], '\'')
			if j < 0 {
				return -1
			}
			i += j + 1
		case '"':
			for i++; i < len(src) && src[i] != '"'; i++ {
				if src[i] == '\\' {
					i++
				} else if src[i] == '$' && i+1 < len(src) && src[i+1] == '(' {
					end := matchParen(src, i+1)
					if end < 0 {
						return -1
					}
					i = end
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitPipeline splits tokens on `|` into stages. Redirections of stderr
// are dropped; any other operator or a stdout redirection is an error.
func splitPipeline(toks []token) ([][]token, error) {
	var stages [][]token
	var cur []token
	for _, t := range toks {
		switch t.kind {
		case tPipe:
			if len(cur) == 0 {
				return nil, fmt.Errorf("empty pipeline stage")
			}
			stages = append(stages, cur)
			cur = nil
		case tOp:
			return nil, fmt.Errorf("unsupported shell operator %q", t.text)
		case tRedir:
			if !strings.HasPrefix(t.text, "2>") {
				return nil, fmt.Errorf("unsupported redirection %q", t.text)
			}
		default:
			cur = append(cur, t)
		}
	}
	if len(cur) == 0 {
		return nil, fmt.Errorf("empty pipeline stage")
	}
	return append(stages, cur), nil
}

// words returns the text of word tokens, failing on any word that uses an
// unbound variable or a substitution.
func words(toks []token) ([]string, error) {
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		if t.subst {
			return nil, fmt.Errorf("command substitution in %q", t.text)
		}
		if len(t.unknown) > 0 {
			return nil, fmt.Errorf("unknown variable $%s", t.unknown[0])
		}
		out = append(out, t.text)
	}
	return out, nil
}

// relPath turns a find/grep/ls path argument into a path relative to the
// project root. Paths outside the project, absolute paths and globs fail.
func relPath(p string) (string, error) {
	if strings.HasPrefix(p, rootMarker) {
		p = "." + strings.TrimPrefix(p, rootMarker)
	} else if strings.Contains(p, rootMarker) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") {
		return "", fmt.Errorf("path %q is not inside the project", strings.ReplaceAll(p, rootMarker, "$ROOT"))
	}
	if strings.ContainsAny(p, "*?[") {
		return "", fmt.Errorf("glob in path %q", p)
	}
	clean := cleanRel(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q leaves the project", p)
	}
	return clean, nil
}

// cleanRel is path.Clean for a relative path, keeping "." for the root.
func cleanRel(p string) string {
	parts := []string{}
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".":
		case "..":
			if len(parts) > 0 && parts[len(parts)-1] != ".." {
				parts = parts[:len(parts)-1]
			} else {
				parts = append(parts, "..")
			}
		default:
			parts = append(parts, seg)
		}
	}
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

// breToERE converts a POSIX basic regular expression (grep, sed without -E)
// to an extended one (grep -E, the form .rebarrc `contains` and `distinct`
// use). GNU escapes \+ \? \| become operators, as GNU grep treats them.
func breToERE(re string) string {
	var b strings.Builder
	inBracket := false
	for i := 0; i < len(re); i++ {
		c := re[i]
		if inBracket {
			b.WriteByte(c)
			if c == ']' {
				inBracket = false
			}
			continue
		}
		switch c {
		case '[':
			inBracket = true
			b.WriteByte(c)
			// A ] right after [ or [^ is a literal member.
			if i+1 < len(re) && re[i+1] == '^' {
				b.WriteByte('^')
				i++
			}
			if i+1 < len(re) && re[i+1] == ']' {
				b.WriteByte(']')
				i++
			}
		case '\\':
			if i+1 >= len(re) {
				b.WriteByte(c)
				continue
			}
			i++
			switch d := re[i]; d {
			case '(', ')', '{', '}', '|', '+', '?':
				b.WriteByte(d)
			default:
				b.WriteByte('\\')
				b.WriteByte(d)
			}
		case '(', ')', '{', '}', '|', '+', '?':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '*':
			// A leading * (or one after ^ or an opening group) is literal in BRE.
			out := b.String()
			if out == "" || strings.HasSuffix(out, "^") || strings.HasSuffix(out, "(") {
				b.WriteString(`\*`)
			} else {
				b.WriteByte(c)
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// fixedToERE quotes a fixed string (grep -F) as an ERE.
func fixedToERE(s string) string {
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(`.[]()*+?{}|^$\`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}
