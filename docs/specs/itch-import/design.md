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
Three tables matter:

- **`download_keys`** — one row per owned game (`game_id`, `owner_id`, timestamps). No title/url here.
- **`games`** — game metadata, keyed by `id` (`title`, `url`, `short_text`, …)
- **`caves`** — one row per install (`game_id`, `install_folder_name`, `verdict` TEXT)

Owned = appears in `download_keys`. Installed = also appears in `caves`.
Butler's schema is normalized: `download_keys` holds only `game_id`, so we
`JOIN games ON games.id = download_keys.game_id` to get `title` and `url`.
The `install_folder_name` in caves gives the local path (under itch's install root,
which is also discoverable from the preference files, but defaults to `~/Applications/itch`
on Linux). We can reconstruct `install_path` as `<itch-install-root>/<install_folder_name>`.

## Scope

In: read butler.db -> write `library_entries` with `source=itchio`
Out: no launcher_uri wiring yet (itch:// deep-link exists but undocumented -- skip for now)

## Source game ID

Use `game_id` (integer, from `download_keys`). String-formatted as `source_game_id`.
