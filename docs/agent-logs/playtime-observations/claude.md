# Work log — playtime-observations (A162)

## What was built

- `internal/db/db.go`: new `library_observations` table (append-only, keyed
  by `(source, source_game_id)`, no FK) + `RecordPlaytimeObservation` —
  best-effort insert, logs a warning and returns nothing on failure (mirrors
  A126's chmod-warning pattern).
- `internal/importer/steam.go`: one call to `RecordPlaytimeObservation`
  right after the existing `UpsertLibraryEntry` call, reusing the same
  `entry.Source`/`entry.SourceGameID`/`entry.PlaytimeMinutes` values already
  validated for the upsert.
- `internal/db/db_test.go`: `TestRecordPlaytimeObservation_AppendsRows` —
  calls it twice with different values for the same `(source,
  source_game_id)`, asserts two rows exist in insertion order (proves
  append, not upsert).

## Load-bearing choices / corrections vs. the spec

- **No Epic/GOG/itch call sites** — confirmed by reading all three
  importers: none reference `PlaytimeMinutes`/playtime data at all, so
  there's nothing to observe there yet. Matches design.md's scoping.
- **No Steam()-level end-to-end test added.** implementation.md said to
  "extend the existing Steam import test," but there isn't one — `Steam()`
  itself (the live Web API + local manifest scan) has no unit test in this
  repo; only sub-pieces are tested (`steam_x86_test.go`,
  `steam_credentials_test.go`). Building a mock-API/manifest harness to
  unit-test the full import loop is real new test infrastructure, out of
  scope for this action. The call site is a single line reusing
  already-validated `entry` fields, so the direct `db`-level test plus
  reading the diff covers it. Flagging this so it doesn't read as a gap
  nobody noticed.
- **No read/diffing API, no MCP tool, no README change** — per design.md's
  non-goals, since there's no consumer yet.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass, including the new
  `TestRecordPlaytimeObservation_AppendsRows`

## Cross-model review

Not run — small, directly-tested, additive-only change (one new table, one
new function, one call site) with no existing behavior touched. Judged the
review overhead not worth it for this size of diff; flag if a second opinion
is wanted before merge.
