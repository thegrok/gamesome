---
project: game-sommelier
feature: sommelier-layer
status: draft
kind: design-lite
created: 2026-07-04
---

# Sommelier layer in the MCP server — design note

## Problem

The product's namesake layer doesn't exist as a shipped artifact. `gamesom mcp`
exposes 5 tools + 1 resource (verified against `cmd/mcp.go` 2026-07-04):
`list_games`, `search_games`, `get_game`, `upsert_profile`, `mark_completed`,
`gamesom://library/summary`. Two things are missing:

1. **No import path in-conversation.** The only way to populate or refresh the
   DB is the CLI (`gamesom import <store>`). A Claude Desktop user — the actual
   release audience per the 2026-07-04 reframing — never opens a terminal.
2. **No persona.** The sommelier briefing (the system prompt in approach.md §
   "Claude session context") lives only in the vault. Nothing in the shipped
   binary carries the role, the anti-fantasy-self rule, or the fit-to-the-moment
   ethos.

The original Phase-4 vehicle — a `/sommelier` **Claude Code skill** — targets
the wrong audience for release: agent-savvy developers. The release audience is
gamers on Claude Desktop who will never install Claude Code. This design recasts
Phase 4 so the persona and the import both travel **inside the MCP server**,
which works for both audiences.

## Approach

### 1. `sommelier` MCP prompt

Register an MCP **prompt** on the server (the official
`modelcontextprotocol/go-sdk` — already the server's SDK — supports prompt
registration) carrying the briefing from approach.md. MCP prompts surface as
invokable commands in clients: `/mcp__gamesom__sommelier` in Claude Code;
via the +/connectors surface in Claude Desktop. One artifact, both audiences —
the Claude-Code-skill plan is superseded, not supplemented.

The prompt text is the approach.md briefing plus a first-move instruction:
check `gamesom://library/summary`; if the DB is empty or stale, offer
`refresh_library` before recommending.

### 2. `refresh_library` tool

A new tool wrapping the existing importers:

- **Auto-detects** which launchers are present on this machine (no source
  argument required; optional `sources` filter)
- Runs each detected importer; per-source failures are **non-fatal** (report
  and continue)
- Returns a human-readable summary Claude can relay: new games, updated
  playtime, per-source counts, what was skipped and why
- Errors are conversational and actionable ("Steam wasn't found — is it
  installed in a custom location?"), never raw Go error chains

Constraint 2 holds (constraints.md): the importer remains the sole source of
truth for ownership/installed state; this tool only moves *when* import runs,
not *who decides*.

### 3. Persona redundancy in tool descriptions

Clients vary in how prominently they surface MCP prompts, so condense the core
role rules ("judge fit to the moment, never ownership; don't push the
fantasy-self game; write trait inferences back via `upsert_profile`") into the
tool descriptions themselves. A Claude that never sees the prompt still absorbs
the ethos from the tools it reads.

## Scope

**In:** the prompt, `refresh_library` + launcher auto-detection, description
rewrites, first-run/empty-DB handling.

**Out:** MCPB packaging (own feature: mcpb-packaging), metadata enrichment
(Phase 3, stays lazy), the session-feedback learning loop (Phase 5),
local-LLM routing (A079).

## Constraints / honesty notes

- **stdout discipline** (findings/002): importers invoked from the MCP process
  must not print to stdout or they corrupt the JSON-RPC stream — audit for
  `fmt.Println` in the import path and route to stderr.
- Import inside a tool call means import *latency* inside a conversation turn;
  if a full multi-store import proves slow, return per-source progress in the
  result text rather than adding async machinery.
- Whether Claude Desktop's prompt surfacing is prominent enough for a
  first-time user is unverifiable until tried — the tool-description
  redundancy (§3) is the hedge, and the A095 bundle description is the other
  onboarding surface.
