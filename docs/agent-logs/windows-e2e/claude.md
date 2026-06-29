# windows-e2e — work log

Implemented by **Claude** (Codex unavailable), per `docs/specs/windows-e2e/`.

## What was built

1. **`internal/db/db.go` — `OpenAt(path)` extraction.** `Open()` now resolves the
   default data dir and delegates to `OpenAt`, which does the `MkdirAll` +
   `sql.Open` + `schema` exec + `migrateEnrichColumns`. Pure refactor: `Open()`
   behaviour is unchanged (same path, 0700 perms, same migrations). Tests use
   `OpenAt(tempdir)` so they never touch the real `gamesom.db`.

2. **`internal/importer/e2e_windows_test.go` — new, `//go:build windows && e2e`,
   `package importer`.** Per-store subtests against the real installs:
   `TestE2E_Steam` (API forced off → local manifest scan), `TestE2E_Itch`,
   `TestE2E_GOG` (→ `GOGGalaxy` on Windows), `TestE2E_Epic`. Shared
   `assertLibraryInvariants` checks: count > 0, non-empty source_game_id /
   source_title / canonical_title / normalized_title, no duplicate
   `(source, source_game_id)`, and installed rows have an `install_path` that
   exists on disk. Steam additionally cross-checks row count against the distinct
   non-tool `appmanifest_*.acf` set, computed with the importer's own helpers.

## Load-bearing choices

- **Source strings verified against the code, not assumed:** `steam`, `itchio`
  (NOT `itch`), `gog`, `epic`. itch's source is `itchio` — asserting `itch` would
  have made `TestE2E_Itch` always fail.
- **`package importer` (internal test), not `importer_test`** — required to reach
  the unexported `steamAppsDirectories` / `isSteamTool` / `parseACF`, so the Steam
  count invariant can't drift from the importer's real discovery logic.
- **Steam expected-set replicates the importer's full filter chain** (appid/name/
  tool filter → int parse → non-empty normalized title) and dedups by appid, to
  match the DB's `UNIQUE(source, source_game_id)` behaviour.

## Verification (on Linux — the sandbox)

- `go vet ./...` ✓
- `go test ./...` ✓ (default tags; importer shows `[no test files]` — e2e correctly excluded)
- `GOOS=windows go build ./...` ✓
- `GOOS=windows go vet -tags e2e ./internal/importer/` ✓ (the tagged test compiles + vets for Windows)

## Not verifiable here (needs the Windows box)

The actual live run — `go test -tags e2e ./internal/importer/` against the real
Steam/itch/GOG/Epic installs — can only run on Windows. That's the real baseline.
Cross-compile + vet prove the code is sound; they don't prove the importers parse
the maintainer's real install artifacts. That assertion is the maintainer's run.
