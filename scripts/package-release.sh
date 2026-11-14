#!/usr/bin/env bash
# package-release.sh — build rebar's release tarballs and checksums.
#
# Each tarball is the full rebar tree at <ref> (`git archive`: scripts/,
# templates/, agents/, bin/ask*, practices/, …) with bin/rebar replaced by a
# binary built from that same tree for one platform. Unpacked, it is a
# complete rebar install — the CLI finds its scripts at bin/../ — so
# setup-rebar.sh needs neither git nor Go when a tarball exists. Its source
# fallback produces the same layout (git archive + go build, same flags).
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
# Output (version v3.1.0 → "3.1.0"):
#   rebar_3.1.0_<os>_<arch>.tar.gz   one per target, top-level dir of the same name
#   rebar_3.1.0_checksums.txt        `sha256sum` format, one line per tarball

set -euo pipefail

usage() { sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'; }

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

# macOS tar would otherwise add AppleDouble ._ entries for extended attributes.
export COPYFILE_DISABLE=1

assets=""
for target in $TARGETS; do
  os="${target%/*}"
  arch="${target#*/}"
  name="rebar_${VER_NAME}_${os}_${arch}"
  echo "package-release: ${name}"

  git -C "$REPO" archive --format=tar --prefix="${name}/" "$REF" | tar -x -C "$WORK"
  (
    cd "$WORK/$name/cli"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags "$LDFLAGS" -o ../bin/rebar .
  )
  chmod 755 "$WORK/$name/bin/rebar"
  tar -C "$WORK" -czf "$OUT/${name}.tar.gz" "$name"
  rm -rf "${WORK:?}/${name}"
  assets="$assets ${name}.tar.gz"
done

# shellcheck disable=SC2086  # $assets is a list of plain file names
(cd "$OUT" && $SHA256 $assets > "rebar_${VER_NAME}_checksums.txt")
echo "package-release: wrote $(echo $assets | wc -w | tr -d ' ') tarball(s) + rebar_${VER_NAME}_checksums.txt to $OUT"
