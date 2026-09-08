# Work log — e2e-installed-assertions (A101)

Agent: Claude (direct implementation, /feature default path)
Date: 2026-07-06

## What was built

Per `docs/specs/e2e-installed-assertions/implementation.md`, no deviations:

- `assertLibraryInvariants` now counts `installed=1` rows and applies the
  `E2E_EXPECT_INSTALLED_<SOURCE>` contract after the non-empty check:
  unset → non-empty import with zero installed fails loudly (message names
  the env var to set); set to integer N → installed count must equal
  exactly N (a wrong declaration fails in either direction); non-integer →
  `t.Fatalf`.
- File-header doc comment documents the contract with an invocation
  example.
- All four `TestE2E_*` funcs inherit the assertion via the shared helper —
  no per-test changes.

## Load-bearing choices

- **Uniform exact-count rule instead of a zero-only escape hatch**: `N=0`
  is the action's declared-zero case; `N>0` gives the human a real
  ground-truth oracle (the launcher-visible count), which the Steam
  self-agreeing `expectedSteamAppIDs` check structurally can't be. One code
  path, and declarations are kept honest.
- Env var keys use the `library_entries.source` strings upper-cased
  (`STEAM`, `ITCHIO`, `GOG`, `EPIC`) — `ITCHIO` not `ITCH`, matching the
  DB, not the CLI subcommand.

## Verification

- `GOOS=windows go vet -tags e2e ./internal/importer/` — clean (the file is
  `//go:build windows && e2e`; cross-compile type-check is the strongest
  gate available off-box).
- `go build ./...`, `go vet ./...`, `go test ./...` — clean (confirms
  nothing leaked outside the tagged file).
- Cannot run the suite here by design — it targets real launcher installs
  on the Windows box. First post-merge run should immediately demand
  installed > 0 for itch/steam (both have installs there).
