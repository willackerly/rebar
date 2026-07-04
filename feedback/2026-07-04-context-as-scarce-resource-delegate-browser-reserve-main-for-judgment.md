# Feedback: Treat main-thread context as a scarce judgment-bearing resource — delegate browser/high-volume execution to subagents; reserve main thread for coordination + nuanced decisions

**Date:** 2026-07-04
**Source:** tak-tdf multi-day integration/coordination session — main context reached 72% (720k/1M), with the browser MCP `computer` tool alone accounting for ~75% of usage
**Type:** foundational-paradigm / improvement
**Status:** proposed
**Template impact:** `practices/multi-agent-orchestration.md` (a new "context economics" section), the agent-guidelines / CLAUDE.md templates (a default-delegation rule), possibly a lint/advisory in the harness
**From:** tak-tdf integration session (Claude Code, willackerly)

## What happened

A long coordination-heavy session drove the local COP through the browser repeatedly (federated
sign-in, IVM render verification, prod checks). Each browser interaction returns full-resolution
screenshots — the single most context-expensive payload class. By session end, main context was at
72% and the `/context` breakdown attributed ~75% of consumption to the browser `computer` tool.

Meanwhile, the work that *genuinely needed* the main thread — the cross-repo coordination fabric
(inbox memos, three-seat plan clearance, load-bearing architecture calls) — is exactly the work that
depends on the accumulated whole-picture context. Every screenshot that filled the window was
crowding out the resource the nuanced decisions actually run on.

The two are in direct tension, and the resolution is a paradigm, not a one-off:

## The paradigm (proposed as foundational)

**Main-thread context is a scarce, judgment-bearing resource. Match work to the surface that fits it.**

1. **Browser MCP calls → subagent BY DEFAULT.** Spawn a subagent with a crisp end-to-end instruction
   set AND an explicit end-state / success criteria; it drives the browser and returns a *conclusion*
   (pass/fail per checked case + evidence paths), not raw frames. The main thread keeps the verdict,
   not the pixels. (tak-tdf already has a `tactical-screenshot-scout` agent for exactly this.)
   - *Nuance:* delegation is cleanest when the end-state is known (verification, scripted flows).
     Genuinely exploratory browser debugging can still be delegated as "characterize X and report."
     Keep main-thread browser only for short, truly-interactive moments where you must react live.
2. **Reserve the main thread for COORDINATION and deep/nuanced/long-term decisions.** These need the
   full accumulated context and judgment — cross-repo coordination, architecture that's hard to
   reverse, synthesis across many inputs. This is what the scarce resource is *for*.
3. **Context is a budget to manage deliberately**, like tokens or time. High-volume, well-specified,
   low-judgment execution (browser automation, bulk file sweeps, large log triage) should be pushed
   to subagents/workflows that return distilled results. The main window's value is judgment continuity,
   not raw execution throughput.

## Why it fits rebar

Rebar is about doing serious, long-horizon work reliably. Long horizons mean long sessions, and long
sessions make context the binding constraint — the thing that, once exhausted, forces a cold start
mid-thought. Encoding "delegate the context-heavy execution, spend main-thread context on judgment and
coordination" as a default protects exactly the capability rebar exists to sustain: carrying a complex,
multi-party effort coherently over time. It pairs naturally with the multi-agent orchestration practice
already in rebar — this is the *when to reach for a subagent* rule that the orchestration tooling assumes.
