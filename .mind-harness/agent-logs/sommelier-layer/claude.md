# sommelier-layer — work log

Implemented by **Claude** (Codex unavailable), per `docs/specs/sommelier-layer/`.

## What was built

1. **`internal/importer/detect.go` — new, `DetectedSources() []string`.** Presence
   check for each launcher, reusing the importers' own unexported path resolvers
   (`steamAppsDirectories`, `itchConfigDir`, `heroicConfigDir`, `gogGalaxyDBPath`,
   `epicManifestsDir`) plus `os.Stat`, since none of the importers themselves
   distinguish "not installed" from "installed but empty/failed."

2. **`cmd/mcp.go` — `refresh_library` tool.** Auto-detects launchers (or accepts an
   explicit `sources` filter), runs each detected importer under a stdout-capture
   wrapper, diffs `library_entries` row counts per source before/after to report
   `new_games`, and returns a human-readable summary plus a structured per-source
   JSON result. Per-source failures are non-fatal and get a conversational error
   message (unmatched errors fall back to the raw Go error rather than a guessed
   friendly string).

3. **`cmd/mcp.go` — `captureStdout` helper.** Swaps `os.Stdout` to a pipe for the
   duration of a single importer call and returns what it wrote, so the importers'
   existing `fmt.Println`/`fmt.Printf` progress lines don't corrupt the MCP stdio
   JSON-RPC stream (findings/002's stdout-discipline rule). Verified safe against
   `go-sdk@v1.6.1`: `mcp.StdioTransport.Connect` captures its own `*os.File`
   reference to stdout at server-start time, so reassigning the package-level
   `os.Stdout` variable mid-call doesn't affect what the transport writes to.

4. **`cmd/mcp.go` — `sommelier` MCP prompt.** `registerPrompts(s)`, called from
   `mcpCmd.RunE` alongside `registerTools`/`registerResources`. Static text (no
   arguments): the approach.md "Claude session context" briefing verbatim, plus a
   first-move instruction to check `gamesom://library/summary` and offer
   `refresh_library` if it looks empty/stale.

5. **Persona redundancy in tool descriptions.** Appended one line each to
   `list_games`, `search_games`, `get_game` (fit-to-the-moment / ownership-is-
   authoritative) and `upsert_profile` (what it's for vs. raw metadata).
   `mark_completed` left as-is — plain state mutation, no persona rule applies.

## Load-bearing choices

- **Source key is `itchio`, not `itch`.** `internal/importer/itch.go` writes
  `library_entries.source = "itchio"`. The `windows-e2e` work-log flagged this
  exact trap already; caught it here before committing rather than after. All of
  `DetectedSources()`, the `refresh_library` input schema enum, the
  `knownImportSources` dispatch map, and `friendlyImportError` use `itchio`.
- **Presence detection lives in `internal/importer`, not `cmd`.** The path
  resolvers it needs (`steamAppsDirectories`, `itchConfigDir`, etc.) are
  unexported; `DetectedSources()` is the only new exported surface.
- **`captureStdout` is a process-wide `os.Stdout` swap, scoped to one importer
  call.** Flagged (not solved) as a theoretical race if two tool calls overlap —
  MCP clients call tools serially in practice, so no mutex was added.
- **`GOG(db)` dispatches to `HeroicGOG` on Linux, `GOGGalaxy` elsewhere** — this is
  existing behavior in `internal/importer/heroic_gog.go`, unchanged; `refresh_library`
  just calls `importer.GOG` and lets it pick.

## Verification (sandbox — no launchers installed)

- `go build ./...` ✓
- `go vet ./...` ✓
- `go test ./...` ✓ (no existing tests broken)
- Ad-hoc sanity check (not committed): `DetectedSources()` returns `[]` when
  `HOME` points at an empty temp dir — confirms the presence checks correctly
  report "nothing found" rather than false-detecting.

## Not verifiable here

- Actually running `refresh_library` against a real Steam/itch/GOG/Epic install —
  none present in this sandbox.
- MCP prompt surfacing in an actual Claude Desktop client (the `+`/connectors UI).
- Whether `captureStdout`'s pipe-and-goroutine approach holds up under a very
  large import (e.g. a first-run Steam scan with thousands of manifests) — the
  buffered read should handle any realistic output size, but hasn't been stress
  tested.
