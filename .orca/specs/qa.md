# Task: qa — verify the full feature (test-only)

Repo: `perfectpixel-studio` (Go 1.25 + Wails v2). Depends on **frontend** and **linux** (both merged).
Work on branch `feat/qa-providers-linux`. Do NOT merge to master/integration. Commit cleanly.
**This role writes NO feature code** — only a QA report (+ any test-only fixes if a role's change
broke a build, coordinated via `ask`/coordinator, never silent feature edits).

## Goal

Verify the Definition of Done and report PASS/FAIL with evidence.

## Definition of Done (verify each)

1. `cloudflare` and `custom` providers appear in Settings; each is selectable; `ValidateKey` runs on save.
2. `go build ./... && go test ./...` green (all providers, old + new).
3. Mock/unit tests for cloudflare + custom pass with NO real API keys (httptest only).
4. Live tests skip cleanly when env keys are absent (`PP_LIVE_TEST` unset → skip).
5. Frontend builds (`cd frontend && npm ci && npm run build`) green; i18n has `help_cloudflare`/`help_custom`
   in en/es/ko/zh; `.env.example` lists the 4 new vars.
6. Linux AppImage workflow + script present: `.github/workflows/release-gui.yml` valid YAML,
   `scripts/build-appimage.sh` executable, README documents the Linux build.
7. No API keys/secrets committed anywhere in the diff (`git grep -nE 'sk-[A-Za-z0-9]|token|secret'` — false
   positives like "api_token"/"apiToken" field names are fine, but no real values).

## What to run

```bash
go build ./... && go test ./... 2>&1 | tail -40
go test ./internal/gen/ -run 'TestNewFactory|TestModelsFor|TestDefaultModelFor|Cloudflare|Custom' -v 2>&1 | tail -60
cd frontend && npm ci && npm run build 2>&1 | tail -40
# inspect: .github/workflows/release-gui.yml (yaml), scripts/build-appimage.sh (executable), README, .env.example
git grep -nE 'AKIA[0-9A-Z]{16}|sk-[A-Za-z0-9]{20,}|[0-9a-f]{40}' -- ':!go.sum' ':!package-lock.json' | head
```

## Deliverable

- `.orca/handoff/qa.md`: a table of each DoD item → PASS/FAIL with the command + a 1–3 line evidence snippet.
- For each FAIL: the failing role, the exact error, and a suggested fix (do NOT fix feature code yourself;
  send it to the coordinator, who will route a fix task).
- Final verdict: `PASS` only if all DoD items pass; otherwise `FAIL` with the blocking list.

## Constraints

- No feature code edits. Test-only changes allowed only to add/repair tests, not to mask failures.
- Match existing style; never log/commit keys.