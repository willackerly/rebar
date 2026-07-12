# check-registry.sh dies silently under set -e on any zero-ref contract (from TDFLite-tak, 2026-07-11)

**Script:** `scripts/check-registry.sh` (rebar-scripts 2026.03.20, marked
DEPRECATED in favor of compute-registry.sh but still shipped).

**Bug:** under `set -euo pipefail`, the check-2 loop's
`ref_count=$(grep -rn ... | grep -v ... | wc -l | tr -d ' ')` substitution
returns the greps' non-zero status when a contract has ZERO code
references (pipefail: both greps exit 1; wc/tr exit 0 but pipefail keeps
the 1). `set -e` then kills the script mid-loop — **before** the loop's own
orphan-handling branch (including the TODO.md-tracked escape hatch) can
run, and before the FAIL/OK summary prints. Symptom: both section headers
print, nothing else, exit 1 — indistinguishable from "no findings" except
by exit code.

**Trigger observed:** TDFLite's umbrella contract C9-TDFLITE-JS had 0
`CONTRACT:C9` code headers (its packages carry their own W-series
contracts) → silent exit 1.

**Fix suggestion:** append `|| true` inside the substitution (the count is
what matters, not the grep status), e.g.
`ref_count=$({ grep -rn ... || true; } | { grep -v ... || true; } | wc -l | tr -d ' ')`
— or drop `set -e` for the counting loop. Same pattern risk exists
anywhere a rebar script counts grep hits under pipefail.

**Severity:** low (deprecated script), but the silent-death mode defeats
the check's purpose and the same idiom may live in maintained scripts —
worth a sweep.
