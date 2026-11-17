package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The python servers wrote their JSON with json.dumps: ", " and ": "
// separators (or "," and ": " with indent=2), non-ASCII escaped as \uXXXX,
// keys in insertion order. The classic ASK API and ask-server still answer
// in that format (pyDumps writes the same bytes); MCP messages are the
// SDK's.

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

// decode parses one message as python's json.loads does (one value).
func decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("Extra data")
	}
	if _, err := dec.Token(); err == nil {
		return nil, errors.New("Extra data")
	}
	return v, nil
}

// pyType is the python type name of a decoded JSON value.
func pyType(v any) string {
	switch x := v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case string:
		return "str"
	case []any:
		return "list"
	case map[string]any:
		return "dict"
	case json.Number:
		if strings.ContainsAny(string(x), ".eE") {
			return "float"
		}
		return "int"
	}
	return fmt.Sprintf("%T", v)
}

// truthy is python truthiness of a decoded JSON value.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	case json.Number:
		f, err := x.Float64()
		return err != nil || f != 0
	}
	return true
}

// noGet is the AttributeError of calling .get on a non-dict.
func noGet(v any) string {
	return fmt.Sprintf("'%s' object has no attribute 'get'", pyType(v))
}
