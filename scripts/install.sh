#!/bin/sh
# Build and install tssh-manager for the current user:
#   ~/.local/bin/tssh-manager
#   ~/.local/share/applications/tssh-manager.desktop
#   ~/.local/share/icons/hicolor/scalable/apps/tssh-manager.svg
# Usage: scripts/install.sh [--no-build] | --uninstall
# Also works from an unpacked release tarball (./install.sh next to the binary).
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
if [ -x "$HERE/tssh-manager" ] && [ -f "$HERE/tssh-manager.svg" ]; then
  # release tarball layout: prebuilt binary, nothing to build
  cd "$HERE"; SRC_BIN=./tssh-manager; SRC_SVG=./tssh-manager.svg; PREBUILT=1
else
  cd "$HERE/.."; SRC_BIN=build/bin/tssh-manager; SRC_SVG=build/tssh-manager.svg; PREBUILT=0
fi

BIN="$HOME/.local/bin"
APPS="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICONS="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"

refresh() {
  command -v update-desktop-database >/dev/null && update-desktop-database -q "$APPS" || true
  command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -q -t "$ICONS" 2>/dev/null || true
}

if [ "${1:-}" = "--uninstall" ]; then
  rm -f "$BIN/tssh-manager" "$APPS/tssh-manager.desktop" "$ICONS/scalable/apps/tssh-manager.svg"
  for s in 48 128 256; do rm -f "$ICONS/${s}x${s}/apps/tssh-manager.png"; done
  refresh
  echo "uninstalled (settings and backups in ~/.config/tssh-manager are kept)"
  exit 0
fi

if [ "$PREBUILT" = 0 ] && [ "${1:-}" != "--no-build" ]; then
  WAILS="$(command -v wails || echo "$HOME/go/bin/wails")"
  VER="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
  "$WAILS" build -tags webkit2_41 -clean -trimpath -ldflags "-s -w -X main.version=$VER" >/dev/null
fi

install -Dm755 "$SRC_BIN" "$BIN/tssh-manager"
install -Dm644 "$SRC_SVG" "$ICONS/scalable/apps/tssh-manager.svg"
if command -v rsvg-convert >/dev/null; then
  for s in 48 128 256; do
    mkdir -p "$ICONS/${s}x${s}/apps"
    rsvg-convert -w "$s" -h "$s" "$SRC_SVG" -o "$ICONS/${s}x${s}/apps/tssh-manager.png"
  done
fi

mkdir -p "$APPS"
cat > "$APPS/tssh-manager.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=tssh-manager
GenericName=SSH Host Manager
Comment=Manage ~/.ssh/config hosts for tssh; connect in your terminal
Comment[zh_CN]=管理 tssh 的 ~/.ssh/config 主机，在终端里连接
Exec=$BIN/tssh-manager
TryExec=$BIN/tssh-manager
Icon=tssh-manager
Terminal=false
Categories=Network;RemoteAccess;
Keywords=ssh;tssh;trzsz;server;host;config;
StartupWMClass=tssh-manager
StartupNotify=true
EOF
command -v desktop-file-validate >/dev/null && desktop-file-validate "$APPS/tssh-manager.desktop"
refresh
echo "installed: $BIN/tssh-manager"
