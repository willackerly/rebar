# Feedback: inbox-watch multi-inbox mode self-echoes on your own outbound deposits — scope to own inbox only

**Date:** 2026-07-06
**Source:** `practice/inbox-watch` (+ the `rebar-inbox-watch` skill template)
**Type:** anti-pattern
**Status:** proposed
**Template impact:** `practice/inbox-watch` (the "multi-inbox capable — one directory per
repo pair" guidance), the inbox-watch skill template, and the coldstart skill's step-5 pointer
**From:** Claude (Fable 5) session, go-tak-server, 2026-07-06

## What Happened

During a live three-repo coordination night (a coordinated release bump: memos flying in both
directions), a watcher armed per the practice's multi-inbox guidance — own `inbox/` PLUS peer
inbox dirs filtered on filenames naming this repo — fired **four separate times on files this
repo's own agent had just deposited into peer inboxes**. Each self-echo cost a wake + read +
"ignore, that's mine" turn, and the human noticed the noise before asking for the fix. The
project owner then issued a standing SOP: every repo listens to its OWN inbox only.

Two aggravators worth recording:
1. The wide watcher had been armed by a *previous* session and **outlived it** (harness
   monitors can persist across session restarts), so a fresh session that armed a correctly
   scoped watcher still received peer-dir echoes from the stale one — double coverage, split
   provenance.
2. Filename-filtering peer dirs for your own repo's name selects FOR your own outbound memos
   (they're titled `<your-repo>-...` by the memo convention) — the filter makes self-echo
   *more* likely, not less.

## What Was Expected

The practice should have steered toward the symmetric contract that actually holds in a
multi-repo mesh: deposits addressed to a peer go IN the peer's inbox; every repo watches only
its own. Under that contract, watching peer dirs buys nothing (anything for you arrives in
your inbox) and costs self-echo + double-processing.

## Suggestion

1. In `practice/inbox-watch`, replace the "multi-inbox capable — one directory per repo pair"
   framing with: **watch your own `inbox/` only**; multi-inbox is for the rare repo that
   legitimately owns several inbound dirs, never for peer dirs. State the failure mode
   (self-echo on outbound deposits) explicitly.
2. Add a spinup check to the arming procedure: monitors can outlive sessions — before arming,
   look for an existing watcher; if a stale wide one is found, stop it and re-arm scoped.
3. go-tak-server's local skill copy already carries both edits
   (`.claude/skills/rebar-inbox-watch/SKILL.md`, commit `1a8ed03`) — usable as the wording
   donor.
