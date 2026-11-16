package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The python servers wrote their JSON with json.dumps: ", " and ": "
// separators (or "," and ": " with indent=2), non-ASCII escaped as \uXXXX,
// keys in insertion order. pyDumps writes the same bytes, so a recorded
// session compares byte for byte.

// field is one key of an ordered object.
type field struct {
	key string
	val any
}

// object is a JSON object whose keys keep their order.
type object []field

// pyDumps is json.dumps(v) (indent 0: compact) or json.dumps(v, indent=2).
func pyDumps(v any, indent int) string {
	var b strings.Builder
	pyWrite(&b, v, indent, 0)
	return b.String()
}

func pyWrite(b *strings.Builder, v any, indent, depth int) {
	newline := func(d int) {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", indent*d))
	}
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	case json.Number:
		b.WriteString(x.String())
	case json.RawMessage:
		b.Write(x)
	case string:
		pyString(b, x)
	case []string:
		a := make([]any, len(x))
		for i, s := range x {
			a[i] = s
		}
		pyWrite(b, a, indent, depth)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
				if indent == 0 {
					b.WriteByte(' ')
				}
			}
			if indent > 0 {
				newline(depth + 1)
			}
			pyWrite(b, e, indent, depth+1)
		}
		if indent > 0 {
			newline(depth)
		}
		b.WriteByte(']')
	case object:
		if len(x) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, f := range x {
			if i > 0 {
				b.WriteByte(',')
				if indent == 0 {
					b.WriteByte(' ')
				}
			}
			if indent > 0 {
				newline(depth + 1)
			}
			pyString(b, f.key)
			b.WriteString(": ")
			pyWrite(b, f.val, indent, depth+1)
		}
		if indent > 0 {
			newline(depth)
		}
		b.WriteByte('}')
	case map[string]any:
		// Decoded request values: Python kept the sender's key order; a
		// map cannot, so keys are sorted (only echoed ids are maps).
		o := object{}
		for _, k := range sortedKeys(x) {
			o = append(o, field{k, x[k]})
		}
		pyWrite(b, o, indent, depth)
	default:
		panic(fmt.Sprintf("mcp: no JSON form for %T", v))
	}
}

// pyString writes s as json.dumps does with ensure_ascii.
func pyString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r >= 0x20 && r < 0x7f:
				b.WriteRune(r)
			case r < 0x10000:
				fmt.Fprintf(b, `\u%04x`, r)
			default:
				r -= 0x10000
				fmt.Fprintf(b, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			}
		}
	}
	b.WriteByte('"')
}
