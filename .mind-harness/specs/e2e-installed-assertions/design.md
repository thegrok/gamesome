---
feature: e2e-installed-assertions
status: approved
created: 2026-07-06
author: claude
type: lite   # test-suite gap fix, one env-var contract to define
---

# Design (lite) — e2e installed-state assertions (A101)

## Problem

The windows-e2e suite (A082, PR #16) is supposed to verify real-world
installed-state accuracy, but it structurally couldn't catch any of the A100
bugs: `assertLibraryInvariants` only checks that a row *claiming*
`installed=1` has a real path — nothing checks the reverse. A source that
silently under-reports (every row `installed=0`) is indistinguishable from a
source with nothing installed. `TestE2E_Steam`'s extra oracle recomputes
"expected" via the same helpers the importer uses, so a wrong path list
agrees with itself.

## Decision

Add an installed-count assertion to `assertLibraryInvariants`, governed by
one env contract — only a human standing at the machine knows ground truth:

- **`E2E_EXPECT_INSTALLED_<SOURCE>` unset (default):** a non-empty import
  reporting **zero** installed rows **fails loudly**, with a message naming
  the env var. This flips the silent blind spot to a loud failure by
  default.
- **Set to an integer N:** the installed count must equal **exactly N**.
  `N=0` is the action's "human explicitly declares zero-installed as
  expected"; `N>0` lets the human pin the true count they can see in the
  launcher — the ground-truth oracle the Steam self-agreement check can't
  be.

`<SOURCE>` is the upper-cased library source: `STEAM`, `ITCHIO`, `GOG`,
`EPIC`.

One uniform rule rather than a zero-only escape hatch: same code path,
strictly more diagnostic power, and the declaration stays honest (declaring
0 while 3 are installed also fails).

## Out of scope

Restructuring the Steam expected-appids oracle (its self-agreement is noted
in the action but the fix shape is the human-declared count above), fixture
tests (they live beside, not in, the e2e suite), CI wiring (the suite is
human-run by design).
