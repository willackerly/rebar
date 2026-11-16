package steward

import (
	"testing"
)

func TestJQString(t *testing.T) {
	cases := map[string]string{
		"plain":            "plain",
		"ünïcode ✓":        "ünïcode ✓",
		"caf\xe9 x":        "caf� x", // a lead byte without its continuation: one U+FFFD
		"\x80\xbf":         "��",     // stray continuation bytes: one each
		"\xc0\xaf":         "��",     // C0 is never valid
		"\xe0\x80\x80":     "�",      // overlong: the whole sequence
		"\xed\xa0\x80":     "�",      // a surrogate
		"\xf4\x90\x80\x80": "�",      // beyond U+10FFFF
		"ab\xe2\x82":       "ab�",    // cut short at the end: the rest
		"\xff":             "�",
	}
	for in, want := range cases {
		if got := jqString(in); got != want {
			t.Errorf("jqString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJQPretty(t *testing.T) {
	v := object{
		{"s", "q\"b\\n\n\t\r\b\f\x01\x7f/<>& "},
		{"empty", []any{}},
		{"obj", object{}},
		{"list", []any{1, true, object{{"k", "v"}}}},
		{"null", nil},
	}
	want := `{
  "s": "q\"b\\n\n\t\r\b\f\u0001\u007f/<>&` + " " + `",
  "empty": [],
  "obj": {},
  "list": [
    1,
    true,
    {
      "k": "v"
    }
  ],
  "null": null
}
`
	if got := jqPretty(v); got != want {
		t.Errorf("jqPretty:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseJSONStream(t *testing.T) {
	vals, err := parseJSONStream([]byte(`{"b":1,"a":[2.5,"x"]}` + "\n" + `{"c":{}}{"d":null}`))
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, v := range vals {
		got += jqPretty(v)
	}
	want := "{\n  \"b\": 1,\n  \"a\": [\n    2.5,\n    \"x\"\n  ]\n}\n{\n  \"c\": {}\n}\n{\n  \"d\": null\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if _, err := parseJSONStream([]byte(`{"a":`)); err == nil {
		t.Error("truncated JSON parsed")
	}
}

func TestParseContractFile(t *testing.T) {
	cases := []struct{ path, id, version string }{
		{"/x/CONTRACT-S1-FOO.2.1.md", "S1-FOO", "2.1"},
		{"CONTRACT-S1-FOO.3.md", "S1-FOO", "3.0"},
		{"CONTRACT-PLAIN.md", "PLAIN", "1.0"},
		{"CONTRACT-A.B.1.2.3.md", "A.B.1", "2.3"},
		{"CONTRACT-X.1.0.impl.md", "X.1.0.impl", "1.0"},
	}
	for _, tc := range cases {
		if id, v := parseContractFile(tc.path); id != tc.id || v != tc.version {
			t.Errorf("parseContractFile(%q) = %q %q, want %q %q", tc.path, id, v, tc.id, tc.version)
		}
	}
}

func TestActionItems(t *testing.T) {
	all := []*contract{
		{id: "D", lifecycle: "draft", sections: [5]bool{true, false, true, false, false}},
		{id: "T", lifecycle: "testing", discoveries: []discovery{{"dispute", "x"}, {"dispute", "y"}, {"discovery", "z"}}},
		{id: "A", lifecycle: "active"},
	}
	a := actionItems(all, 5, 7)
	want := actions{
		architect: []string{"D is DRAFT — missing sections: Behavioral,Tests,Implementing Files", "T has 2 DISPUTE discovery(ies) — needs resolution"},
		englead:   []string{"T is TESTING — needs test files for verification", "2 enforcement check(s) failing"},
		product:   []string{"T has 1 DISCOVERY item(s) — needs triage"},
		dev:       []string{"T is TESTING — needs test files", "A is ACTIVE — needs implementing files"},
	}
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if !eq(a.architect, want.architect) || !eq(a.englead, want.englead) || !eq(a.product, want.product) || !eq(a.dev, want.dev) {
		t.Errorf("actionItems = %+v\nwant %+v", a, want)
	}
}
