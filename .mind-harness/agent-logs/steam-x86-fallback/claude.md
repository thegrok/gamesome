# Work log — steam-x86-fallback (A109)

Agent: Claude (direct implementation, /feature default path)
Date: 2026-07-06

## What was built

Per `docs/specs/steam-x86-fallback/implementation.md`, no deviations:

- `programFilesX86()`: prefers `os.Getenv("PROGRAMFILES(X86)")`, falls back
  to `os.Getenv("PROGRAMFILES") + " (x86)"` when that's empty.
- `steamAppsDirectories()`'s Windows branch calls it instead of reading
  `PROGRAMFILES(X86)` inline.
- `steam_x86_test.go`: env-var-present wins (today's exact behavior),
  fallback derivation when absent (the A102 case), empty when neither set.

## Load-bearing choices

- **Env var still wins when present** — this is a fallback, not a
  replacement, so any environment that does carry `PROGRAMFILES(X86)`
  (including a non-standard custom value) behaves identically to before.
- **Derivation, not a second env-var lookup**: WOW64 guarantees
  `%ProgramFiles(x86)%` is `%ProgramFiles%` + `" (x86)"` on the same drive
  on every 64-bit Windows install — this isn't a guess, it's the OS's own
  convention. On genuine 32-bit Windows (no x86 counterpart), the derived
  path simply doesn't exist and the glob scan finds nothing there, same as
  today's behavior for any other absent candidate directory.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — clean, all three new
  tests confirmed running via `-v -run`.
- `programFilesX86()` has no `GOOS` branch, so it's fully testable outside
  Windows — the actual bug (subprocess missing the parenthesized var) is
  covered by `TestProgramFilesX86_FallsBackWhenUnset`.
- Not verifiable here: an actual `refresh_library` run from Claude Desktop
  on the Windows box. Expected post-merge: 82 installed, matching the
  terminal run that already works.

## Provenance

Root cause established via the A102 `--debug-env` instrument (PR #24), run
on-machine 2026-07-06 — findings doc Sixth addendum has the raw evidence
(env dumps from both a terminal and a Claude-Desktop-spawned launch).
