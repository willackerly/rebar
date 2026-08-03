# The published canonical `Status:` grep does not match the parsers of record (from sch-repos, 2026-08-02)

**DISPOSITION 2026-08-02 — IMPLEMENTED, options 1 + 3-partial.** `conventions.md` now
publishes the *matching* regex `^\*{0,2}Status:` alongside the *authoring* form
`**Status:** value`, with an explicit instruction that the two must change in the same
commit. Beyond the filing: the parser was also fixed rather than merely documented — the
strip is now a single asterisk-tolerant `s/^\*{0,2}Status:\*{0,2}//`, so `*Status:* active`
parses to `active` instead of being mangled into a stub-or-draft penalty. Verified across
all three asterisk forms. The `*)` arm now also prints the raw source line, since naming
the mangled value gave an author nothing to act on. Option 2 (emit the regex from tooling)
is the durable fix and stays open — the shared-string discipline is documented, not yet
mechanized.

---

**Docs:** `conventions.md` — "Declared Maturity (`Status:` field)", the
machine-greppable form (~line 329).
**Scripts:** `scripts/check-compliance.sh` (Check 9), `scripts/cold-start-checks.sh`.

**Bug:** `conventions.md` publishes this as *the* canonical machine-greppable form:

```bash
grep -E '^\*\*Status:\*\*' architecture/CONTRACT-*.md
```

but both parsers of record accept a looser pattern (`check-compliance.sh` ~line 287):

```bash
grep -m1 -E '^\*{0,2}Status:' "$contract" | sed -e 's/^\*\*Status:\*\*//' -e 's/^Status://' ...
```

`^\*{0,2}Status:` admits zero, one, or two asterisks. So a contract written
`Status: active` (bare, no bold) is **counted by the parser but invisible to the
published grep**. Anyone auditing maturity with the documented command gets a
different answer than CI does — and the discrepancy is silent in both directions.

**Second-order consequence — the single-asterisk case is mis-parsed into a penalty.**
For a line `*Status:* active`:

| step | result |
|---|---|
| `grep -E '^\*{0,2}Status:'` | matches (one asterisk) |
| `sed 's/^\*\*Status:\*\*//'` | no match (needs two) |
| `sed 's/^Status://'` | no match (line starts with `*`) |
| `awk '{print $1}'` | `*Status:*` |
| `tr -d '*'` → case-fold | `status:` |

`status:` is not in the maturity vocabulary, so it falls to the `*)` arm: a `WARN`
plus `stub_draft++`. A contract that declared itself `active` in a near-miss format
is silently weighted as stub-or-draft. Given Check 9 can demote a tier, a
formatting slip becomes a badge penalty with only a `WARN` to explain it.

**Fix suggestion:** make the published string and the parser string the same string,
and stop typing it in two places. Options, cheapest first:

1. **Correct `conventions.md`** to publish `grep -E '^\*{0,2}Status:'` — matches the
   parsers today, one-line change, no code risk.
2. **Emit the canonical grep from tooling** (e.g. `rebar status --grep`, or a
   comment in `check-compliance.sh` that `conventions.md` quotes by reference) so
   prose cannot drift from the parser again. This is the durable fix.
3. **Tighten the parser instead** — accept only `**Status:**`, and let near-miss
   formats fail loudly rather than be counted as stub-or-draft. Stricter, but it
   would newly fail any adopter already using a bare `Status:` line, so it needs a
   migration note.

Whichever is chosen, the `*)` fallback arm deserves a sharper message: it currently
reports the *parsed* value, which for a near-miss is the mangled `status:` rather
than the author's intended word, making the WARN hard to act on.

**Severity:** low today — every contract we inspected uses the bold form, so no
badge is currently wrong. Filed because it is a **correctness invariant of the
feature v3 is built on**, and because the "one documented grep finds every
declaration" property is exactly what an external tool or a downstream repo would
rely on.

**Why we hit it:** we are designing a proposed generalization of `Status:` from
architecture contracts to arbitrary artifacts (a cross-format status line for
research repos, memo to follow in `inbox/`). A hard requirement there is "one grep
finds every declaration, in every format." Checking whether v3 already achieves
that for contracts turned up this divergence. If the generalization proceeds, we
would want the published regex and the parser regex to be a single shared string
from day one — so this is worth settling in v3 rather than inheriting.

**Verification:** both patterns read directly from source (`conventions.md`,
`scripts/check-compliance.sh`, `scripts/cold-start-checks.sh`). The single-asterisk
trace above is a hand-execution of the pipeline, not an observed field failure. No
rebar or adopter file was modified in producing this filing.
