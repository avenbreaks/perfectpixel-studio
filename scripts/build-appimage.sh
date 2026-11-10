#!/usr/bin/env bash
# Build the PerfectPixel GUI (Wails) as a Linux AppImage.
#
# What this does:
#   1. `wails build -platform linux/amd64`  ->  build/bin/perfectpixel
#      (wails runs `frontend:install` + `frontend:build` first, so npm must work)
#   2. linuxdeploy deploys the binary + GTK3/webkit2gtk libs into an AppDir
#   3. Bundle WebKitGTK's auxiliary executables and binary-patch the
#      libwebkit2gtk PKGLIBEXECDIR so they are found at runtime (see below)
#   4. linuxdeploy packages the AppDir into perfectpixel-x86_64.AppImage
#
# Why the binary patch (the hard part):
#   WebKitGTK spawns WebKitNetworkProcess / WebKitWebProcess / injected-bundle
#   from a COMPILE-TIME path (PKGLIBEXECDIR = /usr/lib/x86_64-linux-gnu/
#   webkit2gtk-4.0). Release builds ignore WEBKIT_EXEC_PATH and
#   WEBKIT_INJECTED_BUNDLE_PATH (both are #if ENABLE(DEVELOPER_MODE)-gated, and
#   Ubuntu ships developer mode OFF). linuxdeploy bundles the .so but not those
#   helper executables, so on a host without webkit2gtk-4.0 (e.g. Ubuntu 24.04)
#   the spawn fails with "No such file or directory". We rewrite the two path
#   strings in-place (same length, null-padded -> ELF-safe) to a writable dir
#   that AppRun populates with the bundled helpers before exec.
#
# Host requirements (see .github/workflows/release-gui.yml for the apt list):
#   Go 1.25, the Wails CLI, libgtk-3-dev, libwebkit2gtk-4.0-dev, pkg-config,
#   librsvg2-dev, gobject-introspection, libgirepository1.0-dev, file, curl,
#   python3.
#
# webkit2gtk ABI: the default Wails tags link webkit2gtk-4.0 (Ubuntu 22.04).
#   On Ubuntu 24.04 (webkit2gtk-4.1 only) set WAILS_TAGS="-tags webkit2_41";
#   this script then auto-selects the 4.1 helper dir and patch strings.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

APP_NAME="perfectpixel"
BUILD_BIN="$REPO_ROOT/build/bin"
DESKTOP_FILE="$REPO_ROOT/build/linux/$APP_NAME.desktop"
ICON_SRC="$REPO_ROOT/build/linux/$APP_NAME.png"

# Optional build tags (e.g. "-tags webkit2_41" when building on Ubuntu 24.04).
WAILS_TAGS="${WAILS_TAGS:-}"

# webkit2gtk ABI: mirror Wails' cgo tags (default 4.0, 4.1 when webkit2_41).
WEBKIT_ABI="${WEBKIT_ABI:-4.0}"
if [[ "$WAILS_TAGS" == *webkit2_41* ]]; then
  WEBKIT_ABI="4.1"
fi
WEBKIT_STAGE="/tmp/pp-webkit${WEBKIT_ABI}"
WEBKIT_HELPER_DIR="/usr/lib/x86_64-linux-gnu/webkit2gtk-${WEBKIT_ABI}"

LINUXDEPLOY_URL="https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage"
GTK_PLUGIN_URL="https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh"

for tool in wails curl python3; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: '$tool' not found on PATH" >&2; exit 1; }
done

[ -f "$DESKTOP_FILE" ] || { echo "error: missing $DESKTOP_FILE" >&2; exit 1; }
[ -f "$ICON_SRC" ] || { echo "error: missing $ICON_SRC" >&2; exit 1; }
[ -x "$REPO_ROOT/scripts/patch-webkit-so.py" ] || { echo "error: missing scripts/patch-webkit-so.py" >&2; exit 1; }

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

# 2. Temporary work area. linuxdeploy finds plugins next to its own binary and
#    writes the output AppImage into the CWD, so keep tools/ and out/ separate.
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/perfectpixel-appimage.XXXXXX")"
trap 'rm -rf "$WORKDIR"' EXIT

APPDIR="$WORKDIR/AppDir"
TOOLSDIR="$WORKDIR/tools"
OUTDIR="$WORKDIR/out"
APPRUN="$WORKDIR/AppRun"

mkdir -p "$APPDIR" "$TOOLSDIR" "$OUTDIR"

# 3. Fetch linuxdeploy + the GTK plugin.
LINUXDEPLOY="$TOOLSDIR/linuxdeploy-x86_64.AppImage"
GTK_PLUGIN="$TOOLSDIR/linuxdeploy-plugin-gtk.sh"
curl -fsSL "$LINUXDEPLOY_URL" -o "$LINUXDEPLOY"
curl -fsSL "$GTK_PLUGIN_URL" -o "$GTK_PLUGIN"
chmod +x "$LINUXDEPLOY" "$GTK_PLUGIN"

# 4. Custom AppRun: stage the bundled WebKit helpers into the patched dir,
#    disable the bwrap sandbox, source the GTK hook, then exec the app.
cat > "$APPRUN" <<APPRUN_EOF
#!/bin/bash
# PerfectPixel AppImage entrypoint.
STAGE="$WEBKIT_STAGE"
HELPERS="\$APPDIR/usr/lib/x86_64-linux-gnu/webkit2gtk-$WEBKIT_ABI"

# Put the bundled WebKit auxiliary executables where the patched
# libwebkit2gtk expects them (WEBKIT_STAGE == patched PKGLIBEXECDIR).
mkdir -p "\$STAGE"
cp -a "\$HELPERS/." "\$STAGE/"

# Also symlink the bundled shared libs into $STAGE: WebKit's process launcher
# strips LD_LIBRARY_PATH for auxiliary processes, and their RUNPATH is
# $ORIGIN, so they resolve libwebkit2gtk/libjavascriptcoregtk relative to
# their own dir (which otherwise has only the copied helpers, no libs).
for d in "\$APPDIR/usr/lib" "\$APPDIR/usr/lib/x86_64-linux-gnu"; do
  [ -d "\$d" ] || continue
  for f in "\$d"/*.so*; do
    [ -e "\$f" ] || continue
    ln -sf "\$f" "\$STAGE/\$(basename "\$f")"
  done
done

# bwrap sandboxing cannot start inside an AppImage; disable it.
export WEBKIT_FORCE_SANDBOX=0
export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1

# GTK/GLib paths for the bundled libraries.
if [ -f "\$APPDIR/apprun-hooks/linuxdeploy-plugin-gtk.sh" ]; then
  . "\$APPDIR/apprun-hooks/linuxdeploy-plugin-gtk.sh"
fi

exec "\$APPDIR/usr/bin/$APP_NAME" "\$@"
APPRUN_EOF
chmod +x "$APPRUN"

# 5. Phase A: deploy binary + libs + GTK resources into the AppDir (no package).
cd "$OUTDIR"
"$LINUXDEPLOY" \
  --appdir "$APPDIR" \
  --executable "$BUILD_BIN/$APP_NAME" \
  --desktop-file "$DESKTOP_FILE" \
  --icon-file "$ICON_SRC" \
  --custom-apprun "$APPRUN" \
  --plugin gtk

# 6. Phase B: bundle the WebKit helper executables and patch libwebkit2gtk.
if [ ! -d "$WEBKIT_HELPER_DIR" ]; then
  echo "error: $WEBKIT_HELPER_DIR not found (install libwebkit2gtk-${WEBKIT_ABI}-0 runtime)" >&2
  exit 1
fi
for helper in WebKitNetworkProcess WebKitWebProcess; do
  [ -x "$WEBKIT_HELPER_DIR/$helper" ] || { echo "error: $WEBKIT_HELPER_DIR/$helper missing" >&2; exit 1; }
done

mkdir -p "$APPDIR/usr/lib/x86_64-linux-gnu/webkit2gtk-${WEBKIT_ABI}"
cp -a "$WEBKIT_HELPER_DIR/." "$APPDIR/usr/lib/x86_64-linux-gnu/webkit2gtk-${WEBKIT_ABI}/"

mapfile -t WEBKIT_SO_LIBS < <(find "$APPDIR/usr/lib" -maxdepth 1 -type f -name "libwebkit2gtk-${WEBKIT_ABI}.so*" -print)
if [ "${#WEBKIT_SO_LIBS[@]}" -eq 0 ]; then
  echo "error: bundled libwebkit2gtk-${WEBKIT_ABI}.so not found in $APPDIR/usr/lib" >&2
  exit 1
fi
python3 "$REPO_ROOT/scripts/patch-webkit-so.py" --abi "$WEBKIT_ABI" --target "$WEBKIT_STAGE" "${WEBKIT_SO_LIBS[@]}"

# 7. Phase C: package the (now fixed-up) AppDir into an AppImage.
"$LINUXDEPLOY" \
  --appdir "$APPDIR" \
  --output appimage

# 8. Publish the artifact next to the binary (build/bin is gitignored).
APPIMAGE="$(find "$OUTDIR" -maxdepth 1 -name '*.AppImage' -print -quit)"
if [ -z "$APPIMAGE" ]; then
  echo "error: linuxdeploy did not produce an AppImage" >&2
  exit 1
fi

FINAL="$BUILD_BIN/$APP_NAME-$(uname -m).AppImage"
mv "$APPIMAGE" "$FINAL"
echo "AppImage: $FINAL"
ls -la "$FINAL"