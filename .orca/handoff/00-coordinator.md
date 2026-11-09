# 00 — Coordinator handoff

Repo: `avenbreaks/perfectpixel-studio` (Go 1.25 + Wails v2 + React/Vite/TS)
Base branch: `master` (= `origin/main` @ `a1385cc`, "fix: inject magenta background hint… (#5)")
Integration branch: `feat/providers-linux` (created off `master`, empty so far)

> ⚠️ **Setup note (important).** This Orca worktree was created empty (a single
> "Initial commit" with no tree). The real code lives on GitHub at
> `https://github.com/avenbreaks/perfectpixel-studio.git` (branch `main`).
> Coordinator added `origin`, fetched, and repointed local `master` →
> `origin/main`. There is **no `main` local branch**; treat `master` as the base.
> The remote's default branch is `main` — if a role needs to push upstream, use
> `main`, but this task only requires local branches + CI workflows (no push).

## Objectives (from prompt)

- **A.1** Add `cloudflare` provider — Cloudflare Workers AI (`account_id` + `api_token`).
- **A.2** Add `custom` provider — OpenAI-compatible generic (`BaseURL` + optional key + free model).
- **A.3** *(optional, gated)* other public image APIs — propose list via `ask` first.
- **A.4** *(optional, gated)* Claude "prompt enhancer" (Anthropic) — Claude has no image model; gate before work.
- **B** Build Linux GUI as AppImage + a CI job that releases it.

Definition of done: new providers selectable in Settings, `ValidateKey` works,
generate succeeds (mock tests always; live tests only when env keys present),
AppImage builds in CI and runs on Ubuntu 22.04/24.04.

## Codebase map (what each file does)

- `internal/gen/provider.go` — provider registry (data-driven-ish): id constants,
  `SupportedProviders []string` (UI order), `modelCatalog map[string][]string`,
  `ModelsFor`, `Provider` interface (`GenerateImage(ctx, prompt, refImages [][]byte, aspectRatio string) ([]byte, error)`, `ValidateKey(ctx) error`),
  `DefaultModelFor`, `ProviderLabel`, `New(provider, apiKey, model) (Provider, error)`,
  helpers `aspectHint`, `decodeDataOrDownload`, `parseAspect`.
- `internal/gen/gemini.go` — `DefaultModel = "gemini-3-pro-image"`, `*Client`, model fallback chain.
- `internal/gen/openai.go` — `*OpenAI`: `/v1/images/generations` (JSON) + `/v1/images/edits` (multipart, field `image[]`), retry loop, test-overridable endpoints.
- `internal/gen/openrouter.go` — `*OpenRouter`: chat/completions, base64 image_url parts.
- `internal/gen/fal.go` — `*Fal`: `https://fal.run/<model>[/edit]`, `endpoint(hasRefs bool)`.
- `internal/gen/byteplus.go` — `*BytePlus`: `/api/v3/images/generations`, `endpoint` test override, `bpSizeFor`.
- `internal/config/config.go` — `ProviderCfg{APIKey,Model}`; `Settings{Provider, Gemini/OpenAI/OpenRouter/Fal/BytePlus, Legacy…}`;
  `Cfg(provider) *ProviderCfg` (pointer, defaults to Gemini); `Load()` (legacy migration + env fallback);
  `Save()`; `loadEnvFallback()` (reads `.env`/`.env.local` + OS env for a **hardcoded key list**).
- `app.go` — Wails bindings: `GetSettings() SettingsInfo`, `SaveProviderKey`, `SaveProviderModel`, `SetProvider`.
  `SaveProviderModel`/`SetProvider` contain **hardcoded provider switch lists** that must be extended for new providers.
  `ProviderInfo{HasKey,KeyPreview,Model,Models}`; `SettingsInfo{Provider, Providers map[string]ProviderInfo}`.
- `frontend/src/components/SettingsModal.tsx` — hardcoded `PROVIDERS: {key,label,placeholder}[]`;
  renders tabs, key input, model `<Select>` + custom model input. Calls `SaveProviderKey/SaveProviderModel/SetProvider`.
- `frontend/src/i18n/{ui.en,ui.es,ui.ko,ui.zh}.ts` — `Record<string,string>` dicts; missing key falls back en→key.
  Provider help strings are keyed `help_<provider>`.
- `frontend/wailsjs/go/main/*` — generated bindings (`wails generate` or `wails dev/build` regenerates). Don't hand-edit.
- `wails.json` — outputfilename `perfectpixel`; frontend build `npm run build`.
- `.env.example` — env-var documentation. New vars must be appended (keep existing).
- `.github/workflows/release-ppgen.yml` — existing workflow that cross-compiles the **headless CLI** `cmd/ppgen`
  to 5 platforms and attaches to a `v*` GitHub Release. **Not** the GUI. The new GUI AppImage job is a separate workflow.

## Key contracts for the new providers

- `ProviderCfg` currently has only `apiKey` + `model`. `cloudflare` needs `account_id` + `api_token`
  (two creds, not one key), and `custom` needs a `BaseURL`. The **core** role generalizes `ProviderCfg`
  (add `BaseURL` and `AccountID` fields) and makes the registry data-driven where useful.
- New env vars (append to `loadEnvFallback` key list **and** `.env.example`):
  - `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`
  - `CUSTOM_IMAGE_BASE_URL`, `CUSTOM_IMAGE_API_KEY`
- Cloudflare Workers AI REST: `POST https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{model}`
  with `Authorization: Bearer <api_token>`, JSON body `{prompt, ...}`; response `{result: {image: <b64>}}`
  (verify against current docs — flux/stable-diffusion are text-to-image; img2img support is model-dependent).
  Candidate models: `@cf/black-forest-labs/flux-1-schnell`, `@cf/stabilityai/stable-diffusion-xl-base-1.0`.
  Workers AI has a free tier but requires an account_id + token (not a plain single key).
- `custom` (OpenAI-compatible): build URLs as `{BaseURL}/v1/images/generations` and `{BaseURL}/v1/images/edits`
  (trim trailing `/` on BaseURL). API key optional (local engines like LocalAI/ComfyUI-proxy may have none);
  only send `Authorization` when key is non-empty. This subsumes LocalAI, ComfyUI-proxy, Together, etc.

## Provider interface / how `New` is dispatched

`gen.New(provider, apiKey, model)` is the single factory. `app.go` calls it with `cfg.APIKey`. For
cloudflare/custom, the factory signature or the config flow must accommodate the extra fields
(`AccountID` for cloudflare; `BaseURL` + optional key for custom). Prefer extending `ProviderCfg`
and having `app.go` build the right client (e.g. new `gen.NewCloudflare(accountID, token, model)`,
`gen.NewCustom(baseURL, key, model)`) — but **keep the existing `New(provider, apiKey, model)`
signature working** for the 5 current providers so old tests stay green.

## Test patterns to follow

- Unit: `internal/gen/*_test.go` use `httptest.NewServer` + a `roundTripFunc` RoundTripper and a `fakePNG`
  (`[]byte{0x89,'P','N','G',1,2,3}`). Override the client's endpoint field (e.g. `c.endpoint = srv.URL + …`).
- `internal/gen/provider_test.go` — `TestNewFactory` asserts a concrete type per provider (must add
  `*gen.Cloudflare`, `*gen.Custom` cases); `TestModelsFor` asserts every provider in `SupportedProviders`
  has non-empty models **and** `models[0] == DefaultModelFor(p)`; `TestDefaultModelFor`.
- `internal/gen/live_test.go` — smoke tests gated on `PP_LIVE_TEST=1` + provider env key; `t.Skip` otherwise.
  Add live tests for cloudflare/custom following this exact pattern. **Never** log/commit keys.
- `internal/config/config_test.go` — `TestParseEnvFile` (add new env keys), `TestSettingsCfg` (add new
  provider cfg mapping).

## Verification commands (each role runs the relevant subset)

```bash
go build ./... && go test ./...            # Go
(cd frontend && npm ci && npm run build)   # frontend
wails build                                # GUI (Linux: needs libgtk-3-dev, libwebkit2gtk-4.1-dev; use -tags webkit2_41 if needed)
```

## Conventions

- Match existing code style. New comments **may** be English; **do not modify existing Korean comments**.
- Do not change behavior of existing providers; all existing tests must stay green.
- Never log/commit API keys.
- Each role writes `.orca/handoff/<role>.md`.

## Roles & order (pipeline)

| role | deps | scope | notes |
|------|------|-------|-------|
| core | — | `internal/gen/provider.go`, `internal/config/` | refactor registry + add `BaseURL`/`AccountID` to `ProviderCfg`, safe legacy config migration |
| cloudflare | core | `internal/gen/cloudflare.go` (+`_test`) | Workers AI provider |
| custom | core | `internal/gen/custom.go` (+`_test`) | OpenAI-compatible provider |
| linux | core | `build/linux/**`, `.github/workflows/**`, `scripts/`, README | AppImage + release-gui workflow |
| frontend | cloudflare, custom | `frontend/src/**`, `app.go` bindings, `.env.example` | dynamic Settings fields + i18n |
| qa | frontend, linux | test-only | PASS/FAIL report |

Merge order: core → (cloudflare, custom, linux) → frontend → qa. Coordinator merges each passing
role into `feat/providers-linux`, and starts the next wave from that branch.