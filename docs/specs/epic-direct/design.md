---
feature: epic-direct
status: draft
created: 2026-06-23
actions: A075
---

# Design — Epic Games Launcher direct import (A075)

## Problem

On Windows and macOS, users may have Epic Games Launcher installed natively
without Heroic. The Linux importer (heroic.go) doesn't run on those platforms.
Epic stores installation manifests in a well-known local path in a clean JSON
format — no API or authentication required.

## Approach

Epic writes a .item JSON file per game into a manifests directory:

| Platform | Path |
|----------|------|
| Windows  | %PROGRAMDATA%\Epic\EpicGamesLauncher\Data\Manifests\ |
| macOS    | ~/Library/Application Support/Epic/EpicGamesLauncher/Data/Manifests/ |

Each .item file (JSON) contains:

```json
{
  "AppName": "Fortnite",
  "DisplayName": "Fortnite",
  "InstallLocation": "C:\\...",
  "bIsInstalled": true,
  "bIsIncompleteInstall": false,
  "AppCategories": ["public", "games"]
}
```

Owned = file exists. Installed = bIsInstalled == true and bIsIncompleteInstall == false.
Skip DLCs: AppCategories must contain "games" (standard community approach — Epic
doesn't expose a clean isDLC flag in manifests).

Source tag: "epic" (same as heroic.go — same data source, different reader).
LauncherURI: "com.epicgames.launcher://apps/<AppName>?action=launch".

## Scope

In:
- Windows + macOS only (runtime.GOOS switch for path)
- Read .item files from manifests dir
- Write library_entries with source=epic

Out:
- Linux (covered by heroic.go)
- No metadata enrichment beyond what the manifest provides
- No handling of Epic's cloud-only games (no local manifest)

## Files changed

- internal/importer/epic.go — new file, Epic(database) func
- cmd/import.go — wire import epic subcommand; skip gracefully on Linux
