---
feature: itch-unkeyed-games
status: approved
created: 2026-07-06
author: claude
type: lite   # importer gap-fix; one real decision, flagged below
---

# Design (lite) — import free/unkeyed itch.io games (A107)

## Problem

The itch importer only sees games reachable from `download_keys`, but butler
creates **no** download-key row when a game is claimed free. Installed caves
for such games are invisible: Dr. Langeskov and Battle for Wesnoth (both
installed, both free) never enter the library — 15 installed in the itch app
vs 13 in gamesom, found verifying PR #19 (2026-07-06). Squarely
`constraints.md` #2 territory: installed-state accuracy, "what's actually
playable tonight".

## Decision

Second pass: `itchUnkeyedInstalledGames(butlerDB)` selects games that have a
**cave but no download key**, joined to butler's `games` table for title/URL,
keeping the `classification = 'game'` filter for parity with the keyed query.
`Itch()` appends its results to the keyed list; the existing import loop
(cave-path resolution, `os.Stat` install check, upsert) handles both
identically.

Kept as a **separate helper** rather than widening the keyed query with an
OR: the keyed/unkeyed distinction stays explicit in code, so if the owned
semantics below ever changes, it's a one-line flip at the call site — and
`itchOwnedGames` (A108) stays untouched.

## Owned semantics — PROPOSAL, ratify at review

**Claimed-free ≈ owned (`Owned: 1`), same as keyed games.** Rationale: itch's
own model puts claimed games in your library; "do I own Dr. Langeskov" is
yes in every everyday sense. A distinct flag would need a schema change the
problem doesn't justify — and the separate-helper structure means demoting
unkeyed games to a different flag later is a one-line change, not a
migration. Flagged in the PR body for Victor's explicit ok.

## Out of scope

Uninstalled free claims (butler has no record of them at all — nothing to
import), collections/wishlist, other importers, the e2e suite (A101).
