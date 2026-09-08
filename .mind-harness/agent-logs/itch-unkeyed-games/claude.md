# Work log — itch-unkeyed-games (A107)

Agent: Claude (direct implementation, /feature default path)
Date: 2026-07-06

## What was built

Per `docs/specs/itch-unkeyed-games/implementation.md`, no deviations:

- `itchUnkeyedInstalledGames(butlerDB)`: cave-only games (no `download_keys`
  row), joined to `games` for title/URL, `classification = 'game'` filter,
  DISTINCT for multi-cave games.
- `Itch()` appends the unkeyed list to the keyed list; `NOT IN` keeps the
  two disjoint, so no dedupe needed at the append.
- Fixture schema grew `caves` + `install_locations` (full column set
  `itchCavePaths` reads, so future tests can reuse it end-to-end).
- `TestItchUnkeyedInstalledGames`: unkeyed cave returns, two-cave game
  returns once, keyed game excluded, non-`game` classification excluded.

## Load-bearing choices

- **Owned semantics: `Owned: 1` for unkeyed games** — the design proposal
  (claimed-free ≈ owned), flagged in the PR body for Victor to ratify. The
  separate-helper structure makes demoting later a one-line change.
- **Separate helper, not a widened keyed query.** Keeps the keyed/unkeyed
  distinction explicit and leaves `itchOwnedGames` (A108) untouched.
- No legacy-schema fallback tier: both tables the new query touches exist in
  the legacy schema `itchCavePathsLegacy` targets, so an error here is a
  real error, not an old-butler signature.

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all pass; both itch unit tests confirmed running via
  `-v -run TestItch`.
- Not verifiable here: real butler.db. Ground truth after merge: import on
  the Windows box should surface Dr. Langeskov + Battle for Wesnoth (+1 —
  the 15-vs-12 gap names a third; the app-vs-gamesom diff will identify it).
