# Feedback: Bootstrap-template script drift + macOS bash 3.2 path-norm bug

**Date:** 2026-04-25
**Source:** `templates/project-bootstrap/scripts/check-freshness.sh`, `scripts/check-doc-refs.sh`
**Type:** bug
**Status:** proposed
**Template impact:**
- `templates/project-bootstrap/scripts/check-freshness.sh` — stale relative to canonical `scripts/check-freshness.sh`; needs sync
- `scripts/check-doc-refs.sh` — bash 3.2 parameter-expansion bug, fix needed in canonical
- `templates/project-bootstrap/scripts/check-doc-refs.sh` — same fix needed
**From:** TDFLite (`~/dev/OpenTDF/TDFLite`), REBAR Tier 3 adoption push 2026-04-25

## What Happened

While pushing TDFLite from Tier 1 → Tier 3 (v0.4.0), I copied the full enforcement script set from `templates/project-bootstrap/scripts/` and wired the pre-commit hook. Two of the scripts failed silently or with 100%-false-positive output on macOS bash 3.2.57(1)-release.

### Bug 1 — `check-doc-refs.sh` path normalization (canonical script, real bug)

In the path-resolution block:

```bash
while [[ "$resolved" == */./* ]]; do
  resolved="${resolved/\/.\//\/}"
done
```

On macOS bash 3.2, the parameter expansion `${resolved/\/.\//\/}` produces a string with literal backslashes — `docs/./ABAC-CONVENTIONS.md` becomes `docs\/ABAC-CONVENTIONS.md` instead of `docs/ABAC-CONVENTIONS.md`. The subsequent `grep -Fxq` against the tracked file list never matches, so every relative-path link that contains `./` is reported as broken.

**Symptom on a freshly-adopted project:** check-doc-refs reports broken refs for every relative link that uses `./foo.md` syntax. Not catchable by reading the script — looks like a tracking issue in the project itself.

**Reproduction:** macOS Sequoia, bash 3.2.57(1)-release, GNU `${var/pattern/replacement}` semantics. Works correctly on bash 4.x and zsh. Trace:

```
+ resolved=docs/./ABAC-CONVENTIONS.md
+ [[ docs/./ABAC-CONVENTIONS.md == */./* ]]
+ resolved='docs\/ABAC-CONVENTIONS.md'      # ← literal backslash
+ [[ docs\/ABAC-CONVENTIONS.md == */./* ]]  # ← exits loop, but corrupted
```

### Bug 2 — `check-freshness.sh` template is stale relative to canonical (already fixed in `scripts/`)

Architect agent confirmed: rebar's canonical `scripts/check-freshness.sh:43-46` already wraps the grep in `set +o pipefail` / `set -o pipefail`. **But the bootstrap template at `templates/project-bootstrap/scripts/check-freshness.sh` does not have this fix** — it still has the original failing pattern. Anyone copying the bootstrap template (which I did) gets the broken version.

**Failing pattern in the template:**

```bash
set -euo pipefail
...
while IFS= read -r file; do
  date_str=$(grep -o 'freshness: ...' "$file" 2>/dev/null | head -1 | sed '...')
  [ -z "$date_str" ] && continue
```

When grep exits 1 (no match — the common case), pipefail makes the pipeline exit 1, set -e kills the whole script. Symptom: silent exit-1 with no output. The `[ -z ... ] && continue` line never runs.

## What Was Expected

1. **Bug 1 (canonical):** check-doc-refs.sh should resolve `./foo.md` to `dir/foo.md` correctly on the most common dev OS (macOS).
2. **Bug 2 (template drift):** The bootstrap template should not ship a known-broken version of a script that's already been fixed in canonical. Either templates should re-copy from canonical at template-build time, or there should be a check that flags drift.

## Suggestion

### For Bug 1 — patch `scripts/check-doc-refs.sh` (and the bootstrap copy)

Sed-based replacement, portable across bash 3.2 and 4+/zsh:

```bash
# Replace the parameter-expansion-based loops with sed:
resolved="$(echo "$resolved" | sed -e 's#/\./#/#g' -e 's#^\./##')"
# (and keep the existing `*/../*` collapse loop — that one uses sed already)
```

I confirmed this passes against TDFLite's mixed `docs/`, `inbox/`, `architecture/`, `agents/` markdown corpus (122 tracked .md files in repo, ~1700 in vendor/). PR-ready; happy to send if you'd like.

### For Bug 2 — sync templates from canonical, OR add drift detection

Two options:

**A. Manual one-shot sync (cheapest):** copy the 4-line pipefail toggle pattern from `scripts/check-freshness.sh:43-46` into `templates/project-bootstrap/scripts/check-freshness.sh`. Probably worth a sweep across all `templates/project-bootstrap/scripts/*.sh` to catch other drift.

**B. Drift-detection check:** add a `scripts/check-bootstrap-sync.sh` that diff's `scripts/*.sh` against `templates/project-bootstrap/scripts/*.sh` (modulo allowed customization markers) and fails if they diverge silently. The ci-check.sh already references "Bootstrap Sync" as a check that's currently SKIPed because the script doesn't exist — so the slot is reserved.

### Bonus — TDFLite-local adaptation (`vendor/`)

TDFLite has a vendored Go module (`vendor/`) which most rebar projects don't. `check-doc-refs.sh` walks every tracked `*.md`, including third-party docs in `vendor/` that reference paths only resolving in the upstream repo's layout. We patched locally with:

```bash
case "$src" in
  ...
  vendor/*) continue ;;
esac
```

Probably not worth canonicalizing (most projects don't have vendor/), but worth mentioning the pattern: **if a project pulls third-party markdown into its tracked tree, doc-refs needs an exclusion or the noise floor swamps real failures (97 false positives in our case).** Maybe worth supporting a `.rebar/doc-refs-exclude.txt` (path-prefix patterns) alongside the existing `.rebar/doc-refs-allow.txt` (exact paths).

## Status of fixes in TDFLite

Both patches live at:

- `/Users/will/dev/OpenTDF/TDFLite/scripts/check-doc-refs.sh` (sed-based normalization + vendor/ exclusion)
- `/Users/will/dev/OpenTDF/TDFLite/scripts/check-freshness.sh` (now using upstream's pipefail-toggle pattern, with a comment pointing here)

TDFLite v0.4.0 ships at REBAR Tier 3 ENFORCED with all 10 ci-check enforcements passing.

## Validation

- macOS Sequoia, bash 3.2.57(1), zsh 5.9
- TDFLite repo: 122 tracked .md files, 3 contracts, 13 source files with CONTRACT: headers
- After fixes: ci-check 10 passed / 0 failed / 1 skipped (bootstrap-sync, since that script doesn't exist)

---

## Addendum (2026-04-25 EOD) — Bug 3: scripts/steward.sh silently skips any enforcement check declared with args

Surfaced when `steward.sh` reported "6/7 enforcement passing" but `ci-check.sh` reported all 10 enforcements green. Root cause:

```bash
# scripts/steward.sh, around line 234 (canonical):
local key="${entry%%:*}"
local script="${entry##*:}"            # ← captures "compute-registry.sh --check"
local script_path="$SCRIPT_DIR/$script"  # ← path becomes ".../scripts/compute-registry.sh --check"
local result="skip"
if [ -x "$script_path" ]; then          # ← test fails: literal " --check" makes it a non-existent path
  if "$script_path" >/dev/null 2>&1; then
    result="pass"
```

The `checks` array contains one entry with args:
```bash
"registry:compute-registry.sh --check"
```

For that entry, `script_path` becomes `".../scripts/compute-registry.sh --check"` — not a real file, so `-x` is false, so the check silently degrades to `skip`. The enforcement count drops from 7 to 6 even when the underlying script would pass.

**Fix shipped in TDFLite (steward.sh):**

```bash
local script_with_args="${entry##*:}"
local script="${script_with_args%% *}"
local args=""
if [ "$script" != "$script_with_args" ]; then
  args="${script_with_args#* }"
fi
local script_path="$SCRIPT_DIR/$script"
...
if "$script_path" $args >/dev/null 2>&1; then
```

After fix: TDFLite steward shows **7/7 enforcement passing** — the registry check now actually runs.

**Severity:** medium — silently downgrades the steward score for any project that uses the canonical `checks` array (which includes `compute-registry.sh --check`). Easy to miss because the user-facing output ("6/7 passing") looks plausible.
