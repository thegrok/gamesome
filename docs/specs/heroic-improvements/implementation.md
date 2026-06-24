---
feature: heroic-improvements
status: draft
created: 2026-06-24
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: main
---

# Implementation — Add GOG import via Heroic nile cache (A064)

## Load-bearing contract

`importer.HeroicGOG(database)` reads nile's library + installed caches from the
Flatpak Heroic path and upserts `library_entries` rows with `source="gog"` and
`launcher_uri="nile://launch/<app_name>"`. Behaviour mirrors `importer.Heroic()`
exactly except: different paths, different source tag, different URI scheme, and
**no IsDLC filter** (nile entries omit that field).

## Files changed

### `internal/importer/heroic_gog.go` (new)

Path vars:
```go
var nileLibraryPath = filepath.Join(
    os.Getenv("HOME"),
    ".var/app/com.heroicgameslauncher.hgl/config/heroic/store_cache/nile_library.json",
)

var nileInstalledPath = filepath.Join(
    os.Getenv("HOME"),
    ".var/app/com.heroicgameslauncher.hgl/config/heroic/nileConfig/nile/installed.json",
)
```

Types (mirror heroic.go but without `Install.IsDLC`):
```go
type nileLibraryFile struct {
    Library []nileLibraryEntry `json:"library"`
}
type nileLibraryEntry struct {
    AppName     string `json:"app_name"`
    Title       string `json:"title"`
    IsInstalled bool   `json:"is_installed"`
}
type nileInstalledEntry struct {
    AppName     string `json:"app_name"`
    InstallPath string `json:"install_path"`
}
```

`HeroicGOG(database *sql.DB) error`:
- reads + parses nile_library.json via `readNileLibrary()`
- reads + parses nile/installed.json via `readNileInstalled()` (non-fatal on error)
- loops entries: skip if Title/AppName empty or normalize.Title returns ""
- upserts game + library entry with `source="gog"`, `LauncherURI="nile://launch/<app_name>"`
- prints `"Imported %d games from GOG (%d installed)\n"`

### `cmd/import.go` (extend)

Add `importGOGCmd`:
```go
var importGOGCmd = &cobra.Command{
    Use:   "gog",
    Short: "Import from GOG via Heroic/nile cache",
    RunE: func(cmd *cobra.Command, args []string) error {
        database, err := db.Open()
        if err != nil {
            return fmt.Errorf("open db: %w", err)
        }
        defer database.Close()
        if err := importer.HeroicGOG(database); err != nil {
            fmt.Fprintf(os.Stderr, "error: %v\n", err)
            os.Exit(1)
        }
        db.SetMeta(database, "last_import_gog", time.Now().UTC().Format(time.RFC3339))
        return nil
    },
}
```

In `init()`: add `importCmd.AddCommand(importGOGCmd)`.

## Integration points

- `db.UpsertGame` / `db.UpsertLibraryEntry` — same DB layer as all other importers
- `normalize.Title` — same title normalizer
- No new dependencies

## Scope boundary

In:
- `internal/importer/heroic_gog.go` — new importer
- `cmd/import.go` — new `import gog` subcommand

Out:
- Native Heroic install support
- Windows / macOS
- DLC filtering (nile entries don't carry an IsDLC field)
- Tests (no importer tests exist in the repo; don't add now)

## Sequencing

1. New file `heroic_gog.go` — self-contained, no dependency on existing heroic.go
2. Extend `cmd/import.go` — add cmd + wire in init()
3. `go build ./...` to confirm compilation
