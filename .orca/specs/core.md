# Task: core — data-driven provider registry + config generalization

Repo: `perfectpixel-studio` (Go 1.25 + Wails v2). Work on branch `feat/core-providers-linux`
(created off the base you were given). Do NOT merge to master/integration. Commit cleanly.

## Goal (this role only — do NOT implement cloudflare/custom providers)

Refactor the provider registry to be data-driven and generalize the config model so two
later roles can add `cloudflare` and `custom` providers **without editing the registry file**.
You establish the contract; you do not add the two new providers.

## Scope (edit ONLY these)

- `internal/gen/provider.go` — data-driven registry refactor.
- `internal/config/config.go` — add fields + provider cfg entries + env fallback.
- `internal/config/config_test.go` — tests for the above.
- `internal/gen/provider_test.go` — update for the new registry shape (keep all current assertions green).
- *(allowed, narrow)* add a one-line `init()` registration to each existing provider file
  `internal/gen/{gemini,openai,openrouter,fal,byteplus}.go` IF your registry uses init-based
  self-registration. Do not change any other behavior in those files.

## What to build

1. **Data-driven registry in `provider.go`** that preserves EXACT current external behavior:
   - `SupportedProviders` order must stay: gemini, openai, openrouter, fal, byteplus.
   - `ModelsFor`, `DefaultModelFor`, `ProviderLabel`, `New(provider, apiKey, model)` all behave identically.
   - Prefer an `init()`-based self-registration pattern: a package-level ordered registry +
     `registerProvider(def)`; each provider file registers itself in its own `init()`. This lets the
     later `cloudflare.go` / `custom.go` be fully self-contained files (no shared-file edit → no merge conflict).
   - Preserve UI order independent of init order (e.g. a `sortOrder` field, or sort by explicit priority).

2. **`internal/config/config.go`**:
   - Add to `ProviderCfg`: `BaseURL string` (json `baseUrl,omitempty`) and `AccountID string` (json `accountId,omitempty`).
     Use `omitempty` so existing user config files on disk stay clean and round-trip unchanged.
   - Add to `Settings`: `Cloudflare ProviderCfg \`json:"cloudflare"\`` and `Custom ProviderCfg \`json:"custom"\``.
   - Extend `Cfg(provider)` to return `&s.Cloudflare` / `&s.Custom` for those ids (keep gemini default fallback).
   - Extend `Load()` env fallback and `loadEnvFallback()` key list with:
     `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`, `CUSTOM_IMAGE_BASE_URL`, `CUSTOM_IMAGE_API_KEY`.
     Populate `s.Cloudflare.AccountID`, `s.Cloudflare.APIKey`, `s.Custom.BaseURL`, `s.Custom.APIKey` from env when empty.
   - **Legacy migration safety**: loading an existing `config.json` written by the current code must still work
     (new fields simply default empty; old fields unchanged). Do not rename/remove any existing JSON field.

3. **Constructors for multi-field providers (contract for later roles):**
   - Keep `New(provider, apiKey, model)` for the existing 5 single-key providers, unchanged signature.
   - Do NOT implement `NewCloudflare`/`NewCustom` (later roles do). Just leave a clear comment in
     provider.go showing where/how a provider with extra fields (AccountID, BaseURL) registers its constructor.

## Constraints

- `go build ./... && go test ./...` must pass. Existing provider behavior and existing tests stay green.
- Match existing code style; new comments may be English; do NOT modify existing Korean comments.
- Do not touch `app.go`, frontend, workflows, or add the cloudflare/custom provider IDs/types.
- Never log/commit secrets.

## Evidence of done

- `go build ./... && go test ./...` green (paste the tail of `go test` output).
- `go test ./internal/config/ -run TestSettingsCfg -v` and `./internal/gen/ -run 'TestNewFactory|TestModelsFor|TestDefaultModelFor' -v` green.
- Write `.orca/handoff/core.md` documenting: the registry structure + how a new provider registers itself,
  the new `ProviderCfg` fields + env vars, and the exact constructor contract the cloudflare/custom roles must follow.