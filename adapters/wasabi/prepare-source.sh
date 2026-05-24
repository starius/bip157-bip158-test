#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${WASABI_SOURCE:-}" ]]; then
  echo "WASABI_SOURCE is not set. Run inside nix develop." >&2
  exit 1
fi

if [[ -z "${WASABI_P2P_PATCH:-}" ]]; then
  echo "WASABI_P2P_PATCH is not set. Run inside nix develop." >&2
  exit 1
fi

dest="${1:-$(pwd)/adapters/wasabi/.wasabi-src}"
rm -rf "$dest"
mkdir -p "$dest"
cp -R "$WASABI_SOURCE"/. "$dest"/
chmod -R u+w "$dest"
for patch in "$(dirname "$WASABI_P2P_PATCH")"/*.patch; do
  git -C "$dest" apply "$patch"
done
printf '%s\n' "$dest"
