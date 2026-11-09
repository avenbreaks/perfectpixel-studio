# core — data-driven provider registry + config generalization

Branch: `feat/core-providers-linux` (off `a1385cc`). This role establishes the **contract**
that the `cloudflare` and `custom` roles consume. It does **not** implement those two providers.

## What changed

| File | Change |
|------|--------|
| `internal/gen/provider.go` | Replaced the hardcoded `modelCatalog` map + `switch`-based `DefaultModelFor`/`ProviderLabel`/`New` with a package-level, data-driven registry. |
| `internal/gen/gemini.go` | Added `init()` that self-registers gemini. |
| `internal/gen/openai.go` | Added `init()` that self-registers openai. |
| `internal/gen/openrouter.go` | Added `init()` that self-registers openrouter. |
| `internal/gen/fal.go` | Added `init()` that self-registers fal. |
| `internal/gen/byteplus.go` | Added `init()` that self-registers byteplus. |
| `internal/gen/provider_test.go` | Added `TestSupportedProvidersOrder` (exact UI order assertion). |
| `internal/config/config.go` | Added `ProviderCfg.BaseURL` + `ProviderCfg.AccountID`, `Settings.Cloudflare` + `Settings.Custom`, extended `Cfg()`, `Load()` env fallback, and `loadEnvFallback()` key list. |
| `internal/config/config_test.go` | Added tests for new cfg mapping, new env vars, and legacy-config migration safety. |

## Registry structure

`internal/gen/provider.go` now defines:

```go
type providerDef struct {
	id           string   // registry id, e.g. "gemini"
	label        string   // UI display name
	defaultModel string   // must equal models[0]
	models       []string // selectable models, newest first (models[0] == defaultModel)
	sortOrder    int      // UI ordering; lower sorts first
	newFunc      func(apiKey, model string) Provider // nil for multi-field providers
}

var providerRegistry = map[string]*providerDef{}
func registerProvider(def providerDef) { ... } // panics on empty id, empty models, or duplicate id

var SupportedProviders []string // filled by provider.go's init(), sorted by sortOrder (tiebreak: id asc)
```

`ModelsFor`, `DefaultModelFor`, `ProviderLabel`, and `New(provider, apiKey, model)` are all
now registry lookups (with gemini as the fallback for unknown ids, matching prior behavior).
`SupportedProviders` order is **independent of `init()` registration order** because it is
sorted by `sortOrder`. Current order: `gemini(1), openai(2), openrouter(3), fal(4), byteplus(5)`.

## How a new provider registers itself (the contract)

A new provider is a **single self-contained file** — it does **not** edit `provider.go`.
It adds its own `init()`:

```go
func init() {
	registerProvider(providerDef{
		id:           "cloudflare",              // e.g. ProviderCloudflare
		label:        "Cloudflare",              // UI name
		defaultModel: "@cf/black-forest-labs/flux-1-schnell",
		models:       []string{"@cf/black-forest-labs/flux-1-schnell", "@cf/stabilityai/stable-diffusion-xl-base-1.0"},
		sortOrder:    6,                          // pick the next free slot (6, 7, ...)
		// newFunc is nil: this provider needs extra fields, so it does NOT use gen.New.
	})
}
```

Rules:
- `models` must be non-empty and `models[0] == defaultModel` (a test asserts this).
- `sortOrder` must be unique (pick 6 for cloudflare, 7 for custom, etc.). The tie-break is
  id-ascending, but keep it unique to be safe.
- If `newFunc == nil`, `gen.New(provider, ...)` returns an explicit error
  (`"<id> 프로바이더는 전용 생성자가 필요합니다"`), so a multi-field provider MUST expose its
  own typed constructor and the app layer MUST call it directly.

## New `ProviderCfg` fields + env vars

`internal/config/config.go` — `ProviderCfg` now has (all with `omitempty` so old config files
round-trip cleanly and new keys are only written when non-empty):

```go
type ProviderCfg struct {
	APIKey    string `json:"apiKey"`
	Model     string `json:"model"`
	BaseURL   string `json:"baseUrl,omitempty"`  // custom (OpenAI-compatible) base URL
	AccountID string `json:"accountId,omitempty"` // Cloudflare Workers AI account id
}
```

`Settings` gains `Cloudflare ProviderCfg \`json:"cloudflare"\`` and
`Custom ProviderCfg \`json:"custom"\``. `Cfg(provider)` now returns `&s.Cloudflare` for
`"cloudflare"` and `&s.Custom` for `"custom"` (gemini remains the unknown-id fallback).

New env fallback (wired into both `Load()` and `loadEnvFallback()`):

| env var | populates |
|---------|-----------|
| `CLOUDFLARE_ACCOUNT_ID` | `s.Cloudflare.AccountID` (when empty) |
| `CLOUDFLARE_API_TOKEN`  | `s.Cloudflare.APIKey`    (when empty) |
| `CUSTOM_IMAGE_BASE_URL` | `s.Custom.BaseURL`       (when empty) |
| `CUSTOM_IMAGE_API_KEY`  | `s.Custom.APIKey`        (when empty) |

Legacy migration safety: `Load()` unmarshals into `Settings`; absent fields default to zero
values and no existing JSON field was renamed/removed. `Save()` uses `json.MarshalIndent` with
the `omitempty` tags, so empty `baseUrl`/`accountId` are not emitted.

## Exact constructor contract for cloudflare / custom roles

- Keep `gen.New(provider, apiKey, model)` for the **5 existing single-key providers** — unchanged
  signature, still green (`TestNewFactory`).
- Do **not** register cloudflare/custom into `providerDef.newFunc`. Their `newFunc` is `nil`.
- Instead, expose typed constructors and call them directly from the app layer using the cfg:
  - `gen.NewCloudflare(accountID, apiToken, model string) *Cloudflare`
    — reads `cfg.AccountID` + `cfg.APIKey` (the Workers AI token).
  - `gen.NewCustom(baseURL, apiKey, model string) *Custom`
    — reads `cfg.BaseURL` (required) + `cfg.APIKey` (optional); trim trailing `/` on BaseURL.
- Register their **metadata** (id/label/defaultModel/models/sortOrder) via `registerProvider` in
  their own file's `init()` so `SupportedProviders`, `ModelsFor`, `DefaultModelFor`,
  `ProviderLabel`, and `GetSettings()` see them automatically — but their **construction** stays
  in the typed constructor (app.go / frontend role will wire that call site).

## Verification

All in-scope Go packages build and test green:

```text
$ go test ./internal/config/ ./internal/gen/ -v
ok  perfectpixel/internal/config
ok  perfectpixel/internal/gen
```

Targeted evidence commands pass:

```text
$ go test ./internal/config/ -run TestSettingsCfg -v          # PASS
$ go test ./internal/gen/ -run 'TestNewFactory|TestModelsFor|TestDefaultModelFor' -v  # PASS
```

### Caveat: `go build ./...` at the repo root

`go build ./...` and `go test ./...` fail **only** at the root `main` package with:

```text
main.go:12:12: pattern all:frontend/dist: no matching files found
```

`main.go` embeds `//go:embed all:frontend/dist`, and `frontend/dist` is a **gitignored** build
artifact that does not exist in a fresh clone. Rebuilding it requires `(cd frontend && npm ci &&
npm run build)`, which currently fails on an **unrelated, pre-existing environment issue** in
`frontend` (TypeScript 4.6 pulls in a global `/home/kavn/node_modules/@types/node/ffi.d.ts` that
TS 4.6 cannot parse). This is the frontend role's domain and is outside this task's scope. Every
non-root Go package — including `internal/config`, `internal/gen`, `internal/sprite`, and all
`cmd/*` — builds and tests clean.