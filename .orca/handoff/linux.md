# linux — Linux GUI AppImage + release CI

Branch: `avenbreaks/linux` (off `feat/providers-linux` @ `8b8d789` = core already merged).
The task text named `feat/linux-providers-linux`; the coordinator-assigned worktree branch is
`avenbreaks/linux`, which is what this work is committed to. Nothing was merged to
`master`/`feat/providers-linux`.

## Files changed

| File | Change |
|------|--------|
| `build/linux/perfectpixel.desktop` | NEW — desktop entry (`Exec=perfectpixel`, `Icon=perfectpixel`, `Categories=Graphics;2DGraphics;RasterGraphics;`). |
| `build/linux/perfectpixel.png` | NEW — 512x512 PNG derived from `build/appicon.png` (1024x1024). Required because linuxdeploy **rejects** 1024x1024 icons (valid sizes are ≤512). |
| `scripts/build-appimage.sh` | NEW (executable) — full deterministic build: `wails build -platform linux/amd64` → linuxdeploy AppDir → AppImage. |
| `.github/workflows/release-gui.yml` | NEW — `v*` tag + `workflow_dispatch`; builds on ubuntu-22.04, uploads artifact, publishes to Release on tag. |
| `README.md` | Added "## Linux / AppImage build" section (after "Install & run"); existing content untouched. |
| `.orca/handoff/linux.md` | This file. |

`release-ppgen.yml`, `internal/`, `app.go`, `wails.json`, frontend source, and `.env.example`
were NOT modified.

## Build script (`scripts/build-appimage.sh`)

1. `wails build -platform linux/amd64` → `build/bin/perfectpixel` (runs frontend install + build).
2. Assembles a linuxdeploy AppDir (binary + desktop + icon) in a `mktemp -d` work area.
3. Downloads `linuxdeploy` + `linuxdeploy-plugin-gtk` into `$WORKDIR/tools`.
4. `linuxdeploy --appdir … --executable build/bin/perfectpixel --desktop-file … --icon-file build/linux/perfectpixel.png --plugin gtk --output appimage`.
5. Moves the result to `build/bin/perfectpixel-x86_64.AppImage` (gitignored).

Optional override: `WAILS_TAGS="-tags webkit2_41" ./scripts/build-appimage.sh` for building on
a host that only has webkit2gtk-4.1.

## Exact apt deps (ubuntu-22.04)

```bash
sudo apt-get update
sudo apt-get install -y \
  build-essential \
  pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.0-dev \
  libayatana-appindicator3-dev \
  librsvg2-dev \
  gobject-introspection \
  libgirepository1.0-dev \
  file \
  curl
```

Plus `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0` (the Wails CLI; no cgo needed
for the CLI itself) and Node 20 via `actions/setup-node@v4` (frontend build).

## AppImage tooling + versions

- `linuxdeploy` — `continuous` release, `linuxdeploy-x86_64.AppImage`.
  Verified locally: `linuxdeploy version 1-alpha (git commit ID 07333c6), GitHub actions build 373 built on 2026-11-01 21:00:06 UTC`.
- `linuxdeploy-plugin-gtk` — `master` branch `linuxdeploy-plugin-gtk.sh` (373 lines, no tagged release).
- The `appimage` **output** plugin is bundled inside the linuxdeploy AppImage (`--output appimage`
  needs no separate download). The `gtk` plugin is a separate shell script and must sit next to
  the linuxdeploy AppImage — the script puts both in `$WORKDIR/tools`, which is verified to be
  discovered by linuxdeploy.

## Trigger / artifact behavior

- Workflow `release-gui` triggers on `push: tags: ["v*"]` **and** `workflow_dispatch`.
- `build` job (ubuntu-22.04): setup Go 1.25 + Node 20 → apt deps → install Wails CLI →
  `./scripts/build-appimage.sh` → `actions/upload-artifact@v4` with
  `path: build/bin/perfectpixel-*.AppImage`.
- `release` job (only on `refs/tags/`): `actions/download-artifact@v4` (merge-multiple) →
  `softprops/action-gh-release@v2` with `files: dist/perfectpixel-*.AppImage`,
  `generate_release_notes: true`. `permissions: contents: write`.

## webkit2gtk ABI decision (important deviation)

The task text said to install `libwebkit2gtk-4.1-dev`, but **ubuntu-22.04 does not ship it**
(4.1 landed in 23.04/Debian 12; 22.04 only has `libwebkit2gtk-4.0-dev`). Building on 24.04
would also break 22.04 runtime (newer glibc). So the workflow deliberately builds on
**ubuntu-22.04 with `libwebkit2gtk-4.0-dev` and the default Wails tags** (verified against the
Wails v2.12.0 source: `#cgo !webkit2_41 pkg-config: webkit2gtk-4.0`), and linuxdeploy bundles
the webkit2gtk-4.0 + GTK3 libs into the AppImage. Result: the same AppImage runs on 22.04
(native 4.0) and 24.04 (bundled 4.0 libs, since 24.04 dropped the 4.0 runtime).

`-tags webkit2_41` is documented in the script + README as the override for anyone building on
24.04, but it is NOT what CI uses (it would break the 22.04 target).

## Verified locally vs CI-only

**Verified locally (no sudo, no native deps):**
- `bash -n scripts/build-appimage.sh` passes.
- `release-gui.yml` parses as YAML (PyYAML; note PyYAML 1.1 maps the `on:` key to `True` — a
  known YAML-1.1 quirk; GitHub Actions uses YAML 1.2 where `on` is a plain string key, and the
  existing `release-ppgen.yml` uses the identical `on:` shape).
- linuxdeploy CLI flags (`--appdir --executable --desktop-file --icon-file --plugin gtk --output appimage`)
  confirmed against the real tool.
- End-to-end smoke test with a dummy ELF + the committed 512x512 icon produced a **runnable**
  `Echo-x86_64.AppImage` (`file` = self-mounting ELF; executes and exits 0). This also caught and
  fixed the 1024x1024 icon rejection.

**Not verified locally (needs sudo + native deps + Wails CLI, unavailable in this sandbox):**
- `wails build -platform linux/amd64` (the real binary) — requires `libgtk-3-dev`,
  `libwebkit2gtk-4.0-dev`, pkg-config, and the Wails CLI; none installable without passwordless sudo.
- The `gtk` plugin's actual GTK3/webkit2gtk bundling (needs the GTK/rsvg/gir dev files).
- The frontend `tsc && vite build` step — the sandbox has a pre-existing global
  `/home/kavn/node_modules/@types/node/ffi.d.ts` that TS 4.6 cannot parse (frontend role's
  concern); a clean CI runner does not have that file.

## TODO / risks

- linuxdeploy + plugin-gtk are unpinned (`continuous` / `master`). Pin to explicit versions once
  a release cadence stabilizes if reproducibility is required.
- The AppImage bundles webkit2gtk and will be large (likely 100–200 MB); acceptable but worth
  noting for Release page size.
- Not tested on a real 22.04/24.04 desktop; CI is the source of truth for the first real build.
  If the AppImage fails to launch on 24.04, check that linuxdeploy actually bundled
  `libwebkit2gtk-4.0.so.37` (and not a host 4.1 lib) — the `gtk` plugin's `--bundle` behavior may
  need an extra flag for webkit-specific data files.
- `StartupWMClass=perfectpixel` is a best-guess; adjust if window grouping/dock icon misbehaves.
---

# Follow-up (2026-11-10): WebKit auxiliary-process crash fix

The first AppImage bundled `libwebkit2gtk-4.0.so.37` but NOT WebKitGTK's helper
executables, so on Ubuntu 24.04 (no webkit2gtk-4.0) launch failed with:
`Failed to spawn child process /usr/lib/x86_64-linux-gnu/webkit2gtk-4.0/WebKitNetworkProcess`.

## What changed (this follow-up)

| File | Change |
|------|--------|
| `scripts/patch-webkit-so.py` | NEW — binary-patches the two webkit path strings baked into the bundled `libwebkit2gtk-*.so` to a writable dir (same-length, null-padded → ELF-safe). |
| `scripts/build-appimage.sh` | Rewrote packaging into a 3-phase flow: linuxdeploy deploy → bundle WebKit helpers + patch the .so → linuxdeploy package. Added a custom AppRun. |
| `README.md` | Added a short "WebKit auxiliary-process fix" note. |
| `.orca/handoff/linux.md` | This section. |

## The actual root cause (why the env-var fix would NOT work)

WebKitGTK spawns its helpers from a **compile-time** path. In release builds both
override env vars are `#if ENABLE(DEVELOPER_MODE)`-gated and Ubuntu ships developer
mode OFF:

- `WEBKIT_EXEC_PATH` — dev-mode only. Verified: `strings libwebkit2gtk-4.1.so.0`
  contains **0** occurrences of the string, but does contain the hardcoded
  `/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1`.
- `WEBKIT_INJECTED_BUNDLE_PATH` — also dev-mode gated in webkitgtk 2.40/2.44/2.46
  source (`WebKitWebContext.cpp`); present in 2.52 but can't be relied on for the
  4.0 build. So we patch the injected-bundle path string directly instead.

Two path strings are embedded in `libwebkit2gtk` (confirmed with `strings` + `count`):
`/usr/lib/x86_64-linux-gnu/webkit2gtk-4.0` (40 bytes) and
`/usr/lib/x86_64-linux-gnu/webkit2gtk-4.0/injected-bundle/` (57 bytes). Only
`libwebkit2gtk` contains them (not `libjavascriptcoregtk`).

## The fix

1. `patch-webkit-so.py --abi 4.0 --target /tmp/pp-webkit4.0 <lib.so>` rewrites the
   57-byte injected-bundle string first (it contains the 40-byte string as a prefix),
   then the 40-byte PKGLIBEXECDIR, to `/tmp/pp-webkit4.0{/injected-bundle/}`. Strictly
   shorter + null-padded → same file size, SONAME/NEEDED/relocations intact.
2. `build-appimage.sh` bundles the whole `/usr/lib/x86_64-linux-gnu/webkit2gtk-4.0/`
   dir (WebKitNetworkProcess, WebKitWebProcess, WebKitGPUProcess, MiniBrowser,
   `injected-bundle/`) into `AppDir/usr/lib/x86_64-linux-gnu/webkit2gtk-4.0/`.
3. Custom AppRun (passed via `--custom-apprun`) does, before `exec`:
   `mkdir -p /tmp/pp-webkit4.0 && cp -a "$APPDIR/usr/lib/x86_64-linux-gnu/webkit2gtk-4.0/." /tmp/pp-webkit4.0/`,
   then `export WEBKIT_FORCE_SANDBOX=0` + `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1`,
   sources `apprun-hooks/linuxdeploy-plugin-gtk.sh`, and `exec usr/bin/perfectpixel "$@"`.
4. `WEBKIT_ABI` is 4.0 by default; auto-switches to 4.1 when `WAILS_TAGS` contains
   `webkit2_41` (staging dir becomes `/tmp/pp-webkit4.1`).

## Verified locally (Pop!_OS 24.04, webkit2gtk-4.1 as the stand-in)

- `patch-webkit-so.py` against the real `libwebkit2gtk-4.1.so.0.19.7`: same size,
  old strings → 0, new strings present, `readelf -d` NEEDED list + SONAME intact,
  `ldd` resolves, relocations intact.
- The exact `find` glob selects only the real `.so` (skips the SONAME symlink).
- AppRun end-to-end (dummy `usr/bin/perfectpixel` + dummy helpers + dummy gtk hook):
  stages helpers into `/tmp/pp-webkit4.0/`, sets sandbox vars, sources the gtk hook,
  execs the app with args.
- Two-phase linuxdeploy flow: deploy-only run + `--custom-apprun` + `--output appimage`
  preserves the custom AppRun and arbitrary AppDir files in the final AppImage.

## NOT verified locally (CI-only)

- The real `wails build` binary and the actual `libwebkit2gtk-4.0` lib (no sudo → can't
  install `libwebkit2gtk-4.0-dev`; sandbox has no 4.0 runtime). The 4.0 strings are the
  same length/structure as 4.1, so the patch is equivalent.
- A real GUI launch of the patched 4.0 AppImage on 22.04/24.04 — coordinator will do the
  final `workflow_dispatch` launch test before any release.

## Risks / TODO

- `/tmp/pp-webkit4.0` is shared by all running instances; a hostile/other process could
  tamper with it. Acceptable for now; could switch to a per-user XDG dir + patch that path.
- The `.so` string patch assumes `libwebkit2gtk-4.0` keeps these exact path strings; if a
  future webkit2gtk relocates the helpers (e.g. drops `PKGLIBEXECDIR`), the patch script
  will fail loudly (`no path strings found`) and needs revisiting.
- `WEBKIT_FORCE_SANDBOX=0` is deprecated in webkit2gtk ≥2.52; we also set
  `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1` for forward-compat. Both are harmless no-ops
  on versions that don't recognize them.

---

# Follow-up 2 (2026-11-10): symlink bundled libs into the WebKit stage dir

Second empirically-confirmed issue: the staged WebKitNetworkProcess/WebKitWebProcess
could not load `libwebkit2gtk-4.0.so.37` — WebKit's process launcher strips
`LD_LIBRARY_PATH` for auxiliary processes, and their RUNPATH is `$ORIGIN`, so they
only looked in `/tmp/pp-webkit4.0` (which had helpers but no libs).

Fix (in the custom AppRun, after staging the helpers): symlink every bundled shared
lib from `$APPDIR/usr/lib` and `$APPDIR/usr/lib/x86_64-linux-gnu` into `$STAGE`, so
the whole `$ORIGIN` RUNPATH chain resolves for the helper processes:

```bash
for d in "$APPDIR/usr/lib" "$APPDIR/usr/lib/x86_64-linux-gnu"; do
  [ -d "$d" ] || continue
  for f in "$d"/*.so*; do
    [ -e "$f" ] || continue
    ln -sf "$f" "$STAGE/$(basename "$f")"
  done
done
```

Verified locally by the coordinator: with helpers + symlinked libs in `/tmp/pp-webkit4.0`,
the AppImage launches and stays alive (Wails "Ctrl+C detected. Shutting down..."); the
gvfs "undefined symbol g_task_set_static_name" lines are benign host-module noise.

Changed: `scripts/build-appimage.sh` (AppRun heredoc) + this note.
