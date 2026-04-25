#!/usr/bin/env bash
# check-decay-patterns.sh — Flag soft-hardening patterns that look like
# done work but ship a longer-fuse failure mode.
#
# Source: rebar/feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md
#
# An author cannot reliably self-audit hardening work because the author's
# context (right now, with the test fresh) matches the consumer's context
# (six months from now, scanning a dashboard). This script applies external
# state — a structural lens — at commit time.
#
# Eight named patterns from the feedback. The grep-detectable subset is
# implemented; the semantic ones (hermeticity, env-name plausibility) are
# left for the author's self-audit prompt linked from AGENTS.template.md.
#
# Modes:
#   ./scripts/check-decay-patterns.sh              # scan staged files
#   ./scripts/check-decay-patterns.sh --all        # scan everything tracked
#   ./scripts/check-decay-patterns.sh --paths a b  # scan specific paths
#   ./scripts/check-decay-patterns.sh --warn       # flag but don't fail
#
# Exit code: 0 if no findings (or --warn), 1 if findings in default mode.
#
# Bash 3.2 compatible (macOS default).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALLOWLIST="$PROJECT_ROOT/.rebar/decay-patterns-allow.txt"

MODE="staged"
WARN_ONLY=0
declare -a EXPLICIT_PATHS=()

while [ $# -gt 0 ]; do
  case "$1" in
    --all) MODE="all"; shift ;;
    --paths) MODE="explicit"; shift; while [ $# -gt 0 ] && [ "${1:0:2}" != "--" ]; do EXPLICIT_PATHS+=("$1"); shift; done ;;
    --warn) WARN_ONLY=1; shift ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "check-decay-patterns: unknown arg '$1'" >&2; exit 2 ;;
  esac
done

cd "$PROJECT_ROOT"

# Resolve the file set to scan.
declare -a FILES=()
case "$MODE" in
  staged)
    while IFS= read -r f; do FILES+=("$f"); done < <(git diff --cached --name-only --diff-filter=ACMR 2>/dev/null)
    if [ ${#FILES[@]} -eq 0 ]; then
      # Fall back to working-tree changes (useful when running outside a hook).
      while IFS= read -r f; do FILES+=("$f"); done < <(git diff --name-only --diff-filter=ACMR 2>/dev/null)
    fi
    ;;
  all)
    while IFS= read -r f; do FILES+=("$f"); done < <(git ls-files)
    ;;
  explicit)
    FILES=("${EXPLICIT_PATHS[@]+"${EXPLICIT_PATHS[@]}"}")
    ;;
esac

if [ ${#FILES[@]} -eq 0 ]; then
  echo "check-decay-patterns: no files to scan."
  exit 0
fi

# Filter to spec/test files + workflow files. Other files don't have these
# patterns and would just produce noise.
declare -a SCAN_FILES=()
for f in "${FILES[@]+"${FILES[@]}"}"; do
  [ -f "$f" ] || continue
  case "$f" in
    *.spec.ts|*.spec.tsx|*.spec.js|*.spec.jsx|*.spec.mjs) SCAN_FILES+=("$f") ;;
    *.test.ts|*.test.tsx|*.test.js|*.test.jsx|*.test.mjs) SCAN_FILES+=("$f") ;;
    *_test.go|*_spec.rb|*_test.py|*test_*.py)              SCAN_FILES+=("$f") ;;
    *playwright*.config.*|*vitest*.config.*|*jest*.config.*) SCAN_FILES+=("$f") ;;
    .github/workflows/*.yml|.github/workflows/*.yaml)      SCAN_FILES+=("$f") ;;
  esac
done

if [ ${#SCAN_FILES[@]} -eq 0 ]; then
  echo "check-decay-patterns: no spec/config/workflow files in scan set."
  exit 0
fi

# Findings: <file>:<line>:<pattern-id>:<message>
findings_tmp="$(mktemp)"
trap 'rm -f "$findings_tmp"' EXIT
: > "$findings_tmp"

# Helper: scan with a regex, emit findings.
scan() {
  local pattern_id="$1" regex="$2" message="$3" file
  shift 3
  for file in "$@"; do
    [ -f "$file" ] || continue
    # Use grep -nE; allow pattern to fail gracefully on no-match.
    while IFS=: read -r lineno _; do
      [ -z "$lineno" ] && continue
      # Allowlist check by file:line:pattern triple.
      if [ -f "$ALLOWLIST" ] && grep -Fxq -- "${file}:${lineno}:${pattern_id}" "$ALLOWLIST" 2>/dev/null; then
        continue
      fi
      echo "${file}:${lineno}:${pattern_id}:${message}" >> "$findings_tmp"
    done < <(grep -nEi "$regex" "$file" 2>/dev/null || true)
  done
}

# -------------------------------------------------------------------------
# Pattern 1 — Silenced failures
# testInfo.fail() / test.fail() / it.fail() / expect.fail() / .skip() with TODO
# -------------------------------------------------------------------------
scan "P1-silenced-failure" \
  '(testInfo\.fail|test\.fail|it\.fail|describe\.fail|expect\.fail)\(' \
  "Pattern 1 (silenced failure): use a real failing assertion + CI continue-on-error instead — silencing the test makes the bug invisible to anyone glancing at the dashboard." \
  "${SCAN_FILES[@]}"

# -------------------------------------------------------------------------
# Pattern 2 — Inverted assertion (heuristic — flags for review)
# `.toBe(null)` / `.toBeNull()` / `.toEqual([])` / `.not.toContain` in security/audit specs
# -------------------------------------------------------------------------
declare -a SEC_FILES=()
for f in "${SCAN_FILES[@]+"${SCAN_FILES[@]}"}"; do
  case "$f" in
    *security*|*audit*|*regression*|*detector*) SEC_FILES+=("$f") ;;
  esac
done
if [ ${#SEC_FILES[@]} -gt 0 ]; then
  scan "P2-inverted-assertion" \
    '\.(toBe\(null\)|toBeNull\(\)|toEqual\(\[\]\)|not\.toContain)' \
    "Pattern 2 (inverted assertion): assertion passes when the thing being tested is broken — a future engineer 'fixing' this test will silently elide the signal. Restate positively + use expected-fail at the workflow level. Or pair with a negative-control test that stages the violation." \
    "${SEC_FILES[@]}"
fi

# -------------------------------------------------------------------------
# Pattern 4 — Hand-copied "keep in sync" data
# Comments containing `keep .* in sync` / `update when` / `mirrors`
# -------------------------------------------------------------------------
scan "P4-keep-in-sync" \
  '(keep[[:space:]]+(this|in|them)[[:space:]]+(in[[:space:]]+)?sync|update[[:space:]]+when[[:space:]]+upgrading|mirrors[[:space:]]+the[[:space:]]+upstream)' \
  "Pattern 4 (silent drift on upgrade): a comment saying 'keep in sync' is a wish. Read the source of truth at runtime and throw on parse failure, instead of hand-copying." \
  "${SCAN_FILES[@]}"

# -------------------------------------------------------------------------
# Pattern 5 — Magic-string project/name gating
# `testInfo.project.name === 'literal'` or `project.name == 'literal'`
# -------------------------------------------------------------------------
scan "P5-magic-string-gating" \
  "testInfo\.project\.name[[:space:]]*[!=]==?[[:space:]]*['\"]" \
  "Pattern 5 (magic-string gating): renaming the project silently breaks the gate, no compile error. Use testInfo.project.metadata.<flag> instead — moving metadata with the project definition makes the coupling explicit." \
  "${SCAN_FILES[@]}"

# -------------------------------------------------------------------------
# Pattern 7 — Plausible test-only env var names (heuristic)
# VITE_ALLOW_*, *_ALLOW_HOST, *_DISABLE_AUTH — generic-sounding test loosening
# -------------------------------------------------------------------------
scan "P7-plausible-env-name" \
  '(VITE_ALLOW_ALL|ALLOW_ALL_HOSTS|DISABLE_AUTH|SKIP_VERIFY|BYPASS_(AUTH|TLS|CORS))' \
  "Pattern 7 (too-plausible-to-refuse): this env var name reads like normal infrastructure config — a deploy script copy-paste won't be flagged. Make the name long and obviously test-only (e.g., FOO_PLAYWRIGHT_REAL_CHROME_LOOSENING) and require a second test-only env var to activate." \
  "${SCAN_FILES[@]}"

# -------------------------------------------------------------------------
# Report
# -------------------------------------------------------------------------
count="$(wc -l < "$findings_tmp" | tr -d ' ')"

if [ "$count" -eq 0 ]; then
  echo "check-decay-patterns: OK — no soft-hardening patterns detected in scan set."
  exit 0
fi

echo "check-decay-patterns: $count finding(s) in scan set"
echo ""
echo "Each line: <file>:<line>:<pattern-id>:<guidance>"
echo ""
cat "$findings_tmp"
echo ""
echo "If a finding is intentional + reviewed, allowlist by adding"
echo "  <file>:<line>:<pattern-id>"
echo "to .rebar/decay-patterns-allow.txt (one per line, # for comments)."
echo ""
echo "Reference: feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md"

if [ "$WARN_ONLY" -eq 1 ]; then
  exit 0
fi
exit 1
