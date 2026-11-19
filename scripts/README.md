# Scripts (maintainer tooling)

rebar is a single Go binary: every check, suite and utility adopters run is
built into `rebar` (`rebar check`, `rebar run <name>`; the list and the
`SKIP_*` variables are in [docs/CONFIGURATION.md](../docs/CONFIGURATION.md#what-rebar-check-runs)).
Adopting projects carry no copies of rebar's scripts; `templates/project-bootstrap/scripts/`
holds two thin entry points that call the CLI. Projects with copies made
before rebar 4.0: [UPGRADING.md](../UPGRADING.md).

This directory holds the tooling that maintains rebar itself. It runs only in
a rebar checkout.

| Script | What it does |
|--------|--------------|
| `sync-bootstrap.sh` (+ `--check`) | Keep `templates/project-bootstrap/scripts/` down to the thin entry points, copy the contract template, and refresh the binary's embedded copy of the template and the `bin/` shims (`cli/internal/assets/files/`); run it after editing them. `--check` is the Bootstrap Sync step of `rebar check` here; `go test` fails on drift too |
| `tests/run.sh [PATTERN]` | The shell regression tests (`tests/*.test.sh`), in parallel, against a `rebar` built from `cli/` (or `REBAR_TEST_BIN`, else `bin/rebar`); the Script Tests step of `rebar check` here. Helpers: `tests/lib.sh` |
| `package-release.sh <version>` | Build the release binaries (`rebar_<ver>_<os>_<arch>`, self-contained, one per platform) and `rebar_<ver>_checksums.txt`; what `.github/workflows/release.yml` runs on a `v*` tag. `--targets`, `--out`, `--ref`, `--repo` |
| `release-notes.sh <tag>` | Print CHANGELOG.md's `## <tag>` section, the GitHub Release body `.github/workflows/release.yml` publishes (it fails a tag without one). `--link-base URL` makes relative links absolute |
| `gen-released-blobs.sh` (+ `--check`) | Regenerate `cli/internal/migrate/released-blobs.txt` and `released/` from the release tags, so `rebar migrate` recognizes unmodified copies of every released script; run at release time, after tagging |
| `test-e2e-live.sh` | End-to-end smoke against a live LLM. Gated on the `claude` CLI; per-repo gates skip cleanly when adopted repos aren't on disk. Version triple-check (binary / `.rebar-version` / latest tag), MCP server discovery + tools/list smoke, ASK HTTP server smoke (if running), and live keyword-acceptance queries. Run after every meaningful update |

In this repository:

```bash
cd cli && go build -o ../bin/rebar .   # bin/rebar is git-ignored (not tracked since 4.0): build it
bin/rebar check                        # the full suite, including the two steps above
```

rebar's own `scripts/` has no `ci-check.sh` / `pre-commit.sh`: CI runs
`rebar check` built from the commit (`.github/workflows/ci.yml`), and a local
hook can be `printf '#!/bin/sh\nexec bin/rebar check --pre-commit\n'`.

## Dependencies

- **bash 3.2+** (macOS default) for these scripts; **go** to build the CLI.
- **git**, **grep**, **find**, **awk**, **sed**: standard Unix tools.
