---
feature: epic-installed-fallback
status: draft
created: 2026-07-05
actions: A100
---

# Design — Epic installed-state: EGL manifest cross-check in the Heroic path (A100, Epic half)

*Lite design note — bugfix. Fix shape confirmed with Victor 2026-07-05.*

## Problem

On Victor's real Windows machine, `gamesom import epic` reports every Epic game
as not-installed. Root cause (confirmed, findings 003 fourth addendum):
`epicFromHeroicCache` (`internal/importer/epic.go`) treats a failed
`readHeroicInstalled()` — Heroic's `legendaryConfig/legendary/installed.json`,
which doesn't exist when Heroic is only used for GOG — as a mere warning,
substitutes an empty map, and still returns `nil`. That partial failure reported
as full success short-circuits `Epic()`'s designed fallback chain
(Heroic cache → Legendary CLI → EGL manifests), so the cross-check against
Epic's own launcher manifests — the real source of truth for installed state —
never runs.

## Fix shape — B, chosen over A100's literal fix-shape note

Two candidate shapes were traced; Victor picked **B** (2026-07-05):

- **A (as noted in A100):** surface the `readHeroicInstalled` error so `Epic()`
  falls through to Legendary CLI → manifests. Rejected: on a machine without
  legendary on PATH it detours into a `winget install` + Epic auth prompt (or,
  declined, an installed-only import that discards the successfully-read owned
  library for that run), and it regresses Linux — a Heroic user with no
  `installed.json` (nothing installed) currently gets a correct owned-only
  import, which A would turn into a hard error with no EGL manifests to fall
  through to.
- **B (chosen):** when `installed.json` is unreadable, keep the owned library
  from Heroic's cache and read install status from the EGL manifests
  (`EpicInstalledMap()`) directly inside the Heroic path. Same
  "EGL manifests as source of truth" intent A100 confirms, with no legendary
  detour, no owned-library loss, and no Linux regression (on Linux
  `EpicInstalledMap()` errors and we keep today's warn-plus-empty-map
  behavior).

`Epic()`'s existing fall-through chain is untouched — it still covers the case
where the Heroic cache itself (`legendary_library.json`) is missing.

## Scope

In:
- `epicFromHeroicCache`: on `readHeroicInstalled()` failure, build install info
  from `EpicInstalledMap()` instead of an empty map (new small helper).

Out:
- The union case: `installed.json` reads *successfully* but misses games
  installed via native EGL. Not Victor's confirmed failure mode; would change
  behavior for the happy path. Known residual gap — leave until it's observed.
- `LauncherURI` for EGL-installed games in the Heroic path stays
  `legendary://launch/<app>` (pre-existing; A100 is installed-state only).
- The itch.io half of A100 (separate fix), A101 (e2e blind spot), A102 (Steam
  via MCP subprocess).
