---
feature: platform-data-dir
status: approved
created: 2026-07-06
author: claude
type: lite   # contained platform-correctness fix + migration
---

# Design (lite) — platform-idiomatic data dir + legacy migration (A096)

## Problem

`internal/db/db.go dataDir()` hardcodes XDG semantics on every OS, so on
Windows — the primary platform — the DB lands at
`C:\Users\<u>\.local\share\gamesom` (verified 2026-07-04). With the MCPB
bundle (A095) about to put gamesom in front of non-developer users, the data
dir should be idiomatic per platform from day one.

## Decision

**New `dataDir()`:**

- `XDG_DATA_HOME` set → `$XDG_DATA_HOME/gamesom` on **every** OS (unchanged
  escape hatch, also what keeps tests hermetic).
- Windows → `%LOCALAPPDATA%\gamesom` (error if `LOCALAPPDATA` unset, same
  posture as `itchConfigDir`'s `APPDATA` check).
- macOS → `~/Library/Application Support/gamesom`.
- Linux/other → `~/.local/share/gamesom` (unchanged).

**Migration, not dual-read:** on `Open()`, if the idiomatic path has no DB
and the legacy path (`~/.local/share/gamesom`) has one, **move** it
(`os.Rename`, plus `-wal`/`-shm` sidecars if present) before opening.
Everyone converges on the idiomatic path — a permanent dual-read would leave
old installs non-idiomatic forever, which defeats the point. If the rename
fails (exotic cross-volume home), fall back to opening the legacy path with
a warning — survival beats idiom.

Migration is naturally a no-op on Linux (legacy == new) and under
`XDG_DATA_HOME` (override wins, and it's the same dir the legacy logic used).
Real moves happen only on Windows/macOS — exactly where the path was wrong.

## Out of scope

Config-file locations (gamesom has none), the itch/steam/epic config-dir
resolvers (they read *other apps'* dirs — those are correct as-is), MCPB
manifest wiring (A095).
