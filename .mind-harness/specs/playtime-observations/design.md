---
feature: playtime-observations
status: approved
created: 2026-07-26
author: claude
type: lite   # additive-only instrumentation; design discussion already happened in-session (see below)
actions: A162
---

# Design (lite) — implicit play-history observations (A162 / Phase 5a)

**Lite-design call**: this touches a new table, which the `/feature` checkpoint
defaults to a full design pass for. Marked lite anyway because the actual
design conversation already happened (this doc records it) and the change is
narrowly additive: one new table, no changes to any existing table, query, or
MCP tool behavior.

## Problem

`spec/approach.md`'s Phase 5 (learning loop) was originally planned entirely
post-alpha. But `library_entries` is current-state-shaped —
`UNIQUE(source, source_game_id)`, and `UpsertLibraryEntry`
(`internal/db/db.go:336`) overwrites `playtime_minutes` on every import
(`ON CONFLICT ... DO UPDATE SET playtime_minutes = excluded.playtime_minutes`).
So every alpha user's import runs are **already discarding** the exact signal
Phase 5a wants to capture — deferring 5a past the alpha doesn't defer the
cost, it just makes the loss permanent for the entire alpha period. Phase 5b
(in-conversation feedback capture) is genuinely better designed after
watching real alpha usage; Phase 5a needs no such observation — it's pure
mechanical instrumentation — so there's no offsetting benefit to waiting.

## Decision

Add an append-only table:

```sql
CREATE TABLE IF NOT EXISTS library_observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    source_game_id TEXT NOT NULL,
    observed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    playtime_minutes INTEGER NOT NULL
);
```

Keyed by `(source, source_game_id)` rather than `library_entries.id` — the
importer already has the source + source-game-id pair on hand at insert time
(that's the pair `library_entries`' own `UNIQUE` constraint uses), so no
extra lookup/join is needed at write time. Joins back to `library_entries`
happen later, at read time, whenever something actually consumes this data.

**Scope: Steam only.** Confirmed by reading every importer
(`internal/importer/{steam,epic,gog,itch}.go`): only `steam.go` populates
`PlaytimeMinutes` (from the Steam Web API's `playtime_forever`, which is
cumulative — so diffing consecutive observations yields
minutes-played-in-the-interval for free, per approach.md). Epic/GOG/itch
importers don't currently read or set playtime at all, so writing
zero-value observations for them would be noise, not signal. If a future
importer starts exposing real playtime (e.g., from a launcher cache), it can
start writing observations the same way — the table isn't Steam-specific by
schema, just by what actually calls into it today.

**Write path, not read path.** This action ships the table and the write
(one `INSERT` per Steam import, alongside the existing upsert). It does
**not** build a diffing query or expose the history through any MCP tool —
there's no consumer yet (that's Phase 5b or whatever needs it next), and
building a read API speculatively ahead of a real consumer is exactly the
kind of premature structure to avoid. `SELECT ... WHERE source = 'steam' AND
source_game_id = ? ORDER BY observed_at` is trivial to add later once
something needs it.

## Non-goals (explicit, per 2026-07-26 constraint)

- **No telemetry, no cross-user aggregation, no export destination.** This
  is single-user, local-only data — the same db file already on the user's
  machine, nothing phoned home, nothing shared or aggregated across users.
  The user was explicit about this: comfortable building the spec against
  their own data, not comfortable with anything that deals in other people's
  data. This rules out, now and as a standing constraint on however Phase 5b
  or any future learning-loop work evolves: a shared/aggregate dataset,
  opt-in usage sharing, or any "learn from all users" framing.
- No changes to `library_entries`, `list_games`, `get_game`, installed-state
  logic, or any existing MCP tool's behavior/schema.
- No diffing query, no new MCP tool, no README/user-facing surface (this is
  invisible instrumentation — nothing to document until it's consumed).
- Phase 5b (conversational feedback capture) stays a separate, deferred
  action — not part of A162.

## Load-bearing contract

- Every import from Steam writes exactly one `library_observations` row with
  the current `playtime_forever` value, regardless of whether it changed
  since the last import (an unchanged reading is itself informative — it
  confirms the interval had zero play, not just missing data).
- Writing an observation must never block or fail the import itself — same
  posture as the `0600` chmod in A126 (best-effort, log-and-continue), since
  this is instrumentation, not a load-bearing part of the import's own
  correctness.
- No retroactive backfill — history starts at whatever moment this ships,
  same as any turn-on-now instrumentation.
