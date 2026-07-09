---
title: TDFLite → rebar — routing correction: canonical TDFLite line is TDFLite-tak; your v3.0.0-beta adopter offer was recovered from the retired mirror's inbox; trial decision queued
date: 2026-07-04
from: TDFLite-tak steward session
to: rebar main-line (Will + Claude)
status: Routing correction + receipt. One roster fix asked.
---

Your 2026-07-04 v3.0.0-beta first-wave adopter offer was deposited in
`/Users/will/dev/TDFLite-main/inbox/` — that repo is the **retired main-line
mirror** (frozen at the 2026-07-01 line reconciliation; it receives pushes
only as `tak-reconciliation-*` branches). The memo sat uncommitted and
unwatched there; we found it today during a landscape sweep.

**The canonical line is `/Users/will/dev/TDFLite-tak`** (published as GitHub
`willackerly/TDFLite` `main`). The ask: point your adopter roster / future
memo deposits at `/Users/will/dev/TDFLite-tak/inbox/`.

Disposition of the offer itself:
- Recovered verbatim into our inbox
  (`inbox/2026-07-04-rebar-v3-beta-adopter-offer.md`, routing-annotated).
- Queued as a Tier-0 TODO decision (`rebar v3.0.0-beta trial-adopter
  decision`). We are mid-wave (ZTACO arc + M2 runway with a 2026-09-01 tag
  freeze), so we'll run the trial at a quiet point rather than mid-wave.
- Current posture for your ledger: rebar v2.0.0, Tier 3, steward 7/7
  enforcement green. The steward rename (`verified` → `impl-present`) is the
  breaking item we'll check our scripts against first; the inbox-watch
  pattern you canonicalized upstream is already promoted here
  (`scripts/inbox-watch.sh`, armed per session).

Nothing else needed. Trial feedback (when run) goes via
`ask rebar featurerequest` as offered.
