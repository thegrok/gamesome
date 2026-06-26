---
feature: epic-direct
status: implemented
created: 2026-06-23
updated: 2026-06-26
actions: A075
---

# Design — Epic Games Launcher direct import (A075)

## Problem

On Windows and macOS, users may have Epic Games Launcher installed natively
without Heroic. The Linux importer (heroic.go) doesn't run on those platforms.
EGL manifests only cover locally-installed games, not the full owned library.

## Approach

Two-layer strategy under a single `gamesom import epic` command:

**Layer 1 — Legendary CLI (full library)**
Legendary is a cross-platform Epic Games CLI that authenticates with Epic's API
and returns the complete owned library as JSON. `gamesom import epic` auto-installs
Legendary via the platform package manager if absent, runs `legendary auth`
interactively if credentials are missing, then calls `legendary list-games --json`.

| Platform | Install command                               |
|----------|-----------------------------------------------|
| Windows  | `winget install derrod.legendary`             |
| macOS    | `brew install legendary`                      |
| Linux    | Not supported — use `gamesom import heroic`   |

**Layer 2 — EGL manifests (installed status)**
`legendary list-installed` only knows games Legendary itself installed. Games
installed through the Epic Games Launcher are tracked separately in:

| Platform | Path                                                                     |
|----------|--------------------------------------------------------------------------|
| Windows  | `%PROGRAMDATA%\Epic\EpicGamesLauncher\Data\Manifests\`                   |
| macOS    | `~/Library/Application Support/Epic/EpicGamesLauncher/Data/Manifests/`   |

Each `.item` file (JSON) contains `AppName`, `DisplayName`, `InstallLocation`,
`bIsIncompleteInstall`, and `AppCategories`. The library from Legendary is
cross-referenced against these manifests to set installed status and install path.

DLC filter: `AppCategories` must contain `"games"`.
Source tag: `"epic"` (matches heroic.go — idempotent upsert handles overlap).
LauncherURI: `com.epicgames.launcher://apps/<AppName>?action=launch`.

**Fallback**
If the user declines to install Legendary, the command falls back to importing
only locally-installed games from the EGL manifests (owned = installed in that case).

## Scope

In:
- Windows + macOS (runtime.GOOS switch for manifest path and package manager)
- Full owned library via Legendary, installed status via EGL manifests
- Auto-install Legendary with user prompt; auto-run auth flow if needed
- Write library_entries with source=epic

Out:
- Linux (covered by heroic.go)
- No metadata enrichment beyond what Legendary/manifests provide

## Files changed

- `internal/importer/epic.go` — `EpicInstalledMap()`, unified `Epic()` entry point, `epicFromManifests()` fallback
- `internal/importer/legendary.go` — Legendary detection, winget/brew install prompt with fresh-PATH resolution, auth flow, `list-games --json` fetch
- `cmd/import.go` — `import epic` subcommand
