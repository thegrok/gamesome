---
project: game-sommelier
feature: sommelier-layer
status: draft
kind: implementation
created: 2026-07-04
grounded-at: 11f46bb   # origin/main head when spec written
---

# Sommelier layer in the MCP server — implementation

Grounded in `cmd/mcp.go`, `cmd/import.go`, `internal/importer/*.go` at `origin/main`
(11f46bb). Three changes, all inside `cmd/mcp.go` plus one new importer-package helper.

## Change 1 — `refresh_library` tool

New tool added in `registerTools` (`cmd/mcp.go`), alongside the existing five.

**Auto-detection.** None of the four importers currently expose a "is this launcher
present" check — they either silently no-op (Steam: `steamManifestScan` globs and
just returns 0/0 if the dir is absent, `Steam()` always returns `nil`) or surface an
absence as a generic error (`Itch`/`HeroicGOG`/`GOGGalaxy`/`Epic` open a path that
doesn't exist and return `fmt.Errorf(...)`). That asymmetry means "not detected" and
"detected but failed" aren't distinguishable from the importer's return value alone.
So detection is a **separate pre-check** using each importer's existing (unexported)
path resolver, stat'd for existence before deciding whether to call it:

| Source | Presence check |
|--------|-----------------|
| steam  | any dir in `steamAppsDirectories()` exists (`os.Stat`) |
| itch   | `itchConfigDir()/db/butler.db` exists |
| gog    | linux: `heroicConfigDir()/store_cache/gog_library.json` exists; else `gogGalaxyDBPath()` exists |
| epic   | `epicManifestsDir()` exists |

These resolvers are unexported in `internal/importer`, so the presence check itself
must live in that package (new file `internal/importer/detect.go`) and export a
`DetectedSources() []string` that `cmd/mcp.go` calls. Reuses the resolvers as-is — no
behavior change to the importers.

**Handler logic** (`refresh_library` in `cmd/mcp.go`):

1. Read optional `sources` arg (`[]string`); if empty, use `importer.DetectedSources()`.
2. Validate any explicit `sources` entries against the known set (`steam`, `itch`,
   `gog`, `epic`); unknown names go into the summary as `"unknown source"` errors,
   not silently dropped.
3. For each source in the resolved list:
   - `before := countBySource(db, source)` — inline `SELECT COUNT(*) FROM
     library_entries WHERE source = ?` (no new `db` package helper needed; matches
     the existing inline-SQL style already used in `registerTools`).
   - Run the importer under stdout capture (Change 2): `Steam(db)`, `Itch(db)`,
     `GOG(db)`, `Epic(db)`.
   - `after := countBySource(db, source)`.
   - Record `{source, ok: err == nil, new_games: after - before, message}` — on
     success `message` is the captured stdout (already a human-readable one-liner
     per importer, e.g. `"Imported 12 games from itch.io (4 installed)"`); on
     error, `message` is a conversational wrap of `err.Error()` (see below), never
     the raw wrapped Go error chain.
4. Failures are **non-fatal** — continue to the next source; the tool call itself
   only returns an error if *zero* sources could even be attempted (e.g. explicit
   `sources` list was entirely unknown names).
5. Return `jsonResult` of the per-source array **plus** a synthesized human-readable
   summary string as a second `TextContent` block (or lead with the summary line and
   follow with the JSON — match whichever shape `list_games` etc. use; here it's a
   single `jsonResult`, so prepend a `"summary"` string field to the returned object
   rather than a second content block, for consistency with the rest of the file).

**Conversational error wrap.** A small lookup from known Go error substrings to a
human line, e.g.:
- itch: `"open ... butler.db"` → `"itch.io wasn't found — is it installed?"`
- gog (linux): `"open ... gog_library.json"` → `"GOG wasn't found via Heroic — install games through Heroic first."`
- epic: `"no games found"` / manifest dir missing → `"Epic Games Launcher wasn't found — is it installed?"`
- steam: Steam never errors today, so no wrap needed; if `steamAPIImport` warns
  (logged via `log.Printf`, not returned), that's already swallowed as non-fatal —
  leave as-is, don't plumb it through as a tool-visible warning (out of scope; would
  require changing `Steam()`'s signature).

Fallback for any unmatched error: `err.Error()` as-is rather than inventing a message
for a case not seen in testing — better an honest raw string than a wrong friendly one.

## Change 2 — stdout capture helper

**Why it's needed:** `server.ServeStdio`-equivalent in this SDK
(`mcp.StdioTransport`) owns the process's stdout for JSON-RPC framing (findings/002).
Every importer prints human-readable progress via `fmt.Println`/`fmt.Printf`
(`steam.go:46,52,55`; `itch.go:162`; `gog.go:137`; `heroic_gog.go:104`;
`epic.go:93,102,156,214,258`; `legendary.go` prompts). Calling these directly from a
tool handler would write raw text into the stdio stream and corrupt the client's
JSON-RPC parsing.

**Verified safe to swap `os.Stdout` globally:** the SDK's `StdioTransport.Connect`
(`go-sdk@v1.6.1/mcp/transport.go:105`) captures the `*os.File` pointer once, at
server startup (`nopCloserWriter{os.Stdout}`), into the connection object it uses for
all subsequent writes. Reassigning the package-level `os.Stdout` variable afterward
does not change what that connection writes to — it already holds the original file
descriptor. So a scoped swap-capture-restore around a single importer call is safe
*with respect to the transport*.

**Helper** (new, `cmd/mcp.go` or a small `internal/importer` export — keeping it in
`cmd` since it's an MCP-process concern, not an importer concern):

```go
// captureStdout runs fn with os.Stdout temporarily redirected to a pipe, returning
// whatever fn wrote to stdout. The MCP stdio transport already holds its own
// reference to the original stdout file descriptor (captured at Connect time), so
// this swap does not affect JSON-RPC framing.
func captureStdout(fn func() error) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("create pipe: %w", err)
	}
	orig := os.Stdout
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fnErr := fn()

	os.Stdout = orig
	w.Close()
	captured := <-outCh
	r.Close()
	return strings.TrimSpace(captured), fnErr
}
```

**Scope limitation to flag, not solve:** this is a process-global swap. If two
`refresh_library` calls (or a `refresh_library` call overlapping any other tool
handler that writes to stdout — none currently do) run concurrently, their captures
could interleave or clobber each other's `os.Stdout` pointer. In practice MCP clients
issue one tool call at a time and wait for the response, so this is a theoretical
gap, not an observed one — **out of scope to add a mutex for** unless it's actually
hit. `log.Printf` calls (e.g. `steam.go:43`) already go to stderr by default and need
no handling.

## Change 3 — `sommelier` MCP prompt

New `registerPrompts(s *mcp.Server)` function in `cmd/mcp.go`, called from `mcpCmd`'s
`RunE` alongside `registerTools`/`registerResources`.

```go
s.AddPrompt(&mcp.Prompt{
	Name:        "sommelier",
	Description: "Brief Claude as your Computer Game Sommelier for this session.",
}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{{
			Role:    "user",
			Content: &mcp.TextContent{Text: sommelierBriefing},
		}},
	}, nil
})
```

`sommelierBriefing` is a package-level string constant: the approach.md § "Claude
session context" text verbatim, plus a first-move instruction appended:

> Before recommending, check the `gamesom://library/summary` resource. If it's empty
> or looks stale, offer to run `refresh_library` before making a recommendation.

No arguments needed (`Prompt.Arguments` stays nil) — the briefing is static text, not
templated.

## Change 4 — persona redundancy in tool descriptions

Per design §3, condense the role rules into the five existing tool descriptions so a
client that never surfaces the prompt still gets the ethos. Edits are description-string
only, no behavior change:

- `list_games` / `search_games` / `get_game`: append "Fit-to-the-moment judgment only
  — installed/owned state here is authoritative; never infer ownership beyond what
  this returns."
- `upsert_profile`: append "Use this to persist sommelier judgments (energy, friction,
  session fit) you infer during a conversation, not raw game metadata."
- `mark_completed`: no persona-relevant rule to add (it's a plain state mutation);
  leave as-is.

## Scope boundary

**In:** `refresh_library` tool, `internal/importer/detect.go` (`DetectedSources`),
stdout-capture helper, `sommelier` prompt, description edits on 4 of 5 existing tools.

**Out (per design.md):** MCPB packaging (A095), metadata enrichment (Phase 3),
session-feedback learning loop (Phase 5), local-LLM routing (A079). Also out:
plumbing Steam API warnings through as tool-visible text, a mutex around
`captureStdout` (see Change 2), any change to `legendary.go`'s interactive install
prompt (`refresh_library` should skip/not-invoke the Epic-via-Legendary path if it
would block on a prompt — **note only**: `Epic()` calls into
`epicFromManifests`/`epicFromHeroicCache`, not the interactive Legendary path
directly, per `epic.go:93` gating on Legendary being found; confirm this doesn't
block during Stage 6 verification, don't assume).

## Verification plan

- `go build ./...`, `go vet ./...`, `go test ./...` (no existing tests should break;
  no new unit tests are load-bearing enough to require here — `refresh_library`'s
  correctness is "did it call the right importers," which the existing importer
  tests already cover; a thin test can assert `DetectedSources()` returns `[]` on a
  box with `HOME`/`APPDATA` pointed at an empty temp dir, if that's cheap).
- **Cannot verify in this sandbox:** actually driving `refresh_library` end-to-end
  against a real launcher install (no Steam/itch/GOG/Epic present here) — that's a
  human pass with a real library, same caveat as `windows-e2e`. Also cannot verify
  MCP prompt surfacing in an actual Claude Desktop client from this sandbox.
