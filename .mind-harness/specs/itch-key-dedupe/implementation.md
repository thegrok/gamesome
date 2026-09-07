---
feature: itch-key-dedupe
status: ready
created: 2026-07-06
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: main
---

# Implementation — dedupe itch download keys in the import summary (A108)

## Load-bearing contract

`Itch()` must process each distinct owned game **once**, so the printed
"Imported N games (M installed)" summary always matches the number of library
rows the run actually upserted. Multiple `download_keys` rows for one
`game_id` must not inflate either count. Import behavior is otherwise
byte-identical: same games, same owned/installed flags, same paths.

## Files changed

### `internal/importer/itch.go`

**1. New helper `itchOwnedGames` replacing the inline download-keys block
(itch.go:126–150):**

```go
// itchOwnedGames returns each distinct owned game from butler's download
// keys. A game can hold several keys (direct purchase + bundle grant), so
// DISTINCT collapses them — all selected columns join from the single games
// row per game_id, making duplicate key rows column-identical.
func itchOwnedGames(butlerDB *sql.DB) ([]itchGame, error) {
	rows, err := butlerDB.Query(`
		SELECT DISTINCT dk.game_id, g.title, g.url
		FROM download_keys dk
		JOIN games g ON g.id = dk.game_id
		WHERE g.classification = 'game'`)
	if err != nil {
		return nil, fmt.Errorf("query itch download keys: %w", err)
	}
	defer rows.Close()

	var games []itchGame
	for rows.Next() {
		var game itchGame
		var title, url sql.NullString
		if err := rows.Scan(&game.ID, &title, &url); err != nil {
			return nil, fmt.Errorf("scan itch download key: %w", err)
		}
		game.Title = title.String
		game.URL = url.String
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read itch download keys: %w", err)
	}
	return games, nil
}
```

**2. Call site in `Itch()`** — replace the inline query + scan block with:

```go
games, err := itchOwnedGames(butlerDB)
if err != nil {
	return err
}
```

The import loop iterates `games` (`for _, game := range games`); `key.GameID`
becomes `game.ID` in the caves lookup and `SourceGameID`.

**3. Drop the `itchDownloadKey` struct** (itch.go:45–48) — it wrapped
`itchGame` with a duplicate of its ID field and has no remaining use.

### `internal/importer/itch_test.go` (new)

Unit test `TestItchOwnedGames_DedupesMultipleKeys`: build a fixture butler DB
in `t.TempDir()` (create `games` + `download_keys` with the columns the query
reads; the modernc driver is registered via the existing `internal/db`
import). Insert one game with **two** download keys, one game with one key,
and one non-`game` classification row. Assert exactly 2 games return, each
`game_id` once, and the non-game row is excluded.

## Integration points

- `itchCavePaths` / `itchCavePathsLegacy` — untouched.
- `db.UpsertGame` / `db.UpsertLibraryEntry` — untouched; they were already
  collapsing duplicates, which is why only the counters were wrong.
- No CLI, schema, or MCP surface changes.

## Sequencing

First PR of the 2026-07-06 six-deep stack; branches off `origin/main`
(post-PR-#19). A107 stacks on this branch and will reuse the
`itchOwnedGames` seam.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` (new unit test runs in the
  sandbox — pure-Go driver, fixture DB, no real butler.db needed).
- Real-machine confirmation (optional, at Victor's leisure): re-run
  `gamesom.exe import itch` on the Windows box — summary should print 12
  installed, matching the DB.

## Scope boundary

Only the download-keys query path and its counters. Do not touch the caves
queries, unkeyed-cave import (A107), owned-semantics decisions (A107), other
importers, or the e2e suite (A101).
