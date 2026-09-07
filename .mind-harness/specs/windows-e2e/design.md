---
project: game-sommelier
feature: windows-e2e
status: draft
kind: design-lite
created: 2026-06-29
---

# Windows live-install e2e test suite — design note

## Problem

gamesom's store importers have **zero test coverage of the import path**. The only
test is `internal/normalize/title_test.go` (one pure unit test). Every importer
parses real launcher install artifacts (Steam `appmanifest_*.acf`, GOG
`galaxy-2.0.db`, itch `butler.db`, Epic manifests, Heroic JSON) and none of that
parsing is verified by anything. Today the only way to know an importer still works
is to run it and eyeball the output — manual QA on every change.

**Windows is the real target platform**: apart from console exclusives, virtually
every PC title runs on Windows, so that's where gamesom will be used most. The
maintainer has a Windows box with all the stores installed and real libraries. That
machine is the right end-to-end baseline.

## Goal

A **self-checking live-install e2e** run with one command on the Windows box, so a
parser regression is caught automatically instead of by manual inspection.

## Approach: invariant assertions against the real library

The test runs each importer against the **real** launcher directories and asserts
**invariants that hold for any real library** — not a frozen "expected" answer. This
is the load-bearing design choice: invariants survive installing/uninstalling games,
so the suite never needs re-baselining as the library changes.

Per-importer invariant classes:

- import returns no error; imported count > 0
- every `games` row: non-empty `canonical_title` and `normalized_title`
- every `library_entries` row: `source` set correctly, `source_game_id` non-empty
- no duplicate `(source, source_game_id)` pairs
- rows with `installed = 1`: the recorded `install_path` exists on disk
- **Steam local scan**: imported count == (`appmanifest_*.acf` files found − filtered
  tools/redistributables). The strongest check — catches a parser regression without
  anyone knowing the exact library contents.

## Scope

**In:**
- Live e2e for the local-install path of: `Steam` (local manifest scan), `Itch`,
  `GOG` (→ `GOGGalaxy` on Windows), `Epic`.
- A `db.OpenAt(path)` refactor so the e2e writes to a **throwaway temp DB** and never
  touches the real `gamesom.db`.

**Out (this pass):**
- The Steam **web API** path (`steamAPIImport`, hardcoded `api.steampowered.com`,
  needs a key). Local-scan only for now. Covering it later means making the base URL
  injectable for `httptest`.
- `SteamCollectionsImport` deep coverage — it's a post-import *completion marker*, not
  a library importer (operates on already-imported rows), so its invariants differ.
- Captured-fixture / golden-file parser tests that run cross-platform.
- Linux/macOS e2e (tracked separately).

## Constraints / honesty notes

- The Windows-gated importer code only executes in a Windows build, so it **cannot be
  verified on Linux**. CI/sandbox verification is limited to `GOOS=windows go build`
  (cross-compile) + `go vet`/`go test` on the non-gated parts. The actual live run
  only proves out on the Windows box.
- The e2e test file is therefore gated `//go:build windows && e2e` and invoked with
  `go test -tags e2e ./internal/importer/`. Default `go test ./...` stays fast and
  platform-neutral.
