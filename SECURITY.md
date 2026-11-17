# Security

## Reporting a vulnerability

Report it privately through GitHub: the repository's **Security** tab →
**Report a vulnerability**. Please don't open a public issue for it. If the
private form isn't available, open an issue asking for a private contact,
with no details.

## Supported versions

Fixes go into the latest release. Upgrade with `setup-rebar.sh` (see
[UPGRADING.md](UPGRADING.md)).

## What is scanned, and when

rebar's CLI is Go; [govulncheck](https://go.dev/doc/security/vuln/) checks it
against the Go vulnerability database:

| When | What | Workflow |
|------|------|----------|
| Every PR and push to `main` / `release/*` | Source (`cli/`): vulnerabilities reachable from rebar's code, in its modules or the standard library | `ci.yml` |
| Every release tag | Each built binary (`-mode=binary`); any finding stops the release before upload | `release.yml` |
| Weekly (and on demand) | `main`'s source and the latest release's binaries, so new advisories against unchanged code surface | `vulncheck.yml` |

Dependabot proposes weekly, grouped updates of the Go modules and of the
SHA-pinned GitHub Actions (`.github/dependabot.yml`).

## Go toolchain policy

Release binaries are built with the latest patch of the newest supported Go
release (`go-version: stable` in `release.yml`), so standard-library fixes ship
in the next rebar release. The Go version is printed in the release log and
stated in the Release notes. `cli/go.mod`'s `go` line is only the minimum for
building from source; CI builds and tests with it too. Building from source,
use a [supported Go release](https://go.dev/doc/devel/release#policy) — the
binary includes your toolchain's standard library and its vulnerabilities.

rebar v3.1.0's binaries were built with go1.25.0 and include standard-library
vulnerabilities fixed in later Go releases; upgrade to v3.1.1 or later.
