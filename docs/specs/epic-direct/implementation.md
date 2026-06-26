---
feature: epic-direct
status: implemented
created: 2026-06-24
updated: 2026-06-26
owner: claude
---

# Implementation — Epic Games import (A075)

## Scope

In: `epic.go`, `legendary.go`, `heroic.go`, `heroic_gog.go`, `cmd/import.go`
Out: no schema changes, no new dependencies

## Load-bearing contract

- `Epic()` tries layers in order: Heroic cache → Legendary CLI → EGL manifests
- Heroic cache is the preferred path: no auth, full library, cross-platform
- `heroicConfigDir()` in `heroic.go` drives the platform switch for all Heroic-based importers
- Legendary on macOS: print install hint and fall through (no prompt); Windows: winget auto-install
- EGL manifests: Windows/macOS only; owned = file exists; DLC filter via `AppCategories`
- `HeroicGOG()` shares `heroicConfigDir()` — adding a platform adds GOG support for free
- Source tag `"epic"` across all Epic layers; `"gog"` for GOG; idempotent upserts handle overlap

## `internal/importer/heroic.go`

Replaced hardcoded Flatpak paths with `heroicConfigDir() (string, error)`:

```go
func heroicConfigDir() (string, error) {
    switch runtime.GOOS {
    case "linux":
        return filepath.Join(os.Getenv("HOME"), ".var/app/com.heroicgameslauncher.hgl/config/heroic"), nil
    case "darwin":
        home, _ := os.UserHomeDir()
        return filepath.Join(home, "Library", "Application Support", "heroic"), nil
    case "windows":
        return filepath.Join(os.Getenv("APPDATA"), "heroic"), nil
    }
}
```

`readHeroicLibrary()` and `readHeroicInstalled()` now call `heroicConfigDir()` instead of using package-level path vars.

## `internal/importer/epic.go`

`Epic()` entry point:

```text
Epic()
  → epicFromHeroicCache()   // reads heroic legendary_library.json + installed.json
  → findLegendary() + legendaryListGames() + EpicInstalledMap()
  → epicFromManifests()     // EGL .item files, Windows/macOS only
```

`epicFromHeroicCache()` calls `readHeroicLibrary()` / `readHeroicInstalled()` from `heroic.go`,
upserts with `LauncherURI = "legendary://launch/<AppName>"`.

Legendary and EGL paths use `LauncherURI = "com.epicgames.launcher://apps/<AppName>?action=launch"`.

## `internal/importer/legendary.go`

- `findLegendary()` / `findLegendaryWithFreshPath()` — PATH lookup; Windows also checks registry-updated PATH via PowerShell
- `promptInstallLegendary()` — Windows: winget prompt; macOS: print hint + return error (triggers manifest fallback); Linux: hard error
- `legendaryListGames()` — runs `legendary list-games --json`, handles unauthenticated case with interactive `legendary auth`

## `internal/importer/heroic_gog.go`

`HeroicGOG()` reads `store_cache/nile_library.json` and `nileConfig/nile/installed.json`
via `heroicConfigDir()`. Source tag `"gog"`, LauncherURI `"nile://launch/<AppName>"`.

## `cmd/import.go`

- `import epic` → `importer.Epic(database)` + sets `last_import_epic` meta
- `import gog` → `importer.HeroicGOG(database)` + sets `last_import_gog` meta

## Verification

- `go build ./...` ✓
- `go vet ./...` ✓
- `go test ./...` ✓
- `gamesom import epic` → 406 games via Heroic on macOS
- `gamesom import gog` → runs (0 games without GOG login in Heroic)
