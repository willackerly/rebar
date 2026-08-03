# Feedback: Check 8 counts fenced CONSUMES.md examples as real declarations — every freshly bootstrapped project fails compliance out of the box

**Date:** 2026-08-02
**Source:** rebar seat — found while verifying the Check 9 degenerate-population fix on a scratch bootstrap project
**Type:** bug (P1 — adoption on-ramp)
**Status:** implemented 2026-08-02
**Template impact:** `scripts/check-compliance.sh` (Check 8) + bootstrap copy
**From:** rebar session 2026-08-02 (Claude Code, willackerly)

## Reproduced on: 2026-08-02, rebar `main` @ `6c8dd77`

```bash
$ T=$(mktemp -d) && cp -r templates/project-bootstrap/. "$T/" && cd "$T"
$ echo v3.0.0-beta.2 > .rebar-version
$ bash scripts/check-compliance.sh
=== Federation (CONSUMES.md present) ===
FAIL: CONSUMES.md declares dependencies but `rebar contract drift-check` is not wired into CI
```

A project created five seconds ago, which has declared no dependency on anything,
fails compliance for not wiring drift-check into CI.

## Cause

Check 8 counted entries with a line-anchored grep:

```bash
grep -cE '^## [A-Za-z0-9_-]+/[A-Za-z0-9_.-]+$' "$CONSUMES_FILE"
```

`templates/project-bootstrap/CONSUMES.md` documents the declaration format with
five **worked examples** — `## rebar/C1-AGENTS.2.0`, `## blindpipe/C2-ZK-VERIFY.1.3`,
`## fontkit/C1-GLYPH-REGISTRY.4.0`, and two more — inside a ` ```markdown ` fence.
`grep` is line-oriented and knows nothing about fences, so all five counted as
real declarations.

The file is doing exactly what a template should: showing a concrete, realistic
example rather than an abstract placeholder. The checker punished it for that.

Note the near-miss that hid this. `office180`'s hand-written `CONSUMES.md`
documents the same format with **angle-bracket placeholders**
(`## <owner_repo>/<contract_id>.<version>`), which the character class
`[A-Za-z0-9_-]` does not match — so office180 correctly reports "present, no
entries." Rebar's own repo has no `CONSUMES.md` at all, so it never ran the code
path. The one artifact that *did* trip it was the one every adopter starts from.

## Fix

Fence-aware counting — a documented example is never a declaration:

```awk
/^```/ { fence = !fence; next }
!fence && /^## [A-Za-z0-9_-]+\/[A-Za-z0-9_.-]+$/ { n++ }
END { print n + 0 }
```

Verified: a fresh bootstrap project with one honestly-declared `draft` contract now
reports `OK: CONSUMES.md present but no entries declared` and
`OK: Rebar compliance verified`. Before this fix plus the Check 9 degenerate-population
guard, that same project failed **twice**.

## The pattern worth extracting

sch-repos named this exact class the same day, in a different context, while
withdrawing their own status-line design:

> an anchored single-marker regex cannot distinguish a real declaration from a
> document *quoting* one, so `conventions.md` itself, any ADR, and the spec
> document would all have hard-errored.

They caught it in design review. Rebar had already shipped it. **Any
line-anchored regex that counts declarations must be fence-aware**, because
documentation quoting the format is not a hypothetical — it is the single most
likely place the pattern appears. Worth auditing the other counters
(`check-contract-refs.sh`, `compute-registry.sh`, the `Status:` parsers) against
the same question.

## Severity

P1 on the adoption on-ramp. It does not affect established repos — it affects
**every new one**, and it presents as the framework accusing a brand-new project
of a federation violation it could not possibly have committed. Same class as the
Check 9 degenerate-population bug filed by sch-repos the same day: correct
arithmetic, wrong on the smallest input, landing squarely on first-run experience.
