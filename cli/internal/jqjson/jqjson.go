// Package jqjson reads and writes JSON the way jq does, for the native
// ports of scripts that build or print JSON with jq: two-space indentation,
// keys in the order they were added, empty arrays and objects as [] and {},
// jq's string escaping and its repair of invalid UTF-8, and jq -r's raw
// strings.
package jqjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Strings given to the writers should pass through RepairUTF8 first, as
// jq's --arg and -R do; ParseStream does that for the strings it reads.

// Field is one key of an object, in order.
type Field struct {
	Key string
	Val any
}

// Object is a JSON object whose keys keep their order.
type Object []Field

// Get returns the value of key and whether it is present.
func (o Object) Get(key string) (any, bool) {
	for _, f := range o {
		if f.Key == key {
			return f.Val, true
		}
	}
	return nil, false
}

// set is o with key set to v: replaced in place if present (jq keeps the
// first position and the last value of a repeated key), else appended.
func (o Object) set(key string, v any) Object {
	for i := range o {
		if o[i].Key == key {
			o[i].Val = v
			return o
		}
	}
	return append(o, Field{Key: key, Val: v})
}

// Pretty is `jq .` of v: indented, with a trailing newline.
func Pretty(v any) string {
	var b strings.Builder
	writeValue(&b, v, 0)
	b.WriteByte('\n')
	return b.String()
}

// Compact is `jq -c .` of v, without the trailing newline: what jq's
// tojson, tostring and string interpolation make of a non-string.
func Compact(v any) string {
	var b strings.Builder
	writeValue(&b, v, -1)
	return b.String()
}

// Raw is what `jq -r` prints for v, without the trailing newline: a string
// as is, anything else as Pretty.
func Raw(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSuffix(Pretty(v), "\n")
}

// Truthy is jq's truth: everything but null and false.
func Truthy(v any) bool {
	return v != nil && v != false
}

// Path follows keys from v through nested objects, as jq's .a.b does: nil
// when a key is missing or a value on the way is null. ok is false when a
// value on the way is neither an object nor null (jq's "Cannot index"
// error).
func Path(v any, keys ...string) (any, bool) {
	for _, k := range keys {
		switch x := v.(type) {
		case nil:
			return nil, true
		case Object:
			v, _ = x.Get(k)
		default:
			return nil, false
		}
	}
	return v, true
}

func writeValue(b *strings.Builder, v any, depth int) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case json.Number:
		b.WriteString(x.String())
	case string:
		writeString(b, x)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		if depth < 0 {
			b.WriteByte('[')
			for i, e := range x {
				if i > 0 {
					b.WriteByte(',')
				}
				writeValue(b, e, depth)
			}
			b.WriteByte(']')
			return
		}
		b.WriteString("[\n")
		for i, e := range x {
			indent(b, depth+1)
			writeValue(b, e, depth+1)
			if i < len(x)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		indent(b, depth)
		b.WriteByte(']')
	case Object:
		if len(x) == 0 {
			b.WriteString("{}")
			return
		}
		if depth < 0 {
			b.WriteByte('{')
			for i, f := range x {
				if i > 0 {
					b.WriteByte(',')
				}
				writeString(b, f.Key)
				b.WriteByte(':')
				writeValue(b, f.Val, depth)
			}
			b.WriteByte('}')
			return
		}
		b.WriteString("{\n")
		for i, f := range x {
			indent(b, depth+1)
			writeString(b, f.Key)
			b.WriteString(": ")
			writeValue(b, f.Val, depth+1)
			if i < len(x)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		indent(b, depth)
		b.WriteByte('}')
	default:
		panic(fmt.Sprintf("jqjson: no JSON form for %T", v))
	}
}

func indent(b *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
}

// writeString writes s as jq does: \" \\ \b \f \n \r \t, \u00xx for the
// other control characters and DEL, everything else as is.
func writeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(b, `\u%04x`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
}

// UTF-8 byte classes of jq's decoder (src/jv_unicode.c).
const utf8Continuation = 255

func utf8Length(c byte) int {
	switch {
	case c < 0x80:
		return 1
	case c < 0xc0:
		return utf8Continuation
	case c < 0xc2:
		return 0
	case c < 0xe0:
		return 2
	case c < 0xf0:
		return 3
	case c < 0xf5:
		return 4
	}
	return 0
}

var utf8Bits = [5]byte{0, 0x7f, 0x1f, 0x0f, 0x07}
var utf8FirstCodepoint = [5]int{0, 0, 0x80, 0x800, 0x10000}

// String is s as jq writes a string: quoted, with jq's escaping (s should
// be RepairUTF8'd).
func String(s string) string {
	var b strings.Builder
	writeString(&b, s)
	return b.String()
}

// RepairUTF8 is s as jq holds a string it was given (--arg, -R): every
// invalid UTF-8 sequence replaced by U+FFFD the way jq's decoder
// (jvp_utf8_next) splits them.
func RepairUTF8(s string) string {
	valid := true
	for i := 0; i < len(s); {
		n, cp := utf8Next(s[i:])
		if cp < 0 {
			valid = false
			break
		}
		i += n
	}
	if valid {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		n, cp := utf8Next(s[i:])
		if cp < 0 {
			b.WriteString("�")
		} else {
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String()
}

// utf8Next decodes the first sequence of s (non-empty): its length and
// code point, -1 when invalid. A port of jq's jvp_utf8_next.
func utf8Next(s string) (int, int) {
	first := s[0]
	length := utf8Length(first)
	switch {
	case first < 0x80:
		return 1, int(first)
	case length == 0 || length == utf8Continuation:
		return 1, -1
	case length > len(s):
		return len(s), -1
	}
	cp := int(first & utf8Bits[length])
	for i := 1; i < length; i++ {
		ch := s[i]
		if utf8Length(ch) != utf8Continuation {
			return i, -1
		}
		cp = cp<<6 | int(ch&0x3f)
	}
	if cp < utf8FirstCodepoint[length] || (0xd800 <= cp && cp <= 0xdfff) || cp > 0x10ffff {
		return length, -1
	}
	return length, cp
}

// ParseStream reads the JSON values in data, one after another, as jq
// reads its input, keeping the order of object keys. Numbers are
// json.Number, objects Object. On an error it returns the values before it.
func ParseStream(data []byte) ([]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var vals []any
	for {
		v, err := parseValue(dec)
		if err == io.EOF {
			return vals, nil
		}
		if err != nil {
			return vals, err
		}
		vals = append(vals, v)
	}
}

func parseValue(dec *json.Decoder) (any, error) {
	return parseValueAt(dec, true)
}

// parseValueAt reads one value; only at the top level does the end of the
// input end the stream instead of truncating a value.
func parseValueAt(dec *json.Decoder, top bool) (any, error) {
	tok, err := dec.Token()
	if err == io.EOF && !top {
		return nil, io.ErrUnexpectedEOF
	}
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := parseValueAt(dec, false)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, io.ErrUnexpectedEOF
			}
			return arr, nil
		case '{':
			obj := Object{}
			for dec.More() {
				k, err := dec.Token()
				if err == io.EOF {
					return nil, io.ErrUnexpectedEOF
				}
				if err != nil {
					return nil, err
				}
				key, ok := k.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not a string")
				}
				v, err := parseValueAt(dec, false)
				if err != nil {
					return nil, err
				}
				obj = obj.set(RepairUTF8(key), v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, io.ErrUnexpectedEOF
			}
			return obj, nil
		}
		return nil, fmt.Errorf("unexpected %v", t)
	case string:
		return RepairUTF8(t), nil
	default:
		return t, nil // bool, json.Number, nil
	}
}
