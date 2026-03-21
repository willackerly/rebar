# Project Profiles

Different projects need different subsets of the rebar kit.
These profiles tell you what to adopt and what to skip.

## How to Use

Profiles have two dimensions — pick one from each:

1. **Project type** — what kind of software you're building
2. **Team size** — how many people work on it

If your project spans multiple profiles (e.g., a monorepo with a
web frontend and a crypto library), combine the relevant parts.

## By Project Type

| Profile | Best For |
|---------|----------|
| [web-app.md](web-app.md) | SPA, SSR, web frontend + API backend |
| [api-service.md](api-service.md) | Backend API, microservice, data pipeline |
| [crypto-library.md](crypto-library.md) | Security-critical library with interop needs |
| [cli-tool.md](cli-tool.md) | Command-line tool, developer utility |

## By Team Size

| Profile | Best For | Tier | Overhead |
|---------|----------|------|----------|
| [solo-dev.md](solo-dev.md) | 1 dev, 1-3 repos | 1 (Partial) | ~15 min setup |
| [small-team.md](small-team.md) | 2-10 devs, shared repos | 2 (Adopted) | ~45 min setup |
| [department.md](department.md) | 10+ devs, cross-repo deps | 3 (Enforced) | ~2 hours setup |
