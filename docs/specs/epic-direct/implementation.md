---
feature: epic-direct
status: draft
created: 2026-06-24
owner: claude
---

# Implementation — Epic Games Launcher direct import (A075)

## Scope

In: internal/importer/epic.go (new) + cmd/import.go (wire subcommand)
Out: no changes to other importers, no Linux path

## Load-bearing contract

- runtime.GOOS switch: "windows" uses %PROGRAMDATA%, "darwin" uses ~/Library/...
- Only files with extension .item are read from the manifests dir
- DLC filter: skip entries where AppCategories does not contain "games"
- Installed = bIsInstalled == true AND bIsIncompleteInstall == false
- Source tag: "epic" (matches heroic.go — idempotent upsert handles overlap)
- On Linux: return a clear error ("epic import not supported on Linux; use gamesom import heroic")

## internal/importer/epic.go

```go
package importer

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "runtime"
    "strings"

    "github.com/thegrok/gamesom/internal/db"
    "github.com/thegrok/gamesom/internal/normalize"
)

type epicManifest struct {
    AppName              string   `json:"AppName"`
    DisplayName          string   `json:"DisplayName"`
    InstallLocation      string   `json:"InstallLocation"`
    BIsInstalled         bool     `json:"bIsInstalled"`
    BIsIncompleteInstall bool     `json:"bIsIncompleteInstall"`
    AppCategories        []string `json:"AppCategories"`
}

func epicManifestsDir() (string, error) {
    switch runtime.GOOS {
    case "windows":
        pd := os.Getenv("PROGRAMDATA")
        if pd == "" {
            pd = `C:\ProgramData`
        }
        return filepath.Join(pd, "Epic", "EpicGamesLauncher", "Data", "Manifests"), nil
    case "darwin":
        home, err := os.UserHomeDir()
        if err != nil {
            return "", err
        }
        return filepath.Join(home, "Library", "Application Support", "Epic", "EpicGamesLauncher", "Data", "Manifests"), nil
    default:
        return "", fmt.Errorf("epic import not supported on %s; use gamesom import heroic", runtime.GOOS)
    }
}

// Epic imports games from the Epic Games Launcher manifests directory.
func Epic(database *sql.DB) error {
    dir, err := epicManifestsDir()
    if err != nil {
        return err
    }

    entries, err := os.ReadDir(dir)
    if err != nil {
        return fmt.Errorf("read epic manifests dir %s: %w", dir, err)
    }

    imported, installedCount := 0, 0
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".item") {
            continue
        }

        data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
        if err != nil {
            log.Printf("warning: read %s: %v", entry.Name(), err)
            continue
        }

        var m epicManifest
        if err := json.Unmarshal(data, &m); err != nil {
            log.Printf("warning: parse %s: %v", entry.Name(), err)
            continue
        }

        if !isEpicGame(m.AppCategories) {
            continue
        }
        if m.DisplayName == "" || m.AppName == "" {
            continue
        }

        norm := normalize.Title(m.DisplayName)
        if norm == "" {
            continue
        }

        gameID, err := db.UpsertGame(database, m.DisplayName, norm)
        if err != nil {
            log.Printf("warning: upsert game %q: %v", m.DisplayName, err)
            continue
        }

        installed := 0
        if m.BIsInstalled && !m.BIsIncompleteInstall {
            installed = 1
            installedCount++
        }

        e := db.LibraryEntry{
            GameID:       gameID,
            Source:       "epic",
            SourceGameID: m.AppName,
            SourceTitle:  m.DisplayName,
            Owned:        1,
            Installed:    installed,
            InstallPath:  m.InstallLocation,
            LauncherURI:  fmt.Sprintf("com.epicgames.launcher://apps/%s?action=launch", m.AppName),
        }
        if err := db.UpsertLibraryEntry(database, e); err != nil {
            log.Printf("warning: upsert library entry %q: %v", m.DisplayName, err)
            continue
        }
        imported++
    }

    fmt.Printf("Imported %d games from Epic (%d installed)\n", imported, installedCount)
    return nil
}

func isEpicGame(categories []string) bool {
    for _, c := range categories {
        if c == "games" {
            return true
        }
    }
    return false
}
```

## cmd/import.go addition

Add alongside the existing import subcommands:

```go
var importEpicCmd = &cobra.Command{
    Use:   "epic",
    Short: "Import games from Epic Games Launcher (Windows/macOS)",
    RunE: func(cmd *cobra.Command, args []string) error {
        database, err := db.Open(dbPath)
        if err != nil {
            return err
        }
        defer database.Close()
        return importer.Epic(database)
    },
}

func init() {
    importCmd.AddCommand(importEpicCmd)
}
```

## Verification

- go build ./...
- go vet ./...
- go test ./...
- gamesom import epic — should list count of games imported and installed
- Cross-check a known game title appears in gamesom status output
