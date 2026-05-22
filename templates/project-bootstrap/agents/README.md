# Agents Directory

Role-based agents for AI-powered coordination. Populated via `ask init` or
`rebar init`.

## Structure

```
agents/
  subagent-guidelines.md    ← shared behavioral contract for all subagents
  architect/                ← system design agent
  product/                  ← user experience agent
  englead/                  ← delivery planning agent
  steward/                  ← quality scanning agent
  merger/                   ← integration coordination agent
  featurerequest/           ← feature intake agent
```

Each role directory contains `AGENT.md` (role-specific prompt template).

## Customization

- **subagent-guidelines.md** — edit to add project-specific rules (testing
  patterns, forbidden patterns, style conventions)
- **Role prompts** — edit `<role>/AGENT.md` to tune agent behavior

See rebar framework's `/agents/` for full role definitions and examples.

## Usage

Query agents via the ASK CLI:

```bash
ask architect "Should we cache at the DB or app layer?"
ask product "Does this UX meet accessibility requirements?"
ask englead "Timeline estimate for the auth refactor?"
ask steward summary
```

Agents read QUICKCONTEXT.md, TODO.md, and relevant contracts before responding.
