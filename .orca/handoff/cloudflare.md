# cloudflare — Cloudflare Workers AI text-to-image provider

Branch: `avenbreaks/cloudflare` (off `8b8d789` = core merged into feat/providers-linux).

## Files changed

| File | Change |
|------|--------|
| `internal/gen/cloudflare.go` (new) | Self-contained provider: `init()` registry self-registration, `*Cloudflare` type, `NewCloudflare(accountID, apiToken, model)`, `GenerateImage`, `ValidateKey`. |
| `internal/gen/cloudflare_test.go` (new) | Unit tests (httptest + `roundTripFunc`, `fakePNG`). |
| `internal/gen/cloudflare_live_test.go` (new) | Live smoke test `TestCloudflareLive` (own file per coordinator note — NOT in shared `live_test.go`, to avoid merge conflict with the `custom` role). |

No existing file was modified (registry stays data-driven; `provider.go` untouched).

## Models registered

Via `init()` in `cloudflare.go` (`sortOrder: 6`, `newFunc: nil` per core contract):

- id `"cloudflare"`, label `"Cloudflare"`
- default `@cf/black-forest-labs/flux-1-schnell`
- list `["@cf/black-forest-labs/flux-1-schnell", "@cf/stabilityai/stable-diffusion-xl-base-1.0"]` (`models[0] == defaultModel`)

Both model IDs verified against current Cloudflare docs (developers.cloudflare.com).

## Constructor & credentials

`gen.NewCloudflare(accountID, apiToken, model)` reads `ProviderCfg.AccountID` + `ProviderCfg.APIKey`
(the Workers AI token). Env fallback `CLOUDFLARE_ACCOUNT_ID` / `CLOUDFLARE_API_TOKEN` is already
wired by core in `internal/config`. `gen.New("cloudflare", …)` returns the expected "전용 생성자가
필요합니다" error (`newFunc == nil`) — construction goes through the typed constructor, per core.md.

## Request / response shape

- `POST https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{model}`
  - `Authorization: Bearer <api_token>`, `Content-Type: application/json`
  - body `{"prompt": "<prompt + aspectHint>"}` (Workers AI flux/SD take no aspect param → prompt hint reused)
- Response (flux family): `{"result":{"image":"<base64>"},"success":true}` → base64-decoded to bytes.
- Defensive fallback: `result.data` (some model/version variants) also accepted.
- Binary output models (e.g. `@cf/stabilityai/stable-diffusion-xl-base-1.0`, whose output schema is
  `contentType: image/png, format: binary`) return raw PNG bytes with `Content-Type: image/*` — handled
  by returning the body directly.
- `ValidateKey`: `GET https://api.cloudflare.com/client/v4/accounts/{account_id}` (Bearer) — 200 ok,
  401/403 invalid, other → error. Empty AccountID/token rejected early in both `GenerateImage` and
  `ValidateKey`.

## Retry

429/5xx retried up to 3 attempts (1 + 2 retries) with 2s→4s→8s backoff, respecting `ctx`.

## Test results

```text
$ go test ./internal/gen/ -run 'TestNewFactory|TestModelsFor|TestDefaultModelFor|Cloudflare' -v
--- SKIP: TestCloudflareLive (PP_LIVE_TEST=1 설정 시에만 실행)
--- PASS: TestCloudflareGenerateSuccess / RunURL / BinaryResponse / DataFieldFallback
        / MissingCreds / Non200Error / Retry429 / ValidateKey200 / ValidateKey401
--- PASS: TestNewFactory / TestDefaultModelFor / TestModelsFor
ok  perfectpixel/internal/gen  6.035s

$ go test ./internal/gen/ ./internal/config/
ok  perfectpixel/internal/gen     6.036s
ok  perfectpixel/internal/config  0.002s

$ go build ./internal/... ./cmd/...   # clean
```

`go build ./...` / `go test ./...` fail **only** at the root `main` package with the pre-existing
`main.go:12:12: pattern all:frontend/dist: no matching files found` (gitignored `frontend/dist`;
frontend role's domain, out of scope — same caveat documented in core.md).

## TODO / notes for later waves

- **flux output format**: Cloudflare docs examples show flux-1-schnell returning `image/jpeg` (base64),
  while the task contract assumes PNG. `GenerateImage` is format-agnostic (just base64-decodes bytes),
  and the live test falls back to `jpeg.Decode` if `png.Decode` fails. Frontend sprite slicer must accept
  JPEG as well as PNG for the flux default (or the default model should be swapped to a PNG-returning one).
- **img2img**: `refImages` is currently ignored (text-to-image path only). SDXL supports `image_b64` /
  `image[]` for img2img if the reference-image edit path is needed later.
- **SDXL deprecation**: `@cf/stabilityai/stable-diffusion-xl-base-1.0` is flagged "Planned Deprecation
  11/20/2026" in the docs; consider swapping the secondary model before then.
- App layer (`app.go` bindings) + frontend Settings still need wiring to call `NewCloudflare` with the
  two-credential config — that is the frontend role's scope.