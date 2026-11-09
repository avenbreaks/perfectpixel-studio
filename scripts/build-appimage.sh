#!/usr/bin/env bash
# Build the PerfectPixel GUI (Wails) as a Linux AppImage.
#
# What this does:
#   1. `wails build -platform linux/amd64`  ->  build/bin/perfectpixel
#      (wails runs `frontend:install` + `frontend:build` first, so npm must work)
#   2. Assemble a linuxdeploy AppDir (binary + .desktop + icon)
#   3. `linuxdeploy` + `linuxdeploy-plugin-gtk`  ->  perfectpixel-x86_64.AppImage
#      (bundles GTK3 + webkit2gtk + friends so the result runs on Ubuntu 22.04
#      AND 24.04 without relying on the host's webkit ABI)
#
# Host requirements (see .github/workflows/release-gui.yml for the full apt list):
#   Go 1.25, the Wails CLI, libgtk-3-dev, libwebkit2gtk-4.0-dev, pkg-config,
#   librsvg2-dev, gobject-introspection, libgirepository1.0-dev, file, curl.
#
# webkit2gtk ABI note: the default Wails build tags link webkit2gtk-4.0
#   (Ubuntu 22.04). If you build on Ubuntu 24.04, which only ships
#   webkit2gtk-4.1, set WAILS_TAGS="-tags webkit2_41". The CI workflow
#   deliberately builds on ubuntu-22.04 with 4.0 and bundles the libs, so the
#   same AppImage runs on both LTS releases.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

APP_NAME="perfectpixel"
BUILD_BIN="$REPO_ROOT/build/bin"
DESKTOP_FILE="$REPO_ROOT/build/linux/$APP_NAME.desktop"
ICON_SRC="$REPO_ROOT/build/linux/$APP_NAME.png"

# Optional build tags (e.g. "-tags webkit2_41" when building on Ubuntu 24.04).
WAILS_TAGS="${WAILS_TAGS:-}"

LINUXDEPLOY_URL="https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage"
GTK_PLUGIN_URL="https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh"

for tool in wails curl; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: '$tool' not found on PATH" >&2; exit 1; }
done

[ -f "$DESKTOP_FILE" ] || { echo "error: missing $DESKTOP_FILE" >&2; exit 1; }
[ -f "$ICON_SRC" ] || { echo "error: missing $ICON_SRC" >&2; exit 1; }

# 1. Build the GUI binary (also builds the frontend via wails).
if [ -n "$WAILS_TAGS" ]; then
  wails build -platform linux/amd64 -tags "$WAILS_TAGS"
else
  wails build -platform linux/amd64
fi

if [ ! -x "$BUILD_BIN/$APP_NAME" ]; then
  echo "error: wails build did not produce $BUILD_BIN/$APP_NAME" >&2
  exit 1
fi

# 2. Temporary work area. linuxdeploy looks for plugins next to its own binary
#    and writes the output AppImage into the current working directory, so keep
#    tools/ and out/ separate to make the final artifact unambiguous.
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/perfectpixel-appimage.XXXXXX")"
trap 'rm -rf "$WORKDIR"' EXIT

APPDIR="$WORKDIR/AppDir"
TOOLSDIR="$WORKDIR/tools"
OUTDIR="$WORKDIR/out"

mkdir -p "$APPDIR" "$TOOLSDIR" "$OUTDIR"

# Icon is a committed 512x512 PNG derived from build/appicon.png (linuxdeploy
# rejects non-standard sizes like 1024x1024). Its name matches the desktop
# entry's Icon=perfectpixel.

# 3. Fetch linuxdeploy + the GTK plugin.
LINUXDEPLOY="$TOOLSDIR/linuxdeploy-x86_64.AppImage"
GTK_PLUGIN="$TOOLSDIR/linuxdeploy-plugin-gtk.sh"
curl -fsSL "$LINUXDEPLOY_URL" -o "$LINUXDEPLOY"
curl -fsSL "$GTK_PLUGIN_URL" -o "$GTK_PLUGIN"
chmod +x "$LINUXDEPLOY" "$GTK_PLUGIN"

# 4. Bundle GTK3/webkit2gtk libraries and produce the AppImage.
(
  cd "$OUTDIR"
  "$LINUXDEPLOY" \
    --appdir "$APPDIR" \
    --executable "$BUILD_BIN/$APP_NAME" \
    --desktop-file "$DESKTOP_FILE" \
    --icon-file "$ICON_SRC" \
    --plugin gtk \
    --output appimage
)

# 5. Publish the artifact next to the binary (build/bin is gitignored).
APPIMAGE="$(find "$OUTDIR" -maxdepth 1 -name '*.AppImage' -print -quit)"
if [ -z "$APPIMAGE" ]; then
  echo "error: linuxdeploy did not produce an AppImage" >&2
  exit 1
fi

FINAL="$BUILD_BIN/$APP_NAME-$(uname -m).AppImage"
mv "$APPIMAGE" "$FINAL"
echo "AppImage: $FINAL"
ls -la "$FINAL"