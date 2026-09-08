---
feature: e2e-verbose-logging
status: ready
created: 2026-07-13
author: claude
repo: git@github.com:thegrok/gamesome.git
base-branch: main
---

# Implementation — verbose per-source output in windows e2e suite (A110)

## Load-bearing contract

The invariant checks themselves don't change — only what gets logged.
Reporting happens after the invariant loop (so counts reflect what was
actually scanned) and uses `t.Logf`, which Go only prints under `-v` or on
a failing test — a clean `-tags e2e` run without `-v` stays silent exactly
as today.

## Files changed

### `internal/importer/e2e_windows_test.go`

In `assertLibraryInvariants` (currently lines 51–141), after the row-scan
loop finishes (after the `for rows.Next()` loop, before the
`E2E_EXPECT_INSTALLED_<SOURCE>` branch at line 110) and after that branch
resolves the "declared" state, add one `t.Logf` call reporting:

- `count` (row count) and `installedCount`
- whether `E2E_EXPECT_INSTALLED_<SOURCE>` was set, and if so, to what value

Restructure the existing `if v := os.Getenv(expectEnv); v != ""` block
slightly so the declared/undeclared state is captured in a local
(`declared string`, e.g. `"undeclared"` or `"declared=N"`) before the
single summary log line, rather than logging twice from inside both
branches — one line per source, e.g.:

```go
t.Logf("[%s] rows=%d installed=%d %s=%s", source, count, installedCount, expectEnv, declared)
```

Keep the existing `t.Errorf` failure behavior in the declared-mismatch and
undeclared-zero-installed branches untouched — this is additive logging,
not a change to pass/fail logic.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — the e2e file is
  behind `//go:build windows && e2e`, so it doesn't compile or run on this
  Linux sandbox at all; these three commands won't touch it.
- Can't be run/verified here: the actual `-tags e2e` suite only builds and
  executes on Windows with real launcher installs. Human pass on the
  Windows box: `go test -v -tags e2e ./internal/importer/` should show one
  `rows=… installed=… E2E_EXPECT_INSTALLED_<SOURCE>=…` line per source on a
  clean PASS; running without `-v` should stay silent as before.

## Scope boundary

Logging only. No change to invariant logic, no new env vars/flags, no
touching the non-e2e importer tests.
