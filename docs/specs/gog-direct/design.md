---
feature: gog-direct
status: implemented
created: 2026-06-23
implemented: 2026-06-25
actions: A076
---

# Design — GOG Galaxy direct import (A076)

## Problem

On Windows and macOS, users with GOG Galaxy installed natively have no import
path into gamesom (Heroic covers Linux only). GOG Galaxy stores its library in
a local SQLite database — no API or authentication required.

## Approach

GOG Galaxy 2.0 maintains a SQLite database:

| Platform | Path |
|----------|------|
| Windows  | %PROGRAMDATA%\GOG.com\Galaxy\storage\galaxy-2.0.db |
| macOS    | ~/Library/Application Support/GOG.com/Galaxy/storage/galaxy-2.0.db |

modernc.org/sqlite is already a dependency (used by itch.go).

### Schema

GOG Galaxy uses a denormalized design: game properties are stored as rows in
a GamePieces table, not columns. Each row: (releaseKey, gamePieceTypeId, value).

Relevant tables (verified against a real Galaxy 2.0 install on 2026-06-25):

```sql
-- Product list (one row per owned product; also contains steam_*, etc.)
SELECT releaseKey FROM LibraryReleases WHERE releaseKey LIKE 'gog_%';

-- Title for a given releaseKey
-- GamePieces unique key includes userId+languageId, so use LIMIT 1
SELECT value FROM GamePieces
WHERE releaseKey = ? AND gamePieceTypeId = (
    SELECT id FROM GamePieceTypes WHERE type = 'title'
) LIMIT 1;
-- value is JSON: {"title": "The Witcher 3"}

-- Installed products with path (keyed by integer productId)
SELECT productId, installationPath FROM InstalledBaseProducts;
-- Join to releaseKey: CAST(REPLACE(releaseKey, 'gog_', '') AS INTEGER) = productId
```

Note: `InstalledExternalProducts` exists but stores UUID-style productIds for
Epic and other launchers — it is NOT used for GOG titles.

Source game ID: the releaseKey (format: gog_<numeric_id>, e.g. gog_1207658924).
Source tag: "gog".
LauncherURI: "goggalaxy://openGame/<numeric_id>".

### Schema risk

The GamePieces/GamePieceTypes schema is community-documented and unofficial —
GOG has never published it. It has been stable across Galaxy 2.0 releases, but
a future Galaxy update could change it. The importer returns a descriptive error
if GamePieceTypes or the 'title' type is missing, rather than silently importing
nothing.

## Scope

In:
- Windows + macOS only
- Owned game list from LibraryReleases (gog_* keys only)
- Title lookup from GamePieces via GamePieceTypes
- Installed status + path from InstalledBaseProducts
- Write library_entries with source=gog

Out:
- Linux (covered by heroic.go + nile)
- DLC filtering via isVisibleInLibrary if it's cheap, otherwise defer
- No ratings, playtime, or metadata beyond title + install state

## Files changed

- internal/importer/gog.go — new file, GOG(database) func
- cmd/import.go — wire import gog --source galaxy subcommand
  (alongside existing import gog which uses heroic/nile on Linux)
