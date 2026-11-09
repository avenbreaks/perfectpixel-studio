# Task: linux — Linux GUI AppImage + release CI

Repo: `perfectpixel-studio` (Go 1.25 + Wails v2). Depends on **core** only (merged already).
Work on branch `feat/linux-providers-linux`. Do NOT merge to master/integration. Commit cleanly.

## Goal

Make the GUI build as a Linux AppImage and add a GitHub Actions job that builds and releases it.

## Scope (edit ONLY)

- `build/linux/**` (new AppImage packaging files).
- `.github/workflows/` (add a NEW workflow, e.g. `release-gui.yml`; do NOT break the existing `release-ppgen.yml`).
- `scripts/` (any AppImage packaging scripts).
- `README.md` (add a short "Linux / AppImage build" section; keep existing content).
- `.env.example` is NOT yours (frontend owns it). `wails.json` — only touch if strictly required for Linux build.

## What to build

1. **AppImage packaging for the Wails app** (`perfectpixel`, outputfilename already set in `wails.json`).
   - Standard approach: `wails build -platform linux/amd64` produces a binary under `build/bin/`, then wrap with
     `linuxdeploy` + `linuxdeploy-plugin-appimage` (or `appimagetool`) into a `.AppImage`.
   - Provide an icon (`build/appicon.png` exists — reuse or reference it), a `.desktop` file, and the AppRun glue
     as needed by linuxdeploy.
   - Add a `scripts/build-appimage.sh` (or Makefile target) that does the full build deterministically.

2. **CI workflow** `.github/workflows/release-gui.yml`:
   - Trigger: push of `v*` tags AND `workflow_dispatch` (mirror the existing release-ppgen triggers).
   - Job: `ubuntu-22.04` runner; install Go 1.25 and the Wails Linux deps
     (`libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, plus `libappindicator3-dev`/`libayatana-appindicator3-dev`,
     `pkg-config`, and the linuxdeploy/appimagetool tooling). Use `sudo apt-get update && sudo apt-get install -y ...`.
   - Build the frontend (`npm ci` is already run by `wails build` via `frontend:install`/`frontend:build`), then
     `wails build -platform linux/amd64` and package the AppImage via the script above.
   - Upload the `.AppImage` as a build artifact, and on `v*` tag publish it to GitHub Release
     (reuse `softprops/action-gh-release@v2` pattern from the existing workflow). `permissions: contents: write`.

## Constraints

- Do NOT modify `internal/`, `app.go`, frontend source, or the existing `release-ppgen.yml`.
- The AppImage must run on Ubuntu 22.04 AND 24.04 (test/note the webkit2gtk ABI: prefer `-tags webkit2_41` for
  4.1 if the host differs, and document it).
- Match existing style; new comments may be English; don't modify existing Korean comments.
- If you cannot install the native deps in this sandbox to fully build, still deliver the workflow + script +
  README, and clearly state in your handoff what was verified vs not.

## Evidence of done

- `scripts/build-appimage.sh` exists and is executable; `.github/workflows/release-gui.yml` lints (valid YAML).
- If a local `wails build` succeeds, include the binary/AppImage output and its `file`/size.
- README has the new section.
- Write `.orca/handoff/linux.md`: files changed, exact apt deps, AppImage tooling + versions, trigger/artifact
  behavior, what was verified locally vs CI-only, TODO/risks.