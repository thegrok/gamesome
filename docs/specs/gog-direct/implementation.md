---
feature: gog-direct
status: implemented
created: 2026-06-24
implemented: 2026-06-25
owner: claude
---

# Implementation — GOG Galaxy direct import (A076)

## Scope

In: internal/importer/gog.go (new) + cmd/import.go (wire subcommand)
Out: no changes to heroic_gog.go (Linux path); no Linux support here

## Load-bearing contract

- runtime.GOOS switch: "windows" uses %PROGRAMDATA%, "darwin" uses ~/Library/...
- On Linux: return clear error ("gog galaxy import not supported on Linux; use gamesom import gog")
- LibraryReleases WHERE releaseKey LIKE 'gog_%' is the ownership source of truth
  (table also contains steam_*, epic_*, etc. — filter is required)
- Title extracted from GamePieces WHERE gamePieceTypeId matches type='title'; value is JSON {"title": "..."}
  Add LIMIT 1 — unique key includes userId+languageId so multiple rows can exist per releaseKey
- Installed = productId present in InstalledBaseProducts (NOT InstalledExternalProducts)
  InstalledBaseProducts.productId is int64; join via CAST(REPLACE(releaseKey,'gog_','') AS INTEGER)
  InstalledBaseProducts.installationPath holds the real install path
- Source tag: "gog" (matches heroic_gog.go — idempotent upsert handles overlap)
- If GamePieceTypes table or 'title' type not found: return descriptive error, not silent empty import
- releaseKey is the sourceGameID (e.g. "gog_1207658924")

## internal/importer/gog.go

See implementation at `internal/importer/gog.go`. Key queries used (actual verified schema):

```sql
-- Owned GOG titles only (LibraryReleases also contains steam_*, etc.)
SELECT releaseKey FROM LibraryReleases WHERE releaseKey LIKE 'gog_%'

-- Title (LIMIT 1: unique key includes userId+languageId)
SELECT value FROM GamePieces
WHERE releaseKey = ? AND gamePieceTypeId = ? LIMIT 1

-- Install path (productId is int64 matching numeric part of releaseKey)
SELECT productId, installationPath FROM InstalledBaseProducts
```

Install map is `map[int64]string`; lookup key is `strconv.ParseInt(strings.TrimPrefix(releaseKey, "gog_"), 10, 64)`.

## cmd/import.go addition

Subcommand `gog-galaxy` added to `importCmd`. Uses `db.Open()` (no args) matching
all other subcommands. Sets meta key `last_import_gog_galaxy` on success.

Note: subcommand is `gog-galaxy` (not `gog`) to avoid collision with the existing
`import gog` subcommand which uses Heroic/nile on Linux.

## Verification

- go build ./... ✓
- go vet ./... ✓
- gamesom import gog-galaxy — output: "Imported 479 games from GOG Galaxy (10 installed)" ✓
