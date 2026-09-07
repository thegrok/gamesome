---
feature: playtime-observations
status: approved
created: 2026-07-26
author: claude
type: implementation
actions: A162
---

# Implementation — implicit play-history observations (A162 / Phase 5a)

Ground truth pulled from `gamesome` repo head (2026-07-26, post-PR-#34):
`internal/db/db.go` (schema + `UpsertLibraryEntry`),
`internal/importer/steam.go` (only source that sets `PlaytimeMinutes`),
`internal/importer/{epic,gog,itch}.go` (confirmed: none read or set
playtime today).

## Schema change — `internal/db/db.go`

Add to the `schema` const, alongside the existing tables:

```sql
CREATE TABLE IF NOT EXISTS library_observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    source_game_id TEXT NOT NULL,
    observed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    playtime_minutes INTEGER NOT NULL
);
```

No `UNIQUE` constraint — append-only by design, multiple rows per
`(source, source_game_id)` over time is the point. No `FOREIGN KEY` to
`library_entries` — keyed by the same `(source, source_game_id)` pair
`library_entries` itself uses, joined at read time if ever needed, not
constrained at write time (avoids an extra lookup in the hot import path
and avoids coupling this table's insert to `library_entries` row lifecycle).

## Write path — new function in `internal/db/db.go`

```go
// RecordPlaytimeObservation appends a playtime snapshot. Best-effort: an
// error here must never fail the import that triggered it (see A162 design,
// "load-bearing contract") — instrumentation, not import correctness.
func RecordPlaytimeObservation(db *sql.DB, source, sourceGameID string, playtimeMinutes int) {
	if _, err := db.Exec(
		`INSERT INTO library_observations (source, source_game_id, playtime_minutes) VALUES (?, ?, ?)`,
		source, sourceGameID, playtimeMinutes,
	); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record playtime observation for %s/%s: %v\n", source, sourceGameID, err)
	}
}
```

Returns nothing (mirrors the chmod warning pattern from A126) — callers
shouldn't have to handle an error from something that's explicitly allowed
to fail silently-but-logged.

## Call site — `internal/importer/steam.go`

Immediately after the existing `db.UpsertLibraryEntry(database, entry)` call
(around line 132) succeeds:

```go
if err := db.UpsertLibraryEntry(database, entry); err != nil {
    log.Printf("warning: upsert library entry %q: %v", g.Name, err)
    continue
}
db.RecordPlaytimeObservation(database, entry.Source, entry.SourceGameID, entry.PlaytimeMinutes)
imported++
```

No call added to `epic.go`/`gog.go`/`itch.go` — they have no playtime data
to observe (confirmed by reading all three; none reference `PlaytimeMinutes`
or `PlaytimeForever`). Adding a zero-value call there would just write noise
rows forever.

## Out of scope (see design.md "Non-goals")

- No diffing/read query, no MCP tool exposure — no consumer exists yet.
- No `epic`/`gog`/`itch` observations (no source data to observe).
- No telemetry, export, or cross-user aggregation of any kind.
- No `README.md` change — this is invisible instrumentation, nothing
  user-facing to document yet.

## Verification

- `go build ./...`, `go vet ./...`
- A new test in `internal/db/db_test.go`:
  `TestRecordPlaytimeObservation_AppendsRows` — call it twice with different
  values for the same `(source, source_game_id)`, assert two rows exist
  (not one upserted row) and both values are readable back in order.
- A new test in `internal/importer` (or extend the existing Steam import
  test) asserting that a Steam import run produces exactly one
  `library_observations` row per imported game.

## Scope boundary

In: `internal/db/db.go` (schema + `RecordPlaytimeObservation`),
`internal/importer/steam.go` (one call site), their tests.
Out: everything under design.md's "Non-goals".
