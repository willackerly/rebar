package jqjson

import (
	"encoding/json"
	"testing"
)

func TestCompactRawTruthyPath(t *testing.T) {
	vals, err := ParseStream([]byte(`{"a":{"b":[1,"x",null,{}]},"s":"str","n":2.50,"a2":true,"a":{"c":false}}`))
	if err != nil || len(vals) != 1 {
		t.Fatalf("parse: %v %v", vals, err)
	}
	v := vals[0]
	// A repeated key keeps its first position and its last value, as in jq.
	if got, want := Compact(v), `{"a":{"c":false},"s":"str","n":2.50,"a2":true}`; got != want {
		t.Errorf("Compact = %s, want %s", got, want)
	}
	if got := Raw("multi\nline"); got != "multi\nline" {
		t.Errorf("Raw(string) = %q", got)
	}
	if got, want := Raw(Object{{Key: "k", Val: []any{json.Number("1")}}}), "{\n  \"k\": [\n    1\n  ]\n}"; got != want {
		t.Errorf("Raw(object) = %q, want %q", got, want)
	}
	for _, tc := range []struct {
		v    any
		want bool
	}{{nil, false}, {false, false}, {true, true}, {"", true}, {json.Number("0"), true}, {Object{}, true}} {
		if Truthy(tc.v) != tc.want {
			t.Errorf("Truthy(%#v) = %v", tc.v, !tc.want)
		}
	}
	if got, ok := Path(v, "a", "c"); !ok || got != false {
		t.Errorf("Path(.a.c) = %v %v", got, ok)
	}
	if got, ok := Path(v, "missing", "x"); !ok || got != nil {
		t.Errorf("Path(.missing.x) = %v %v (null indexes to null)", got, ok)
	}
	if _, ok := Path(v, "s", "x"); ok {
		t.Error("Path(.s.x) indexed a string")
	}
	if got := String("q\"\x01\x7f/é"); got != `"q\"\u0001\u007f/é"` {
		t.Errorf("String = %s", got)
	}
}
