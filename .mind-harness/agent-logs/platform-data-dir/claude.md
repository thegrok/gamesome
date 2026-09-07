# Work log — platform-data-dir (A096)

Agent: Claude (direct implementation, /feature default path)
Date: 2026-07-06

## What was built

Per `docs/specs/platform-data-dir/implementation.md`, no deviations:

- `dataDir()` rewritten: `XDG_DATA_HOME` override on every OS, then
  `%LOCALAPPDATA%\gamesom` (Windows, error if unset — same posture as
  `itchConfigDir`'s APPDATA check), `~/Library/Application Support/gamesom`
  (macOS), `~/.local/share/gamesom` (Linux).
- `legacyDataDir()`: the pre-A096 fallback, verbatim.
- `resolveDBPath(dir, legacyDir)`: move-don't-dual-read migration —
  `os.Rename` of `gamesom.db` + `-wal`/`-shm` sidecars, never clobbers an
  existing idiomatic DB, falls back to the legacy path with a stderr warning
  if the move fails. Pure function of two dirs = hermetic tests.
- `internal/db/db_test.go` (new): migrate-with-sidecar, never-clobber,
  nothing-to-migrate (no side-effect dirs), same-path no-op (the Linux
  case), XDG override.

## Load-bearing choices

- **Migration logs to stderr via `log`, never stdout** — MCP mode reserves
  stdout for JSON-RPC.
- **Move, not dual-read**: everyone converges on the idiomatic path; the
  fallback-to-legacy branch only triggers when the rename itself fails
  (survival beats idiom).
- The new-DB-exists check returns early *before* looking at legacy, so a
  user who already ran a post-A096 build never has an old DB resurrected
  over their current one.

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all pass; the five new db tests confirmed running via
  `-v` (migration log line visible in test output).
- Not verifiable here: the `windows`/`darwin` switch arms (Linux sandbox).
  Logic-reviewed; ground truth is Victor's next run on the Windows box —
  expect a one-time `migrated gamesom.db: … → …` stderr line and the DB
  under `%LOCALAPPDATA%\gamesom`.
