package cmd

import (
	"testing"
)

// Parity of check-ground-truth (runParity) and its golden files
// (testdata/check-ground-truth; see native_registry_test.go).

var groundTruthCheck = parityCheck{name: "check-ground-truth", golden: "check-ground-truth"}

// gtTree is a small project the metrics below count.
var gtTree = map[string]string{
	"api/a/one_test.go":                   "package a\n",
	"api/a/two_test.go":                   "package a\n",
	"api/a/main.go":                       "package a\n// @contract:S1-FOO.1.0\n",
	"api/node_modules/x/dep.go":           "package x\n",
	"sdk/go-sdk/go.mod":                   "module x\n",
	"sdk/web-sdk/package.json":            "{}\n",
	"sdk/other/.keep":                     "",
	"agents/architect/AGENT.md":           "# a\n",
	"agents/steward/AGENT.md":             "# s\n",
	"architecture/CONTRACT-S1-FOO.1.0.md": "# c\n",
	"architecture/CONTRACT-S1-FOO.2.0.md": "# c\n",
	"architecture/CONTRACT-C2-BAR.1.0.md": "# c\n",
	"architecture/CONTRACT-TEMPLATE.md":   "# t\n",
}

const gtRC = `tier = 3
metric.test_files.paths    = api
metric.test_files.names    = *_test.go
metric.sdks.type      = dirs
metric.sdks.paths     = sdk
metric.sdks.min_depth = 1
metric.sdks.max_depth = 1
metric.sdks.names     = *-sdk
metric.agents.paths     = agents
metric.agents.min_depth = 2
metric.agents.max_depth = 2
metric.agents.names     = AGENT.md
metric.contract_files.paths = architecture
metric.contract_files.max_depth = 1
metric.contract_files.names = CONTRACT-*.md
metric.contract_files.exclude_names = CONTRACT-TEMPLATE.md # the template
metric.contract_ids.paths = architecture
metric.contract_ids.names = CONTRACT-*.md
metric.contract_ids.exclude_names = CONTRACT-TEMPLATE.md
metric.contract_ids.distinct = ^CONTRACT-(.*)\.[0-9]+\.[0-9]+\.md$
metric.headed.contains = @contract:[A-Za-z0-9_-]+\.[0-9]+\.[0-9]+
metric.go_all.names = *.go
metric.go_all.exclude_dirs = node_modules
metric.nothing.paths = no-such-dir
`

const gtMetrics = "test_files = 2\nsdks = 2\nagents = 2\ncontract_files = 3\ncontract_ids = 2\nheaded = 1\ngo_all = 3\nnothing = 0\n"

// gtCase is gtTree with a .rebarrc, a METRICS and extra files.
func gtCase(rc, metrics string, extra map[string]string) map[string]string {
	files := map[string]string{}
	for k, v := range gtTree {
		files[k] = v
	}
	if rc != "" {
		files[".rebarrc"] = rc
	}
	if metrics != "" {
		files["METRICS"] = metrics
	}
	for k, v := range extra {
		files[k] = v
	}
	return files
}

var groundTruthParityCases = []parityCase{
	{name: "every metric matches", files: gtCase(gtRC, gtMetrics, nil)},
	{name: "print", files: gtCase(gtRC, "", nil), args: []string{"--print"}},
	{name: "print ignores the tier", files: gtCase("tier = 1\n"+gtRC[len("tier = 3\n"):], "", nil), args: []string{"--print"}},
	{name: "tier 1 skips", files: gtCase("tier = 1\n"+gtRC[len("tier = 3\n"):], "test_files = 9\n", nil)},
	{
		name:  "tier 1, named in checks_enable",
		files: gtCase("tier = 1\nchecks_enable = check-ground-truth bogus-check\n"+gtRC[len("tier = 3\n"):], "test_files = 9\n", nil),
	},
	{name: "no METRICS file", files: gtCase(gtRC, "", nil)},
	{name: "METRICS.md", files: gtCase(gtRC, "", map[string]string{"METRICS.md": "# Metrics\n\ntest_files = 2\nsdks >= 1\n"})},
	{name: "METRICS wins over METRICS.md", files: gtCase(gtRC, "test_files = 2\n", map[string]string{"METRICS.md": "test_files = 7\n"})},
	{name: "no metrics declared", files: gtCase("tier = 3\n", gtMetrics, nil)},
	{name: "no .rebarrc", files: gtCase("", gtMetrics, nil)},
	{
		name: "drift, floors, new keys",
		files: gtCase(gtRC, "# comment = 1\n  test_files >= 1\nsdks >= 3\nagents=5\ncontract_files =3\ncontract_ids >= two\n"+
			"headed = 1 # trailing comment\ngo_all = \"3\"\nnothing ==0\n", nil),
	},
	{
		name:  "METRICS values: quotes, escapes, CRLF, a later duplicate",
		files: gtCase(gtRC, "test_files = 'a\\tb'\nsdks = 2\r\nagents = \"2\"\ncontract_files = it\\'s\ncontract_ids = 2\ncontract_ids = 9\nheaded = 'x\\\\y'\ngo_all = '\\x41'\nnothing = 0\n", nil),
	},
	{
		name:  "a floor that overflows",
		files: gtCase(gtRC, "test_files >= 99999999999999999999\nsdks >= 0002\n", nil),
	},
	{name: "REBAR_RCFILE", files: gtCase(gtRC, "only = 1\n", map[string]string{"alt.rc": "metric.only.paths = api\nmetric.only.names = main.go\n"}), env: []string{"REBAR_RCFILE=$ROOT/alt.rc"}},
	{
		name: "find details",
		files: gtCase(`metric.all.paths = .
metric.dirs_all.type = dirs
metric.dirs_all.paths = sdk
metric.twice.paths = api api
metric.git_file.names = .git
metric.excluded_start.paths = api/node_modules
metric.excluded_start.exclude_dirs = node_modules
metric.shallow_prune.paths = api
metric.shallow_prune.min_depth = 2
metric.shallow_prune.exclude_dirs = node_modules a
metric.bad_depth.max_depth = one
metric.neg_depth.min_depth = -1
metric.brackets.names = [!o]*_test.go
metric.contains_dirs.type = dirs
metric.contains_dirs.contains = .
metric.contains_bad.contains = (
metric.contains_line.contains = ^package x$
metric.distinct_optional.paths = architecture
metric.distinct_optional.distinct = ^CONTRACT-(S)?.*
metric.symlinks.paths = links
metric.symlinked_start.paths = linkdir
metric.trailing_slash.type = dirs
metric.trailing_slash.paths = sdk/
metric.trailing_slash.max_depth = 0
metric.dot_names.names = .*
metricXodd.names = *.md
`, "", map[string]string{
			"sub/.git":        "gitdir: elsewhere\n",
			".git/config":     "[core]\n",
			"links/real.txt":  "x\n",
			"api/b/crlf.go":   "package x\r\n",
			"api/b/notpkg.go": "package xy\n",
		}),
		links: map[string]string{"links/link.txt": "real.txt", "links/dirlink": "../sdk", "linkdir": "sdk"},
		args:  []string{"--print"},
	},
	{
		name:    "metrics_strict: a key missing from METRICS fails",
		files:   gtCase(gtRC+"metrics_strict = true\n", "test_files = 2\n", nil),
		pending: "metrics_strict",
	},
	{
		name:    "metrics_strict: a missing METRICS fails",
		files:   gtCase(gtRC, "", nil),
		env:     []string{"REBAR_METRICS_STRICT=yes"},
		pending: "metrics_strict",
	},
	{
		name:    "metrics_strict: every key present passes",
		files:   gtCase(gtRC+"metrics_strict = on\n", gtMetrics, nil),
		pending: "metrics_strict",
	},
	{
		name:    "metrics_strict off by env",
		files:   gtCase(gtRC+"metrics_strict = true\n", "test_files = 2\n", nil),
		env:     []string{"REBAR_METRICS_STRICT=false"},
		pending: "metrics_strict",
	},
}

func TestGroundTruthParity(t *testing.T) {
	runParity(t, groundTruthCheck, groundTruthParityCases)
}

func TestGroundTruthGolden(t *testing.T) {
	runGolden(t, groundTruthCheck, groundTruthParityCases)
}
