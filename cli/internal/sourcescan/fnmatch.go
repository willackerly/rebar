package sourcescan

// fnmatch reports whether name matches the shell glob pattern the way
// fnmatch(3) with no flags does, which is how grep applies --include,
// --exclude and --exclude-dir patterns: `*` matches any run of bytes
// (including `/`), `?` any one byte, `[...]` a bracket expression with `!`
// or `^` negation, ranges and POSIX classes ([:alpha:] …), and `\` quotes the
// next character. A `[` without its closing `]` is an ordinary character.
// Matching is on bytes, as grep does in the C locale.
//
// path.Match differs on exactly the forms adopters write in .rebarrc:
// it reads `[!x]` as a set holding `!`, rejects POSIX classes, and its `*`
// stops at `/`.
func fnmatch(pattern, name string) bool {
	px, nx := 0, 0
	// The position after the last `*` and the name position it resumes
	// from on a mismatch (0 = no `*` seen yet).
	starPx, starNx := 0, 0
	for px < len(pattern) || nx < len(name) {
		if px < len(pattern) {
			switch c := pattern[px]; c {
			case '*':
				px++
				starPx, starNx = px, nx+1
				continue
			case '?':
				if nx < len(name) {
					px++
					nx++
					continue
				}
			case '[':
				if nx < len(name) {
					if matched, width, ok := matchBracket(pattern[px:], name[nx]); ok {
						if matched {
							px += width
							nx++
							continue
						}
					} else if name[nx] == '[' {
						px++
						nx++
						continue
					}
				}
			case '\\':
				lit := byte('\\')
				width := 1
				if px+1 < len(pattern) {
					lit, width = pattern[px+1], 2
				}
				if nx < len(name) && name[nx] == lit {
					px += width
					nx++
					continue
				}
			default:
				if nx < len(name) && name[nx] == c {
					px++
					nx++
					continue
				}
			}
		}
		if starNx > 0 && starNx <= len(name) {
			px, nx = starPx, starNx
			starNx++
			continue
		}
		return false
	}
	return true
}

// matchBracket matches c against the bracket expression at the start of p
// (p[0] == '['). It returns whether c is in the set and the expression's
// width; ok is false when the expression has no closing `]`.
func matchBracket(p string, c byte) (matched bool, width int, ok bool) {
	i := 1
	negate := false
	if i < len(p) && (p[i] == '!' || p[i] == '^') {
		negate = true
		i++
	}
	first := true
	for i < len(p) {
		if p[i] == ']' && !first {
			return matched != negate, i + 1, true
		}
		first = false
		// [:class:]
		if p[i] == '[' && i+1 < len(p) && p[i+1] == ':' {
			if end := indexFrom(p, ":]", i+2); end >= 0 {
				if inClass(p[i+2:end], c) {
					matched = true
				}
				i = end + 2
				continue
			}
		}
		lo, n := bracketChar(p, i)
		if n == 0 {
			break
		}
		i += n
		hi := lo
		if i+1 < len(p) && p[i] == '-' && p[i+1] != ']' {
			if h, m := bracketChar(p, i+1); m > 0 {
				hi = h
				i += 1 + m
			}
		}
		if lo <= c && c <= hi {
			matched = true
		}
	}
	return false, 0, false
}

// bracketChar is the character at p[i] inside a bracket expression, with a
// `\` quoting the next one, and how many bytes it took (0 at the end).
func bracketChar(p string, i int) (byte, int) {
	if i >= len(p) {
		return 0, 0
	}
	if p[i] == '\\' && i+1 < len(p) {
		return p[i+1], 2
	}
	return p[i], 1
}

func indexFrom(s, sub string, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// inClass reports whether c is in the POSIX character class name (C locale).
// An unknown class matches nothing.
func inClass(name string, c byte) bool {
	upper := 'A' <= c && c <= 'Z'
	lower := 'a' <= c && c <= 'z'
	digit := '0' <= c && c <= '9'
	switch name {
	case "alpha":
		return upper || lower
	case "digit":
		return digit
	case "alnum":
		return upper || lower || digit
	case "upper":
		return upper
	case "lower":
		return lower
	case "xdigit":
		return digit || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
	case "space":
		return c == ' ' || ('\t' <= c && c <= '\r')
	case "blank":
		return c == ' ' || c == '\t'
	case "punct":
		return 0x21 <= c && c <= 0x7e && !(upper || lower || digit)
	case "print":
		return 0x20 <= c && c <= 0x7e
	case "graph":
		return 0x21 <= c && c <= 0x7e
	case "cntrl":
		return c < 0x20 || c == 0x7f
	}
	return false
}

// Match reports whether name matches the shell glob pattern as fnmatch(3)
// with no flags does (see fnmatch): find's -name and grep's --include
// patterns.
func Match(pattern, name string) bool {
	return fnmatch(pattern, name)
}
