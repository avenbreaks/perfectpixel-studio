# Task: frontend — dynamic Settings fields + bindings + i18n

Repo: `perfectpixel-studio` (Go 1.25 + Wails v2 + React/Vite/TS). Depends on **cloudflare** and
**custom** (both merged; their providers + config fields exist). Work on branch
`feat/frontend-providers-linux`. Do NOT merge to master/integration. Commit cleanly.

## Goal

Make the Settings modal support the two new providers with their non-standard credential fields,
wire the Go bindings, add i18n for all 4 languages, and document the new env vars.

## Scope (edit ONLY)

- `app.go` — extend the Wails bindings (Settings info + save methods).
- `frontend/src/components/SettingsModal.tsx` — dynamic fields.
- `frontend/src/i18n/{ui.en,ui.es,ui.ko,ui.zh}.ts` — new strings (English source of truth; translate the rest).
- `frontend/wailsjs/go/main/*` — regenerated bindings (do not hand-edit if `wails generate module` is available;
  otherwise update `.d.ts`/`.js`/`models.ts` to exactly match `app.go`).
- `.env.example` — append new env vars, keep existing content.

## Requirements

1. **Providers with extra fields.** The Settings modal currently assumes one API-key input per provider.
   Now support:
   - `cloudflare`: two fields — **Account ID** (`CLOUDFLARE_ACCOUNT_ID`) and **API token** (`CLOUDFLARE_API_TOKEN`).
   - `custom`: two fields — **Base URL** (`CUSTOM_IMAGE_BASE_URL`, required) and **API key** (`CUSTOM_IMAGE_API_KEY`, optional).
   - Existing 5 providers keep their single-key input unchanged.
2. **Backend bindings (`app.go`)**:
   - `GetSettings` must expose, per provider, whatever the UI needs to pre-fill and show "saved" state:
     extend `ProviderInfo` with `BaseURL string` and `AccountID string` (AccountID may be shown raw; BaseURL raw;
     key preview stays masked as today). Keep `HasKey` meaningful per provider (cloudflare: account+token present;
     custom: BaseURL present).
   - Add a save path that persists the extra fields AND runs `ValidateKey`. Reuse the core/cloudflare/custom
     constructor contract from `.orca/handoff/{core,cloudflare,custom}.md`. For example a single
     `SaveProviderConfig(provider string, cfg ProviderConfigArgs) error` with `{APIKey, BaseURL, AccountID, Model string}`,
     or extend `SaveProviderKey` — your choice, but keep `SetProvider`/`SaveProviderModel` working for all providers
     (their hardcoded switch lists in `app.go` must become data-driven / include the new ids).
3. **i18n**: add `help_cloudflare` and `help_custom` (and any new labels like account id / base url / api token)
   to all 4 ui files. Match existing tone. Missing key falls back to en → key, so en must be complete.
4. **.env.example**: append (keep existing lines):
   ```
   # Cloudflare Workers AI
   CLOUDFLARE_ACCOUNT_ID=
   CLOUDFLARE_API_TOKEN=

   # Custom OpenAI-compatible endpoint
   CUSTOM_IMAGE_BASE_URL=
   CUSTOM_IMAGE_API_KEY=
   ```

## Constraints

- Do NOT change backend provider implementations (`internal/gen/*`, `internal/config/*`).
- Existing 5 providers must keep working in the UI exactly as today.
- Match existing style; new comments may be English; do NOT modify existing Korean comments.
- Never log/commit keys.

## Verification

- `go build ./... && go test ./...` green.
- `cd frontend && npm ci && npm run build` green (or `npm install` if `npm ci` fails on lockfile).
- If `wails` is available, `wails build` (or `wails generate module`) regenerates bindings — run it and confirm no drift.

## Evidence of done

- `go build ./...` and `npm run build` both green (paste tails).
- New provider tabs render with correct fields (describe/verify via the built bundle or component code).
- Write `.orca/handoff/frontend.md`: binding signatures added, ProviderInfo fields, which env vars, i18n keys added,
  how SettingsModal renders cloudflare/custom, test/build results, TODO.