# Task: custom — OpenAI-compatible generic provider

Repo: `perfectpixel-studio` (Go 1.25 + Wails v2). Depends on the **core** role's data-driven
registry (already merged into the integration branch you started from). Work on branch
`feat/custom-providers-linux`. Do NOT merge to master/integration. Commit cleanly.

## Goal

Implement a generic OpenAI-compatible image provider (covers LocalAI, ComfyUI-proxy, Together, etc.)
as a new, self-contained file.

## Scope (edit ONLY)

- `internal/gen/custom.go` (new) + `internal/gen/custom_test.go` (new).
- Add your provider's self-registration `init()` in `custom.go` (core's registry supports this;
  do NOT hand-edit `provider.go`).
- `internal/gen/live_test.go`: add a live smoke test gated on env (see pattern below).

## Contract (established by core — read `.orca/handoff/core.md` first)

- `Provider` interface: `GenerateImage(ctx, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error)` and `ValidateKey(ctx) error`.
- Registry is data-driven; register from your own `init()`: id `"custom"`, label `"Custom (OpenAI-compatible)"`,
  a default model (suggest `"gpt-image-1"` as a neutral default; the model list should contain at least that default
  so `models[0] == defaultModel`).
- Creds from `ProviderCfg{APIKey, Model, BaseURL, AccountID}`: use `BaseURL` (required) and `APIKey` (optional).
  Env fallback: `CUSTOM_IMAGE_BASE_URL`, `CUSTOM_IMAGE_API_KEY` (already wired by core).

## Implementation

- `type Custom struct { BaseURL, APIKey, Model string; HTTP *http.Client; generationEndpoint, editEndpoint string /* test override */ }`
- Constructor `NewCustom(baseURL, apiKey, model string) *Custom` (trim trailing `/` on BaseURL).
- Build endpoints: `{BaseURL}/v1/images/generations` and `{BaseURL}/v1/images/edits`.
- `GenerateImage`:
  - No ref images → POST `/v1/images/generations` JSON `{"model","prompt","n":1,"size","response_format":"b64_json"}`.
    Map aspect ratio to a `size` (reuse `openAISizeFor` from `openai.go` — same package, no import needed).
  - With ref images → POST `/v1/images/edits` multipart (fields `model`, `prompt`, `n`, `size`, and `image[]`
    parts per PNG). Model the multipart builder on `OpenAI.buildEditBody`.
  - Response: OpenAI-style `{"data":[{"b64_json"|"url"}]}`; accept either, decode base64 or download the URL.
  - Retry 429/5xx up to 3 attempts, backoff 2s→4s→8s, respect ctx.
- `Authorization: Bearer <APIKey>` ONLY when APIKey is non-empty (local engines often need none).
- `ValidateKey`: when APIKey is empty → ok (keyless is allowed). Otherwise `GET {BaseURL}/v1/models`
  (or `/v1/models/{model}`); 200 → ok, 401/403 → invalid. If the engine 404s `/v1/models`, treat as "cannot verify"
  but do not hard-fail — prefer a format/leniency check similar to `fal`/`byteplus` (len check) as the fallback.
  Decide and document the final behavior in your handoff.
- Return a clear error when `BaseURL` is empty (e.g. "Custom Base URL is not configured").

## Tests

- Unit (`custom_test.go`): `httptest.NewServer` + `roundTripFunc`; override `generationEndpoint`/`editEndpoint`.
  Cover: generations success (b64_json + url), edits multipart body contains `image[]`, missing BaseURL error,
  keyless request omits Authorization header, non-200 error, retry on 429.
- Live (`live_test.go`): add `TestCustomLive` that skips unless `PP_LIVE_TEST=1` AND `CUSTOM_IMAGE_BASE_URL` set.
  Never log keys.

## Constraints

- `go build ./... && go test ./...` green. Existing tests stay green.
- Match existing style; new comments may be English; don't modify existing Korean comments or other providers.
- Never log/commit API keys.

## Evidence of done

- `go test ./internal/gen/ -run 'TestNewFactory|TestModelsFor|TestDefaultModelFor|Custom' -v` green
  (TestNewFactory includes `*gen.Custom`; TestModelsFor includes `custom`).
- `go build ./...` clean.
- Write `.orca/handoff/custom.md`: files changed, model(s) registered, request/response shape, ValidateKey behavior, test results, TODO.