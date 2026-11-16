package checks

import (
	"regexp"
	"strings"
)

// breRegexp compiles a POSIX basic regular expression, as grep (without -E)
// reads one, with GNU's extensions (\| \+ \? \< \> \b \B \w \W \s \S):
// for scripts that build a grep pattern from data, like check-doc-refs'
// `grep -q "^${dir}/"`, where a . in a path matches any character. It
// returns nil for a pattern grep rejects (an unbalanced [ or \() or one Go
// cannot run (a back-reference); the script's grep then fails too, or
// cannot be matched exactly.
func breRegexp(bre string) *regexp.Regexp {
	var b strings.Builder
	// atStart: where * is literal and ^ an anchor (the start, after \( or \|).
	atStart := true
	for i := 0; i < len(bre); i++ {
		c := bre[i]
		start := atStart
		atStart = false
		switch c {
		case '^':
			if start {
				b.WriteString("^")
				atStart = true // ^* matches a literal *
				continue
			}
			b.WriteString(`\^`)
		case '$':
			if i == len(bre)-1 || strings.HasPrefix(bre[i+1:], `\)`) || strings.HasPrefix(bre[i+1:], `\|`) {
				b.WriteString("$")
			} else {
				b.WriteString(`\$`)
			}
		case '*':
			if start {
				b.WriteString(`\*`)
			} else {
				b.WriteString("*")
			}
		case '.':
			b.WriteString(".")
		case '[':
			end, class, ok := breBracket(bre, i)
			if !ok {
				return nil
			}
			b.WriteString(class)
			i = end
		case '\\':
			if i+1 >= len(bre) {
				return nil // trailing backslash
			}
			i++
			switch n := bre[i]; n {
			case '(':
				b.WriteString("(")
				atStart = true
			case ')':
				b.WriteString(")")
			case '|':
				b.WriteString("|")
				atStart = true
			case '{':
				b.WriteString("{")
			case '}':
				b.WriteString("}")
			case '+', '?':
				b.WriteByte(n)
			case '<', '>', 'b':
				b.WriteString(`\b`)
			case 'B', 'w', 'W':
				b.WriteString(`\` + string(n))
			case 's':
				b.WriteString(`[[:space:]]`)
			case 'S':
				b.WriteString(`[^[:space:]]`)
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				return nil
			default:
				b.WriteString(regexp.QuoteMeta(bre[i : i+1]))
			}
		default:
			b.WriteString(regexp.QuoteMeta(bre[i : i+1]))
		}
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}

// breBracket translates the bracket expression starting at bre[i] ('[')
// into Go syntax, returning the index of its closing ']'.
func breBracket(bre string, i int) (end int, class string, ok bool) {
	var b strings.Builder
	b.WriteByte('[')
	j := i + 1
	if j < len(bre) && bre[j] == '^' {
		b.WriteByte('^')
		j++
	}
	if j < len(bre) && bre[j] == ']' {
		b.WriteString(`\]`)
		j++
	}
	for ; j < len(bre); j++ {
		c := bre[j]
		switch {
		case c == ']':
			b.WriteByte(']')
			return j, b.String(), true
		case c == '[' && j+1 < len(bre) && (bre[j+1] == ':' || bre[j+1] == '=' || bre[j+1] == '.'):
			kind := bre[j+1]
			k := strings.Index(bre[j+2:], string(kind)+"]")
			if k < 0 {
				return 0, "", false
			}
			body := bre[j+2 : j+2+k]
			switch kind {
			case ':':
				b.WriteString("[:" + body + ":]")
			default: // [=x=] and [.x.]: the character itself
				b.WriteString(regexp.QuoteMeta(body))
			}
			j += 2 + k + 1
		case c == '\\' || c == '[':
			b.WriteString(`\` + string(c))
		default:
			b.WriteByte(c)
		}
	}
	return 0, "", false
}
