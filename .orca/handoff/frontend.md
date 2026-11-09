# frontend — dynamic Settings fields + bindings + i18n

Branch: `avenbreaks/frontend` (per-role branch off `feat/providers-linux` @ `3db11a6`).
Depends on cloudflare + custom (both merged; their providers + `ProviderCfg` fields exist).

## Files changed

| File | Change |
|------|--------|
| `app.go` | `ProviderInfo` + `BaseURL`/`AccountID`; `GetSettings` fills them and computes per-provider `HasKey`; `provider()` routes cloudflare/custom through typed constructors; new `ProviderConfigArgs` + `SaveProviderConfig`; `SetProvider`/`SaveProviderModel` made data-driven via `isSupportedProvider`. |
| `frontend/src/components/SettingsModal.tsx` | Dynamic credential fields (key / accountId / baseUrl) per provider; new cloudflare + custom tabs; calls `SaveProviderConfig` for multi-field providers. |
| `frontend/src/i18n/ui.{en,es,ko,zh}.ts` | New strings (en = source of truth). |
| `frontend/wailsjs/go/main/App.d.ts` / `App.js` | Hand-added `SaveProviderConfig` binding (wails CLI unavailable; signatures mirror `app.go`). |
| `frontend/wailsjs/go/models.ts` | `ProviderInfo` + `baseUrl`/`accountId`; new `ProviderConfigArgs` class. |
| `.env.example` | Appended `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`, `CUSTOM_IMAGE_BASE_URL`, `CUSTOM_IMAGE_API_KEY`. |
| `frontend/tsconfig.json` | Added `"types": []` to fix the pre-existing tsc break (global `/home/kavn/node_modules/@types/node/ffi.d.ts` that TS 4.6 cannot parse). |

No backend provider/config implementations were modified.

## Binding signatures added (app.go)

```go
type ProviderInfo struct {
	HasKey     bool     `json:"hasKey"`
	KeyPreview string   `json:"keyPreview"`
	Model      string   `json:"model"`
	Models     []string `json:"models"`
	BaseURL    string   `json:"baseUrl"`   // custom (raw)
	AccountID  string   `json:"accountId"` // cloudflare (raw)
}

type ProviderConfigArgs struct {
	APIKey    string `json:"apiKey"`
	BaseURL   string `json:"baseUrl"`
	AccountID string `json:"accountId"`
}

func (a *App) SaveProviderConfig(provider string, args ProviderConfigArgs) error
```

`SaveProviderKey(provider, key)` is unchanged (still used by the 5 single-key providers).
`SetProvider(provider)` / `SaveProviderModel(provider, model)` no longer hardcode a switch list;
they accept any id in `gen.SupportedProviders` (now gemini, openai, openrouter, fal, byteplus,
cloudflare, custom).

## HasKey semantics (per provider)

- cloudflare → `AccountID != "" && APIKey != ""`
- custom → `BaseURL != ""` (key is optional)
- everything else → `APIKey != ""`

`KeyPreview` stays masked (`xxxx····xxxx`); `BaseURL`/`AccountID` are exposed raw so the UI can
pre-fill and show "saved" state.

## Env vars (already wired by core in `internal/config`)

- `CLOUDFLARE_ACCOUNT_ID` → `Settings.Cloudflare.AccountID`
- `CLOUDFLARE_API_TOKEN`  → `Settings.Cloudflare.APIKey`
- `CUSTOM_IMAGE_BASE_URL` → `Settings.Custom.BaseURL`
- `CUSTOM_IMAGE_API_KEY`  → `Settings.Custom.APIKey`

## i18n keys added (all 4 ui files)

`account_id`, `api_token`, `base_url`, `api_key_optional`, `save_config`,
`toast_config_saved`, `help_cloudflare`, `help_custom`.

## How SettingsModal renders cloudflare/custom

`PROVIDERS` is now a list of metas with a `fields: ICredField[]` array. Single-key providers keep
one `{id:"key", type:"password"}` field. cloudflare renders `accountId` (text, required) + `key`
(token, password, required); custom renders `baseUrl` (text, required) + `key` (password, optional).
Saved pre-fill: key shows masked `saved_as`, accountId/baseUrl show raw values as placeholder.
The save button is labelled `save_config` ("Save") for multi-field providers and calls
`SaveProviderConfig`; single-key providers keep `SaveProviderKey`. Model select/input is unchanged
and works for all providers now that `SaveProviderModel` is data-driven.

## Test/build results (this worktree)

```text
$ cd frontend && npm ci && npm run build   # tsc + vite build green → frontend/dist produced
$ go build ./...                           # exit 0 (root main package now embeds dist)
$ go test ./...                            # ok  perfectpixel / internal/config / internal/gen / internal/sprite
$ go vet ./...                             # exit 0
```

`frontend/dist` is gitignored (as before).

## TODO / notes

- `wails generate module` was not run (wails CLI not installed here); the three binding files were
  hand-edited to exactly match `app.go` (`ProviderConfigArgs`, `SaveProviderConfig`,
  `ProviderInfo.baseUrl/accountId`). If wails is later available, running `wails generate module`
  should produce equivalent output — confirm no drift before merge.
- Cloudflare `flux-1-schnell` may return JPEG; `app.go` already handles it (gallery.go
  blank-imports `image/jpeg`). Not touched.
- The `tsconfig.json` `types:[]` change is required for the pre-existing tsc failure; it is the
  frontend role's domain (coordinator explicitly assigned this fix).