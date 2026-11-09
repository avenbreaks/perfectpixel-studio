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