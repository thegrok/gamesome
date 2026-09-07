---
feature: readme
status: approved
created: 2026-07-25
author: claude
type: lite   # content/structure decisions only, no system/UX/schema design
actions: A069, A126
---

# Design (lite) — gamesome README (A069) + Steam key security note (A126)

**2026-07-26 scope amendment**: A126 (security note + `0600` db-file
permission) folded into this same branch/PR — cheap, same territory (the
Steam-key-storage disclosure), and A126's README half would otherwise
duplicate what this PR already writes. A126's code half (`0600` on the db
file) is added here too. See "Out of scope" below for what's still excluded.

## Problem

`gamesome` (repo `thegrok/gamesome`) has **no README at all** — root listing
confirms it (checked 2026-07-25, repo head `aa501b8`). Everything functional
on the critical path is shipped (persona-config/A099 was the last piece,
merged PR #33) and the release path is now just README + demo (A069/A080) →
tag v0.1.0 (A081). The release audience is Claude Desktop gamers, not
developers — the README is their first and primary onboarding surface.

## Scope decisions

- **Structure**: what/why hook → feature list → install (three paths) →
  per-platform import guide (four stores × three OSes) → MCP config snippet
  → demo → security note → dev/build-from-source.
- **"Which download for which situation" guide** (decided 2026-07-11, in the
  action text): `.mcpb` = one-click Claude Desktop install, **Windows/macOS
  only** (no Desktop on Linux, even though GoReleaser/build-mcpb.sh also
  produces a linux `.mcpb` — it has no Desktop client to install into);
  `tar.gz`/`zip` = the CLI binary, for Linux and any non-Desktop MCP client
  (Claude Code `.mcp.json`, etc.). State this as a decision table, not prose,
  so it scans fast for a non-developer.
- **Demo (A080 gate)**: A080 (asciinema recording) is not done yet. Write the
  Demo section with the intended embed markup commented out and a plain-text
  placeholder line, so landing A080 later is a one-line swap, not a
  restructure. Don't block A069 on A080 — the action text explicitly frames
  the embed as gated, not the whole README.
- **Per-platform import table**: pull from the as-built specs, not memory —
  `epic-direct`/`heroic-improvements` (Heroic-first chain, GOG platform-routed:
  Linux via Heroic nile cache, Windows/macOS via GOG Galaxy SQLite directly),
  `itch-install-location` (butler caves + install_locations, any custom
  location), `steam-x86-fallback` + `steam-onboarding` (manifests-only
  default; full ownership via in-conversation `set_steam_credentials`, not an
  install-time credential prompt — A097 explicitly rejected that).
- **MCP config snippet**: the repo's own `.mcp.json` is stale (references a
  nonexistent `mcp/` Python dir + `uv run gamesom-mcp` — predates the Go
  rewrite/rename). The README snippet must reflect what actually ships today:
  `{"command": "<path-to-gamesome-binary>", "args": ["mcp"]}`, matching
  `packaging/mcpb/manifest.template.json`'s `mcp_config` and `cmd/mcp.go`'s
  registered command. Flag the stale `.mcp.json` as a separate finding, not
  silently fixed inline (out of scope for a README-only action) — noted in
  the work-log.
- **Security note (A126, folded in)**: promoted from an inline aside in the
  Steam section to its own `## Security` heading, linked from the Steam
  section. Content is the already-shipped disclosure language (MCPB manifest
  `long_description` / `sommelier` prompt: plain text, revocable at
  steamcommunity.com/dev/apikey) plus the new `0600` file-permission fact.
- **`0600` db-file permission (A126, folded in)**: `internal/db/db.go`'s
  `OpenAt` chmods the db file to `0600` right after schema migration
  (best-effort — a chmod failure logs a warning but doesn't stop startup,
  since survival beats hardening for a single-user local file). Covered by
  `TestOpenAt_RestrictsFilePermissions` in `db_test.go` (skipped on Windows —
  POSIX permission bits don't apply there). OS keychain encryption-at-rest
  stays explicitly deferred, per A126's original decision (2026-07-14).
- **Tool list**: enumerate the nine MCP tools straight from `cmd/mcp.go`
  `AddTool` calls + `packaging/mcpb/manifest.template.json`'s `tools` array
  (the two are supposed to stay mirrored — `TestManifestPromptsMirrorServer`
  covers prompts, not tools, so cross-check by hand here).

## Out of scope

- Fixing the stale root `.mcp.json` (flagged, not fixed — separate from a
  README action).
- A080's actual asciinema recording/embed.
- OS keychain / encryption-at-rest for the Steam key (deferred in A126's
  original 2026-07-14 decision, not reopened here).
- Any other CLI/MCP behavior change.
