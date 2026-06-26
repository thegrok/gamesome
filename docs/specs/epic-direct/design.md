---
feature: epic-direct
status: implemented
created: 2026-06-23
updated: 2026-06-26
actions: A075
---

# Design — Epic Games import (A075)

## Problem

On Windows and macOS, users may have Epic Games Launcher installed natively
without Heroic. The Linux importer (heroic.go) didn't run on those platforms.
EGL manifests only cover locally-installed games, not the full owned library.

## Approach

Three-layer strategy under a single `gamesom import epic` command, tried in order:

**Layer 1 — Heroic cache (preferred)**
Heroic Games Launcher caches the full owned Epic library as JSON on disk —
no network call or auth required at import time. Cross-platform paths:

| Platform | Heroic config path                                      |
|----------|---------------------------------------------------------|
| Linux    | `~/.var/app/com.heroicgameslauncher.hgl/config/heroic/` |
| macOS    | `~/Library/Application Support/heroic/`                 |
| Windows  | `%APPDATA%\heroic\`                                     |

Library: `store_cache/legendary_library.json`
Installed: `legendaryConfig/legendary/installed.json`

**Layer 2 — Legendary CLI**
For users who have Legendary but not Heroic. Auto-installs on Windows via
winget. On macOS, prints install instructions (`brew install pipx && pipx
install legendary-gl`) and falls through to Layer 3 without prompting.

**Layer 3 — EGL manifests (installed-only fallback)**
Reads `.item` files directly from the Epic Games Launcher manifests directory.
Only locally-installed games are visible; ownership = file exists.

| Platform | Path                                                                   |
|----------|------------------------------------------------------------------------|
| Windows  | `%PROGRAMDATA%\Epic\EpicGamesLauncher\Data\Manifests\`                 |
| macOS    | `~/Library/Application Support/Epic/EpicGamesLauncher/Data/Manifests/` |

DLC filter: `AppCategories` must contain `"games"`.
Source tag: `"epic"` across all layers (idempotent upsert handles overlap).
LauncherURI: `legendary://launch/<AppName>` (Heroic/Legendary layers), or
`com.epicgames.launcher://apps/<AppName>?action=launch` (EGL direct fallback).

## GOG

`gamesom import gog` reads Heroic's nile cache (same config dir, same
cross-platform path resolution). No non-Heroic GOG fallback yet.

## Scope

In:

- Linux + macOS + Windows (`heroicConfigDir()` switch in heroic.go covers all)
- Full owned library via Heroic cache; installed status via installed.json
- Legendary CLI fallback (Windows auto-install, macOS hint + fall-through)
- EGL manifest fallback (installed games only, Windows/macOS)
- GOG via Heroic nile cache (`import gog`)

Out:

- Non-Heroic GOG fallback (GOG Galaxy direct — future)
- No metadata enrichment beyond what Heroic/Legendary/manifests provide

## Files changed

- `internal/importer/heroic.go` — `heroicConfigDir()` cross-platform switch replacing hardcoded Flatpak paths
- `internal/importer/epic.go` — `Epic()` tries Heroic cache first via `epicFromHeroicCache()`, then Legendary CLI, then EGL manifests
- `internal/importer/legendary.go` — Legendary detection, winget install prompt, macOS hint, auth flow, `list-games --json`
- `internal/importer/heroic_gog.go` — `HeroicGOG()` reads nile cache using `heroicConfigDir()`
- `cmd/import.go` — `import epic` and `import gog` subcommands
