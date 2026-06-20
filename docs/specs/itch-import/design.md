---
feature: itch-import
status: draft
created: 2026-06-20
---

# Design — itch.io import (A063)

## Problem

gamesom can't see itch.io purchases. The itch desktop app keeps a local SQLite at
`~/.config/itch/db/butler.db` listing owned and installed games — no API key or
login required. We should read it the same way Steam/Heroic read their local stores.

## Approach

Read `butler.db` directly (read-only) using `modernc.org/sqlite` (already a dep).
Two tables matter:

- **`download_keys`** — one row per owned game (`game_id`, `game` JSON blob with title/url)
- **`caves`** — one row per install (`game_id`, `install_folder_name`, `verdict` JSON)

Owned = appears in `download_keys`. Installed = also appears in `caves`.
The `game` column in `download_keys` is a JSON blob with `id`, `title`, `url`.
The `install_folder_name` in caves gives the local path (under itch's install root,
which is also discoverable from the preference files, but defaults to `~/Applications/itch`
on Linux). We can reconstruct `install_path` as `<itch-install-root>/<install_folder_name>`.

## Scope

In: read butler.db -> write `library_entries` with `source=itchio`
Out: no launcher_uri wiring yet (itch:// deep-link exists but undocumented -- skip for now)

## Source game ID

Use `game_id` (integer, from `download_keys`). String-formatted as `source_game_id`.
