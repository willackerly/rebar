#!/usr/bin/env bash
# package-release.sh — build rebar's release binaries and checksums.
#
# One self-contained binary per platform, built from the tree at <ref>
# (`git archive` of cli/): the bootstrap template, the ask shims, bin/install
# and the completions are compiled in (cli/internal/assets), so a binary
# alone runs `rebar new / adopt / init / migrate / check / ask / mcp`.
# setup-rebar.sh downloads one, verifies its checksum and runs
# `rebar install-shims` to lay out the companions beside it; its source
# fallback builds the same binary (same flags) from a git checkout.
#
# Releases up to v3.1.0 shipped tarballs of the whole tree instead;
# setup-rebar.sh still installs those.
#
# Used by .github/workflows/release.yml; runs the same locally.
#
# Usage:
#   scripts/package-release.sh <version> [--ref REF] [--out DIR] [--targets "os/arch ..."] [--repo DIR]
#
#   <version>   stamped into the binary (cli/cmd.Version) and the asset names
#   --ref       git ref to package (default: HEAD)
#   --repo      repository to package (default: the one holding this script)
#   --out       output directory (default: dist)
#   --targets   default: "darwin/amd64 darwin/arm64 linux/amd64 linux/arm64"
#
# Output (version v3.2.0 → "3.2.0"):
#   rebar_3.2.0_<os>_<arch>      one executable per target
#   rebar_3.2.0_checksums.txt    `sha256sum` format, one line per binary

set -euo pipefail

usage() { sed -n '2,29p' "$0" | sed 's/^# \{0,1\}//'; }

VERSION=""
REF="HEAD"
OUT="dist"
TARGETS="darwin/amd64 darwin/arm64 linux/amd64 linux/arm64"
REPO="$(cd "$(dirname "$0")/.." && pwd)"

while [ $# -gt 0 ]; do
  case "$1" in
    --ref)       REF="$2"; shift 2 ;;
    --ref=*)     REF="${1#--ref=}"; shift ;;
    --out)       OUT="$2"; shift 2 ;;
    --out=*)     OUT="${1#--out=}"; shift ;;
    --targets)   TARGETS="$2"; shift 2 ;;
    --targets=*) TARGETS="${1#--targets=}"; shift ;;
    --repo)      REPO="$2"; shift 2 ;;
    --repo=*)    REPO="${1#--repo=}"; shift ;;
    -h|--help)   usage; exit 0 ;;
    -*)          echo "package-release: unknown option: $1" >&2; exit 2 ;;
    *)           if [ -n "$VERSION" ]; then echo "package-release: one version only" >&2; exit 2; fi
                 VERSION="$1"; shift ;;
  esac
done

if [ -z "$VERSION" ]; then
  usage >&2
  exit 2
fi

VER_NAME="${VERSION#v}"
LDFLAGS="-s -w -X github.com/willackerly/rebar/cli/cmd.Version=${VERSION}"

for tool in git go tar; do
  command -v "$tool" >/dev/null 2>&1 || { echo "package-release: $tool is required" >&2; exit 1; }
done
if command -v sha256sum >/dev/null 2>&1; then
  SHA256="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA256="shasum -a 256"
else
  echo "package-release: sha256sum or shasum is required" >&2
  exit 1
fi

git -C "$REPO" rev-parse --verify --quiet "${REF}^{commit}" >/dev/null \
  || { echo "package-release: unknown ref: $REF" >&2; exit 1; }

mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/rebar-package.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

git -C "$REPO" archive --format=tar "$REF" cli | tar -x -C "$WORK"
[ -f "$WORK/cli/go.mod" ] || { echo "package-release: $REF has no cli/go.mod" >&2; exit 1; }

assets=""
for target in $TARGETS; do
  os="${target%/*}"
  arch="${target#*/}"
  name="rebar_${VER_NAME}_${os}_${arch}"
  echo "package-release: ${name}"
  (
    cd "$WORK/cli"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/$name" .
  )
  chmod 755 "$OUT/$name"
  assets="$assets $name"
done

# shellcheck disable=SC2086  # $assets is a list of plain file names
(cd "$OUT" && $SHA256 $assets > "rebar_${VER_NAME}_checksums.txt")
echo "package-release: wrote $(echo $assets | wc -w | tr -d ' ') binary(ies) + rebar_${VER_NAME}_checksums.txt to $OUT"
