---
feature: itch-import
status: implemented
created: 2026-06-20
updated: 2026-06-26
owner: claude
---

# Implementation — itch.io import (A063)

## Files changed

| File                        | Action                                         |
|-----------------------------|------------------------------------------------|
| `internal/importer/itch.go` | CREATE — `itchConfigDir()`, `Itch(database)`   |
| `cmd/import.go`             | MODIFY — add `importItchCmd` + `init()` wiring |

## `internal/importer/itch.go`

### `itchConfigDir() (string, error)`

```go
func itchConfigDir() (string, error) {
    switch runtime.GOOS {
    case "windows":
        return filepath.Join(os.Getenv("APPDATA"), "itch"), nil
    case "darwin":
        home, _ := os.UserHomeDir()
        return filepath.Join(home, "Library", "Application Support", "itch"), nil
    default: // linux
        home, _ := os.UserHomeDir()
        return filepath.Join(home, ".config", "itch"), nil
    }
}
```

Butler DB: `filepath.Join(configDir, "db", "butler.db")`
Install root: `filepath.Join(configDir, "apps", caveFolder)`

### `Itch(database *sql.DB) error`

1. `itchConfigDir()` → derive butler DB path
2. Open read-only: `sql.Open("sqlite", path+"?mode=ro&_busy_timeout=5000")`
3. Query owned: `SELECT dk.game_id, g.title, g.url FROM download_keys dk JOIN games g ON g.id = dk.game_id WHERE g.classification = 'game'`
4. Query installed: `SELECT game_id, install_folder_name FROM caves` → `map[int64]string`
5. For each owned game: upsert game + library entry with `source="itchio"`, `source_game_id=strconv.FormatInt(game_id, 10)`
6. Print: `Imported N games from itch.io (M installed)`

## Load-bearing contract

- Butler DB opened read-only — never write to it
- `source = "itchio"` is the identity key
- `UNIQUE(source, source_game_id)` handles idempotency
- If butler.db is absent, return a clear error (not silent skip)
- `_busy_timeout=5000` avoids lock conflicts while itch app is running

## Verification

- `go build ./...` ✓
- `go vet ./...` ✓
- `go test ./...` ✓
- `gamesom import itch` → 42 games on macOS
