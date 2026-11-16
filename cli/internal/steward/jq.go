package steward

import "github.com/willackerly/rebar/cli/internal/jqjson"

// The steward script builds its JSON with jq and prints it with `jq .`;
// jqjson writes the same bytes. The report is built from the local object
// type (short literals) and converted for printing.

// field is one key of an object, in order.
type field struct {
	key string
	val any
}

// object is a JSON object whose keys keep their order.
type object []field

// get returns the value of key, nil when absent.
func (o object) get(key string) any {
	for _, f := range o {
		if f.key == key {
			return f.val
		}
	}
	return nil
}

// jqPretty is `jq .` of v: indented, with a trailing newline.
func jqPretty(v any) string {
	return jqjson.Pretty(toJQ(v))
}

// toJQ converts the objects in v to jqjson's.
func toJQ(v any) any {
	switch x := v.(type) {
	case object:
		o := make(jqjson.Object, len(x))
		for i, f := range x {
			o[i] = jqjson.Field{Key: f.key, Val: toJQ(f.val)}
		}
		return o
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = toJQ(e)
		}
		return a
	}
	return v
}

// jqString is s as jq holds a string it was given (--arg, -R).
func jqString(s string) string { return jqjson.RepairUTF8(s) }

// parseJSONStream reads the JSON values in data as jq reads its input.
func parseJSONStream(data []byte) ([]any, error) { return jqjson.ParseStream(data) }
