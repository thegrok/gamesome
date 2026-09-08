---
feature: itch-install-location
status: draft
created: 2026-07-05
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: main
---

# Implementation — itch.io installed-state: resolve real install locations (A100, itch half)

## Load-bearing contract

For every cave in butler's DB, the importer must resolve the cave's **real**
install path — `custom_install_folder` when set, else the cave's
`install_locations.path` + `install_folder_name`, else the legacy
`itchConfigDir()/apps/` guess — and mark the game installed only when that
path exists on disk. If the new query fails against an older/different butler
schema, the importer must warn and fall back to today's exact behavior (never
a hard error, never silent).

## Files changed

### `internal/importer/itch.go` (only file)

**1. New helper replacing the inline cave query (~line 94):**

```go
// itchCavePaths maps game_id → resolved install path for every cave in
// butler's DB. Butler records where a cave actually lives: a custom folder
// (full path, set via itch's Preferences), or an install-location row joined
// by id. The legacy configDir/apps guess is only a last resort — and the
// whole-query fallback below keeps older butler schemas importing exactly as
// before this fix.
func itchCavePaths(butlerDB *sql.DB, configDir string) (map[int64]string, error) {
	paths := make(map[int64]string)
	rows, err := butlerDB.Query(`
		SELECT c.game_id, c.install_folder_name, c.custom_install_folder, l.path
		FROM caves c
		LEFT JOIN install_locations l ON c.install_location_id = l.id`)
	if err != nil {
		log.Printf("warning: itch caves/install_locations query failed (%v); falling back to legacy apps-dir assumption", err)
		return itchCavePathsLegacy(butlerDB, configDir)
	}
	defer rows.Close()
	for rows.Next() {
		var gameID int64
		var folderName, customFolder, locationPath sql.NullString
		if err := rows.Scan(&gameID, &folderName, &customFolder, &locationPath); err != nil {
			return nil, fmt.Errorf("scan itch cave: %w", err)
		}
		switch {
		case customFolder.String != "":
			paths[gameID] = customFolder.String
		case locationPath.String != "":
			paths[gameID] = filepath.Join(locationPath.String, folderName.String)
		default:
			paths[gameID] = filepath.Join(configDir, "apps", folderName.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read itch caves: %w", err)
	}
	return paths, nil
}
```

**2. `itchCavePathsLegacy`** — today's query and path logic, extracted verbatim
(`SELECT game_id, install_folder_name FROM caves`, path =
`filepath.Join(configDir, "apps", folderName)`), same scan/error handling.

**3. Call site in `Itch()`** — replace the inline cave-query block
(itch.go:94–112) with:

```go
caves, err := itchCavePaths(butlerDB, configDir)
if err != nil {
	return err
}
```

**4. Install check in the import loop (itch.go:134–142)** — the map now holds
full resolved paths:

```go
installed := 0
installPath := ""
if p, ok := caves[key.GameID]; ok {
	if _, err := os.Stat(p); err == nil {
		installed = 1
		installPath = p
	}
}
```

**5. Drop the now-unused `itchCave` struct** (itch.go:50–53) — the helper
scans into locals.

## Integration points

- `itchConfigDir()` — unchanged; still needed for the butler.db path and the
  legacy fallback.
- `db.LibraryEntry` / `db.UpsertLibraryEntry` — unchanged; `InstallPath` now
  receives the resolved real path.
- No signature, schema, or CLI changes; no other importer touched.

## Sequencing

Single commit; no migration, no ordering constraints. Independent of PR #18
(Epic half) — different files, either merge order works.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` in the repo.
- What can't be verified here: real-machine behavior (Linux sandbox has no
  butler.db / itch installs). Ground truth is Victor running
  `gamesom.exe import itch` on the Windows box — expected: the games installed
  under `Games\itch.io` flip to installed with correct paths. Optional extra
  evidence if `sqlite3` is available there:
  `sqlite3 butler.db ".schema caves"` / `".schema install_locations"`.
- Note A101: the windows-e2e suite as it stands would not catch a regression
  here.

## Scope boundary

Only the cave query and install-path resolution change. Do not touch the
`download_keys` owned-library query, launcher URIs, other importers, or the
e2e suite (A101 is its own action).
