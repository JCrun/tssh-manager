#!/usr/bin/env bash
# Download the official tssh release (https://github.com/trzsz/trzsz-ssh, MIT),
# verify it against the release's checksums file, and place it for bundling.
#
#   scripts/fetch-tssh.sh <linux-amd64|windows-amd64|macos-universal> <bin-dir> <license-dir>
#
# TSSH_VERSION selects the release (default below). macos-universal merges
# the arm64 and x86_64 binaries with lipo, so it must run on macOS.
set -euo pipefail
TSSH_VERSION="${TSSH_VERSION:-0.1.27}"
target="$1" bindir="$2" licdir="$3"
base="https://github.com/trzsz/trzsz-ssh/releases/download/v${TSSH_VERSION}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }

curl -fsSL --retry 3 -o "$work/checksums.txt" "$base/tssh_${TSSH_VERSION}_checksums.txt"

# fetch <platform> <ext>: download, verify, unpack; prints the unpacked dir
fetch() {
  local f="tssh_${TSSH_VERSION}_$1.$2"
  curl -fsSL --retry 3 -o "$work/$f" "$base/$f"
  local want got
  want="$(awk -v f="$f" '$2 == f { print $1 }' "$work/checksums.txt")"
  got="$(sha256 "$work/$f")"
  if [ -z "$want" ] || [ "$want" != "$got" ]; then
    echo "checksum mismatch for $f: want '$want', got '$got'" >&2
    exit 1
  fi
  mkdir -p "$work/x"
  case "$2" in
    zip) if command -v unzip >/dev/null; then (cd "$work/x" && unzip -q "../$f"); else 7z x -bd -o"$work/x" "$work/$f" >/dev/null; fi ;;
    tar.gz) tar -C "$work/x" -xzf "$work/$f" ;;
  esac
  echo "$work/x/tssh_${TSSH_VERSION}_$1"
}

mkdir -p "$bindir" "$licdir"
case "$target" in
  linux-amd64) install -m755 "$(fetch linux_x86_64 tar.gz)/tssh" "$bindir/tssh" ;;
  windows-amd64) cp "$(fetch windows_x86_64 zip)/tssh.exe" "$bindir/tssh.exe" ;;
  macos-universal)
    arm="$(fetch macos_aarch64 tar.gz)/tssh"
    x86="$(fetch macos_x86_64 tar.gz)/tssh"
    lipo -create -output "$bindir/tssh" "$arm" "$x86"
    chmod 755 "$bindir/tssh"
    ;;
  *) echo "unknown target $target" >&2; exit 2 ;;
esac
curl -fsSL --retry 3 -o "$licdir/tssh-LICENSE.txt" "https://raw.githubusercontent.com/trzsz/trzsz-ssh/v${TSSH_VERSION}/LICENSE"
echo "bundled tssh ${TSSH_VERSION} for $target -> $bindir"
