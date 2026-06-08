#!/usr/bin/env bash
# Render Formula/triage.rb from a goreleaser dist/ and commit to lolay/homebrew-tap.
#
# Usage: scripts/publish-formula.sh VERSION [dist-dir]
#   VERSION — semver without v prefix (e.g. 0.1.0)
#   dist-dir — goreleaser output (default: dist)
#
# Env:
#   HOMEBREW_TAP_TOKEN — PAT with Contents: write on lolay/homebrew-tap (required unless DRY_RUN=1)
#   DRY_RUN=1 — render formula to stdout only; no git clone/push
set -euo pipefail

VERSION="${1:?VERSION required (no v prefix)}"
DIST="${2:-dist}"
DRY_RUN="${DRY_RUN:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/triage.rb.tmpl"
CHECKSUMS="${DIST}/checksums.txt"

if [[ ! -f "$CHECKSUMS" ]]; then
  echo "checksums file not found: $CHECKSUMS" >&2
  exit 1
fi

sha_for() {
  local archive="$1"
  local hash
  hash="$(awk -v f="$archive" '$2 == f || $2 == "./" f { print $1; exit }' "$CHECKSUMS")"
  if [[ -z "$hash" ]]; then
    echo "sha256 not found for $archive in $CHECKSUMS" >&2
    exit 1
  fi
  printf '%s' "$hash"
}

SHA_DARWIN_ARM64="$(sha_for "triage_${VERSION}_darwin_arm64.tar.gz")"
SHA_DARWIN_AMD64="$(sha_for "triage_${VERSION}_darwin_amd64.tar.gz")"
SHA_LINUX_ARM64="$(sha_for "triage_${VERSION}_linux_arm64.tar.gz")"
SHA_LINUX_AMD64="$(sha_for "triage_${VERSION}_linux_amd64.tar.gz")"

render_formula() {
  sed \
    -e "s/__VERSION__/${VERSION}/g" \
    -e "s/__SHA_DARWIN_ARM64__/${SHA_DARWIN_ARM64}/g" \
    -e "s/__SHA_DARWIN_AMD64__/${SHA_DARWIN_AMD64}/g" \
    -e "s/__SHA_LINUX_ARM64__/${SHA_LINUX_ARM64}/g" \
    -e "s/__SHA_LINUX_AMD64__/${SHA_LINUX_AMD64}/g" \
    "$TEMPLATE"
}

if [[ "$DRY_RUN" == "1" ]]; then
  render_formula
  exit 0
fi

if [[ -z "${HOMEBREW_TAP_TOKEN:-}" ]]; then
  echo "HOMEBREW_TAP_TOKEN is required (or set DRY_RUN=1)" >&2
  exit 1
fi

TAP_DIR="$(mktemp -d)"
trap 'rm -rf "$TAP_DIR"' EXIT

git clone --depth 1 \
  "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/lolay/homebrew-tap.git" \
  "$TAP_DIR"

FORMULA_PATH="${TAP_DIR}/Formula/triage.rb"
mkdir -p "$(dirname "$FORMULA_PATH")"

NEW_FORMULA="$(mktemp)"
trap 'rm -rf "$TAP_DIR" "$NEW_FORMULA"' EXIT
render_formula >"$NEW_FORMULA"

if [[ -f "$FORMULA_PATH" ]] && cmp -s "$NEW_FORMULA" "$FORMULA_PATH"; then
  echo "Formula already at triage ${VERSION}; nothing to commit"
  exit 0
fi

cp "$NEW_FORMULA" "$FORMULA_PATH"

git -C "$TAP_DIR" config user.name "triage-release-bot"
git -C "$TAP_DIR" config user.email "triage-release-bot@lolay.com"
git -C "$TAP_DIR" add Formula/triage.rb
git -C "$TAP_DIR" commit -m "triage ${VERSION}"
git -C "$TAP_DIR" push origin HEAD

echo "Published Formula/triage.rb for triage ${VERSION}"
