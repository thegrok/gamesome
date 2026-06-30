---
project: game-sommelier
feature: windows-e2e
status: draft
created: 2026-06-29
grounded-at: bcb4b9e   # origin/main head when spec written
---

# Windows live-install e2e — implementation

Grounded in the committed code at `origin/main` (bcb4b9e). Two changes: a small
`db` refactor, then the build-tagged e2e test file.

## Change 1 — `internal/db/db.go`: extract `OpenAt(path)`

**Why:** `Open()` hardcodes the path to the real `gamesom.db` and the `schema` const
+ `migrateEnrichColumns` are unexported. The e2e needs a throwaway DB with the exact
same schema, without duplicating the DDL or touching the user's library.

**Diff (refactor, no behaviour change for `Open()`):**

```go
// OpenAt returns a ready-to-use DB at the given path, creating the parent
// directory and applying the schema + enrich migrations. Open() is OpenAt at
// the default data location.
func OpenAt(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	migrateEnrichColumns(db)
	return db, nil
}

func Open() (*sql.DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	return OpenAt(filepath.Join(dir, "gamesom.db"))
}
```

**Contract preserved:** `Open()` behaves identically (same path, same dir perms 0700,
same schema+migrations). Pure extraction. Verify with `go build` + `go vet` +
`go test ./internal/db/...`.

## Change 2 — `internal/importer/e2e_windows_test.go` (new, build-tagged)

**Build constraint:** first line `//go:build windows && e2e`. Default `go test ./...`
ignores it; run with `go test -tags e2e ./internal/importer/` on Windows.

**Package:** `package importer` (an *internal* test, NOT `importer_test`). Load-bearing:
it gives the test access to the unexported path resolvers and parsers
(`steamAppsDirectories()`, `isSteamTool()`, `parseACF()`, etc.) so Steam's count
invariant can be computed exactly the way the importer sees the filesystem.

### Harness

- Each subtest opens a **fresh throwaway DB**: `db.OpenAt(filepath.Join(t.TempDir(),
  "e2e.db"))`. Never `db.Open()` — that would hit the real library.
- Run the importer against the **real** launcher dirs (no env overrides — we want the
  genuine install paths on the box).
- For Steam: ensure `STEAM_API_KEY` is **unset** for the subtest (`t.Setenv`) so only
  the local manifest-scan path runs — the web API path is out of scope this pass.

### Shared invariant helper

After each importer runs, assert against the DB via raw SQL on the `*sql.DB`:

```
assertLibraryInvariants(t, database, source):
  - SELECT count(*) FROM library_entries WHERE source = ?   ->  must be > 0
  - every row: source_game_id != "" AND source_title != ""
  - the joined games row: canonical_title != "" AND normalized_title != ""
  - no duplicate (source, source_game_id):
      SELECT source_game_id, count(*) FROM library_entries
      WHERE source=? GROUP BY source_game_id HAVING count(*) > 1   ->  empty
  - every row with installed=1: os.Stat(install_path) succeeds (path exists)
```

### Per-importer subtests

| Subtest | Calls | Source value | Extra invariant |
|---------|-------|--------------|-----------------|
| `TestE2E_Steam`  | `Steam(db)` (API key unset) | `steam` | count of `steam` rows == non-tool `appmanifest_*.acf` across `steamAppsDirectories()` (walk + `parseACF` + `isSteamTool` filter, computed in-test) |
| `TestE2E_Itch`   | `Itch(db)`  | `itch` | — |
| `TestE2E_GOG`    | `GOG(db)` (→ `GOGGalaxy` on Windows) | `gog` | — |
| `TestE2E_Epic`   | `Epic(db)`  | `epic` | — |

Each subtest: open temp DB → run importer → `assertLibraryInvariants`. If a store
isn't installed the importer returns an error; treat that as a **failure** (the
baseline assumes all stores present). If we later want to run on a partial box, gate
per-store with `t.Skip` on a "dir not found" sentinel — note this but don't build it
now.

### Steam count invariant detail

Steam is the strongest check. In-test, replicate the importer's own discovery:
walk `steamAppsDirectories()`, `filepath.Glob("appmanifest_*.acf")`, `parseACF` each,
drop entries where `isSteamTool(name)` is true, collect the distinct appids. Assert
`len(expected) == <count of steam rows in DB>`. Because the test is `package importer`
it uses the *same* helpers the importer uses, so the two can't drift.

## Scope boundary

- No Steam web API coverage (URL not injectable; deferred).
- No new fixtures / testdata; this is live-install only.
- No CLI changes, no schema changes (`OpenAt` reuses the existing `schema`).
- No Linux/macOS e2e.

## Verification plan

- **On Linux (CI / sandbox):** `GOOS=windows go build ./...` (cross-compile proves the
  tagged test + refactor compile for Windows); `go vet ./...`; `go test ./...`
  (default, non-e2e, must stay green). Also `GOOS=windows go vet -tags e2e ./...`.
- **On the Windows box:** `go test -tags e2e ./internal/importer/` against the real
  installs — the actual baseline. This step can only happen on that machine.
