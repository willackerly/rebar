# Contract namespaces: qualify CONTRACT: references with the owning repo

**Source:** willackerly/rebar#8 (`f/namespace-no-scripts`, commits b9ea3ae
and eab6c56, ttschampel), rebuilt on the self-contained install
(`feedback/2026-11-14-self-contained-install.md`, #14), 2026-11-14
**Type:** feature (propose-then-ship; implementation in the same branch)
**Status:** implemented on `feat/contract-namespace` (stacked on
`feat/thin-adoption-scripts`); open questions below

## Problem

A contract ID is unique only inside its repo. Once references cross repos
(federation, `CONSUMES.md`, a shared SDK citing the service contract it
speaks), `CONTRACT:AUTH.1.0` is ambiguous: this repo's AUTH, or the
platform's? Two repos that both own an `AUTH` contract cannot tell their
own implementations from a consumer's citation.

Before this change, a qualified reference was invisible to every check.
`CONTRACT:github.com/org/repo:AUTH.1.0` is matched by no pattern
(`CONTRACT:[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*` stops at the first `.` that is
not followed by digits), so it was never counted, never resolved and never
reported broken. `rebar context developer` turned it into the glob
`CONTRACT-github*.md`.

## Design

### Setting

```text
.rebarrc:  contract_namespace = github.com/acme/widgets     (env REBAR_CONTRACT_NAMESPACE)
```

Read by `_rebar_contract_namespace` (bash) and `config.ContractNamespace`
(Go), with #14's rules (first line wins, `#` comment, env override).
Allowed characters: letters, digits, `. _ / -`. A trailing `/` is dropped.
Anything else (a space, a `:`) is ignored with a warning on stderr and
treated as unset, so a typo cannot widen the match patterns.

### Reference grammar

| Reference in source | With `contract_namespace = github.com/acme/widgets` | Unset |
|---|---|---|
| `CONTRACT:BLOBSTORE.2.1` | this repo's BLOBSTORE 2.1 | same |
| `CONTRACT:github.com/acme/widgets:BLOBSTORE.2.1` | the same contract | invisible (as before) |
| `CONTRACT:github.com/acme/keys:KEY-ROTATION.1.0` | another repo's; counted, not checked | invisible (as before) |

Filenames (`CONTRACT-BLOBSTORE.2.1.md`), contract titles and `CONSUMES.md`
keys stay unnamespaced.

### Who honors it

| Path | Change |
|---|---|
| `_rebar-config.sh` | `_rebar_contract_namespace`, `_rebar_contract_ref_ere` (`CONTRACT:` or `CONTRACT:(<ns>:)?`) |
| `steward.sh` | implementing-files grep uses the ERE |
| `compute-registry.sh` | implementing counts and shadow detection use the ERE; a qualified shadow is reported by its bare ID |
| `check-contract-refs.sh` | own-qualified refs are checked like short ones; foreign ones are rewritten out, counted and reported (`Namespace …: N cross-repo reference(s) … not checked`) |
| `check-registry.sh` | orphan search uses the ERE |
| `check-contract-headers.sh` | presence check already accepted both; the failure hint shows the qualified form |
| `cli/internal/config` | `ContractNamespace`, `LocalContractRef` |
| `rebar context developer` | strips our namespace, skips foreign refs |
| prefix-uniqueness, jtbd, version-bump, integrity scanner, `rebar contract` | keyed on filenames or `CONSUMES.md`; unchanged |

Composes with `contracts_dir`: the two settings are independent and the
test runs them together.

## What came from #8

| #8 | Taken | Reworked |
|---|---|---|
| b9ea3ae `.rebarrc` key, config field | the key name, the `host/org/repo` style, "both forms valid indefinitely" | read through #14's `RCValue` / `_rebar_config` instead of a second `.rebarrc` parser; validated charset |
| b9ea3ae steward pattern `CONTRACT:([…]+:)?<id>\.` | qualified refs count as implementations | **only our namespace counts.** #8 accepted any namespace, so a consumer citing another repo's AUTH made our AUTH look implemented. #8 covered steward only; the same rule now applies to every reference parser listed above |
| eab6c56 template | naming block, cross-repo dependency example, grep hint | the title stays `CONTRACT-{NAME}.{MAJOR}.{MINOR}`: nothing parses titles, and `CONTRACT-{NAMESPACE}:{NAME}` would leave an unfilled placeholder in every contract of a repo without a namespace. The C1-prefix example rename is a numbering change, not part of this |

## Evidence [M]

- `scripts/tests/contract-namespace.test.sh`: namespace alone, namespace +
  `contracts_dir = specs`, unset, invalid. Against the #14 scripts
  (`REBAR_SCRIPTS_DIR=<#14 scripts>`) it fails at T1 (`Checked 1 contract
  references`, the qualified ref unseen).
- Unset parity: base scripts vs this branch on rebar itself and on a fixture
  mixing short, qualified, `example.1.2/repo:`-style and broken refs —
  check-contract-refs, check-registry, compute-registry `--stdout` /
  `--health`, check-contract-headers, prefix-uniqueness, jtbd and steward
  `--json` print identical output and exit codes.
- Writing T6 surfaced a separate bug: `check-registry.sh` exited 1 with no
  output at the first orphan (`grep | grep -v` under `pipefail`). Fixed in
  its own `fix:` commit.

## Red team

- *Foreign namespace with a version-shaped segment.* Without a namespace,
  `CONTRACT:example.1.2/repo:AUTH.1.0` is read by check-contract-refs as a
  reference to `example.1.2`, a false BROKEN. Kept for unset (parity); with
  a namespace the foreign ref is rewritten out before extraction.
- *`CONTRACT:ID.1.0: description`.* The qualified-form test requires
  `<ns>:<ID>.<digit>`, so a trailing colon is not taken for a namespace
  (Go test `trailing colon is not a namespace`).
- *Namespace that prefixes another* (`…/repo` vs `…/repo-fork`). The ERE
  requires `<ns>:` exactly; Go compares the whole namespace. Tested.
- *Self-reference (agreement 06).* Test fixtures and Go tests build
  `CONTRACT:` strings from parts so rebar's own scans skip them; the
  template's examples live in `.md` files the reference scanners skip.
- *Two templates.* `architecture/CONTRACT-TEMPLATE.md` and the bootstrap
  copy were identical by hand only; `sync-bootstrap.sh --check` now
  enforces it.

## Open questions (lean in brackets)

1. **Foreign references: count only, or verify?** A qualified ref to another
   repo could be checked against `CONSUMES.md`, but `CONSUMES.md` keys on
   the ASK owner name (`office180/PPTV-SOURCE`), not a namespace. Needs a
   namespace → owner mapping. [Lean: count only now; revisit with
   `rebar contract drift`.]
2. **Unset repos with qualified refs.** They stay invisible, which is the
   parity requirement but hides real references. An advisory line ("N
   namespace-qualified references ignored; set contract_namespace") would
   change unset output. [Lean: add it as a follow-up, advisory only.]
3. **Should a repo with a namespace require the qualified form?** [Lean:
   no; relaxation, not migration. Qualify where a reader outside the repo
   benefits.]
4. **Namespace format.** Charset only, or require `host/org/repo`? [Lean:
   charset only; Go-module style is the documented convention.]
5. **Default namespace from `git remote`?** `rebar init` could propose one.
   [Lean: not yet; an explicit key is easier to reason about.]
6. **Prefix matching.** steward / registry / check-registry match
   `CONTRACT:<id>` as a prefix, so `CONTRACT:S1` also matches
   `CONTRACT:S1-STEWARD`; #8 tightened steward to `<id>\.`. Unchanged here
   to keep parity. [Lean: separate fix across all three.]

## Disposition

Pending maintainer review.
