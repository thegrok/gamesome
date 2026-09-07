---
feature: itch-unkeyed-games
status: ready
created: 2026-07-06
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: feature/itch-key-dedupe   # stacked on A108 (PR #20)
---

# Implementation — import free/unkeyed itch.io games (A107)

## Load-bearing contract

Every installed cave in butler's DB whose game passes the
`classification = 'game'` filter must produce a library row — whether or not
a `download_keys` row exists. Keyed games import exactly as before (A108
behavior); unkeyed games import with the same cave-path resolution and
install check, `Owned: 1` per the design proposal. No game may appear twice.

## Files changed

### `internal/importer/itch.go`

**1. New helper below `itchOwnedGames`:**

```go
// itchUnkeyedInstalledGames returns games that have an installed cave but no
// download key. Butler creates no download_keys row when a game is claimed
// free, so cave-only games (e.g. free claims that are installed right now)
// are invisible to the keyed query. classification = 'game' matches the
// keyed query's filter.
func itchUnkeyedInstalledGames(butlerDB *sql.DB) ([]itchGame, error) {
	rows, err := butlerDB.Query(`
		SELECT DISTINCT c.game_id, g.title, g.url
		FROM caves c
		JOIN games g ON g.id = c.game_id
		WHERE g.classification = 'game'
		  AND c.game_id NOT IN (SELECT game_id FROM download_keys)`)
	if err != nil {
		return nil, fmt.Errorf("query itch unkeyed caves: %w", err)
	}
	defer rows.Close()

	var games []itchGame
	for rows.Next() {
		var game itchGame
		var title, url sql.NullString
		if err := rows.Scan(&game.ID, &title, &url); err != nil {
			return nil, fmt.Errorf("scan itch unkeyed cave: %w", err)
		}
		game.Title = title.String
		game.URL = url.String
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read itch unkeyed caves: %w", err)
	}
	return games, nil
}
```

(`SELECT DISTINCT` because one game can hold multiple caves — e.g. two
installs at different locations; same double-count hazard A108 fixed for
keys.)

**2. Call site in `Itch()`** — after the `itchOwnedGames` call:

```go
unkeyed, err := itchUnkeyedInstalledGames(butlerDB)
if err != nil {
	return err
}
games = append(games, unkeyed...)
```

The `NOT IN` clause guarantees the two lists are disjoint, so the append
cannot introduce duplicates. The existing loop handles the rest — cave
lookup by `game.ID` resolves the real install path (PR #19 behavior),
`os.Stat` gates installed, entry gets `Owned: 1`.

**Failure mode note:** if the unkeyed query errors on an exotic/older butler
schema, `Itch()` returns the error (same posture as the keyed query — both
tables already exist in the legacy schema `itchCavePathsLegacy` targets, so
there is no older-schema fallback tier to preserve here).

### `internal/importer/itch_test.go`

**3. Extend `fixtureButlerDB` schema** with the `caves` columns the new query
reads: `CREATE TABLE caves (id INTEGER PRIMARY KEY, game_id INTEGER, install_folder_name TEXT, custom_install_folder TEXT, install_location_id INTEGER);`
plus `CREATE TABLE install_locations (id INTEGER PRIMARY KEY, path TEXT);`
(matching what `itchCavePaths` queries, so future tests can reuse the
fixture end-to-end).

**4. New test `TestItchUnkeyedInstalledGames`:** insert a keyed+caved game, a
cave-only game (the Dr. Langeskov case), a cave-only non-`game` row, and a
game with two caves. Assert: only the cave-only `game` rows return, the
two-cave game returns once, the keyed game is excluded (it's the keyed
query's job), the non-game is excluded.

## Integration points

- `itchOwnedGames` (A108) — untouched; this stacks beside it.
- `itchCavePaths` / install check / `db.Upsert*` — untouched; unkeyed games
  flow through the same loop.
- No schema, CLI, or MCP changes. `Owned` stays `1` for all itch rows.

## Sequencing

PR 2 of the 2026-07-06 stack, based on `feature/itch-key-dedupe` (PR #20).
Merge after #20 with a merge commit; GitHub retargets this PR to `main`
automatically.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` in the sandbox (fixture
  DB covers the new query).
- Real-machine ground truth (Victor, at leisure): `gamesom.exe import itch`
  should report 15 installed (was 13 pre-PR-#19-verify, 12 after A108's
  count fix, +3 unkeyed — Dr. Langeskov, Battle for Wesnoth, and whichever
  third the 15-vs-13 gap hides — the app-vs-gamesom diff will name it).

## Scope boundary

Only the unkeyed-cave pass. Do not touch keyed-game behavior, the caves
path-resolution helpers, owned-flag schema, other importers, or the e2e
suite.
