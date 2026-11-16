// Package contracts finds and parses a project's contract files
// (<contracts_dir>/CONTRACT-<ID>.<major>.<minor>.md) and the CONTRACT:
// references to them in source, the way rebar's check scripts do. It is
// shared by the native checks (cli/internal/checks) that replace
// check-contract-refs, check-registry, check-jtbd-presence,
// check-prefix-uniqueness and, later in the stack, compute-registry.
package contracts

import (
	"os"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/config"
)

// Marker starts every contract reference in source. It is spelled in two
// parts here, and in the patterns below, so that this file is not itself a
// reference to scan.
const Marker = "CONTRACT" + ":"

// Dir is the project's contracts directory as the scripts read it
// (_rebar_contracts_dir in scripts/_rebar-config.sh): env
// REBAR_CONTRACTS_DIR, then .rebarrc contracts_dir, then architecture,
// normalized by NormDir. Unlike config.ContractsDirFor it keeps an
// absolute value or one that leaves the repo, as the scripts do.
func Dir(root string) string {
	d := NormDir(config.RCValueOr(root, "contracts_dir", "REBAR_CONTRACTS_DIR", config.DefaultContractsDir))
	if d == "" {
		return config.DefaultContractsDir
	}
	return d
}

// NormDir drops leading "./" and trailing "/" from a relative directory
// (./docs/specs/ → docs/specs; "./" → ""), keeping a lone "/". Mirrors
// _rebar_norm_dir.
func NormDir(d string) string {
	for strings.HasPrefix(d, "./") {
		d = d[2:]
	}
	for d != "/" && strings.HasSuffix(d, "/") {
		d = d[:len(d)-1]
	}
	return d
}

// List returns the names in dir that the scripts' glob
// "$dir"/CONTRACT-*.md matches, in byte order (bash sorts a glob by the
// locale's collation, which is byte order in the C locale). Entries of any
// type are listed, as the glob lists them; callers check for regular
// files. A missing directory lists nothing.
func List(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if len(n) >= len("CONTRACT-.md") && strings.HasPrefix(n, "CONTRACT-") && strings.HasSuffix(n, ".md") {
			names = append(names, n)
		}
	}
	return names
}

// IsIndex reports whether a CONTRACT-*.md name is one of the files that
// live beside the contracts without being one: the templates, the registry
// (and its template) and the gap tracker.
func IsIndex(name string) bool {
	switch name {
	case "CONTRACT-TEMPLATE.md", "CONTRACT-SEAM-TEMPLATE.md", "CONTRACT-REGISTRY.md",
		"CONTRACT-REGISTRY.template.md", "CONTRACT-GAPS.md":
		return true
	}
	return false
}

// IsCompanion reports whether a contract file name is a companion
// (CONTRACT-<ID>.<v>.impl.md), not a contract.
func IsCompanion(name string) bool {
	return strings.HasSuffix(name, ".impl.md")
}

var (
	idMajorMinorRE = regexp.MustCompile(`^(.+)\.([0-9]+)\.([0-9]+)$`)
	idMajorRE      = regexp.MustCompile(`^(.+)\.([0-9]+)$`)
)

// ParseName splits a contract file name (CONTRACT-S1-FOO.2.1.md) into its
// ID and version numbers, as compute-registry.sh and check-jtbd-presence.sh
// do: <ID>.<major>.<minor>, else <ID>.<major> (minor 0), else the whole
// stem is the ID (version 1.0). The numbers keep their digits as written.
func ParseName(name string) (id, major, minor string) {
	stem := strings.TrimSuffix(strings.TrimPrefix(name, "CONTRACT-"), ".md")
	if m := idMajorMinorRE.FindStringSubmatch(stem); m != nil {
		return m[1], m[2], m[3]
	}
	if m := idMajorRE.FindStringSubmatch(stem); m != nil {
		return m[1], m[2], "0"
	}
	return stem, "1", "0"
}

// RefPrefix is the regular expression (POSIX ERE, also valid Go syntax) for
// the start of a reference to one of this repo's contracts, to which a
// caller appends the ID pattern: the marker alone without a namespace,
// with one also the marker followed by "<namespace>:". Mirrors
// _rebar_contract_ref_ere.
func RefPrefix(namespace string) string {
	if namespace == "" {
		return Marker
	}
	// `.` is the only metacharacter a valid namespace can contain.
	return Marker + "(" + strings.ReplaceAll(namespace, ".", `\.`) + ":)?"
}

// refRE is a reference as check-contract-refs.sh greps for it: the marker,
// then ID.major.minor where every part may be empty.
var refRE = mustLongest(Marker + `[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*`)

// FirstRef returns the first reference in text as check-contract-refs.sh
// extracts it (`grep -o <ref> | head -1 | sed 's/<marker>//'`): the text
// after the marker, or "" when there is none.
func FirstRef(text string) string {
	m := refRE.FindString(text)
	return strings.TrimPrefix(m, Marker)
}

// HasRef reports whether text holds a reference that FirstRef would find.
func HasRef(text string) bool {
	return refRE.MatchString(text)
}

// qualifiedSearchRE is check-contract-refs.sh's search pattern with a
// namespace configured: references with or without a namespace.
var qualifiedSearchRE = mustLongest(Marker + `([A-Za-z0-9._/-]+:)?[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*`)

// HasQualifiedRef reports whether text holds a reference with or without a
// namespace, the lines check-contract-refs.sh scans when a namespace is set.
func HasQualifiedRef(text string) bool {
	return qualifiedSearchRE.MatchString(text)
}

var (
	shortRefRE   = mustLongest(Marker + `([A-Za-z0-9_-]+\.[0-9]+\.[0-9]+)`)
	foreignRefRE = mustLongest(Marker + `([A-Za-z0-9._/-]+:[A-Za-z0-9][A-Za-z0-9_-]*\.[0-9])`)
)

// crossMarker replaces the marker of a reference qualified with another
// repo's namespace.
const crossMarker = "CROSSREF:"

// Localize rewrites the references in text for a repo whose namespace is
// namespace (non-empty), as check-contract-refs.sh's sed does: this repo's
// "<marker><namespace>:" becomes the bare marker, and a reference qualified
// with any other namespace gets the marker CROSSREF: so FirstRef skips it.
// A would-be namespace that starts like <ID>.<v> is not one (in
// <ID>.<v>:see.2 the reference is <ID>.<v>). cross is the number of
// CROSSREF: in the result, as the script counts them.
func Localize(text, namespace string) (out string, cross int) {
	text = strings.ReplaceAll(text, Marker+namespace+":", Marker)
	text = shortRefRE.ReplaceAllString(text, "CONTRACT~${1}")
	text = foreignRefRE.ReplaceAllString(text, crossMarker+"${1}")
	text = strings.ReplaceAll(text, "CONTRACT~", Marker)
	return text, strings.Count(text, crossMarker)
}

// mustLongest compiles a POSIX ERE with grep's and sed's leftmost-longest
// matching.
func mustLongest(expr string) *regexp.Regexp {
	return regexp.MustCompilePOSIX(expr)
}
