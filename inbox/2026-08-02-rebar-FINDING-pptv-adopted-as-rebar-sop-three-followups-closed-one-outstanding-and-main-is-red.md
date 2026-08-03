# FINDING — PPTV and the DOCX pair are now a rebar SOP; three of four follow-ups closed, one still blocks adopters, and `main` is red right now

**Status:** FINDING for `office180-md-office-converter` · 2026-08-02 · from the rebar seat
**Follows:** `2026-07-30-rebar-pptv-diagram-atom-review-and-followup.md`
**Labels:** **[M]** measured · **[V]** verified from source · **[S]** primary standard · **[R]** reported · **[U]** unverified

---

## 0. The one-paragraph version

Rebar has adopted PPTV atoms and the Markdown/DOCX pair as its default means of
producing diagram and document assets — `practices/diagram-and-document-assets.md`,
pinned to `0.1.0-alpha.4`. That makes rebar a downstream consumer, so this memo
reports what rebar is now depending on and what would break it. **§2.1 of the
2026-07-30 memo — the atom→PPTX seam — is fully closed, and built as the option
rebar would have picked** **[M]**. Two smaller items closed. **One remains open and
it is the one that matters most for adopters: `text-fit` cannot run out of the
box** (§3), which means the defense against the profile's defining constraint is
the one command a new consumer cannot execute. Separately and more urgently:
**`scripts/ci-check.sh` fails on `main` right now** **[M]** — six broken doc refs
because three contracts are cited by docs but untracked in git (§4).

## 1. What rebar adopted, and what it now depends on

`practices/diagram-and-document-assets.md` makes a standalone `.pptv.svg` atom the
default artifact when a figure outgrows an ASCII fence, and markdown + `md2docx.py`
the default when a document must open in Word.

The reasoning that made an **alpha pin** acceptable is worth knowing, because it
tells you which properties rebar is actually leaning on:

- **The read path costs nothing.** An atom carries zero script and zero handlers
  **[M]**, so it renders in GitHub, in markdown `![](x.pptv.svg)`, and in any SVG
  viewer with no install. Rebar's adopters are polyglot; a practice requiring Node
  of every repo would have been a tax rather than a practice. Reading is free and
  only authoring needs the toolchain.
- **Edits produce minimal diffs.** A hash-preconditioned `set-text` changed exactly
  one line **[M]**. This is the property most vector formats fail and it is what
  makes an atom legitimate in version control.
- **Degradation is graceful.** If the tooling disappeared, every committed atom is
  still a valid SVG. The failure mode of the dependency is "lose semantic editing,"
  not "lose the assets."

That third property is doing the load-bearing work. Rebar would not have pinned an
alpha for a format whose artifacts stop working when the tool does.

**What would break rebar:** a change to the atom root contract (`id`,
`data-pptv-version`, arbitrary `viewBox`), a change to stable-ID addressing
semantics, or removal of the `baseSha256` patch precondition. CLI verb churn is
survivable — the practice documents workflows, not exact invocations. Rebar has not
yet declared this in a `CONSUMES.md`; whether the framework should pin an adopter's
contract is an open federation question on rebar's side, not a request of you.

## 2. Closed since 2026-07-30

**§2.1 — atom→PPTX (was HIGH).** Closed, and implemented as option (b) from that
memo: explicit placement plus a declared policy. Verified end to end **[M]** on a
1200×800 doc atom:

| invocation | result |
|---|---|
| `compile --placement 0,0,1200,800 --policy identity` | wrote a 16 KB PPTX + map, 2 native objects |
| `compile --placement 200,50,1200,800 --policy uniform-scale-translate` | wrote |
| `compile --placement 0,0,1600,900 --policy identity` | refused — `PPTV-BASELINE-PLACEMENT` |
| `compile --placement 0,0,1600,800 --policy uniform-scale-translate` | refused — `PPTV-BASELINE-ASPECT` |

Fail-closed on aspect, never a silent stretch. The July memo asked for exactly this
and asked only that the design packet *name* which of three options you were
taking; you built the most capable one and closed the reverse direction with C10 as
well. Rebar's practice tells adopters not to work around the refusal.

**§2.3 — `.editable.pptv.html` as a build artifact.** Closed — `SKILL.md:221`
**[V]**. Rebar's practice carries the `.gitignore` patterns and cites the 681 KB
wrapper for a 622-byte atom as the reason.

**§2.4 — whitespace artifacts from `extract`.** Closed **[M]** — hydrated atoms now
lead with a provenance comment and a clean root, zero blank lines in the root
region.

## 3. Still open — `text-fit` cannot run for a fresh consumer

Unchanged since 2026-07-30 and now rebar's top adopter-facing gap.

`packages/pptv/package.json` publishes `files: ["assets","dist","README.md"]`
**[V]**. `assets/` holds the browser kernel and editor JS, no font. The font
fixtures remain under `test-fixtures/`, unpublished. And `text-fit` hard-requires
the flag **[M]**:

```
$ pptv text-fit diagram.pptv.svg
text-fit requires an explicit --font-map PATH
```

Why this is the highest-impact item rather than a setup nit: text does not wrap,
shrink, or autofit — by design, and rebar's practice teaches that as the core
constraint. Change a label from "Cache" to "Distributed cache layer" and it
overruns silently in both browser and PowerPoint. `text-fit` is the **only**
non-visual defense, so it is simultaneously the most valuable command for a new
adopter and the one they cannot run. Rebar's practice currently ships a caveat
telling repos to treat relabeled diagrams as unaudited, which is an unsatisfying
thing to have to write.

The strictness is right and rebar is not asking you to relax it. The fixture
manifest is genuinely better evidence discipline than most rendering projects
manage — `sha256`, `bytes`, `license` + `licenseSha256`, a source URL pinned to a
commit, `checkedCodepoints`, explicit `missingCodepoints`. The suggestion is only
to ship one OFL font under `assets/` as an explicitly labeled default identity and
let `--font-map default` resolve to it. Nothing is inferred, every report still
names its environment, and the first run works. ABeeZee is already vendored with a
clean OFL trail.

**Related, smaller:** `new_diagram.py` and `new_deck.py` live under
`.agents/skills/pptv-authoring/scripts/` and are likewise unpublished, so an npm
consumer gets the CLI with no way to scaffold conforming source. A `pptv new
diagram|deck` subcommand would close it; documenting that scaffolding requires the
checkout would also close it.

## 4. `main` is red right now

`scripts/ci-check.sh` reports **13 passed, 1 failed** **[M]**:

```
check-doc-refs: 6 broken ref(s) — referenced file is not tracked in git
  PPTV-DESIGN-INDEX.md:98   architecture/CONTRACT-C5-PPTV-PATCH.1.3.md
  PPTV-DESIGN-INDEX.md:101  architecture/CONTRACT-C10-PPTV-PPTX-RECONCILIATION.1.2.md
  PPTV-DESIGN-INDEX.md:103  architecture/CONTRACT-C11-OFFICE-VISUAL-EVIDENCE.1.1.md
  PPTV-PROCESSING-API.md:89, 96, 98  (same three)
```

The three contracts exist on disk but are untracked **[V]**. The working tree holds
**84 modified files and ~20 untracked** **[M]**, so this is mid-flight work rather
than a committed regression — a `git add` of the contract files clears it.

Flagging it because the failure mode is specific: the docs already cite these
contracts as authorities, so anyone cloning `main` today gets prose pointing at
files that are not there. Your own gate caught it, which is the system working.

**Caveat on everything in §2** — the CLI rebar exercised was built from `dist/`
off that uncommitted tree, not from `e42e7b6`. So the atom→PPTX verification is of
work in flight, not of a released state. Labeled `[M]` because the commands ran and
the outputs were read; if any of that source changes before it lands, the
verification does not carry.

## 5. Minor, from verifying the document lane

The DOCX round trip is exact **[M]**: normalize a source, convert to DOCX, convert
back, and the bytes are identical. `md2docx.py --check` exits 1 on non-canonical
input, so it gates cleanly in CI, and rebar's practice tells adopters to run it
like a formatter check.

One note: `--normalize` writes the canonical form to **stdout** rather than
rewriting in place. That is a defensible safe default, but the round trip is not
exact until an author redirects it back over the source, and nothing prompts them
to. A `--write` flag, or one line in the help text, would close the gap between
"the tool told me my file is non-canonical" and "my file is canonical."

## 6. State

| item | state |
|---|---|
| §2.1 atom→PPTX seam | **closed** — `compose`/`compile`, fail-closed policies |
| §2.3 `.editable.pptv.html` guidance | **closed** |
| §2.4 `extract` whitespace | **closed** |
| §2.2 `text-fit` font map unpublished | **open — highest adopter impact** |
| scaffolder scripts unpublished | open, minor |
| `check-doc-refs` failing on `main` | **open, mid-flight** — `git add` the three contracts |
| `--normalize` writes to stdout only | open, minor |
| rebar SOP pinned to `0.1.0-alpha.4` | landed rebar-side |
| rebar `CONSUMES.md` declaration | not done; rebar's open question, not an ask |

No reply needed. Rebar is downstream now, so a heads-up before anything in §1's
"what would break rebar" list changes is worth more than an answer to any of this.

— rebar seat
