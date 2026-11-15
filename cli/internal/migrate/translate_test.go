package migrate

import (
	"strings"
	"testing"
)

// settingsText renders a translation the way it lands in .rebarrc, one
// `key = value` per line, for compact comparisons.
func settingsText(key string, d *MetricDef) string {
	var b strings.Builder
	for _, s := range d.Settings(key, "") {
		b.WriteString(s.Key + " = " + s.Value + "\n")
	}
	return b.String()
}

func translateOne(t *testing.T, script string) map[string]Translation {
	t.Helper()
	out := map[string]Translation{}
	for _, tr := range NewTranslator(script).Translate() {
		out[tr.Key] = tr
	}
	return out
}

// The shapes virtru-analyze's customized check-ground-truth.sh uses: plain
// find pipelines, helpers, existence-count loops, -exec grep, distinct.
const analyzeLike = `#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

count_contracts() {
  find "$REPO_ROOT/architecture" -maxdepth 1 -type f -name 'CONTRACT-*.md' \
    ! -name 'CONTRACT-TEMPLATE.md' \
    ! -name '*.impl.md' \
    | wc -l | tr -d ' '
}

count_tests() {
  find "$REPO_ROOT/api" "$REPO_ROOT/sdk" \
    -type f \( \
      -name '*_test.go' -o \
      -name '*.test.ts' \
    \) \
    ! -path '*/node_modules/*' ! -path '*/dist/*' \
    | wc -l | tr -d ' '
}

count_contract_ids() {
  find "$REPO_ROOT/architecture" -maxdepth 1 -type f -name 'CONTRACT-*.md' \
    ! -name 'CONTRACT-TEMPLATE.md' \
    -exec basename {} \; \
    | sed -E 's/^CONTRACT-(.*)\.[0-9]+\.[0-9]+\.md$/\1/' \
    | sort -u | wc -l | tr -d ' '
}

count_headers() {
  find "$REPO_ROOT/sdk" -type f \( -name '*.go' -o -name '*.ts' \) \
    ! -path '*/vendor/*' \
    -exec grep -l 'CONTRACT:[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*' {} + \
    | wc -l | tr -d ' '
}

count_packages() {
  count=0
  for package in bedrock config tenancy; do
    [ -d "$REPO_ROOT/mcp/$package" ] && count=$((count + 1))
  done
  echo "$count"
}

count_helm() {
  count=0
  for server in embed trino; do
    [ -f "$REPO_ROOT/charts/templates/deployment-${server}.yaml" ] \
      && count=$((count + 1))
  done
  echo "$count"
}

compute_metrics() {
  echo "sdk_languages=$(find "$REPO_ROOT/sdk" -mindepth 1 -maxdepth 1 -type d \( -name go-sdk -o -name web-sdk \) | wc -l | tr -d ' ')"
  echo "go_modules=$(find "$REPO_ROOT" -type f -name go.mod ! -path '*/.git/*' ! -path '*/vendor/*' | wc -l | tr -d ' ')"
  echo "contract_files=$(count_contracts)"
  echo "test_files=$(count_tests)"
  echo "contract_ids=$(count_contract_ids)"
  echo "headers=$(count_headers)"
  echo "packages=$(count_packages)"
  echo "helm=$(count_helm)"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  :
fi
`

func TestTranslateAnalyzeShapes(t *testing.T) {
	got := translateOne(t, analyzeLike)
	want := map[string]string{
		"sdk_languages": "metric.sdk_languages.type = dirs\nmetric.sdk_languages.paths = sdk\nmetric.sdk_languages.min_depth = 1\nmetric.sdk_languages.max_depth = 1\nmetric.sdk_languages.names = go-sdk web-sdk\n",
		"go_modules":    "metric.go_modules.names = go.mod\nmetric.go_modules.exclude_dirs = vendor\n",
		"contract_files": "metric.contract_files.paths = architecture\nmetric.contract_files.max_depth = 1\nmetric.contract_files.names = CONTRACT-*.md\n" +
			"metric.contract_files.exclude_names = CONTRACT-TEMPLATE.md *.impl.md\n",
		"test_files": "metric.test_files.paths = api sdk\nmetric.test_files.names = *_test.go *.test.ts\nmetric.test_files.exclude_dirs = node_modules dist\n",
		"contract_ids": "metric.contract_ids.paths = architecture\nmetric.contract_ids.max_depth = 1\nmetric.contract_ids.names = CONTRACT-*.md\n" +
			"metric.contract_ids.exclude_names = CONTRACT-TEMPLATE.md\nmetric.contract_ids.distinct = ^CONTRACT-(.*)\\.[0-9]+\\.[0-9]+\\.md$\n",
		"headers": "metric.headers.paths = sdk\nmetric.headers.names = *.go *.ts\nmetric.headers.exclude_dirs = vendor\n" +
			"metric.headers.contains = CONTRACT:[A-Za-z0-9_-]*\\.[0-9]*\\.[0-9]*\n",
		"packages": "metric.packages.type = dirs\nmetric.packages.paths = mcp\nmetric.packages.min_depth = 1\nmetric.packages.max_depth = 1\nmetric.packages.names = bedrock config tenancy\n",
		"helm":     "metric.helm.paths = charts/templates\nmetric.helm.max_depth = 1\nmetric.helm.names = deployment-embed.yaml deployment-trino.yaml\n",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d translations, want %d: %+v", len(got), len(want), got)
	}
	for key, w := range want {
		tr, ok := got[key]
		if !ok || tr.Err != nil {
			t.Errorf("%s: not translated: %v", key, tr.Err)
			continue
		}
		if s := settingsText(key, tr.Def); s != w {
			t.Errorf("%s:\n got:\n%s want:\n%s", key, s, w)
		}
	}
}

// The examples in the pre-#14 template: relative paths, -o without
// parentheses or -type, ls of a glob, -not.
func TestTranslateOldTemplateExamples(t *testing.T) {
	got := translateOne(t, `compute_metrics() {
  echo "unit_test_files=$(find tests/ -name '*.test.ts' -o -name '*.test.tsx' 2>/dev/null | wc -l | tr -d ' ')"
  echo "contracts=$(ls architecture/CONTRACT-*.md 2>/dev/null | wc -l | tr -d ' ')"
  echo "api_route_modules=$(find src/routes/ -name '*.ts' -not -name 'index.ts' -not -name '*.test.ts' 2>/dev/null | wc -l | tr -d ' ')"
}`)
	cases := map[string]string{
		"unit_test_files":   "metric.unit_test_files.paths = tests\nmetric.unit_test_files.names = *.test.ts *.test.tsx\n",
		"contracts":         "metric.contracts.paths = architecture\nmetric.contracts.max_depth = 1\nmetric.contracts.names = CONTRACT-*.md\n",
		"api_route_modules": "metric.api_route_modules.paths = src/routes\nmetric.api_route_modules.names = *.ts\nmetric.api_route_modules.exclude_names = index.ts *.test.ts\n",
	}
	for key, w := range cases {
		tr := got[key]
		if tr.Err != nil {
			t.Fatalf("%s: %v", key, tr.Err)
		}
		if s := settingsText(key, tr.Def); s != w {
			t.Errorf("%s:\n got:\n%s want:\n%s", key, s, w)
		}
		if len(tr.Def.Notes) == 0 {
			t.Errorf("%s: a loosened translation must carry a note", key)
		}
	}
}

func TestTranslateMoreForms(t *testing.T) {
	got := translateOne(t, `ROOT="${REBAR_PROJECT_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
count_in() {
  find "$ROOT/$1" -type f -name "$2" | wc -l | xargs
}
compute_metrics() {
  local n
  n=$(grep -rlE 'TODO|FIXME' "$ROOT/src" --include='*.go' --exclude-dir=vendor | wc -l)
  echo "flagged=$n"
  echo "go_files=$(count_in src '*.go')"
  echo "pruned=$(find "$ROOT" -name node_modules -prune -o -type f -name '*.js' -print | wc -l | tr -d '[:space:]')"
  echo "bre=$(find . -type f -exec grep -l 'a\(b\|c\)+' {} \; | wc -l)"
  echo "names=$(find docs -type f -printf '%f\n' | sort -u | wc -l)"
}`)
	cases := map[string]string{
		"flagged":  "metric.flagged.paths = src\nmetric.flagged.names = *.go\nmetric.flagged.exclude_dirs = vendor\nmetric.flagged.contains = TODO|FIXME\n",
		"go_files": "metric.go_files.paths = src\nmetric.go_files.names = *.go\n",
		"pruned":   "metric.pruned.names = *.js\nmetric.pruned.exclude_dirs = node_modules\n",
		"bre":      "metric.bre.contains = a(b|c)\\+\n",
		"names":    "metric.names.paths = docs\nmetric.names.distinct = ^(.*)$\n",
	}
	for key, w := range cases {
		tr := got[key]
		if tr.Err != nil {
			t.Fatalf("%s: %v", key, tr.Err)
		}
		if s := settingsText(key, tr.Def); s != w {
			t.Errorf("%s:\n got:\n%s want:\n%s", key, s, w)
		}
	}
}

// Untranslatable forms are errors with a reason, never a guess.
func TestTranslateRefusals(t *testing.T) {
	got := translateOne(t, `compute_metrics() {
  echo "lines=$(cat src/*.go | wc -l)"
  echo "matches=$(grep -rho 'CONTRACT:[A-Z0-9-]*' src | sort -u | wc -l)"
  echo "mixed=$(find src -type f -name a -o -name b | wc -l)"
  echo "sum=$(( $(find a -type f | wc -l) + 1 ))"
  echo "outside=$(find /etc -type f | wc -l)"
  echo "iname=$(find src -iname '*.GO' | wc -l)"
  echo "hash=$(find src -type f -exec grep -l '#pragma' {} + | wc -l)"
  echo "unknown=$(find "$ELSEWHERE" -type f | wc -l)"
  echo "nowc=$(find src -type f)"
  echo "links=$(find src -type l | wc -l)"
}`)
	for _, key := range []string{"lines", "matches", "mixed", "sum", "outside", "iname", "hash", "unknown", "nowc", "links"} {
		tr, ok := got[key]
		if !ok {
			t.Errorf("%s: no translation result", key)
			continue
		}
		if tr.Err == nil {
			t.Errorf("%s: translated to %s, want a refusal", key, settingsText(key, tr.Def))
		}
	}
}

func TestBreToERE(t *testing.T) {
	cases := map[string]string{
		`CONTRACT:[A-Za-z0-9_-]*\.[0-9]*`: `CONTRACT:[A-Za-z0-9_-]*\.[0-9]*`,
		`a\(b\|c\)\{2\}`:                  `a(b|c){2}`,
		`x(y)+?|z`:                        `x\(y\)\+\?\|z`,
		`*star`:                           `\*star`,
		`[()|]`:                           `[()|]`,
		`[]a]`:                            `[]a]`,
	}
	for in, want := range cases {
		if got := breToERE(in); got != want {
			t.Errorf("breToERE(%q) = %q, want %q", in, got, want)
		}
	}
	if got := fixedToERE("a.b*c"); got != `a\.b\*c` {
		t.Errorf("fixedToERE = %q", got)
	}
}

func TestLexAndRelPath(t *testing.T) {
	toks, err := lex(`find "$R/a b" -name '*.go' \( x \) 2>/dev/null | wc -l`, map[string]string{"R": rootMarker})
	if err != nil {
		t.Fatal(err)
	}
	stages, err := splitPipeline(toks)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := words(stages[0])
	if strings.Join(w, ",") != "find,"+rootMarker+"/a b,-name,*.go,(,x,)" {
		t.Fatalf("words = %q", w)
	}
	if _, err := lex(`echo 'open`, nil); err == nil {
		t.Fatal("unterminated quote accepted")
	}
	for in, want := range map[string]string{rootMarker: ".", rootMarker + "/x/": "x", "./a/../b": "b", "src": "src"} {
		if got, err := relPath(in); err != nil || got != want {
			t.Errorf("relPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"/etc", "../up", rootMarker + "/../up", "src/*"} {
		if _, err := relPath(bad); err == nil {
			t.Errorf("relPath(%q) accepted", bad)
		}
	}
}

// S1: translations that equal the original only on some trees are refused
// by the translator or caught on the probe tree.
func TestTranslatorFalsePositives(t *testing.T) {
	got := translateOne(t, `REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
compute_metrics() {
  echo "adr_entries=$(ls "$REPO_ROOT/docs/adr/" | wc -l | tr -d ' ')"
  echo "both=$(grep -rl -e alpha -e beta "$REPO_ROOT/src" | wc -l)"
  echo "errs=$(find "$REPO_ROOT/src" -type f 2>&1 | wc -l)"
  echo "uniq=$(find "$REPO_ROOT/src" -type f -exec basename {} \; | uniq | wc -l)"
  echo "glob=$(find "$REPO_ROOT/src" -type f -name *.go | wc -l)"
  echo "sorted=$(find "$REPO_ROOT/src" -type f -exec basename {} \; | sort | uniq | wc -l)"
}`)
	for _, k := range []string{"adr_entries", "errs", "uniq", "glob"} {
		if got[k].Err == nil {
			t.Errorf("%s: translated to %s, want a refusal", k, settingsText(k, got[k].Def))
		}
	}
	if tr := got["both"]; tr.Err != nil || settingsText("both", tr.Def) != "metric.both.paths = src\nmetric.both.contains = (alpha)|(beta)\n" {
		t.Errorf("grep -e -e: %v %v", tr.Err, tr.Def)
	}
	if tr := got["sorted"]; tr.Err != nil || tr.Def.Distinct == "" {
		t.Errorf("sort | uniq should be a distinct count: %+v", tr)
	}

	// An unmodelled statement around the values refuses every metric.
	got = translateOne(t, `compute_metrics() {
  n=$(find src -type f -name '*.go' | wc -l)
  if [ -d vendor ]; then n=0; fi
  echo "go_files=$n"
}`)
	if got["go_files"].Err == nil || !strings.Contains(got["go_files"].Err.Error(), "does not model") {
		t.Fatalf("if around a metric: %+v", got["go_files"])
	}
	if breToERE(`x(*`) != `x\(*` {
		t.Errorf("breToERE(x(*) = %q", breToERE(`x(*`))
	}
}
