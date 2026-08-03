# Diagram and Document Assets

**Referenced from AGENTS.md. Read when a repo needs a diagram in its docs, or a
document deliverable that must open in Word or PowerPoint.**

Status: **adopted 2026-08-02**, pinned to `@office180/pptv@0.1.0-alpha.4` and the
office180 Python DOCX pair. The alpha pin is deliberate and bounded — see §7.

---

## 1. The problem this replaces

Before this practice, rebar had no diagram convention at all: zero mermaid blocks,
zero `.svg`/`.png` assets, and every visual rendered as ASCII inside a ` ```text `
fence. That floor is free and diffable, and it stops working at roughly six boxes.
Above that, repos either drew nothing or dropped in an opaque PNG that no agent
could read, no reviewer could diff, and no author could edit without the original
tool.

The requirement was never "a drawing tool." It was an asset that is **inspectable,
diffable, agent-editable, renderable without a toolchain, and not proprietary.**

## 2. Two lanes, picked by destination

| You need | Author | Verified property |
|---|---|---|
| A figure inside repo docs | `diagram.pptv.svg` atom | renders anywhere with zero dependencies |
| That figure inside a PowerPoint deliverable | same atom → `pptv compile` | fail-closed placement, no silent stretch |
| A document that must open in Word | markdown → `md2docx.py` with a theme | exact byte round-trip on canonical source |
| A rendered-fidelity target | see [`visual-fidelity.md`](visual-fidelity.md) | unchanged |

The lanes share one property that makes them worth standardizing on together:
**the canonical source is the text file in your repo**, and every Office artifact
is a derived projection carrying a hash back to it.

## 3. The diagram lane

### 3.1 The default artifact

A diagram is one standalone `.pptv.svg` **atom**: a self-contained, ordinary SVG
with a stable root `id`, `data-pptv-version="0.1"`, and any finite-positive
`viewBox`. It is not a one-slide deck — it has no manifest, no theme block, and no
embedded runtime.

Scaffold with the office180 skill's starter, or hand-write the root:

```xml
<svg id="system-overview"
     data-pptv-version="0.1"
     viewBox="0 0 1200 800"
     xmlns="http://www.w3.org/2000/svg">
```

### 3.2 Why this is safe to adopt at alpha

Three properties were verified against the shipped CLI, not read from docs:

- **The read path costs nothing.** An atom contains zero script and zero event
  handlers. It renders in GitHub blob view, in markdown `![](x.pptv.svg)`, and in
  any SVG viewer, with no runtime and no install. A contributor who never installs
  Node participates fully in reading and reviewing.
- **Edits produce minimal diffs.** A hash-preconditioned `set-text` patch changed
  **exactly one line** of the source file. Most vector formats reserialize the whole
  document on any edit, which is what makes them unreviewable in version control.
- **Degradation is graceful.** If the tooling vanished tomorrow, every committed
  atom is still a valid SVG that renders. The `data-pptv-*` attributes are inert
  metadata. This is the property that makes an alpha pin acceptable — the failure
  mode of the dependency is "you lose semantic editing," not "your assets die."

### 3.3 Authoring rules

These are the rules that survive version churn. They are properties of the
authoring discipline, not of any CLI verb.

1. **Every emitted object carries a globally unique, hierarchical, stable ID**
   (`overview.db.box`, not `rect47`). IDs are the addressing scheme for every
   patch, map, and reconciliation. Renaming one is a breaking change to anything
   holding a baseline.
2. **Geometry is explicit. Nothing is inferred.** No implicit layout, no autofit.
3. **DOM order is z-order.** There is no competing `z-index`.
4. **Text does not reflow.** Every text object declares its frame, font family,
   font size, line step, baseline, and hard line. One text object per visible line.
   This is the profile's defining constraint and the most common source of
   surprise — see §3.5.
5. **Group a shape and its label** when they should move as one component.
6. **Patches are hash-preconditioned.** A patch carries `baseSha256`; a patch
   against drifted source is rejected rather than applied. Never bypass this.
7. **Read the source wholesale only as an escape hatch.** The normal agent path is
   `outline` → `show`/`list` the objects you need → patch by stable ID → `validate`.

### 3.4 What is committed and what is not

| Artifact | Committed? |
|---|---|
| `diagram.pptv.svg` | **yes** — canonical source |
| `diagram.pptv.map.json` | yes, *if* you intend to reconcile an edited PPTX against it |
| `diagram.editable.pptv.html` | **no** — generated editor pack |
| `diagram.composed.pptv.html` | **no** — generated aggregation |
| `diagram.pptx` | **no** — derived output; regenerate it |

Add to the repo's `.gitignore`:

```gitignore
*.editable.pptv.html
*.composed.pptv.html
```

The editor pack is not a small file: a 622-byte, two-object atom generates a
**681 KB** wrapper. Committing them silently inflates a repo by three orders of
magnitude per diagram.

### 3.5 The no-reflow tax, stated plainly

Text does not wrap, shrink, or autofit. Change a label from "Cache" to
"Distributed cache layer" and it will overrun its frame, silently, in both the
browser and PowerPoint. The defense is `pptv text-fit`, which produces overflow
evidence without mutating source.

**`text-fit` currently requires an explicit `--font-map` and office180 does not
publish one** (see §7). Until it does, a repo authoring atoms must either build
its own font map or accept that overflow is caught by eye. Treat any diagram whose
labels changed as unaudited until someone has looked at it rendered.

## 4. The PowerPoint lane

An atom compiles directly to a native PPTX with an explicit placement and a
declared policy:

```bash
pptv compile diagram.pptv.svg \
  --placement 0,0,1200,800 \
  --policy identity \
  --output diagram.pptx --map diagram.pptv.map.json
```

Two policies, both fail-closed:

- `identity` — placement extent must equal the source `viewBox` extent exactly.
- `uniform-scale-translate` — placement must match the source aspect ratio exactly.

A non-uniform placement is **refused** (`PPTV-BASELINE-ASPECT`), never silently
stretched. This is the correct default and repos should not work around it: if a
figure needs a different aspect, change the atom's `viewBox` and re-lay it out
deliberately.

The emitted `.map.json` is the baseline that lets `pptv reconcile` turn a
human-edited PPTX back into a reviewable patch against the atom. If you intend to
hand a deck to someone who will edit it in PowerPoint, commit the map.

## 5. The document lane

For a document that must open in Word, markdown remains the canonical source:

```bash
md2docx.py report.md -t themes/neutral.json -o report.docx
docx2md.py report.docx -o report.back.md        # reverse
```

**Verified:** round-tripping a document through DOCX and back returns the source
**byte-for-byte**, provided the markdown is already in canonical spelling. Non-
canonical input round-trips to a normalized variant (e.g. `|---|---|` becomes
`| --- | --- |`).

Gate this in CI. `md2docx.py <file> --check` exits **1** on non-canonical source,
so a repo that round-trips documents should run it the same way it runs a
formatter check. `--normalize` writes the canonical form to stdout; redirect it
back over the source once, then the round trip is exact forever.

Generated DOCX files carry a provenance stamp (tool version, template, template
hash, source hash, generation time). Do not strip it — it is what lets a reader of
a stray `.docx` find the markdown that produced it.

## 6. What this practice does *not* require

Rebar's adopters are polyglot — Python, Go, TypeScript, shell. A practice that
required a Node toolchain of every repo would not be a practice, it would be a tax.

- **Reading, rendering, reviewing, and diffing an atom: no toolchain.** This is the
  common case and it is free.
- **Semantic editing, `text-fit` auditing, and PPTX compilation: Node ≥ 20** plus
  `@office180/pptv`. Required only of repos that actually do those things.
- **The document lane: Python 3.9+** and `python-docx`.

ASCII in a ` ```text ` fence remains valid and always will. A three-box diagram
does not need a file. Reach for an atom when the figure outgrows the fence, not
because the fence is forbidden.

## 7. The pin, the risks, and the revisit gate

This practice pins `@office180/pptv@0.1.0-alpha.4`. Adopting an alpha as doctrine
is a deliberate exception to [`spike-first-contracts.md`](spike-first-contracts.md),
taken because §3.2's graceful-degradation property makes the downside bounded: the
worst case is losing tooling, not losing assets.

**Known-open at adoption time**, all reported to office180:

1. **`text-fit` is unusable for a fresh consumer.** The published package is
   `files: ["assets","dist","README.md"]`; the font fixtures live under
   `test-fixtures/` and are not published. So the one command that defends the
   profile's defining constraint cannot run out of the box. This is the single
   highest-impact open item for adopters.
2. **C7–C11 are `in-progress`**, not verified. Native PPTX save/reopen,
   quantitative visual fidelity, and broad reconciliation are not proven surfaces.
   C4/C5/C6 — source, patch, and resolution, which is everything the diagram lane
   depends on — are `verified`.
3. **Alpha version churn.** The CLI verb surface has changed twice in four days.

**Revisit this practice when** office180 publishes a default font map (retire the
§3.5 caveat), or when the package leaves alpha (retire this section and consider a
`CONSUMES.md` pin). Both are single-section edits — that is why the pin lives in
one place.

**A repo adopting this practice should declare the dependency in its own
`CONSUMES.md`.** Rebar itself has not yet done so; whether the framework should
declare a pin on an adopter's contract is an open federation-posture question and
is the maintainer's call, not this practice's.

## 8. Provenance

Adopted from `office180-md-office-converter` (contracts C1–C11). Every behavioral
claim in §3.2, §4, and §5 was verified by executing the shipped tooling on
2026-08-02 against a working tree at `e42e7b6`+uncommitted; claims about contract
status were read from that repo's `CONTRACT-REGISTRY.md`. The evaluation that led
here is `feedback/2026-07-30-pptv-as-default-diagram-documentation.md`.
