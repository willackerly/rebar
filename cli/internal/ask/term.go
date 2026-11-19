package ask

import (
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/willackerly/rebar/cli/internal/checks"
)

// colours are the script's _C_* escapes: set when stdout is a terminal and
// NO_COLOR is empty, else all "".
type colours struct {
	reset, bold, dim, cyan, green, yellow, blue, magenta, white, bar string
}

func newColours(on bool) colours {
	if !on {
		return colours{}
	}
	return colours{
		reset: "\033[0m", bold: "\033[1m", dim: "\033[2m", cyan: "\033[36m",
		green: "\033[32m", yellow: "\033[33m", blue: "\033[34m",
		magenta: "\033[35m", white: "\033[37m", bar: "\033[38;5;240m",
	}
}

// roleColour maps an agent role to its colour.
func (c colours) role(name string) string {
	switch name {
	case "architect":
		return c.cyan
	case "product":
		return c.green
	case "englead":
		return c.yellow
	case "steward":
		return c.magenta
	case "tester", "merger":
		return c.blue
	case "engineer":
		return c.green
	}
	return c.white
}

// termWidth is $(tput cols 2>/dev/null || echo 80): tput fails without a
// TERM; COLUMNS overrides the terminal's size; with neither stdout nor
// stderr a terminal, terminfo's default is 80.
func (a *App) termWidth() int {
	if os.Getenv("TERM") == "" {
		return 80
	}
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	for _, w := range []any{a.Stdout, a.Stderr} {
		if f, ok := w.(*os.File); ok {
			if n := checks.TerminalWidth(f); n > 0 {
				return n
			}
		}
	}
	return 80
}

// formatResponse is _format_response: the raw text when stdout is not a
// terminal, else a header bar with the project and agent and the text
// wrapped to the terminal with a two-space margin.
func (a *App) formatResponse(agent, response, project string) {
	if !a.stdoutTTY {
		a.out("%s\n", response)
		return
	}
	header := project
	if header == "" {
		header = basename(a.pwd)
	}
	header += ":" + agent
	// GNU sed's s/\(^\|:\)\([a-z]\)/\1\u\2/g: capitalise each component.
	parts := strings.Split(header, ":")
	for i, p := range parts {
		if p != "" && p[0] >= 'a' && p[0] <= 'z' {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	display := strings.Join(parts, ":")

	width := a.termWidth()
	barWidth := min(width, 80)
	bar := strings.Repeat("─", barWidth)
	a.out("\n%s%s%s\n", a.c.bar, bar, a.c.reset)
	a.out("%s%s  %s%s\n", a.c.role(agent), a.c.bold, display, a.c.reset)
	a.out("%s%s%s\n\n", a.c.bar, bar, a.c.reset)
	wrapWidth := max(width-4, 40)
	for _, line := range strings.Split(wrap(response, wrapWidth), "\n") {
		a.out("  %s\n", line)
	}
	a.out("\n")
}

// wrap fills each paragraph of text to width columns, as fmt -w does
// (blank lines and indentation kept, words never split).
func wrap(text string, width int) string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		indent := para[:len(para)-len(strings.TrimLeftFunc(para, unicode.IsSpace))]
		line := indent
		n := utf8.RuneCountInString(indent)
		first := true
		for _, word := range strings.Fields(para) {
			w := utf8.RuneCountInString(word)
			if !first && n+1+w > width {
				out = append(out, line)
				line, n, first = indent, utf8.RuneCountInString(indent), true
			}
			if !first {
				line += " "
				n++
			}
			line += word
			n += w
			first = false
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
