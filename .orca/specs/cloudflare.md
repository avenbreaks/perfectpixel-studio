# Task: cloudflare — Cloudflare Workers AI provider

Repo: `perfectpixel-studio` (Go 1.25 + Wails v2). Depends on the **core** role's data-driven
registry (already merged into the integration branch you started from). Work on branch
`feat/cloudflare-providers-linux`. Do NOT merge to master/integration. Commit cleanly.

## Goal

Implement a Cloudflare Workers AI text-to-image provider as a new, self-contained file.

## Scope (edit ONLY)

- `internal/gen/cloudflare.go` (new) + `internal/gen/cloudflare_test.go` (new).
- Add your provider's self-registration `init()` in `cloudflare.go` (the core role's registry
  supports this; do NOT hand-edit `provider.go` — register from your own file).
- `internal/gen/live_test.go`: add a live smoke test gated on env (see pattern below).

## Contract (established by core — read `.orca/handoff/core.md` first)

- `Provider` interface: `GenerateImage(ctx, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error)` and `ValidateKey(ctx) error`.
- Registry is data-driven; register your provider from your own file's `init()` with: id `"cloudflare"`,
  label `"Cloudflare"`, default model, and a non-empty `[]string` model list where `models[0] == defaultModel`.
- Creds come from `ProviderCfg{APIKey, Model, BaseURL, AccountID}`: use `AccountID` + `APIKey` (the API token).
  Env fallback: `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN` (already wired by core).

## Implementation

- `type Cloudflare struct { AccountID, APIToken, Model string; HTTP *http.Client; endpoint string /* test override */ }`
- Constructor `NewCloudflare(accountID, apiToken, model string) *Cloudflare`.
- REST: `POST https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{model}`
  - Header `Authorization: Bearer <api_token>`, `Content-Type: application/json`.
  - Body `{"prompt": "<prompt + aspectHint>"}` (Workers AI flux/SD don't take an aspect param — reuse `aspectHint`).
  - Response: `{"result": {"image": "<base64 png>"}}`; also accept `{"result": {"data": "<base64>"}}` if docs say so — verify against current Cloudflare docs. Decode base64 → PNG bytes.
- `ValidateKey`: `GET https://api.cloudflare.com/client/v4/accounts/{account_id}` with Bearer token;
  HTTP 200 → ok, 401/403 → invalid, other → error. (Also reject empty AccountID/token early.)
- Retry 429/5xx up to 3 attempts with 2s→4s→8s backoff, respecting ctx.
- Default model: pick from current Workers AI text-to-image models (e.g. `@cf/black-forest-labs/flux-1-schnell` as default,
  also list `@cf/stabilityai/stable-diffusion-xl-base-1.0`). Verify the exact model ids against Cloudflare docs.
- `ValidateKey`/`GenerateImage` must return the missing-credential error when AccountID or token is empty.

## Tests

- Unit (`cloudflare_test.go`): use `httptest.NewServer` + a `roundTripFunc` (copy the pattern in `gemini_test.go`;
  `fakePNG = []byte{0x89,'P','N','G',1,2,3}`). Override `c.endpoint` to the test server. Cover: success decode,
  missing creds, non-200 error body, retry on 429, ValidateKey 200/401.
- Live (`live_test.go`): add `TestCloudflareLive` that `t.Skip`s unless `PP_LIVE_TEST=1` AND
  `CLOUDFLARE_ACCOUNT_ID`+`CLOUDFLARE_API_TOKEN` are set. Never log keys.

## Constraints

- `go build ./... && go test ./...` green. Existing tests stay green.
- Match existing style; new comments may be English; don't modify existing Korean comments or other providers.
- Never log/commit API tokens.

## Evidence of done

- `go test ./internal/gen/ -run 'TestNewFactory|TestModelsFor|TestDefaultModelFor|Cloudflare' -v` green
  (TestNewFactory must include `*gen.Cloudflare`; TestModelsFor must include `cloudflare`).
- `go build ./...` clean.
- Write `.orca/handoff/cloudflare.md`: files changed, models registered, request/response shape, test results, TODO.