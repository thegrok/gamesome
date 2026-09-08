---
feature: itch-key-dedupe
status: approved
created: 2026-07-06
author: claude
type: lite   # straight bugfix — no new system/UX/schema
---

# Design (lite) — dedupe itch download keys in the import summary (A108)

## Problem

Butler's `download_keys` table can hold multiple keys for the same game (direct
purchase + bundle grant — game_id 118243 has 2, confirmed on-machine
2026-07-06). `Itch()`'s import loop iterates key **rows**, so the same game
increments `imported`/`installedCount` once per key, while the
`(source, source_game_id)` upsert correctly collapses to one library row. The
printed summary ("13 installed") disagrees with the DB (12).

## Decision

Dedupe at the query: `SELECT DISTINCT dk.game_id, g.title, g.url`. All three
columns come from the single `games` row joined by `game_id`, so duplicate key
rows are column-identical and DISTINCT collapses them exactly — no Go-side
seen-map needed.

Ride-along refactor: extract the query into `itchOwnedGames(butlerDB) →
[]itchGame` so it has a unit-test seam (fixture butler DB in a temp file — the
driver is pure Go, runs in any sandbox). The `itchDownloadKey` wrapper struct
adds nothing over `itchGame` and is dropped. This is also the seam
A107 (free/unkeyed itch games) will build beside.

## Out of scope

Unkeyed caves (A107), owned-semantics for claimed-free games (A107), any other
importer, the e2e suite (A101).
