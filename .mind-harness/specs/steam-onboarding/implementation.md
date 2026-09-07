---
feature: steam-onboarding
status: ready
created: 2026-07-09
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: main
---

# Implementation — agent-guided Steam credentials onboarding (A111)

## Load-bearing contract

`importer.Steam` must resolve credentials **env-first, db-meta second**:
`STEAM_API_KEY`/`STEAM_ID` environment variables, when set, always win over
values stored in the `meta` table — the CLI path on Grok-NIX keeps working
byte-identically. The db values are only consulted for a credential whose env
var is empty. Storage keys are `steam_api_key` and `steam_id` in the existing
`meta` table (schema already present; `SetMeta`/`GetMeta` helpers already
exist in `internal/db/db.go`).

## Files changed

### `internal/db/db.go`

Add named constants for the meta keys (used by both `cmd` and `importer`):

```go
// Meta keys for the Steam Web API credentials stored in-conversation via the
// set_steam_credentials MCP tool. Env vars STEAM_API_KEY/STEAM_ID take
// precedence over these at import time.
const (
	MetaSteamAPIKey = "steam_api_key"
	MetaSteamID     = "steam_id"
)
```

### `internal/importer/steam.go`

Extract credential resolution from `Steam()` into a helper and add the db
fallback (per-value, so an env var can supply one half and the db the other):

```go
// steamCredentials resolves the Steam Web API credentials: env vars first
// (the CLI path), then the db meta table (stored in-conversation via the
// set_steam_credentials MCP tool). Fallback is per-value.
func steamCredentials(database *sql.DB) (apiKey, steamID string) {
	apiKey = os.Getenv("STEAM_API_KEY")
	if apiKey == "" {
		apiKey = db.GetMeta(database, db.MetaSteamAPIKey)
	}
	steamID = os.Getenv("STEAM_ID")
	if steamID == "" {
		steamID = db.GetMeta(database, db.MetaSteamID)
	}
	return apiKey, steamID
}
```

`Steam()` (~line 33) replaces its two direct `os.Getenv` calls with
`steamCredentials(database)`. Update the two "skipped" messages so they no
longer name only the env vars — new wording, both places:

```
Steam API skipped (no credentials — use the set_steam_credentials tool, or STEAM_API_KEY/STEAM_ID env vars)
```

### `cmd/mcp.go`

**1. New tool `set_steam_credentials` in `registerTools`.** Schema:

```json
{
	"type": "object",
	"required": ["api_key", "steam_id"],
	"properties": {
		"api_key": {"type": "string", "description": "Steam Web API key — 32 hex characters, from https://steamcommunity.com/dev/apikey."},
		"steam_id": {"type": "string", "description": "SteamID64 — the 17-digit number identifying the account (steamcommunity.com/profiles/<SteamID64>)."}
	}
}
```

Tool description (agent-facing): "Store the user's Steam Web API key and
SteamID64 in the local gamesom database so refresh_library can import their
full owned Steam library, not just installed games. Env vars
STEAM_API_KEY/STEAM_ID take precedence when set. The key is stored in plain
text locally and can be regenerated at steamcommunity.com/dev/apikey."

Handler: trim whitespace from both values, validate, store both via
`db.SetMeta(database, db.MetaSteamAPIKey/MetaSteamID, ...)`, return a text
result confirming storage and suggesting `refresh_library` with
`sources: ["steam"]`. If `STEAM_API_KEY` or `STEAM_ID` is set in the server's
own environment, append a warning that the env value will shadow the stored
one at import time.

Extract the validation + store core into a plain function so it's testable
without constructing MCP request plumbing:

```go
// setSteamCredentials validates and stores the Steam Web API credentials in
// the meta table. Returns the user-facing confirmation text.
func setSteamCredentials(database *sql.DB, apiKey, steamID string) (string, error)
```

Validation (mispaste feedback for the agent-guided flow, not security):
- `api_key`: must match `^[0-9A-Fa-f]{32}$`. Error: "that doesn't look like a
  Steam Web API key (expected 32 hex characters) — it's shown at
  https://steamcommunity.com/dev/apikey after registering".
- `steam_id`: must match `^[0-9]{17}$`. Error: "that doesn't look like a
  SteamID64 (expected a 17-digit number) — if the profile URL contains
  /profiles/<number>, that number is it; a custom URL name won't work".

**2. Extend `sommelierBriefing`** with an onboarding paragraph (appended after
the existing refresh_library paragraph):

```
Steam coverage: by default the import sees only *installed* Steam games (local
manifest scan) — the full owned backlog needs a Steam Web API key. If the user
wonders where the rest of their Steam library is, or wants full coverage, offer
to set it up right here in the conversation:
1. Get the key at https://steamcommunity.com/dev/apikey (requires a
   non-limited Steam account — one that has spent at least $5 USD on Steam;
   the "domain" field can be anything, e.g. "localhost").
2. The SteamID64 is the 17-digit number in their profile URL
   (steamcommunity.com/profiles/<number>). If they use a custom profile URL,
   it's shown in the Steam client under Account details, below their username.
3. Store both with set_steam_credentials, then run refresh_library for steam.
When offering, mention: the key is stored in plain text in the local gamesom
database, and can be revoked/regenerated at the same URL any time.
```

### `packaging/mcpb/manifest.template.json`

- **Remove the `user_config` block entirely** (both Steam fields were pending
  the A097 decision; the decision cuts install-time credential entry).
- **Remove the `env` mapping** from `server.mcp_config` (its only entries were
  the two `${user_config.*}` references).
- **Add `set_steam_credentials` to the `tools` array**: "Store the user's
  Steam Web API key + SteamID64 (walked through in-conversation) to unlock
  full owned-library import."
- **Mirror the `sommelierBriefing` change into `prompts[0].text`** — that
  string is a hand-maintained copy of the Go constant and must stay in sync
  (JSON-escaped, `\n` newlines).
- **`long_description`**: append one onboarding sentence: "Out of the box the
  Steam import covers installed games; ask the sommelier to connect your full
  Steam library and it will walk you through it in chat."

### `internal/importer/steam_credentials_test.go` (new)

Hermetic tests for `steamCredentials` using `db.OpenAt` on a `t.TempDir()`
path and `t.Setenv`:

- `TestSteamCredentials_EnvWins` — env vars and meta both set to different
  values; env values returned.
- `TestSteamCredentials_MetaFallback` — env unset (`t.Setenv` to `""`), meta
  set; meta values returned.
- `TestSteamCredentials_PerValueMix` — `STEAM_API_KEY` env set, `STEAM_ID`
  env empty, both in meta; returns env key + meta id.
- `TestSteamCredentials_EmptyWhenNeither` — both surfaces empty; returns
  `"", ""` (Steam() then skips the API import, today's behavior).

### `cmd/mcp_steam_credentials_test.go` (new)

- `TestSetSteamCredentials_StoresValues` — valid key/id against an
  `OpenAt`-temp db; `GetMeta` returns both, confirmation text mentions
  refresh_library.
- `TestSetSteamCredentials_TrimsWhitespace` — values pasted with surrounding
  whitespace store trimmed.
- `TestSetSteamCredentials_RejectsBadKey` — 31 chars / non-hex → error naming
  the expected shape; nothing stored.
- `TestSetSteamCredentials_RejectsBadSteamID` — vanity-name string and
  16-digit number → error; nothing stored.
- `TestSetSteamCredentials_EnvShadowWarning` — `t.Setenv("STEAM_API_KEY", …)`;
  confirmation text carries the shadowing warning.

## Integration points

- `refresh_library` (cmd/mcp.go) → `importer.Steam(database)` — already passes
  the open db handle, so the meta fallback works there with no signature
  change. Same for the CLI `import` path.
- `sommelierBriefing` ↔ `manifest.template.json prompts[0].text` — manual
  mirror; this change touches both, keep them identical.
- Existing MCPB installs that set the (pre-release) `user_config` Steam
  fields lose the env injection on upgrade — acceptable: no public release
  has shipped with those fields in use, and stored-credential onboarding
  replaces them.
- `db_test.go`, `mcp_env_test.go` — untouched.

## Sequencing

Single PR off `main` (fd1d8a8). Not stacked.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — all new tests are
  hermetic (temp-dir db, `t.Setenv`), runnable in this sandbox.
- Not agent-verifiable here: the real conversational flow (Claude Desktop →
  paste key → `set_steam_credentials` → `refresh_library` imports owned
  games) and an MCPB rebuild/install — human pass, listed in the PR's Manual
  testing steps.

## Scope boundary

**In**: the tool, the fallback, the prompt guidance (Go constant + manifest
mirror), the `user_config` cut, tests. **Out**: vanity-URL resolution via the
Steam API, a get/clear-credentials tool surface, keychain storage, any other
importer, any change to the Steam API import logic itself.
