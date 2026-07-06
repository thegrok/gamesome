---
feature: e2e-installed-assertions
status: ready
created: 2026-07-06
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: feature/platform-data-dir   # stacked, PR 4 of the 2026-07-06 run
---

# Implementation — e2e installed-state assertions (A101)

## Load-bearing contract

Running the e2e suite against a machine where a source **has** installed
games but the importer marks none installed must **fail by default**. The
only way to accept zero installed is an explicit human declaration
(`E2E_EXPECT_INSTALLED_<SOURCE>=0`), and a declaration that disagrees with
the DB in either direction also fails.

## Files changed

### `internal/importer/e2e_windows_test.go` (only file)

**1. Count installed rows in `assertLibraryInvariants`'s existing scan
loop** (the loop already reads `installed`):

```go
installedCount := 0
// inside the loop:
if installed == 1 { installedCount++ /* existing path checks stay */ }
```

**2. New assertion after the non-empty check (~line 91):**

```go
// Installed-state accuracy: a source that silently under-reports (every
// row installed=0) must be indistinguishable from nothing-installed only
// when a human says so. E2E_EXPECT_INSTALLED_<SOURCE> pins the count the
// human can see in the launcher; unset, zero installed on a non-empty
// import is a loud failure (the A100/A102 class of bug).
expectEnv := "E2E_EXPECT_INSTALLED_" + strings.ToUpper(source)
if v := os.Getenv(expectEnv); v != "" {
	want, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("[%s] %s=%q is not an integer", source, expectEnv, v)
	}
	if installedCount != want {
		t.Errorf("[%s] installed count = %d, but %s declares %d", source, installedCount, expectEnv, want)
	}
} else if count > 0 && installedCount == 0 {
	t.Errorf("[%s] non-empty import (%d rows) reports 0 installed — an under-reporting importer is indistinguishable from an empty machine; if genuinely nothing is installed for this source, declare it: set %s=0", source, count, expectEnv)
}
```

**3. Imports:** add `strings` (`os`/`strconv` already imported).

**4. Doc comment:** extend the file header's invocation example with the
env contract, e.g.
`E2E_EXPECT_INSTALLED_GOG=0 go test -tags e2e ./internal/importer/`.

## Integration points

- All four `TestE2E_*` funcs get the assertion for free via
  `assertLibraryInvariants` — no per-test changes.
- Sources as stored in `library_entries.source`: `steam`, `itchio`, `gog`,
  `epic` → env vars `E2E_EXPECT_INSTALLED_STEAM` / `_ITCHIO` / `_GOG` /
  `_EPIC`.
- No production code touched.

## Sequencing

PR 4 of the stack, based on `feature/platform-data-dir` (PR #22). No file
overlap with PRs 1–3 (PRs 1–2 touch `itch.go`/`itch_test.go`, not the e2e
file); stacked for merge-order clarity.

## Verification

- The file is `//go:build windows && e2e` — it cannot compile or run on the
  Linux sandbox under default flags. Type-check via cross-compile:
  `GOOS=windows go vet -tags e2e ./internal/importer/`.
- `go build ./...`, `go vet ./...`, `go test ./...` (unchanged packages —
  confirms no accidental spill outside the tagged file).
- Actually running the suite is Victor-on-the-Windows-box by design. First
  post-merge run: expect immediate value — itch should now demand installed
  > 0 (it has installs), and any A102-class Steam under-report fails loud.

## Scope boundary

Only the e2e test file. Do not touch importers, the fixture unit tests, or
the Steam oracle's structure.
