# Work-log — epic-installed-fallback (Claude, 2026-07-05)

## What was built

`internal/importer/epic.go`: when `epicFromHeroicCache` fails to read Heroic's
`legendaryConfig/legendary/installed.json`, it now derives install status from
the native Epic Games Launcher manifests via the new `heroicInstalledFromEGL()`
helper instead of substituting an empty map. One file, +21/-1 — exactly the
spec's diff.

## Load-bearing choices

- **Fix shape B, not A100's literal fix-shape note.** A100 proposed surfacing
  the error so `Epic()` falls through to Legendary CLI → EGL manifests.
  Traced, that path detours a legendary-less Windows machine into a
  `winget install` + auth prompt (or discards the successfully-read owned
  library for the run if declined) and turns Linux no-`installed.json` users'
  working owned-only import into a hard error. Victor confirmed shape B
  2026-07-05: cross-check EGL manifests inside the Heroic path. Recorded in
  `docs/specs/epic-installed-fallback/design.md`.
- **`epicFromHeroicCache` still returns `nil` on installed.json failure** —
  deliberately. `Epic()`'s fall-through chain stays reserved for a missing
  Heroic *library* cache; install-status degradation is handled locally now.
- **No new unit tests.** The failure path is gated on real launcher state
  (`%PROGRAMDATA%` EGL manifests; `epicManifestsDir` hard-errors on Linux), and
  the importer package has no unit-test seam for path injection today. The
  Windows-tagged e2e suite exists but per A101 can't catch installed-state
  under-reporting yet. Ground truth remains Victor's on-machine
  `gamesom.exe import epic` run.

## Spec corrections

None — the spec matched repo head at implementation time.

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — pass (normalize suite; importer has no Linux-runnable tests)
- Not verifiable here: real-machine behavior (Linux sandbox, no EGL/Heroic
  state). Needs the A100 verification loop on the Windows box.
