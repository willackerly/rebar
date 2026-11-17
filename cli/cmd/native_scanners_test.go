package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// Parity of the scanner checks ported in S1 (runParity). Contract
// references in fixture bodies are spelled @contract: (testutil.Tags).

// contract is a contract file body with every JTBD section filled in.
const contract = "# C\n\n## Why this exists\n\nReason.\n\n## Who needs this\n\nUs.\n\n## Scenarios\n\nOne.\n"

func TestFreshnessParity(t *testing.T) {
	staleLine := regexp.MustCompile(`^STALE \((?:-?[0-9]+|N) days\): (.*) — `)
	// Fixed dates (not {{ago:N}}) are a growing number of days old.
	fixedAge := regexp.MustCompile(`(?m)^STALE \(-?[0-9]+ days\)(: .* — [a-z-]+: (19[0-9]{2}|20[01][0-9])-)`)
	runParity(t, parityCheck{
		name:      "check-freshness",
		golden:    "check-freshness",
		normalize: func(out string) string { return fixedAge.ReplaceAllString(out, "STALE (N days)$1") },
		order: func(out, _ string, _ []string) string { return inWalkOrderBy(out, []string{"."}, staleLine, 1) },
	}, []parityCase{
		{
			name: "markers, vocabularies, exclusions",
			files: map[string]string{
				"fresh.md":            "<!-- freshness: {{ago:3}} -->\n",
				"stale.md":            "# S\nfreshness: {{ago:30}}\n",
				"docs/old.md":         "<!-- last-synced: {{ago:100}} -->\n",
				"docs/deep/both.md":   "last-synced: {{ago:1}}\nfreshness: {{ago:20}}\n",
				"first.md":            "freshness: {{ago:2}} then freshness: {{ago:40}}\n",
				"archived.md":         "freshness: archived\nfreshness: {{ago:300}}\n",
				"templates/t.md":      "freshness: {{ago:300}}\n",
				"sub/templates/t.md":  "freshness: {{ago:300}}\n",
				"placeholder.md":      "freshness: YYYY-MM-DD\n",
				"zero.md":             "freshness: 0000-00-00\n",
				"month13.md":          "freshness: 2025-13-01\n",
				"spaces.md":           "freshness:  {{ago:300}}\n",
				"edge14.md":           "freshness: {{ago:14}}\n",
				"edge15.md":           "freshness: {{ago:15}}\n",
				"future.md":           "freshness: {{ago:-5}}\n",
				"crlf.md":             "x\r\nfreshness: {{ago:50}}\r\n",
				"node_modules/x.md":   "freshness: {{ago:300}}\n",
				"a/node_modules/y.md": "freshness: {{ago:300}}\n",
				".git/z.md":           "freshness: {{ago:300}}\n",
				"sub/.git/w.md":       "freshness: {{ago:300}}\n",
				".hidden/h.md":        "freshness: {{ago:16}}\n",
				".md":                 "freshness: {{ago:17}}\n",
				"notes.txt":           "freshness: {{ago:300}}\n",
				"plain.md":            "no marker\n",
			},
		},
		{
			name:  "max age argument",
			files: map[string]string{"a.md": "freshness: {{ago:20}}\n", "b.md": "freshness: {{ago:31}}\n"},
			args:  []string{"30"},
		},
		{
			name:  "a max age that is not a number never reports",
			files: map[string]string{"a.md": "freshness: {{ago:200}}\n"},
			args:  []string{"many"},
		},
		{
			name: ".rebarrc freshness_max_days with a comment",
			files: map[string]string{
				".rebarrc": "freshness_max_days = 7 # a week\n",
				"a.md":     "freshness: {{ago:8}}\n",
				"b.md":     "freshness: {{ago:7}}\n",
			},
		},
		{
			name:  "tier 1 skips",
			files: map[string]string{".rebarrc": "tier = 1\n", "a.md": "freshness: {{ago:80}}\n"},
		},
		{
			name: "tier 1, named in checks_enable, with an unknown name",
			files: map[string]string{
				".rebarrc": "tier = 1\nchecks_enable = check-freshness.sh no-such-check\n",
				"a.md":     "freshness: {{ago:80}}\n",
			},
		},
		{
			name:  "REBAR_TIER and REBAR_CHECKS_ENABLE",
			files: map[string]string{".rebarrc": "tier = 3\n", "a.md": "freshness: {{ago:80}}\n"},
			env:   []string{"REBAR_TIER=1", "REBAR_CHECKS_ENABLE=check-freshness"},
		},
		{
			name:  "REBAR_TIER that is not a number runs the check",
			files: map[string]string{"a.md": "freshness: {{ago:80}}\n"},
			env:   []string{"REBAR_TIER=two"},
		},
		{
			name:  "a tier line with a comment reads as tier 3",
			files: map[string]string{".rebarrc": "tier = 1 # partial\n", "a.md": "freshness: {{ago:80}}\n"},
		},
		{
			name:  "a tab after the tier reads as tier 3",
			files: map[string]string{".rebarrc": "tier = 1\t\n", "a.md": "freshness: {{ago:80}}\n"},
		},
		{
			name:  "the first line starting with tier decides",
			files: map[string]string{".rebarrc": "tier_note = whatever\ntier = 1\n", "a.md": "freshness: {{ago:80}}\n"},
		},
		{
			name:  "REBAR_RCFILE",
			files: map[string]string{".rebarrc": "tier = 1\n", "alt.rc": "freshness_max_days = 100\n", "a.md": "freshness: {{ago:80}}\n"},
			env:   []string{"REBAR_RCFILE=$ROOT/alt.rc"},
		},
		{
			// Ages straddle at least one DST change (any 240 days do in
			// New York); the half-day rounding keeps them whole days.
			name: "a zone with DST",
			files: map[string]string{
				"a.md": "freshness: {{ago:240}}\n",
				"b.md": "freshness: {{ago:300}}\n",
				"c.md": "freshness: {{ago:301}}\n",
				"d.md": "freshness: {{ago:365}}\n",
				"e.md": "freshness: {{ago:120}}\n",
				"f.md": "freshness: {{ago:180}}\n",
			},
			args: []string{"300"},
			env:  []string{"TZ=America/New_York"},
		},
		{
			// Sydney is on summer time from October to April, so for most
			// of the year today and some of these dates are on different
			// offsets, in the other direction from New York's.
			name: "a southern zone with DST",
			files: map[string]string{
				"a.md": "freshness: {{ago:101}}\n",
				"b.md": "freshness: {{ago:150}}\n",
				"c.md": "freshness: {{ago:200}}\n",
				"d.md": "freshness: {{ago:250}}\n",
			},
			args: []string{"100"},
			env:  []string{"TZ=Australia/Sydney"},
		},
		{
			name:  "UTC, where 1970-01-01 is epoch 0",
			files: map[string]string{"a.md": "freshness: 1970-01-01\n", "b.md": "freshness: 1969-12-31\n"},
			env:   []string{"TZ=UTC"},
		},
		{
			name:  "a date whose midnight did not exist",
			files: map[string]string{"gap.md": "freshness: 2018-11-04\n", "before.md": "freshness: 2018-10-21\n"},
			env:   []string{"TZ=America/Sao_Paulo"},
		},
		{
			name:  "a binary file",
			files: map[string]string{"bin.md": "freshness: {{ago:300}}\n\x00\n", "text.md": "plain\n"},
			// BSD grep -o prints "Binary file … matches" on stdout, which
			// the script counts and cannot parse, as the native check;
			// GNU grep (3.5+) prints it on stderr, so the file counts as
			// unmarked.
			divergesOn: "gnu",
			want: `
Checked 1 docs with currency markers (freshness:/last-synced:), 0 stale (>14 days).
1 .md file(s) carry no currency marker at all — unchecked, not verified.
`,
		},
		{
			name:  "a day past the end of the month",
			files: map[string]string{"feb.md": "freshness: 2025-02-30\n"},
			// GNU date rejects it, as the native check; BSD date rolls it
			// over to March 2 and reports it stale.
			divergesOn: "bsd",
			want: `
Checked 1 docs with currency markers (freshness:/last-synced:), 0 stale (>14 days).
0 .md file(s) carry no currency marker at all — unchecked, not verified.
`,
		},
	})
}

func TestContractRefsParity(t *testing.T) {
	broken := regexp.MustCompile(`^BROKEN: ([^:]*):`)
	runParity(t, parityCheck{
		name:   "check-contract-refs",
		golden: "check-contract-refs",
		order: func(out, _ string, _ []string) string { return inWalkOrderBy(out, []string{"."}, broken, 2) },
		exercised: func(out string) bool {
			return strings.Contains(out, "Checked") || strings.Contains(out, "not found")
		},
	}, []parityCase{
		{
			name: "valid, broken and malformed references",
			files: map[string]string{
				"architecture/CONTRACT-S1-FOO.1.0.md": contract,
				"src/a.go":                            "package a\n// @contract:S1-FOO.1.0\n",
				"src/b/b.ts":                          "x\ny\n// @contract:S2-MISSING.2.1 (missing)\n",
				"src/two.go":                          "// @contract:S1-FOO.1.0 and @contract:S9-OTHER.1.0\n// @contract:S9-OTHER.1.0 then @contract:S1-FOO.1.0\n",
				"src/dots.py":                         "# @contract:.. no id\n# @contract:S1-FOO.1 one dot\n# @contract:S1-FOO.1.0.7 extra\n",
				"src/crlf.rs":                         "// @contract:S3-CRLF.1.0\r\n",
				"src/c.txt":                           "@contract:S4-TXT.1.0\n",
				"node_modules/x/d.js":                 "// @contract:S5-NODE.1.0\n",
				"vendor/e.go":                         "// @contract:S5-VENDOR.1.0\n",
				".claude/w/f.go":                      "// @contract:S5-CLAUDE.1.0\n",
				".github/g.js":                        "// @contract:S6-GITHUB.1.0\n",
				"distance.go":                         "// @contract:S7-DIST.1.0\n",
				"src/bin.go":                          "// @contract:S8-BIN.1.0\n\x00\n",
			},
		},
		{
			name:  "no contracts directory",
			files: map[string]string{"src/a.go": "// @contract:S1-FOO.1.0\n"},
		},
		{
			name: "contracts_dir with ./ and a trailing slash",
			files: map[string]string{
				".rebarrc":                     "contracts_dir = ./specs/   # moved\n",
				"specs/CONTRACT-S1-FOO.1.0.md": contract,
				"src/a.go":                     "// @contract:S1-FOO.1.0\n// @contract:S2-BAR.1.0\n",
			},
		},
		{
			name: "an argument is used as given",
			files: map[string]string{
				"docs/CONTRACT-S1-FOO.1.0.md": contract,
				"src/a.go":                    "// @contract:S1-FOO.1.0\n// @contract:S2-BAR.1.0\n",
			},
			args: []string{"docs/"},
		},
		{
			name: "REBAR_CONTRACTS_DIR",
			files: map[string]string{
				"specs/CONTRACT-S1-FOO.1.0.md": contract,
				"src/a.go":                     "// @contract:S1-FOO.1.0\n",
			},
			env: []string{"REBAR_CONTRACTS_DIR=specs"},
		},
		{
			name: "contract_namespace",
			files: map[string]string{
				".rebarrc":                            "contract_namespace = github.com/acme/app/\n",
				"architecture/CONTRACT-S1-FOO.1.0.md": contract,
				"src/a.go": "// @contract:github.com/acme/app:S1-FOO.1.0\n" +
					"// @contract:github.com/other/x:S2-BAR.1.0 and @contract:other:S3-BAZ.2.1\n" +
					"// @contract:S1-FOO.1.0:see.2\n" +
					"// @contract:github.com/acme/app:S9-GONE.1.0\n" +
					"// @contract:github.comXacme/app:S1-FOO.1.0\n" +
					"// @contract:other:S2-BAR.1.0 then @contract:S1-FOO.1.0\n",
			},
		},
		{
			name: "an invalid contract_namespace is ignored",
			files: map[string]string{
				".rebarrc":                            "contract_namespace = S1-FOO.1.0\n",
				"architecture/CONTRACT-S1-FOO.1.0.md": contract,
				"src/a.go":                            "// @contract:S1-FOO.1.0\n// @contract:x:S2-BAR.1.0\n",
			},
		},
		{
			name: "source_extensions and source_exclude_dirs",
			files: map[string]string{
				".rebarrc":                            "source_extensions = .go sh\nsource_exclude_dirs = gen*\n",
				"architecture/CONTRACT-S1-FOO.1.0.md": contract,
				"tools/x.sh":                          "# @contract:S2-SH.1.0\n",
				"src/a.ts":                            "// @contract:S3-TS.1.0\n",
				"generated/b.go":                      "// @contract:S4-GEN.1.0\n",
				"node_modules/c.go":                   "// @contract:S5-NODE.1.0\n",
			},
		},
	})
}

func TestRegistryParity(t *testing.T) {
	runParity(t, parityCheck{
		name:      "check-registry",
		golden:    "check-registry",
		exercised: func(out string) bool { return strings.Contains(out, "===") || strings.Contains(out, "not found") },
	}, []parityCase{
		{
			name: "consistent",
			files: map[string]string{
				"architecture/CONTRACT-S1-FOO.1.0.md":    contract,
				"architecture/CONTRACT-REGISTRY.md":      "- CONTRACT-S1-FOO.1.0.md\n",
				"architecture/CONTRACT-TEMPLATE.md":      "template\n",
				"architecture/CONTRACT-GAPS.md":          "gaps\n",
				"architecture/CONTRACT-SEAM-TEMPLATE.md": "seam\n",
				"src/a.go":                               "// @contract:S1-FOO.1.0\n",
			},
		},
		{
			name: "not in the registry, orphans, tracked in TODO.md",
			files: map[string]string{
				"architecture/CONTRACT-S1-FOO.1.0.md":         contract,
				"architecture/CONTRACT-C2-BAR.1.0.md":         contract,
				"architecture/CONTRACT-I3-BAZ.2.0.md":         contract,
				"architecture/CONTRACT-A-B.1.0.md":            contract,
				"architecture/CONTRACT-A.1.0.md":              contract,
				"architecture/CONTRACT-REGISTRY.md":           "- CONTRACT-S1-FOO.1.0.md\n- CONTRACT-C2-BAR.1.0.md\n- CONTRACT-AxB.1.0.md\n",
				"architecture/CONTRACT-REGISTRY.template.md":  "x\n",
				"architecture/CONTRACT-DIR.1.0.md/readme.txt": "a directory\n",
				"TODO.md":  "- implement C2-BAR.1.0\n",
				"src/a.go": "// @contract:S1-FOO.1.0\n",
			},
		},
		{
			name: "what counts as implementing code",
			files: map[string]string{
				"architecture/CONTRACT-S1-ARCH.1.0.md":   contract,
				"architecture/CONTRACT-S2-DOT.1.0.md":    contract,
				"architecture/CONTRACT-S3-BIN.1.0.md":    contract,
				"architecture/CONTRACT-S4-VEND.1.0.md":   contract,
				"architecture/CONTRACT-S5-MD.1.0.md":     contract,
				"architecture/CONTRACT-S6-SH.1.0.md":     contract,
				"architecture/CONTRACT-S7-MJS.1.0.md":    contract,
				"architecture/CONTRACT-S8-BINARY.1.0.md": contract,
				"architecture/notes.go":                  "// @contract:S1-ARCH.1.0\n",
				"src/dot.go":                             "// @contract:S2-DOTx1y0\n",
				"bin/tool":                               "#!/bin/sh\n# @contract:S3-BIN.1.0\n",
				"vendor/x.go":                            "// @contract:S4-VEND.1.0\n",
				"docs/a.md":                              "@contract:S5-MD.1.0\n",
				"scripts/x.sh":                           "# @contract:S6-SH.1.0\n",
				"web/x.mjs":                              "// @contract:S7-MJS.1.0\n",
				"bin/blob":                               "@contract:S8-BINARY.1.0\n\x00\n",
			},
		},
		{
			name: "no registry file",
			files: map[string]string{
				"architecture/CONTRACT-S1-FOO.1.0.md": contract,
				"src/a.go":                            "// @contract:S1-FOO.1.0\n",
			},
		},
		{
			name:  "no contracts directory",
			files: map[string]string{"src/a.go": "package a\n"},
		},
		{
			name: "an argument is normalized",
			files: map[string]string{
				"specs/CONTRACT-S1-FOO.1.0.md": contract,
				"specs/CONTRACT-REGISTRY.md":   "nothing listed\n",
				"specs/impl.go":                "// @contract:S1-FOO.1.0\n",
			},
			args: []string{"./specs/"},
		},
		{
			name: "contract_namespace and excluded directories",
			files: map[string]string{
				".rebarrc":                            "contract_namespace = acme\nsource_exclude_dirs = gen\n",
				"architecture/CONTRACT-S1-FOO.1.0.md": contract,
				"architecture/CONTRACT-S2-BAR.1.0.md": contract,
				"architecture/CONTRACT-S3-GEN.1.0.md": contract,
				"src/a.go":                            "// @contract:acme:S1-FOO.1.0\n// @contract:other:S2-BAR.1.0\n",
				"src/gen/b.go":                        "// @contract:S3-GEN.1.0\n",
			},
		},
	})
}

func TestContractHeadersParity(t *testing.T) {
	missing := regexp.MustCompile(`^MISSING: (.*)$`)
	runParity(t, parityCheck{
		name:   "check-contract-headers",
		golden: "check-contract-headers",
		order: func(out, root string, args []string) string {
			operands := args
			if len(operands) == 0 {
				for _, d := range strings.Fields(config.RCValueOr(root, "source_dirs", "", sourcescan.DefaultSourceDirs)) {
					operands = append(operands, d)
				}
			}
			return inWalkOrderBy(out, operands, missing, 1)
		},
	}, []parityCase{
		{
			name: "default scope and exemptions",
			files: map[string]string{
				"src/a.go":                   strings.Repeat("x\n", 14) + "// @contract:S1-FOO.1.0\n",
				"src/b.go":                   strings.Repeat("x\n", 15) + "// @contract:S1-FOO.1.0\n",
				"src/b/c.go":                 "package c\n",
				"src/c_test.go":              "package c\n",
				"src/d.pb.go":                "package d\n",
				"src/gen/x.gen.ts":           "x\n",
				"src/vendor/v.go":            "package v\n",
				"lib/build/b.js":             "x\n",
				"lib/dist/b.js":              "x\n",
				"lib/ok.js":                  "// Architecture: see the README\n",
				"internal/e.py":              "# Architecture: none\n",
				"cmd/f.rs":                   "fn main() {}\n",
				"cmd/g.txt":                  "no header\n",
				"cmd/.hidden.go":             "package h\n",
				"app/h_generated.go":         "package h\n",
				"packages/p_generated/x.go":  "package x\n",
				"packages/p/y.spec.ts":       "x\n",
				"packages/p/z.tsx":           "// @contract:S1-FOO.1.0\r\n",
				"client/i.jsx":               "x\n",
				"client/node_modules/j/k.js": "x\n",
				"tools/l.go":                 "not scanned\n",
				"src/empty.go":               "",
				"src/bin.go":                 "\x00// @contract:S1-FOO.1.0\n",
			},
		},
		{
			name: "operands: a trailing slash, a file, a missing one, vendor",
			files: map[string]string{
				"src/a.go":    "package a\n",
				"lib/x.go":    "package x\n",
				"lib/y.go":    "package y\n",
				"vendor/v.go": "package v\n",
			},
			args: []string{"src/", "lib/x.go", "missing", "vendor"},
		},
		{
			name: "CONTRACT_EXTENSIONS overrides .rebarrc",
			files: map[string]string{
				".rebarrc":  "source_extensions = go\n",
				"src/a.go":  "package a\n",
				"src/b.txt": "no header\n",
			},
			env: []string{"CONTRACT_EXTENSIONS=.txt"},
		},
		{
			name: ".rebarrc source_dirs and source_extensions",
			files: map[string]string{
				".rebarrc":   "source_dirs = pkg ./tools/\nsource_extensions = go sh\n",
				"pkg/a.go":   "package a\n",
				"tools/b.sh": "#!/bin/sh\n",
				"src/c.go":   "package c\n",
			},
		},
		{
			name:  "tier 1 skips",
			files: map[string]string{".rebarrc": "tier = 1\n", "src/a.go": "package a\n"},
		},
		{
			name:  "tier 1, named in checks_enable",
			files: map[string]string{".rebarrc": "tier = 1\nchecks_enable = check-contract-headers\n", "src/a.go": "package a\n"},
		},
		{
			name:  "the namespaced form in the hint",
			files: map[string]string{".rebarrc": "contract_namespace = github.com/acme/app\n", "src/a.go": "package a\n"},
		},
		{
			name:  "an invalid namespace is warned about",
			files: map[string]string{".rebarrc": "contract_namespace = bad name\n", "src/a.go": "package a\n"},
		},
		{
			name:  "no source directories",
			files: map[string]string{"tools/a.go": "package a\n"},
		},
		{
			name:  "a symlinked source directory is not followed",
			files: map[string]string{"real/a.go": "package a\n"},
			links: map[string]string{"src": "real"},
		},
	})
}

// registry is a CONTRACT-REGISTRY.md listing the given contract files.
func registry(names ...string) string {
	var b strings.Builder
	b.WriteString("# Contract Registry\n\n## Contract Files\n\n")
	for _, n := range names {
		b.WriteString("- " + n + "\n")
	}
	return b.String()
}

func TestPrefixUniquenessParity(t *testing.T) {
	collisions := registry("CONTRACT-S1-STEWARD.1.0.md", "CONTRACT-S1-STEWARD.2.0.md", "CONTRACT-I3-SCANNER.1.0.md",
		"CONTRACT-I3-LLM-CLIENT.1.0.md", "CONTRACT-I3-ZED.2.md", "CONTRACT-SEAM-X.1.0.md", "CONTRACT-C12-B.1.0.md",
		"CONTRACT-C12-A.1.0.md", "CONTRACT-NOVERSION.md", "CONTRACT-S1-STEWARD.1.0.md", "CONTRACT-p4-low.1.0.md",
		"CONTRACT-P4-UP.1.0.md", "CONTRACT-I3.1.0.md") + "  - CONTRACT-I3-INDENTED.1.0.md\n- CONTRACT-I3-CRLF.1.0.md\r\n"
	runParity(t, parityCheck{name: "check-prefix-uniqueness", golden: "check-prefix-uniqueness"}, []parityCase{
		{name: "no registry", files: map[string]string{"architecture/README.md": "x\n"}},
		{name: "collisions block at tier 3", files: map[string]string{"architecture/CONTRACT-REGISTRY.md": collisions}},
		{name: "advisory at tier 2", files: map[string]string{".rebarrc": "tier = 2\n", "architecture/CONTRACT-REGISTRY.md": collisions}},
		{name: "tier 1 skips", files: map[string]string{".rebarrc": "tier = 1\n", "architecture/CONTRACT-REGISTRY.md": collisions}},
		{
			name:  "named in checks_enable at tier 1: blocking",
			files: map[string]string{".rebarrc": "tier = 1\nchecks_enable = check-prefix-uniqueness\n", "architecture/CONTRACT-REGISTRY.md": collisions},
		},
		{
			name:  "SKIP_PREFIX_UNIQUENESS",
			files: map[string]string{"architecture/CONTRACT-REGISTRY.md": collisions},
			env:   []string{"SKIP_PREFIX_UNIQUENESS=1"},
		},
		{name: "nothing numbered", files: map[string]string{"architecture/CONTRACT-REGISTRY.md": registry("CONTRACT-SEAM-A.1.0.md", "CONTRACT-STEWARD.1.0.md")}},
		{name: "unique", files: map[string]string{"architecture/CONTRACT-REGISTRY.md": registry("CONTRACT-S1-A.1.0.md", "CONTRACT-S2-B.1.0.md", "CONTRACT-S1-A.2.0.md")}},
		{
			name:  "contracts_dir",
			files: map[string]string{".rebarrc": "contracts_dir = specs\n", "specs/CONTRACT-REGISTRY.md": collisions, "architecture/CONTRACT-REGISTRY.md": registry()},
		},
		{
			name:  "REBAR_TIER that is not a number: advisory",
			files: map[string]string{"architecture/CONTRACT-REGISTRY.md": collisions},
			env:   []string{"REBAR_TIER=x"},
		},
	})
}

func TestJTBDPresenceParity(t *testing.T) {
	sections := func(why, who, scen string) string {
		return "# C\n\n## Why this exists\n" + why + "\n## Who needs this\n" + who + "\n## Scenarios\n" + scen
	}
	mixed := map[string]string{
		"architecture/CONTRACT-A.1.0.md":         contract,
		"architecture/CONTRACT-B.1.0.md":         "# old B, no sections\n",
		"architecture/CONTRACT-B.2.0.md":         sections("x\n", "y\n", "\n   \n"),
		"architecture/CONTRACT-C.1.0.md":         "SUPERSEDED BY: CONTRACT-C.2.0\n",
		"architecture/CONTRACT-D.1.0.md":         sections("<!-- guidance -->\n", "<!-- multi\nline\ncomment -->\n  \t\n", "<!-- a --> <!-- b -->\n"),
		"architecture/CONTRACT-E.1.0.md":         sections("### Sub\n", "<!-- a --> real\n", "text <!-- open\nclose --> \n"),
		"architecture/CONTRACT-F.1.9.md":         contract,
		"architecture/CONTRACT-F.1.10.md":        "# F latest, empty\n",
		"architecture/CONTRACT-G.1.0.md":         contract,
		"architecture/CONTRACT-G.md":             "# G unversioned wins the tie\n",
		"architecture/CONTRACT-H.2.md":           sections("a\n", "b\n", "c"),
		"architecture/CONTRACT-I.01.0.md":        "# I.01.0 loses the tie\n",
		"architecture/CONTRACT-I.1.0.md":         contract,
		"architecture/CONTRACT-J.1.0.md":         "## Why this exists\n# Top heading ends it\ncontent\n## Who needs this\nx\n## Scenarios\ny\n",
		"architecture/CONTRACT-K.1.0.md":         "## Why this exists (draft)\nx\n### Who needs this\n## Who needs this\r\n\r\n## Scenarios\r\nz\r\n",
		"architecture/CONTRACT-L.1.0.md":         "## Why this exists\n<!-- open\n## Who needs this\nstill a comment -->\n## Scenarios\nok\n",
		"architecture/CONTRACT-M.1.0.md":         "",
		"architecture/CONTRACT-REGISTRY.md":      "registry\n",
		"architecture/CONTRACT-REGISTRY.old.md":  "registry\n",
		"architecture/CONTRACT-TEMPLATE.md":      "template\n",
		"architecture/CONTRACT-SEAM-TEMPLATE.md": "template\n",
		"architecture/CONTRACT-GAPS.md":          "gaps\n",
		"architecture/CONTRACT-X.1.0.impl.md":    "companion\n",
	}
	with := func(files map[string]string, extra ...string) map[string]string {
		out := map[string]string{}
		for k, v := range files {
			out[k] = v
		}
		for i := 0; i+1 < len(extra); i += 2 {
			out[extra[i]] = extra[i+1]
		}
		return out
	}
	runParity(t, parityCheck{name: "check-jtbd-presence", golden: "check-jtbd-presence"}, []parityCase{
		{name: "no contracts directory", files: map[string]string{"README.md": "x\n"}},
		{name: "no contracts", files: map[string]string{"architecture/CONTRACT-TEMPLATE.md": "x\n", "architecture/README.md": "x\n"}},
		{name: "sections, versions, comments", files: mixed},
		{name: "advisory at tier 2", files: with(mixed, ".rebarrc", "tier = 2\n")},
		{name: "tier 1 skips", files: with(mixed, ".rebarrc", "tier = 1\n")},
		{name: "named in checks_enable at tier 1", files: with(mixed, ".rebarrc", "tier = 1\nchecks_enable = check-jtbd-presence\n")},
		{name: "SKIP_JTBD", files: mixed, env: []string{"SKIP_JTBD=1"}},
		{name: "all present", files: map[string]string{"architecture/CONTRACT-A.1.0.md": contract}},
		{
			name:  "contracts_dir",
			files: map[string]string{".rebarrc": "contracts_dir = docs/specs/\n", "docs/specs/CONTRACT-A.1.0.md": "# empty\n"},
		},
	})
}
