---
feature: cross-platform-importers
status: implemented
created: 2026-06-23
updated: 2026-06-27
actions: A075, A076
---

# Design — Cross-platform importers (A075, A076)

## Problem

All importers were Linux-only (hardcoded Flatpak paths). Windows and macOS users
had no import path for Epic, GOG, Steam, or itch. This PR makes all four stores
cross-platform and adds GOG Galaxy direct import.

## Epic (A075) — three-layer strategy

`gamesom import epic` tries layers in order:

**Layer 1 — Heroic cache (preferred)**
Heroic caches the full owned Epic library as JSON on disk — no auth required at
import time. `heroicConfigDir()` resolves the config path per platform:

| Platform | Path |
|----------|------|
| Linux    | `~/.var/app/com.heroicgameslauncher.hgl/config/heroic/` |
| macOS    | `~/Library/Application Support/heroic/` |
| Windows  | `%APPDATA%\heroic\` |

Library: `store_cache/legendary_library.json` · Installed: `legendaryConfig/legendary/installed.json`

**Layer 2 — Legendary CLI**
For users without Heroic. Auto-installs via winget on Windows. On macOS, prints
a `brew install pipx && pipx install legendary-gl` hint and falls through to
Layer 3 without prompting (legendary-gl is a PITA to install on macOS).

**Layer 3 — EGL manifests (installed-only fallback)**
Reads `.item` JSON files from the EGL Manifests directory. Only locally-installed
games are visible. Windows/macOS only.

| Platform | Path |
|----------|------|
| Windows  | `%PROGRAMDATA%\Epic\EpicGamesLauncher\Data\Manifests\` |
| macOS    | `~/Library/Application Support/Epic/EpicGamesLauncher/Data/Manifests/` |

DLC filter: `AppCategories` must contain `"games"`.
Source tag: `"epic"` across all layers (idempotent upsert handles overlap).

## GOG (A076) — platform-routed

`gamesom import gog` routes by platform:

**Linux → Heroic gogdl cache**
`HeroicGOG()` reads `store_cache/gog_library.json` (gogdl's cache — note: NOT
`nile_library.json`, which is Amazon Games). Uses `is_installed` and
`install.install_path` inline from the JSON.

**Windows / macOS → GOG Galaxy SQLite**
`GOGGalaxy()` reads directly from GOG Galaxy's local SQLite database. No API
or auth required. GOG Galaxy is authoritative on Windows/macOS for both
ownership and install status (Heroic on these platforms may not know about
games installed via Galaxy).

| Platform | Galaxy DB path |
|----------|----------------|
| Windows  | `%PROGRAMDATA%\GOG.com\Galaxy\storage\galaxy-2.0.db` |
| macOS    | `~/Library/Application Support/GOG.com/Galaxy/storage/galaxy-2.0.db` |

Schema: `LibraryReleases` (owned) + `GamePieces`/`GamePieceTypes` (titles) +
`InstalledBaseProducts` (install paths). Schema is community-documented and
unofficial but has been stable across Galaxy 2.0.

Source tag: `"gog"`. LauncherURI: `goggalaxy://openGame/<numeric_id>`.

`import gog-galaxy` is a direct-override alias for `GOGGalaxy()` on any platform.

## Steam and itch — cross-platform paths

`steamAppsDirectories()` and `itchConfigDir()` now resolve per platform
(Linux/macOS/Windows) rather than hardcoding Linux paths.

## Scope

In:
- All four stores cross-platform: Linux + macOS + Windows
- Epic: Heroic cache → Legendary CLI → EGL manifests
- GOG: Linux via Heroic gogdl cache; Windows/macOS via GOG Galaxy SQLite
- Steam: cross-platform steamapps directory resolution
- itch: cross-platform butler.db path resolution

Out:
- Heroic on Windows/macOS as a GOG source (Heroic doesn't track Galaxy-managed installs)
- Heroic on Windows verified (code exists, untested against real Heroic Windows install)
- No metadata enrichment beyond what each source provides

## Files changed

- `internal/importer/heroic.go` — `heroicConfigDir()` cross-platform switch
- `internal/importer/epic.go` — `Epic()` three-layer chain; `epicFromHeroicCache()`; `EpicInstalledMap()`
- `internal/importer/legendary.go` — Legendary detection, winget/pipx install, auth flow
- `internal/importer/heroic_gog.go` — `GOG()` platform router; `HeroicGOG()` reads `gog_library.json`
- `internal/importer/gog.go` — `GOGGalaxy()` reads GOG Galaxy SQLite
- `internal/importer/steam.go` — `steamAppsDirectories()` cross-platform
- `internal/importer/itch.go` — `itchConfigDir()` cross-platform
- `cmd/import.go` — `import epic`, `import gog`, `import gog-galaxy` subcommands
