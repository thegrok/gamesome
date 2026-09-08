# Work-log — itch-install-location (Claude, 2026-07-05)

## What was built

`internal/importer/itch.go`: cave install paths now resolve through butler's
own data — `custom_install_folder` first, else `install_locations.path` +
`install_folder_name`, else the legacy `configDir/apps` guess — via a new
`itchCavePaths()` helper. The inline cave query and the `itchCave` struct are
gone; the import loop's `os.Stat` gate is unchanged but now checks real paths.
One file, matching the spec's diff.

## Load-bearing choices

- **Whole-query fallback, not per-row.** If the joined query itself errors
  (older butler schema, renamed column — the column names come from butler's
  Go models + snake_case convention, not a live `.schema` dump), we log a
  warning naming the failure and run `itchCavePathsLegacy()` — today's exact
  query and path logic. Strictly no worse than pre-fix behavior, and the
  mismatch is visible instead of silent.
- **Default-location installs also go through the DB now.** Previously the
  `apps/` path was hardcoded; now it comes from `install_locations.path` even
  for the default location, with `configDir/apps` only as a last resort when
  both columns are empty. This means a *moved* default location is handled
  too, not just per-cave custom folders.
- **NULL-folder-name parity kept.** A cave row with an empty
  `install_folder_name` resolves to the bare location dir, which can Stat as
  existing — same false-positive class the legacy code had
  (`Join(configDir, "apps", "")`). Left as-is deliberately: parity over an
  unobserved edge case, per the scope boundary.

## Spec corrections

None — the spec matched repo head at implementation time.

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — pass (normalize suite; importer has no Linux-runnable tests)
- Not verifiable here: real-machine behavior (Linux sandbox has no butler.db /
  itch installs). Ground truth is Victor's `gamesom.exe import itch` on the
  Windows box — expected: the `Games\itch.io` installs flip to installed with
  correct paths.
