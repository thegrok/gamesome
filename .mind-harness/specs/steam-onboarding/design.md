---
feature: steam-onboarding
status: approved
created: 2026-07-09
author: claude
type: lite   # design already decided in A097 (decision-type action, 2026-07-08)
---

# Design (lite) — agent-guided Steam credentials onboarding (A111)

## Problem

A Steam Web API key is a developer-grade hurdle for the release audience, and
the local manifest scan only sees *installed* games — a partial ownership
picture. The MCPB `user_config` fields (A095) put the hurdle at install time,
before the user has any relationship with the tool.

## Decision (settled in A097, 2026-07-08)

- **Default is manifests-only**: works out of the box, honest about partial
  coverage. No credentials asked for at install time.
- **The ownership upgrade is agent-guided onboarding in-conversation**: the
  sommelier walks the user through obtaining a Steam Web API key + SteamID64,
  the user pastes them into the chat, and the agent stores them in the gamesom
  db (`meta` table) via a new MCP tool `set_steam_credentials`.
- **Env vars keep precedence** over stored values, so the CLI path on Grok-NIX
  (`STEAM_API_KEY`/`STEAM_ID` in the shell env) still works unchanged.
- **The A095 MCPB `user_config` Steam fields are cut** — they were pending this
  decision and the decision removes install-time credential entry.
- **Public-profile scraping rejected** (fragile, ToS-gray).

## Accepted trade-offs (named in A097, accepted)

- The key lives **plaintext in SQLite** (not the OS keychain).
- The key **transits the conversation once** when pasted.

Both acceptable for a low-stakes, regenerable key (revoke/regenerate any time
at steamcommunity.com/dev/apikey).

## Out of scope

Vanity-URL resolution via the Steam API (the agent guides the user to their
numeric SteamID64 instead), a credentials-clearing/inspection tool surface,
keychain storage, any non-Steam source.
