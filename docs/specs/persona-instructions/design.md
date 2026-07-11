---
feature: persona-instructions
status: approved
created: 2026-07-10
author: claude
type: lite   # wiring + text fold; the two open decisions were settled at design gate
---

# Design (lite) — ship the sommelier persona as MCP server instructions (A117)

## Problem

The sommelier briefing (`sommelierBriefing`, `cmd/mcp.go:603`) only reaches
Claude when the human manually invokes the `sommelier` prompt from the
Connectors menu — a buried surface nobody reaches for mid-conversation. In
practice most gamesom conversations run without the persona: Claude sees the
tools but not the briefing, so the anti-fantasy-self / guilt-reduction stance,
the trait write-back habit, and the Steam-coverage onboarding cues are absent
unless the human remembers the prompt.

MCP has a first-class fix: server **instructions**, delivered in the
`initialize` result and treated as ambient system-level context by clients.
go-sdk v1.6.1 supports it (`ServerOptions.Instructions`, verified against the
vendored source 2026-07-09 and re-verified at spec time); gamesom currently
passes `nil` options to `mcp.NewServer`.

## Decision

1. **Pass the briefing as server instructions.** `mcp.NewServer(impl,
   &mcp.ServerOptions{Instructions: sommelierBriefing})` in `cmd/mcp.go`.
   The persona becomes ambient in every conversation where the connector is
   enabled — no menu interaction required.
2. **Fold the WarGames egg into the briefing text as model-side judgment.**
   Rarely, when the moment genuinely fits (e.g. a late-night "what should I
   play?"), Claude may open with WOPR's "Shall we play a game?" — a nod, not
   a routine. No new prompts; the /game→/wopr restructure idea (A116) stays
   discarded.
3. **Cut the `game` prompt** (assistant-role "Shall we play a game?",
   `cmd/mcp.go:648-658`). With the egg ambient as judgment, a buried
   menu-triggered duplicate has no job left. (Settled at design gate,
   2026-07-10.)
4. **Keep the `sommelier` prompt as an explicit re-brief.** Instructions can
   fade over a long conversation or be dropped by clients that don't surface
   them; a manual re-brief is cheap insurance. Its description is updated to
   reflect the new role (re-brief, not sole delivery). (Settled at design
   gate, 2026-07-10.)

## Non-goals

- No change to the briefing's substantive content beyond the egg fold — the
  persona text itself (backlog-first stance, trait write-back, Steam
  onboarding walkthrough) ships as-is.
- No new prompts, tools, or resources.
- No client-side configuration changes; this rides the MCP initialize
  handshake.
