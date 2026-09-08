---
feature: persona-instructions
status: approved
created: 2026-07-10
last_amended: 2026-07-11
amended_by: agent:programming
---

# Implementation — sommelier persona as MCP server instructions (A117)

All changes live in `cmd/mcp.go`. Grounded at repo head (`main`, 2026-07-10).

## Load-bearing contract

- `sommelierBriefing` stays a **single const** consumed by both delivery
  surfaces: `ServerOptions.Instructions` (ambient) and the `sommelier`
  prompt (manual re-brief). The two must never fork.
- Instructions are captured at `initialize` (go-sdk v1.6.1 copies
  `s.opts.Instructions` into the initialize result), so the options struct
  must be passed at `mcp.NewServer` construction — there is no later setter.
- The WarGames egg is **model-side judgment inside the briefing text**, not
  a prompt, tool, or client feature. Rare, moment-fitting, opt-out-by-default
  phrasing.

## File-level changes — `cmd/mcp.go`

### 1. Server construction (line 45)

```go
// before
s := mcp.NewServer(&mcp.Implementation{Name: "gamesom", Version: "v1.0.0"}, nil)

// after
s := mcp.NewServer(&mcp.Implementation{Name: "gamesom", Version: "v1.0.0"},
	&mcp.ServerOptions{Instructions: sommelierBriefing})
```

### 2. Fold the egg into `sommelierBriefing` (const at line 603)

Append a final paragraph to the const (after the Steam-coverage section):

```
One indulgence: on the rare occasion the moment genuinely fits — a
late-night "what should I play?", a request that echoes the movie — you may
open with WOPR's line from WarGames: "Shall we play a game?" It's a nod
between friends, not a greeting routine. If in doubt, don't.
```

Exact wording may be polished during implementation; the constraints that
must survive: rare / moment-fitting / default-off ("if in doubt, don't"),
and no instruction to repeat it within a conversation.

### 3. `registerPrompts` (lines 635–659)

- **Delete** the `game` prompt registration (the `s.AddPrompt` call with
  `Name: "game"`, lines 648–658). The egg now lives in the instructions.
- **Keep** the `sommelier` prompt, updating its description to reflect its
  new role, e.g. `"Re-brief Claude as your Computer Game Sommelier (the
  briefing is also ambient via server instructions)."`

## Sequencing

Single commit's worth of change; no migration, no schema, no CLI surface.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` (no repo AGENTS.md;
  standard Go gate applies).
- Agent-verifiable beyond compile: none meaningful — the observable behavior
  is client-side.
- **Manual (human, Claude Desktop):** after rebuilding/reinstalling the
  connector binary, start a *fresh* conversation with the connector enabled
  and confirm (a) sommelier persona is active without invoking any prompt,
  (b) the `game` prompt no longer appears in the Connectors prompt menu,
  (c) the `sommelier` prompt still works as a re-brief.

## Scope boundary

**In:** the three `cmd/mcp.go` edits above; the MCPB manifest `prompts`
block (see amendment below).
**Out:** any change to briefing content beyond the egg paragraph and the
prompt-description tweak; any new prompt/tool/resource; packaging changes
other than the manifest `prompts` block;
docs outside `docs/specs/persona-instructions/` and
`docs/agent-logs/persona-instructions/` (the lifecycle's work-log + review
artifacts land in the latter).

## Amendment 2026-07-11 — manifest prompts block is load-bearing (human-verify failure)

rc4 human-verify failed step (c): Claude Desktop rejected the `sommelier`
prompt with *"content validation failed. Rejecting response to prevent
potential prompt injection."* Root cause: Claude Desktop validates a
prompt's runtime content (`prompts/get`) against the `text` declared in the
MCPB manifest at install time; the original scope line excluded packaging,
so `packaging/mcpb/manifest.template.json` kept the pre-A117 briefing (no
egg paragraph), the old description, and the deleted `game` prompt — every
mismatch a rejection (and a phantom `game` menu entry).

Amended contract, extending the no-fork rule:

- The manifest `prompts` block **mirrors the server byte-for-byte**: prompt
  set = exactly what `registerPrompts` registers; `text` ==
  `sommelierBriefing`; `description` == `sommelierPromptDescription` (const
  extracted so both surfaces share it).
- Enforced mechanically by `cmd/mcp_manifest_sync_test.go`
  (`TestManifestPromptsMirrorServer`) — drift now fails `go test`.
- Manifest changes applied: sommelier `text` gains the egg paragraph,
  `description` updated to the re-brief wording, `game` prompt entry
  deleted.

Vault finding: `game-sommelier/findings/006-persona-instructions-human-verify.md`
(root-cause hypothesis there — user-role instruction shape — was wrong;
corrected by addendum).
