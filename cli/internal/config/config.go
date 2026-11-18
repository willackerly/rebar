package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Config struct {
	RepoRoot   string
	RebarDir   string // .rebar/
	Tier       int    // 1, 2, or 3
	Version    string // from .rebar-version
	ScriptsDir string // scripts/
	AgentsDir  string // agents/
	BinDir     string // bin/
	// ContractsDir holds the contracts (CONTRACT-*.md, CONTRACT-REGISTRY.md,
	// the steward's .state/), relative to RepoRoot. It names the contracts
	// location only: some repos keep other design docs in architecture/.
	ContractsDir string
	// ChecksEnable lists the checks .rebarrc checks_enable force-enables
	// below their tier (see ChecksEnableFor).
	ChecksEnable []string
}

// DefaultContractsDir is where contracts live unless .rebarrc says otherwise.
const DefaultContractsDir = "architecture"

// Load reads configuration from .rebarrc and .rebar-version, respecting
// REBAR_TIER env override. Mirrors the logic in scripts/_rebar-config.sh.
func Load(repoRoot string) (*Config, error) {
	c := &Config{
		RepoRoot:   repoRoot,
		RebarDir:   filepath.Join(repoRoot, ".rebar"),
		ScriptsDir: filepath.Join(repoRoot, "scripts"),
		AgentsDir:  filepath.Join(repoRoot, "agents"),
		BinDir:     filepath.Join(repoRoot, "bin"),
		Tier:       3, // default: full enforcement
	}
	c.ContractsDir = ContractsDirFor(repoRoot)
	c.ChecksEnable = ChecksEnableFor(repoRoot)

	// REBAR_TIER env takes precedence
	if env := Getenv("REBAR_TIER"); env != "" {
		t, err := strconv.Atoi(env)
		if err == nil && t >= 1 && t <= 3 {
			c.Tier = t
		}
	} else {
		// Read from .rebarrc
		tier, err := readRebarRC(RCFile(repoRoot))
		if err == nil && tier >= 1 && tier <= 3 {
			c.Tier = tier
		}
	}

	// Read version
	versionBytes, err := os.ReadFile(filepath.Join(repoRoot, ".rebar-version"))
	if err == nil {
		c.Version = strings.TrimSpace(string(versionBytes))
	}

	return c, nil
}

// FindRepoRoot walks up from dir looking for .rebarrc, .rebar/, or .git.
func FindRepoRoot(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		// Check for .rebar/ directory first (strongest signal)
		if info, err := os.Stat(filepath.Join(dir, ".rebar")); err == nil && info.IsDir() {
			return dir, nil
		}
		// Then .rebarrc
		if _, err := os.Stat(filepath.Join(dir, ".rebarrc")); err == nil {
			return dir, nil
		}
		// Then .git (fallback)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a rebar repository (no .rebar/, .rebarrc, or .git found)")
		}
		dir = parent
	}
}

// readRebarRC parses a .rebarrc file for the tier setting.
// Format: key = value lines, comments with #.
func readRebarRC(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if strings.EqualFold(key, "tier") || strings.EqualFold(key, "rebar_tier") {
			return strconv.Atoi(val)
		}
	}
	return 0, fmt.Errorf("tier not found in .rebarrc")
}

// EnsureRebarDir creates the .rebar/ directory structure.
func EnsureRebarDir(repoRoot string) error {
	dirs := []string{
		filepath.Join(repoRoot, ".rebar"),
		filepath.Join(repoRoot, ".rebar", "keys"),
		filepath.Join(repoRoot, ".rebar", "trusted-keys"),
		filepath.Join(repoRoot, ".rebar", "envelopes"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return nil
}

// ContractsDirFor returns the contracts directory of the repo at repoRoot,
// relative to it: env REBAR_CONTRACTS_DIR, then .rebarrc contracts_dir, then
// "architecture". Mirrors _rebar_contracts_dir in scripts/_rebar-config.sh.
// A value that is absolute or leaves the repo falls back to the default.
func ContractsDirFor(repoRoot string) string {
	dir := Getenv("REBAR_CONTRACTS_DIR")
	if dir == "" {
		dir = RCValue(repoRoot, "contracts_dir")
	}
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "." || filepath.IsAbs(dir) || dir == ".." || strings.HasPrefix(dir, ".."+string(filepath.Separator)) {
		return DefaultContractsDir
	}
	return dir
}

// RCFile is the .rebarrc the settings of the repo at repoRoot are read
// from: REBAR_RCFILE when set (`rebar migrate` uses it to try settings
// before writing them), else repoRoot/.rebarrc; under WithEnv, the
// override's RCFile. Mirrors _rebar_rc_file in
// scripts/_rebar-config.sh.
func RCFile(repoRoot string) string {
	if e := override(); e != nil && e.RCFile != "" {
		return e.RCFile
	}
	if f := Getenv("REBAR_RCFILE"); f != "" {
		return f
	}
	return filepath.Join(repoRoot, ".rebarrc")
}

// RCValue returns the value of key in the repo's .rebarrc (RCFile; "" if
// absent), parsed like scripts/_rebar-config.sh: the first `key = value`
// line wins and `#` starts a comment.
func RCValue(repoRoot, key string) string {
	return RCValueIn(RCFile(repoRoot), key)
}

// RCValueIn is RCValue for an explicit settings file, for callers that
// must not depend on the environment (an in-process check run on settings
// that are not written yet).
func RCValueIn(rcFile, key string) string {
	f, err := os.Open(rcFile)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimRight(k, " \t") != key {
			continue
		}
		if i := strings.Index(v, "#"); i >= 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	return ""
}

// RCValueOr returns a .rebarrc setting the way _rebar_config in
// scripts/_rebar-config.sh does: the environment variable envVar when it is
// set and non-empty (pass "" for keys without one), else the key's value in
// the repo's .rebarrc (RCFile), else def when that is absent or empty.
func RCValueOr(repoRoot, key, envVar, def string) string {
	if envVar != "" {
		if v := Getenv(envVar); v != "" {
			return v
		}
	}
	if v := RCValue(repoRoot, key); v != "" {
		return v
	}
	return def
}

// KnownChecks are the names checks_enable accepts: the script basenames
// (without .sh) of every check the suites run. Mirrors _REBAR_KNOWN_CHECKS in
// scripts/_rebar-config.sh.
var KnownChecks = []string{
	"check-bypass-flags", "check-compliance", "check-contract-headers",
	"check-contract-refs", "check-decay-patterns", "check-doc-refs",
	"check-fix-commit", "check-freshness", "check-ground-truth",
	"check-jtbd-presence", "check-prefix-uniqueness", "check-todos",
	"compute-registry", "steward", "sync-bootstrap",
}

// ChecksEnableFor returns the checks the repo at repoRoot force-enables below
// their tier: env REBAR_CHECKS_ENABLE, else .rebarrc checks_enable, split on
// whitespace with any .sh suffix dropped. Unknown names are kept (see
// UnknownChecks); the scripts warn about and ignore them. Mirrors
// _rebar_checks_enable in scripts/_rebar-config.sh.
func ChecksEnableFor(repoRoot string) []string {
	raw := Getenv("REBAR_CHECKS_ENABLE")
	if raw == "" {
		raw = RCValue(repoRoot, "checks_enable")
	}
	var names []string
	for _, f := range strings.Fields(raw) {
		names = append(names, strings.TrimSuffix(f, ".sh"))
	}
	return names
}

// UnknownChecks returns the names that are not in KnownChecks.
func UnknownChecks(names []string) []string {
	var unknown []string
	for _, n := range names {
		if !slices.Contains(KnownChecks, n) {
			unknown = append(unknown, n)
		}
	}
	return unknown
}

// CheckTier is the tier check name runs at: 3 when checks_enable names it,
// otherwise the configured tier. Mirrors _rebar_check_tier.
func (c *Config) CheckTier(name string) int {
	if slices.Contains(c.ChecksEnable, strings.TrimSuffix(name, ".sh")) {
		return 3
	}
	return c.Tier
}

// contractNamespaceRE is the character set a contract namespace may use
// (e.g. github.com/org/repo); idVersionRE is the start of a contract
// ID.version (S1-FOO.1.0), which a namespace must not look like, or the
// reference <ID>.<v>:see.2 would read as qualified. Mirrors
// _rebar_contract_namespace_valid in scripts/_rebar-config.sh.
var (
	contractNamespaceRE = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	idVersionRE         = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[0-9]+\.[0-9]+`)
)

// ValidContractNamespace reports whether ns may be a contract namespace,
// for the configured value and for namespaces parsed from references alike.
func ValidContractNamespace(ns string) bool {
	return contractNamespaceRE.MatchString(ns) && !idVersionRE.MatchString(ns)
}

// qualifiedRefRE splits a namespace-qualified reference body
// (<namespace>:<ID>.<major>...) into namespace and the rest.
var qualifiedRefRE = regexp.MustCompile(`^([A-Za-z0-9._/-]+):([A-Za-z0-9][A-Za-z0-9_-]*\.[0-9].*)$`)

// ContractNamespace returns the contract namespace of the repo at repoRoot:
// env REBAR_CONTRACT_NAMESPACE, then .rebarrc contract_namespace, "" if
// unset or invalid. With a namespace, a reference to one of this repo's
// contracts may carry it (CONTRACT + ":" + namespace + ":" + ID.version);
// references qualified with another namespace belong to other repos.
// Mirrors _rebar_contract_namespace in scripts/_rebar-config.sh.
func ContractNamespace(repoRoot string) string {
	ns := ContractNamespaceValue(repoRoot)
	if ns != "" && !ValidContractNamespace(ns) {
		fmt.Fprintf(os.Stderr, "rebar: ignoring contract_namespace '%s' (allowed: letters, digits, . _ / -; must not start like a contract ID.version)\n", ns)
		return ""
	}
	return ns
}

// ContractNamespaceValue is the configured contract namespace of the repo
// at repoRoot before validation: env REBAR_CONTRACT_NAMESPACE, then .rebarrc
// contract_namespace, trimmed, one trailing / dropped. Mirrors
// _rebar_contract_namespace_value in scripts/_rebar-config.sh.
func ContractNamespaceValue(repoRoot string) string {
	ns := RCValueOr(repoRoot, "contract_namespace", "REBAR_CONTRACT_NAMESPACE", "")
	return strings.TrimSuffix(strings.TrimSpace(ns), "/")
}

// LocalContractRef interprets ref, the text after a "CONTRACT:" marker, for
// a repo whose namespace is namespace. It returns the unqualified reference
// (ID.version...) and true when ref points at this repo's contracts, or
// false when it is qualified with another repo's namespace. With no
// namespace configured every ref is returned unchanged, as before
// namespaces existed.
func LocalContractRef(ref, namespace string) (string, bool) {
	if namespace == "" {
		return ref, true
	}
	m := qualifiedRefRE.FindStringSubmatch(ref)
	if m == nil || !ValidContractNamespace(m[1]) {
		return ref, true
	}
	if m[1] != namespace {
		return "", false
	}
	return m[2], true
}

// IsRebarTree reports whether dir is a rebar source checkout or a rebar
// install tree: it has templates/project-bootstrap/ and either rebar's
// embedded assets (cli/internal/assets, a source checkout since rebar is a
// single binary) or scripts/steward.sh (a release tree or clone from
// before that, v3.1.0 and earlier).
func IsRebarTree(dir string) bool {
	if dir == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "templates", "project-bootstrap")); err != nil || !info.IsDir() {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "cli", "internal", "assets")); err == nil && info.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(dir, "scripts", "steward.sh"))
	return err == nil && !info.IsDir()
}
