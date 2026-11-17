#!/usr/bin/env bash
# Regression test: the federation scripts (scan-consumers.sh,
# flush-notifications.sh) and CONSUMES.md sections with optional fields left out.
#
#   T1  scan-consumers reports a consumer whose section has only the required
#       version_pinned (it exited 1 at the first absent optional field, so
#       flush-notifications saw no consumers for the contract)
#   T2  a section without version_pinned is an error naming the file and the
#       field; the other consumers are still reported
#   T3  flush-notifications --dry-run dispatches to that consumer, with and
#       without jq, and passes on scan-consumers' error for an invalid one
#
# Bash 3.2 compatible.
set -uo pipefail
. "$(dirname "$0")/lib.sh"

rebar_tmpdir consumers
OWNER="$TMP/owner"
mkdir -p "$OWNER"
( cd "$OWNER" && g init -q -b main ) || fail "git init"

consumes() {  # consumes <repo> <section body>
  mkdir -p "$TMP/$1"
  printf '# CONSUMES\n\n## owner/C1-FOO.1.0\n%s\n\n## other/C9-BAR.1.0\n- **version_pinned:** 9.0.0\n' "$2" > "$TMP/$1/CONSUMES.md"
}
consumes full '- **owner_repo:** owner
- **contract_id:** C1-FOO
- **version_pinned:** 1.0.0   # semver
- **pin_date:** 2026-01-02
- **rationale:** uses the foo API
- **notify_on_change:** true   # OPTIONAL hint to owner'
consumes minimal '- **version_pinned:** 1.0.0'
consumes nopin '- **pin_date:** 2026-01-02
- **notify_on_change:** true'

# T1
out=$(REBAR_REPOS="$TMP/full,$TMP/minimal" run_in "$OWNER" scan-consumers.sh C1-FOO)
code=$?
[ $code -eq 0 ] || fail "T1: scan-consumers exited $code with a minimal section: $out"
echo "$out" | grep -q 'full →' || fail "T1: full consumer missing: $out"
echo "$out" | grep -q 'minimal →' || fail "T1: minimal consumer missing: $out"
echo "$out" | grep -q 'version_pinned: *1.0.0$' || fail "T1: version_pinned not reported cleanly: $out"
echo "$out" | grep -q 'pin_date: *(unset)' || fail "T1: absent pin_date not defaulted: $out"
json=$(REBAR_REPOS="$TMP/full,$TMP/minimal" run_in "$OWNER" scan-consumers.sh C1-FOO --json)
[ $? -eq 0 ] || fail "T1: scan-consumers --json failed: $json"
echo "$json" | grep -q '"name":"minimal","path":"[^"]*","version_pinned":"1.0.0","pin_date":"","notify_on_change":"","rationale":""' \
  || fail "T1: minimal consumer JSON: $json"
echo "$json" | grep -q '"name":"full",[^}]*"notify_on_change":"true"' || fail "T1: full consumer JSON: $json"
ok "T1 scan-consumers defaults absent optional fields"

# T2
out=$(REBAR_REPOS="$TMP/nopin,$TMP/minimal" run_in "$OWNER" scan-consumers.sh C1-FOO)
code=$?
[ $code -eq 1 ] || fail "T2: missing version_pinned should exit 1, got $code: $out"
echo "$out" | grep -q "nopin/CONSUMES.md.*version_pinned" || fail "T2: error does not name the file and field: $out"
echo "$out" | grep -q 'minimal →' || fail "T2: valid consumer not reported alongside the error: $out"
echo "$out" | grep -q 'nopin →' && fail "T2: invalid consumer reported: $out"
ok "T2 a section without version_pinned is a clear error"

# T3
mkdir -p "$OWNER/architecture/.state"
printf '# Pending\n\n## C1-FOO: 1.0 → 1.1\n- **detected:** 2026-01-03\n- **status:** pending\n- **commit:** abc1234\n' \
  > "$OWNER/architecture/.state/pending-notifications.md"
out=$(REBAR_REPOS="$TMP/full,$TMP/minimal" run_in "$OWNER" flush-notifications.sh --dry-run)
echo "$out" | grep -q 'DRY: ask_full_featurerequest' || fail "T3: full consumer not dispatched: $out"
echo "$out" | grep -q 'DRY: ask_minimal_featurerequest' || fail "T3: minimal consumer not dispatched: $out"

# Without jq: a PATH holding only the tools the two scripts use.
NOJQ="$TMP/nojq-bin"
mkdir -p "$NOJQ"
for t in bash git dirname basename cat cut tr grep sed awk mktemp mv rm head wc; do
  p=$(command -v "$t") || fail "T3: $t not found"
  ln -s "$p" "$NOJQ/$t"
done
out=$(PATH="$NOJQ" REBAR_REPOS="$TMP/full,$TMP/minimal" run_in "$OWNER" flush-notifications.sh --dry-run)
echo "$out" | grep -q 'DRY: ask_full_featurerequest' || fail "T3: (no jq) full consumer not dispatched: $out"
echo "$out" | grep -q 'DRY: ask_minimal_featurerequest' || fail "T3: (no jq) minimal consumer not dispatched: $out"
ok "T3 flush-notifications dispatches to consumers with optional fields absent"

out=$(REBAR_REPOS="$TMP/nopin,$TMP/minimal" run_in "$OWNER" flush-notifications.sh --dry-run)
echo "$out" | grep -q 'DRY: ask_minimal_featurerequest' || fail "T3: valid consumer dropped next to an invalid one: $out"
echo "$out" | grep -q 'nopin/CONSUMES.md.*version_pinned' || fail "T3: scan-consumers error not shown: $out"
echo "$out" | grep -q 'ask_nopin' && fail "T3: invalid consumer dispatched: $out"
ok "T3 flush-notifications reports an invalid consumer and notifies the rest"
