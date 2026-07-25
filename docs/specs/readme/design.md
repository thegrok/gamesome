---
feature: readme
status: approved
created: 2026-07-25
author: claude
type: lite   # content/structure decisions only, no system/UX/schema design
actions: A069
---

# Design (lite) — gamesome README (A069)

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
- **Security note**: A126 (separate action, not yet done) is the formal
  "security note + `0600` on the db file" action. A069's action text doesn't
  ask for a security section. But the Steam-key-storage disclosure
  ("stored in plain text locally, one-click revocable") is **already shipped
  product copy** — it's in the MCPB manifest's `long_description` and in the
  `sommelier` prompt text the server sends today. Repeating that exact,
  already-decided disclosure in the README's Steam import section is
  documentation accuracy, not new scope — it is not a substitute for A126
  (which additionally covers the `0600` file-permission code change and
  stays open).
- **Tool list**: enumerate the nine MCP tools straight from `cmd/mcp.go`
  `AddTool` calls + `packaging/mcpb/manifest.template.json`'s `tools` array
  (the two are supposed to stay mirrored — `TestManifestPromptsMirrorServer`
  covers prompts, not tools, so cross-check by hand here).

## Out of scope

- Fixing the stale root `.mcp.json` (flagged, not fixed — separate from a
  README action).
- A080's actual asciinema recording/embed.
- A126's `0600` db-file permission change.
- Any CLI/MCP behavior change.
