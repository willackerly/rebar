package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/willackerly/rebar/cli/internal/config"
)

// The tier gate: Go twins of _rebar_tier, _rebar_check_tier,
// _rebar_checks_enable_warn and _rebar_skip in scripts/_rebar-config.sh,
// with the same output. Tiers are kept as the text the scripts compare,
// since REBAR_TIER is used as given (a value that is not a number makes
// the scripts' `[ "$tier" -lt N ]` fail, so the check runs).

// Tier is the configured enforcement tier as _rebar_tier reads it:
// REBAR_TIER when set, as given; else the first line of the .rebarrc
// (config.RCFile) that starts with "tier", taken after its last "=" with
// spaces dropped, when that is 1, 2 or 3; else "3". (So `tier = 1 # note`
// reads as 3, as it does in the scripts.)
func Tier(root string) string {
	if v := os.Getenv("REBAR_TIER"); v != "" {
		return v
	}
	if data, err := os.ReadFile(config.RCFile(root)); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "tier") {
				continue
			}
			v := line
			if i := strings.LastIndex(v, "="); i >= 0 {
				v = strings.TrimLeft(v[i+1:], posixSpace)
			}
			if v = strings.ReplaceAll(v, " ", ""); v == "1" || v == "2" || v == "3" {
				return v
			}
			break
		}
	}
	return "3"
}

// CheckTier is the tier check runs at: "3" when .rebarrc checks_enable
// (env REBAR_CHECKS_ENABLE) names it, else Tier. Mirrors _rebar_check_tier.
func CheckTier(root, check string) string {
	if slices.Contains(config.ChecksEnableFor(root), strings.TrimSuffix(check, ".sh")) {
		return "3"
	}
	return Tier(root)
}

// Skip reports whether check must not run because the tier is below
// minTier, printing the scripts' SKIP line (or, when checks_enable names
// the check, their NOTE line and false). It first warns about unknown
// checks_enable names, once per run (WarnChecksEnable). Mirrors _rebar_skip.
func Skip(ctx *Context, minTier int, check string) bool {
	WarnChecksEnable(ctx)
	tier := Tier(ctx.Root)
	if n, ok := TestInt(tier); !ok || n >= int64(minTier) {
		return false
	}
	if n, ok := TestInt(CheckTier(ctx.Root, check)); ok && n >= int64(minTier) {
		fmt.Fprintf(ctx.Stdout, "NOTE: tier %s < required tier %d; running %s because .rebarrc checks_enable names it\n", tier, minTier, check)
		return false
	}
	fmt.Fprintf(ctx.Stdout, "SKIP: tier %s < required tier %d (set REBAR_TIER or .rebarrc to change, or name it in checks_enable)\n", tier, minTier)
	return true
}

// WarnChecksEnable warns on ctx.Stderr about each checks_enable name that is
// not a check, once per run: like _rebar_checks_enable_warn it does nothing
// when _REBAR_CHECKS_VALIDATED is set, and sets it, so the checks and
// scripts started after it stay quiet.
func WarnChecksEnable(ctx *Context) {
	if os.Getenv("_REBAR_CHECKS_VALIDATED") != "" {
		return
	}
	os.Setenv("_REBAR_CHECKS_VALIDATED", "1")
	for _, name := range config.UnknownChecks(config.ChecksEnableFor(ctx.Root)) {
		fmt.Fprintf(ctx.Stderr, "rebar: WARN: checks_enable names unknown check '%s' (ignored; use a check script name without .sh, e.g. check-ground-truth)\n", name)
	}
}

// ContractNamespace is the project's contract namespace, "" when unset or
// invalid; an invalid value is warned about on ctx.Stderr once per run
// (_REBAR_NAMESPACE_VALIDATED, as _rebar_contract_namespace_warn). Mirrors
// _rebar_contract_namespace.
func ContractNamespace(ctx *Context) string {
	ns := config.ContractNamespaceValue(ctx.Root)
	if ns == "" || config.ValidContractNamespace(ns) {
		return ns
	}
	if os.Getenv("_REBAR_NAMESPACE_VALIDATED") == "" {
		os.Setenv("_REBAR_NAMESPACE_VALIDATED", "1")
		fmt.Fprintf(ctx.Stderr, "rebar: ignoring contract_namespace '%s' (allowed: letters, digits, . _ / -; must not start like a contract ID.version)\n", ns)
	}
	return ""
}

// posixSpace is the [[:space:]] class.
const posixSpace = " \t\n\v\f\r"

// TestInt parses s as test(1)'s -lt / -ge do in bash: optional surrounding
// whitespace, an optional sign, decimal digits. ok is false for anything
// else, where the script's comparison fails.
func TestInt(s string) (int64, bool) {
	n, err := strconv.ParseInt(strings.Trim(s, posixSpace), 10, 64)
	return n, err == nil
}

// path resolves a path the script would use relative to its working
// directory. An empty path stays empty: "" names no file.
func (c *Context) path(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.WorkDir(), p)
}

// isFile is the scripts' [ -f p ]: a regular file, through symlinks.
func (c *Context) isFile(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(c.path(p))
	return err == nil && info.Mode().IsRegular()
}

// isDir is the scripts' [ -d p ].
func (c *Context) isDir(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(c.path(p))
	return err == nil && info.IsDir()
}
