---
feature: untrack-binary
status: approved
created: 2026-07-12
type: lite   # two-line fix, no architecture/UX/schema decision
last_amended: 2026-07-12
amended_by: agent:programming
---

# Implementation (lite) — untrack the committed built binary (A120)

## Problem

Commit `308806a` (2026-06-17, "docs: add clarifying comment about cloud
storage location") accidentally added a ~15MB built binary, `gamesom`, at
repo root alongside an unrelated docs edit. `.gitignore` has no rule that
would have caught it — the existing binary rules (`*.exe`, `*.dll`, `*.so`,
`*.dylib`) only cover platform-specific extensions, and a locally-built Go
binary on Linux/macOS has no extension. Post-A112 the binary would be named
`gamesome` if rebuilt today; same gap, same risk.

## Fix

1. `git rm --cached gamesom` — untrack the committed binary, keep the local
   file (developer's working copy, harmless to leave on disk).
2. Add to `.gitignore`, under the existing "Binaries for programs and
   plugins" section:
   ```
   /gamesom
   /gamesome
   ```
   Anchored to repo root (`/`) so it only ignores the built binary there,
   not any same-named file/dir elsewhere in the tree (there's none today,
   but the anchor is free correctness). Both names covered: `gamesom` for
   any pre-rename working copies still lying around, `gamesome` going
   forward.

## Sequencing

Single commit. No migration, no schema, no CLI surface.

## Verification

- `git status` after `git rm --cached` shows `gamesom` as untracked (not
  gone from disk) and `.gitignore` modified.
- `git status` with a locally-rebuilt `gamesome` binary present shows it
  does NOT appear as untracked (gitignore rule catches it).
- No `go build`/`go vet`/`go test` surface — this touches no Go source.

## Scope boundary

**In:** `git rm --cached gamesom`; the two-line `.gitignore` addition.
**Out:** any other `.gitignore` cleanup; rebuilding or committing a fresh
binary; CI changes (the release binaries are GoReleaser output under
`dist/`, already gitignored, unaffected by this).
