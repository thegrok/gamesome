---
feature: cross-platform-importers
status: implemented
created: 2026-06-24
updated: 2026-06-27
owner: claude
---

# Implementation — Cross-platform importers (A075, A076)

## Scope

In: `heroic.go`, `epic.go`, `legendary.go`, `heroic_gog.go`, `gog.go`, `steam.go`, `itch.go`, `cmd/import.go`
Out: no schema changes

## Load-bearing contracts

- `heroicConfigDir()` is the single platform switch for all Heroic-based importers
- `Epic()`: Heroic cache → Legendary CLI → EGL manifests, in order; first success wins
- `GOG()`: Linux → `HeroicGOG()` (gogdl cache); Windows/macOS → `GOGGalaxy()` (Galaxy SQLite)
- `HeroicGOG()` reads `gog_library.json` — NOT `nile_library.json` (that is Amazon Games / nile)
- `GOGGalaxy()` uses the unofficial but stable Galaxy 2.0 schema; errors clearly if tables missing
- Legendary on macOS: print hint + fall through (no prompt); Windows: winget auto-install
- Source tags: `"epic"` across all Epic layers; `"gog"` across all GOG paths

## `internal/importer/heroic.go`

`heroicConfigDir()` replaces hardcoded Flatpak paths:

```go
switch runtime.GOOS {
case "linux":   return filepath.Join(os.Getenv("HOME"), ".var/app/com.heroicgameslauncher.hgl/config/heroic"), nil
case "darwin":  return filepath.Join(home, "Library", "Application Support", "heroic"), nil
case "windows": return filepath.Join(os.Getenv("APPDATA"), "heroic"), nil
}
```

## `internal/importer/epic.go`

```
Epic()
  → epicFromHeroicCache()     // legendary_library.json + installed.json
  → findLegendary() → legendaryListGames() + EpicInstalledMap()
  → epicFromManifests()       // EGL .item files, Windows/macOS only
```

`EpicInstalledMap()` is exported so Legendary layer can cross-reference EGL for install status.

## `internal/importer/legendary.go`

- `findLegendary()` — PATH lookup; Windows also checks registry-updated PATH via PowerShell
- `promptInstallLegendary()` — Windows: winget; macOS: print hint + return error (triggers EGL fallback); Linux: hard error
- `legendaryListGames()` — `legendary list-games --json`; handles unauthenticated with interactive auth

## `internal/importer/heroic_gog.go`

`GOG()` platform router:
```go
func GOG(database *sql.DB) error {
    if runtime.GOOS == "linux" {
        return HeroicGOG(database)
    }
    return GOGGalaxy(database)
}
```

`HeroicGOG()` reads `store_cache/gog_library.json` (`{"games": [...]}` key, gogdl format).
Uses `runner == "gog"` filter and `install.is_dlc` to skip DLC.
Uses `is_installed` + `install.install_path` inline — no separate installed.json needed.
LauncherURI: `goggalaxy://openGame/<app_name>`.

## `internal/importer/gog.go`

`GOGGalaxy()` reads GOG Galaxy's SQLite database:
- `LibraryReleases` for owned games (releaseKey format: `gog_<numeric_id>`)
- `GamePieceTypes` to resolve title type ID
- `GamePieces` for title JSON (`{"title": "..."}`)
- `InstalledBaseProducts` for install paths (keyed by int64 productId)

Errors explicitly if `GamePieceTypes` lookup fails (schema change detection).

## `cmd/import.go`

- `import epic` → `importer.Epic()`
- `import gog` → `importer.GOG()` (platform-routed)
- `import gog-galaxy` → `importer.GOGGalaxy()` (direct override)

## Verification

| Platform | Store | Result |
|----------|-------|--------|
| macOS    | Epic (Heroic cache) | 406 games ✓ |
| macOS    | Steam | ✓ |
| macOS    | itch  | ✓ |
| Windows  | Epic (EGL manifests) | ✓ |
| Windows  | GOG Galaxy | 479 games, 10 installed ✓ |
| Windows  | Steam | ✓ |
| Windows  | itch  | ✓ |
| Linux    | GOG via Heroic | code fixed (gog_library.json); re-verification needed after nile→gogdl fix |
| Windows  | Heroic | code exists; untested against real Heroic Windows install |
