package checks

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/willackerly/rebar/cli/internal/contracts"
)

// check-prefix-uniqueness (legacy numbered contract IDs): no prefix number
// (S1, I3, ...) is claimed by two different contract IDs in the registry's
// file list. Tier 2 warns, tier 3 blocks.
// Native port of scripts/check-prefix-uniqueness.sh.
func init() {
	register(&Check{Name: "check-prefix-uniqueness", Run: runPrefixUniqueness})
}

var (
	versionSuffixRE = regexp.MustCompile(`\.[0-9]+(\.[0-9]+)?$`)
	prefixNumberRE  = regexp.MustCompile(`^[A-Za-z]+[0-9]+$`)
)

func runPrefixUniqueness(ctx *Context, args []string) int {
	w := ctx.Stdout
	if os.Getenv("SKIP_PREFIX_UNIQUENESS") == "1" {
		fmt.Fprintln(w, "SKIP: check-prefix-uniqueness (SKIP_PREFIX_UNIQUENESS=1)")
		return 0
	}
	if Skip(ctx, 2, "check-prefix-uniqueness") {
		return 0
	}
	tier := CheckTier(ctx.Root, "check-prefix-uniqueness")
	dir := contracts.Dir(ctx.Root)
	registry := ctx.Root + "/" + dir + "/CONTRACT-REGISTRY.md"
	if !isRegular(registry) {
		fmt.Fprintf(w, "SKIP: no %s/CONTRACT-REGISTRY.md (run scripts/compute-registry.sh to generate).\n", dir)
		return 0
	}
	data, _ := os.ReadFile(registry) // unreadable: no lines, as grep 2>/dev/null

	// (prefix, ID) pairs from the "- CONTRACT-<ID>.<v>.md" lines, unique,
	// in byte order (the script's sort -u).
	seen := map[string]bool{}
	var pairs [][2]string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "- CONTRACT-") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(line, "- CONTRACT-"), ".md")
		id = versionSuffixRE.ReplaceAllString(id, "")
		if id == "" {
			continue
		}
		prefix, _, _ := strings.Cut(id, "-")
		if !prefixNumberRE.MatchString(prefix) || seen[prefix+"\t"+id] {
			continue
		}
		seen[prefix+"\t"+id] = true
		pairs = append(pairs, [2]string{prefix, id})
	}
	if len(pairs) == 0 {
		fmt.Fprintln(w, "OK: no numbered contract IDs listed in the registry — nothing to check.")
		return 0
	}
	slices.SortFunc(pairs, func(a, b [2]string) int { return strings.Compare(a[0]+"\t"+a[1], b[0]+"\t"+b[1]) })

	ids := map[string][]string{}
	for _, p := range pairs {
		ids[p[0]] = append(ids[p[0]], p[1])
	}
	var dupes []string
	for prefix, list := range ids {
		if len(list) > 1 {
			dupes = append(dupes, prefix+": "+strings.Join(list, ", "))
		}
	}
	slices.Sort(dupes)
	if len(dupes) == 0 {
		fmt.Fprintf(w, "OK: %d contract ID(s) in registry, no prefix-number collisions.\n", len(pairs))
		return 0
	}

	fmt.Fprintf(w, "FAIL: %d prefix number(s) claimed by more than one contract ID:\n\n", len(dupes))
	for _, d := range dupes {
		fmt.Fprintf(w, "  %s\n", d)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Each prefix number (S1, I3, ...) must map to exactly one contract ID.")
	fmt.Fprintln(w, "Versions of the same ID are fine; two different IDs sharing a number")
	fmt.Fprintln(w, "overload the prefix. Renumber the newer contract to the next free number,")
	fmt.Fprintf(w, "update its implementing-file headers (grep -rn \"%s<old-id>\"), and\n", contracts.Marker)
	fmt.Fprintln(w, "regenerate the registry with scripts/compute-registry.sh.")
	if n, ok := TestInt(tier); ok && n >= 3 {
		return 1
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "WARN: advisory at Tier %s; blocking at Tier 3.\n", tier)
	return 0
}
