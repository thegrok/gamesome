---
feature: gog-direct
status: draft
created: 2026-06-24
owner: claude
---

# Implementation — GOG Galaxy direct import (A076)

## Scope

In: internal/importer/gog.go (new) + cmd/import.go (wire subcommand)
Out: no changes to heroic_gog.go (Linux path); no Linux support here

## Load-bearing contract

- runtime.GOOS switch: "windows" uses %PROGRAMDATA%, "darwin" uses ~/Library/...
- On Linux: return clear error ("gog galaxy import not supported on Linux; use gamesom import gog")
- ProductsInLibrary is the ownership source of truth
- Title extracted from GamePieces WHERE gamePieceTypeId matches type='title'; value is JSON {"title": "..."}
- Installed = productId present in InstalledExternalProducts
- Source tag: "gog" (matches heroic_gog.go — idempotent upsert handles overlap)
- If GamePieceTypes table or 'title' type not found: return descriptive error, not silent empty import
- releaseKey is the sourceGameID (e.g. "gog_1207658924")

## internal/importer/gog.go

```go
package importer

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "runtime"
    "strings"

    "github.com/thegrok/gamesom/internal/db"
    "github.com/thegrok/gamesom/internal/normalize"
    _ "modernc.org/sqlite"
)

func gogGalaxyDBPath() (string, error) {
    switch runtime.GOOS {
    case "windows":
        pd := os.Getenv("PROGRAMDATA")
        if pd == "" {
            pd = `C:\ProgramData`
        }
        return filepath.Join(pd, "GOG.com", "Galaxy", "storage", "galaxy-2.0.db"), nil
    case "darwin":
        home, err := os.UserHomeDir()
        if err != nil {
            return "", err
        }
        return filepath.Join(home, "Library", "Application Support", "GOG.com", "Galaxy", "storage", "galaxy-2.0.db"), nil
    default:
        return "", fmt.Errorf("gog galaxy import not supported on %s; use gamesom import gog", runtime.GOOS)
    }
}

// GOG imports games from GOG Galaxy's local SQLite database.
func GOG(database *sql.DB) error {
    dbPath, err := gogGalaxyDBPath()
    if err != nil {
        return err
    }

    galaxyDB, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
    if err != nil {
        return fmt.Errorf("open galaxy db: %w", err)
    }
    defer galaxyDB.Close()

    // Resolve the gamePieceTypeId for 'title'
    var titleTypeID int
    err = galaxyDB.QueryRow(`SELECT id FROM GamePieceTypes WHERE type = 'title'`).Scan(&titleTypeID)
    if err != nil {
        return fmt.Errorf("GamePieceTypes lookup failed — GOG Galaxy schema may have changed: %w", err)
    }

    // Get owned release keys
    rows, err := galaxyDB.Query(`SELECT releaseKey FROM ProductsInLibrary`)
    if err != nil {
        return fmt.Errorf("query ProductsInLibrary: %w", err)
    }
    defer rows.Close()

    // Get installed paths
    installed := map[string]string{}
    irows, err := galaxyDB.Query(`SELECT productId, installationPath FROM InstalledExternalProducts`)
    if err == nil {
        defer irows.Close()
        for irows.Next() {
            var pid, path string
            if irows.Scan(&pid, &path) == nil {
                installed[pid] = path
            }
        }
    }

    imported, installedCount := 0, 0
    for rows.Next() {
        var releaseKey string
        if err := rows.Scan(&releaseKey); err != nil {
            continue
        }

        // Get title from GamePieces
        var valueJSON string
        err := galaxyDB.QueryRow(
            `SELECT value FROM GamePieces WHERE releaseKey = ? AND gamePieceTypeId = ?`,
            releaseKey, titleTypeID,
        ).Scan(&valueJSON)
        if err != nil {
            continue
        }

        var titleObj struct {
            Title string `json:"title"`
        }
        if err := json.Unmarshal([]byte(valueJSON), &titleObj); err != nil || titleObj.Title == "" {
            continue
        }

        norm := normalize.Title(titleObj.Title)
        if norm == "" {
            continue
        }

        gameID, err := db.UpsertGame(database, titleObj.Title, norm)
        if err != nil {
            continue
        }

        installPath, isInstalled := installed[releaseKey]
        installedFlag := 0
        if isInstalled {
            installedFlag = 1
            installedCount++
        }

        // Extract numeric ID from releaseKey (e.g. "gog_1207658924" -> "1207658924")
        numericID := strings.TrimPrefix(releaseKey, "gog_")

        e := db.LibraryEntry{
            GameID:       gameID,
            Source:       "gog",
            SourceGameID: releaseKey,
            SourceTitle:  titleObj.Title,
            Owned:        1,
            Installed:    installedFlag,
            InstallPath:  installPath,
            LauncherURI:  fmt.Sprintf("goggalaxy://openGame/%s", numericID),
        }
        if err := db.UpsertLibraryEntry(database, e); err != nil {
            continue
        }
        imported++
    }

    fmt.Printf("Imported %d games from GOG Galaxy (%d installed)\n", imported, installedCount)
    return nil
}
```

## cmd/import.go addition

```go
var importGOGCmd = &cobra.Command{
    Use:   "gog-galaxy",
    Short: "Import games from GOG Galaxy (Windows/macOS)",
    RunE: func(cmd *cobra.Command, args []string) error {
        database, err := db.Open(dbPath)
        if err != nil {
            return err
        }
        defer database.Close()
        return importer.GOG(database)
    },
}

func init() {
    importCmd.AddCommand(importGOGCmd)
}
```

Note: subcommand is gog-galaxy (not gog) to avoid collision with the existing
import gog subcommand which uses Heroic/nile on Linux.

## Verification

- go build ./...
- go vet ./...
- go test ./...
- gamesom import gog-galaxy — should list count of games imported and installed
- Spot-check a known GOG title (e.g. The Witcher 3) appears in gamesom status
- Confirm installed vs uninstalled games are correctly distinguished
