# qa — full-feature verification (test-only)

Branch: `avenbreaks/qa` (per-role branch off `feat/providers-linux` @ `26aec9b`).
Depends on frontend + linux (both merged). No feature code written or modified — verification only.

## Final verdict: **PASS**

All 7 Definition-of-Done items pass. No feature edits were needed; no failures routed to the
coordinator. One pre-existing (non-blocking) build-ordering caveat noted at the bottom.

## Definition of Done — evidence table

| # | DoD item | Result | Command + evidence |
|---|----------|--------|--------------------|
| 1 | `cloudflare` + `custom` appear in Settings; selectable; `ValidateKey` runs on save | **PASS** | `grep` in `SettingsModal.tsx` + `app.go`. `PROVIDERS[]` has `cloudflare` (accountId+key fields) and `custom` (baseUrl+key optional) tabs; `isMultiField` routes both to `SaveProviderConfig`, whose backend (`app.go:252`) calls `c.ValidateKey(a.ctx)` before `config.Save`. `GetSettings()` iterates `gen.SupportedProviders` (both registered via `init()` in `cloudflare.go`/`custom.go`). wailsjs bindings `SaveProviderConfig`/`SetProvider` present. |
| 2 | `go build ./... && go test ./...` green (all providers) | **PASS** | `go build ./...` → exit 0; `go test ./...` → `ok perfectpixel / internal/config / internal/gen / internal/sprite` (8.05s). Also `go vet ./...` exit 0. |
| 3 | cloudflare/custom mock tests pass, httptest only, no real keys | **PASS** | `go test ./internal/gen/ -run 'TestNewFactory\|TestModelsFor\|TestDefaultModelFor\|Cloudflare\|Custom' -v` → all PASS (CloudflareGenerateSuccess/RunURL/BinaryResponse/DataFieldFallback/MissingCreds/Non200Error/Retry429/ValidateKey200/ValidateKey401; CustomGenerationsB64/URL/EditsMultipart/MissingBaseURL/KeylessOmitsAuth/Non200/Retry429/ValidateKeyKeyless/ValidateKey401/ValidateKey404Fallback). `httptest` imported 6×/8× in cloudflare/custom tests; no key literals (`sk-`, `AIza`, `CLOUDFLARE_`, etc.). |
| 4 | Live tests skip when env keys absent | **PASS** | `TestCloudflareLive`/`TestCustomLive` both print `--- SKIP` with `PP_LIVE_TEST=1 설정 시에만 실행`; guard is `if os.Getenv("PP_LIVE_TEST") != "1" { t.Skip(...) }` (`cloudflare_live_test.go:17`, `custom_live_test.go:16`). |
| 5 | Frontend builds green; `help_cloudflare`/`help_custom` in en/es/ko/zh; `.env.example` has 4 new vars | **PASS** | `cd frontend && npm ci && npm run build` → exit 0, `dist/` produced (index.js 369.57 KiB). All 4 `ui.{en,es,ko,zh}.ts` contain `help_cloudflare` + `help_custom` (lines 60–61 each). `.env.example` lists `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`, `CUSTOM_IMAGE_BASE_URL`, `CUSTOM_IMAGE_API_KEY`. |
| 6 | Linux AppImage workflow + script; README docs Linux build | **PASS** | `.github/workflows/release-gui.yml` valid YAML (`python3 yaml.safe_load` OK); `scripts/build-appimage.sh` mode `-rwxrwxr-x` + `bash -n` clean; `README.md:259` `## Linux / AppImage build` section documents requirements + `./scripts/build-appimage.sh`. |
| 7 | No API keys/secrets in the diff | **PASS** | `git diff master..HEAD` added lines contain no `AKIA[0-9A-Z]{16}`, no `sk-[A-Za-z0-9]{20,}`, no PEM private-key block. `git grep` for `token|secret` matches only false positives (field names `api_token`/`apiKey`, spec/handoff docs, `Bearer <api_token>` placeholders). The single `[0-9a-f]{40}` hit is in the tracked binary `ppsamples` (initial commit, NOT in the feature diff). |

## Files changed (this role)

| File | Change |
|------|--------|
| `.orca/handoff/qa.md` | This QA report. |

No source/test edits were made. `frontend/dist` and `frontend/node_modules` are build artifacts
(gitignored) and were not committed.

## Non-blocking caveat (pre-existing, not a DoD failure)

`main.go` embeds `//go:embed all:frontend/dist` (unchanged in this diff), and `frontend/dist` is
gitignored — so on a fresh checkout `go build ./...` fails with
`main.go:12:12: pattern all:frontend/dist: no matching files found` **until** the frontend is built
first (`cd frontend && npm ci && npm run build`). The release-gui CI workflow handles this correctly
(`wails build` runs the frontend build first). Not a regression from this feature, but future CI/QA
scripts should run the frontend build before `go build`.

## Test command log (this worktree)

```text
$ cd frontend && npm ci && npm run build   # exit 0 (tsc + vite; only 'use client' warnings)
$ go build ./...                           # exit 0
$ go test ./...                            # ok (4 pkgs)
$ go test ./internal/gen/ -run '...Cloudflare|Custom|Factory...' -v   # 24 PASS, 2 SKIP (live)
$ go vet ./...                             # exit 0
$ python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release-gui.yml'))"  # YAML OK
$ bash -n scripts/build-appimage.sh        # OK; mode rwxrwxr-x
```