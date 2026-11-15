# scripts/

rebar's checks live in the rebar install, not in this project. This directory
holds only two entry points that hand off to the `rebar` CLI, so CI and the git
hook have a stable path to call. Upgrading rebar upgrades every check; there
is nothing to sync.

| File | Runs | Used by |
|------|------|---------|
| `ci-check.sh` | `rebar check` (full suite) | CI |
| `pre-commit.sh` | `rebar check --pre-commit` (TODOs, contract refs, ground truth) | git pre-commit hook |

Both are marked `# rebar-thin-wrapper`; the CLI never runs a marked file
itself. Calling `rebar check` directly from CI works just as well.

## Wire the hook

```bash
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit
```

## Configure, don't copy

Repo-specific settings go in `.rebarrc` (see the commented examples there):
source roots and extensions, the freshness window, registry classification,
and the METRICS ground-truth declarations (`metric.<key>.*`).

## Run a single check or tool

```bash
rebar run                     # list everything the install provides
rebar run check-todos         # one check
rebar run compute-registry    # regenerate architecture/CONTRACT-REGISTRY.md
rebar run steward             # quality scan → STEWARD_REPORT.md
rebar run inbox-watch inbox   # watch the peer inbox
```

Skip a check in CI with its variable, e.g. `SKIP_FRESHNESS=1 ./scripts/ci-check.sh`;
the full list is in the header of the install's `scripts/ci-check.sh`.

## Coming from copied scripts

Older adopters copied all of rebar's scripts here. Those copies still run (the
CLI prefers a project's own full copy and prints a deprecation notice), but they
no longer receive fixes. To migrate, run:

```bash
rebar migrate            # dry run: proposed .rebarrc lines, files to delete, files kept and why
rebar migrate --apply    # write .rebarrc, delete the verified copies, rerun the checks
```

It runs each copy and the install's script against the project and deletes a
copy only when the install, with the settings it derived from that copy,
behaves the same (identical metric values, a byte-identical registry, the same
check findings). `ci-check.sh` and `pre-commit.sh` become the two files above.
Copies it cannot verify stay, with the reason; move those by hand.
