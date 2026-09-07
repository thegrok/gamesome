---
feature: e2e-verbose-logging
status: lite
created: 2026-07-13
author: claude
---

# Design (lite) — verbose per-source output in windows e2e suite (A110)

## Intent

`go test -tags e2e ./internal/importer/` only reports test name + PASS/FAIL.
`assertLibraryInvariants` (`e2e_windows_test.go:51`) already computes row
count and installed count per source but never surfaces them, so a human
running the suite can't tell what was actually checked unless something
fails.

## Scope

Add `t.Logf` reporting of what each source's invariant check saw: row count,
installed count, and whether `E2E_EXPECT_INSTALLED_<SOURCE>` was declared
(and to what value). Visible with `go test -v -tags e2e`; silent by default
on a clean PASS — standard Go `t.Log` behavior, no new flag or config
surface needed.

## Out of scope

No change to what's asserted (invariants stay exactly as written) or to
the duplicate-check query. No new build tags/flags. Not touching the
non-e2e importer tests.
