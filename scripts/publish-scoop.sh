#!/usr/bin/env bash
# Render bucket/triage.json from a goreleaser dist/ and commit to lolay/scoop-bucket.
#
# Usage: scripts/publish-scoop.sh VERSION [dist-dir]
#   VERSION — semver without v prefix (e.g. 0.1.0)
#   dist-dir — goreleaser output (default: dist)
#
# Env:
#   SCOOP_BUCKET_TOKEN — PAT with Contents: write on lolay/scoop-bucket (required unless DRY_RUN=1)
#   DRY_RUN=1 — render manifest to stdout only; no git clone/push
set -euo pipefail

VERSION="${1:?VERSION required (no v prefix)}"
DIST="${2:-dist}"
DRY_RUN="${DRY_RUN:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/triage.json.tmpl"
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

SHA_WINDOWS_AMD64="$(sha_for "triage_${VERSION}_windows_amd64.zip")"
SHA_WINDOWS_ARM64="$(sha_for "triage_${VERSION}_windows_arm64.zip")"

render_manifest() {
  sed \
    -e "s/__VERSION__/${VERSION}/g" \
    -e "s/__SHA_WINDOWS_AMD64__/${SHA_WINDOWS_AMD64}/g" \
    -e "s/__SHA_WINDOWS_ARM64__/${SHA_WINDOWS_ARM64}/g" \
    "$TEMPLATE"
}

if [[ "$DRY_RUN" == "1" ]]; then
  render_manifest
  exit 0
fi

if [[ -z "${SCOOP_BUCKET_TOKEN:-}" ]]; then
  echo "SCOOP_BUCKET_TOKEN is required (or set DRY_RUN=1)" >&2
  exit 1
fi

BUCKET_DIR="$(mktemp -d)"
trap 'rm -rf "$BUCKET_DIR"' EXIT

git clone --depth 1 \
  "https://x-access-token:${SCOOP_BUCKET_TOKEN}@github.com/lolay/scoop-bucket.git" \
  "$BUCKET_DIR"

MANIFEST_PATH="${BUCKET_DIR}/bucket/triage.json"
mkdir -p "$(dirname "$MANIFEST_PATH")"

NEW_MANIFEST="$(mktemp)"
trap 'rm -rf "$BUCKET_DIR" "$NEW_MANIFEST"' EXIT
render_manifest >"$NEW_MANIFEST"

if [[ -f "$MANIFEST_PATH" ]] && cmp -s "$NEW_MANIFEST" "$MANIFEST_PATH"; then
  echo "Manifest already at triage ${VERSION}; nothing to commit"
  exit 0
fi

cp "$NEW_MANIFEST" "$MANIFEST_PATH"

git -C "$BUCKET_DIR" config user.name "triage-release-bot"
git -C "$BUCKET_DIR" config user.email "triage-release-bot@lolay.com"
git -C "$BUCKET_DIR" add bucket/triage.json
git -C "$BUCKET_DIR" commit -m "triage ${VERSION}"
git -C "$BUCKET_DIR" push origin HEAD

echo "Published bucket/triage.json for triage ${VERSION}"
