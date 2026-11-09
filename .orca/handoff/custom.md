# custom — OpenAI-compatible generic provider

Branch: `avenbreaks/custom` (per-role branch; the task preamble's `feat/custom-providers-linux`
name does not exist in this worktree — the coordinator created per-role `avenbreaks/*` branches).

## Files changed

| File | Change |
|------|--------|
| `internal/gen/custom.go` | New. `*Custom` provider: registry `init()`, `NewCustom`, `GenerateImage`, `buildEditBody`, `doRequest`, `ValidateKey`. |
| `internal/gen/custom_test.go` | New. Unit tests (httptest + roundTripFunc). |
| `internal/gen/custom_live_test.go` | New. `TestCustomLive` smoke test (per coordinator note: kept in its own file to avoid a merge conflict with the cloudflare role in `live_test.go`). |

No edits to `provider.go`, existing providers, or Korean comments.

## Model(s) registered

- id `custom`, label `Custom (OpenAI-compatible)`, `sortOrder` 7.
- `defaultModel` `gpt-image-1`, `models` `["gpt-image-1"]` (models[0] == defaultModel).
  The model is a neutral default; actual engine model names (LocalAI, ComfyUI-proxy,
  Together, etc.) are free-form and entered by the user.
- `newFunc` is `nil` (per core contract): construction goes through `gen.NewCustom`, not `gen.New`.

## Request/response shape

- Constructor `NewCustom(baseURL, apiKey, model) *Custom` — trims trailing `/` on BaseURL,
  defaults model via `DefaultModelFor(ProviderCustom)`.
- Endpoints: `{BaseURL}/v1/images/generations` and `{BaseURL}/v1/images/edits`
  (`generationEndpoint`/`editEndpoint` fields are test overrides).
- No ref images → `POST /v1/images/generations`, JSON
  `{"model","prompt","n":1,"size","response_format":"b64_json"}`; `size` via `openAISizeFor`.
- With ref images → `POST /v1/images/edits`, multipart fields `model`,`prompt`,`n`,`size`
  + one `image[]` part per PNG (filename `reference-N.png`).
- Response: OpenAI-style `{"data":[{"b64_json"|"url"}]}`; decodes b64 or downloads the URL.
- Retry: 429/5xx up to 3 attempts, backoff 2s→4s→8s, respects ctx.
- `Authorization: Bearer <APIKey>` only when APIKey is non-empty (keyless engines omit it).

## ValidateKey behavior

- Empty BaseURL → error `Custom Base URL is not configured`.
- Empty APIKey → OK (keyless engines like LocalAI are allowed).
- Otherwise `GET {BaseURL}/v1/models`:
  - 200 → valid
  - 401/403 → `API key is invalid or lacks permission`
  - 404 → cannot verify (some engines don't implement `/v1/models`); fall back to a
    leniency check (`len(key) < 10` → `API key is too short`), else OK — does not hard-fail.
  - other → `key check failed (HTTP <code>)`

## Test results

```
$ go test ./internal/gen/ -run 'TestNewFactory|TestModelsFor|TestDefaultModelFor|Custom' -v
... TestCustomLive (SKIP), TestCustomGenerationsB64 PASS, TestCustomGenerationsURL PASS,
    TestCustomEditsMultipart PASS, TestCustomMissingBaseURL PASS, TestCustomKeylessOmitsAuth PASS,
    TestCustomNon200 PASS, TestCustomRetry429 PASS (2.01s), TestCustomValidateKey* PASS,
    TestNewFactory PASS, TestDefaultModelFor PASS, TestModelsFor PASS
ok  perfectpixel/internal/gen  2.019s

$ go test ./internal/gen/ ./internal/config/
ok  perfectpixel/internal/gen
ok  perfectpixel/internal/config

$ go build ./internal/gen/ ./internal/config/ ./internal/sprite/ ./cmd/...   # clean
```

`go build ./...` fails **only** at the root `main` package with the pre-existing
`main.go:12:12: pattern all:frontend/dist: no matching files found` (gitignored embed,
documented in core.md; frontend role's domain, out of scope).

## TODO / notes

- `app.go` (frontend role) must wire the call site: `cfg := settings.Custom; gen.NewCustom(cfg.BaseURL, cfg.APIKey, cfg.Model)`.
- `ProviderCfg` already carries `BaseURL`; `Cfg("custom")` and `CUSTOM_IMAGE_BASE_URL` /
  `CUSTOM_IMAGE_API_KEY` env fallback are already wired by core.
- Frontend Settings modal still needs dynamic fields for BaseURL + optional key (frontend role).
- Live test not exercised here (no `PP_LIVE_TEST=1` / `CUSTOM_IMAGE_BASE_URL` in this env).