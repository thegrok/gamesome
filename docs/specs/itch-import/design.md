---
feature: itch-import
status: implemented
created: 2026-06-20
updated: 2026-06-26
---

# Design — itch.io import (A063)

## Problem

gamesom can't see itch.io purchases. The itch desktop app keeps a local SQLite
(`butler.db`) listing owned and installed games — no API key or login required.

## Approach

Read `butler.db` directly (read-only) using `modernc.org/sqlite` (already a dep).
Three tables matter:

- **`download_keys`** — one row per owned game (`game_id`, timestamps)
- **`games`** — metadata keyed by `id` (`title`, `url`, `classification`)
- **`caves`** — one row per install (`game_id`, `install_folder_name`)

Owned = appears in `download_keys JOIN games WHERE classification = 'game'`.
Installed = also appears in `caves`. Install path = `<configDir>/apps/<install_folder_name>`.

## Platform paths

`itchConfigDir()` returns the platform-appropriate base directory:

| Platform | Path                                         |
|----------|----------------------------------------------|
| Linux    | `~/.config/itch`                             |
| macOS    | `~/Library/Application Support/itch`         |
| Windows  | `%APPDATA%\itch`                             |

Butler DB: `<configDir>/db/butler.db`
Install root: `<configDir>/apps`

## Scope

In: read `butler.db` → write `library_entries` with `source=itchio`

Out:

- No `launcher_uri` (itch:// deep-link exists but is undocumented)
- No playtime (not in butler.db)
- No install root config reading (default path only)
