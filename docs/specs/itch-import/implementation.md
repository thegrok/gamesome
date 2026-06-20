---
feature: itch-import
status: draft
created: 2026-06-20
owner: Codex (implementation) / Claude (verify)
---

# Implementation — itch.io import (A063)

## Files to create / modify

| File | Action |
|------|--------|
| `internal/importer/itch.go` | CREATE -- `Itch(database)` function |
| `cmd/import.go` | MODIFY -- add `importItchCmd` subcommand + `init()` wiring |

No schema changes needed. `library_entries` already has `source` + `UNIQUE(source, source_game_id)`.

## `internal/importer/itch.go`

### Butler DB path

```go
var itchButlerDBPath = filepath.Join(os.Getenv("HOME"), ".config", "itch", "db", "butler.db")
```

### Structs

```go
// from the games table (joined on download_keys.game_id)
type itchGame struct {
    ID    int64
    Title string
    URL   string
}

type itchDownloadKey struct {
    GameID int64
    Game   itchGame
}

type itchCave struct {
    GameID            int64
    InstallFolderName string
}
```

### `Itch(database *sql.DB) error`

1. Open butler.db read-only: `sql.Open("sqlite", itchButlerDBPath+"?mode=ro")`
2. Query owned games by joining `download_keys` to `games`:
   ```sql
   SELECT dk.game_id, g.title, g.url
   FROM download_keys dk
   JOIN games g ON g.id = dk.game_id
   ```
   Scan `title`/`url` as `sql.NullString` (nullable columns). No JSON parsing —
   butler's schema is normalized, there is no `game` blob column on `download_keys`.
3. Query `caves` for installed games:
   ```sql
   SELECT game_id, install_folder_name FROM caves
   ```
   Build a `map[int64]string` of game_id -> install_folder_name.
4. For each download key:
   - Skip if `game.Title == ""`
   - `normalize.Title(game.Title)` -> skip if empty
   - `db.UpsertGame(database, game.Title, norm)` -> gameID
   - Build `db.LibraryEntry`:
     - `Source`: `"itchio"`
     - `SourceGameID`: `strconv.FormatInt(game.ID, 10)`
     - `SourceTitle`: `game.Title`
     - `Owned`: `1`
     - `Installed`: `1` if in caves map, else `0`
     - `InstallPath`: if installed -> `filepath.Join(itchInstallRoot(), caves[game.ID])`
   - `db.UpsertLibraryEntry(database, entry)`
5. Print: `Imported N games from itch.io (M installed)`

### `itchInstallRoot() string`

Default: `filepath.Join(os.Getenv("HOME"), "Applications", "itch")`.
(Future: read from itch preferences JSON -- skip for now, default covers most Linux setups.)

## `cmd/import.go`

Add after `importSteamCollectionsCmd`:

```go
var importItchCmd = &cobra.Command{
    Use:   "itch",
    Short: "Import from itch.io (butler.db)",
    RunE: func(cmd *cobra.Command, args []string) error {
        database, err := db.Open()
        if err != nil {
            return fmt.Errorf("open db: %w", err)
        }
        defer database.Close()

        if err := importer.Itch(database); err != nil {
            fmt.Fprintf(os.Stderr, "error: %v\n", err)
            os.Exit(1)
        }
        db.SetMeta(database, "last_import_itch", time.Now().UTC().Format(time.RFC3339))
        return nil
    },
}
```

In `init()`, add: `importCmd.AddCommand(importItchCmd)`

## Load-bearing contract

- `butler.db` opened read-only -- never write to it
- `source = "itchio"` is the identity key; must match what `list_games --source itchio` filters on (already in schema)
- `UNIQUE(source, source_game_id)` handles idempotency -- safe to re-run
- If butler.db is absent, return a clear error (not silent skip -- different from Heroic which returns error too)
- butler.db may be locked while itch app is running; `?mode=ro` + `_busy_timeout=5000` avoids write conflicts

## Scope boundary

- No enrichment (that's `enrich` command)
- No launcher_uri (itch:// protocol exists but is undocumented -- omit for now)
- No playtime (not in butler.db)
- No install root config reading (default path only)

## Verification

After Codex edits:
- `go build ./...` -- Codex confirms compilation
- `go vet ./...` -- Claude runs
- `go test ./...` -- Claude runs (no new tests required for this feature; existing suite must stay green)
