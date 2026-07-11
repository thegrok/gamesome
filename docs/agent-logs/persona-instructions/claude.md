# Work-log — persona-instructions (A117), Claude

Session: 2026-07-10, /feature lifecycle, Claude implemented directly.

## What changed (`cmd/mcp.go` only, per spec)

1. `mcp.NewServer` now receives `&mcp.ServerOptions{Instructions:
   sommelierBriefing}` instead of `nil`. Verified against the vendored
   go-sdk v1.6.1 source that `ServerOptions.Instructions` is copied into
   the `initialize` result (`mcp/server.go:62`, `:1477`) — options must be
   set at construction; there is no later setter.
2. Appended the WarGames-egg paragraph to `sommelierBriefing` ("One
   indulgence: … If in doubt, don't."). Kept the spec's constraints:
   rare, moment-fitting, default-off, no repeat instruction.
3. Deleted the `game` prompt registration (assistant-role "Shall we play
   a game?"); kept `sommelier` with its description updated to name its
   new role (re-brief; briefing is also ambient via server instructions).

## Load-bearing choices

- `sommelierBriefing` remains a single const feeding both surfaces
  (instructions + `sommelier` prompt) — the spec's no-fork contract.
- The egg paragraph landed at the end of the briefing, after the Steam
  section, exactly as specced; wording taken from the spec verbatim.

## Spec corrections

None — the spec matched repo head; all three edits applied as written.

## Verification

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — all packages pass (cmd 0.247s; db/importer/normalize cached).
- Not agent-verifiable: client-side behavior (instructions surfaced by
  Claude Desktop, prompt menu contents). Listed as manual steps in PR #29.

## Codex cross-model review

Ran 2026-07-10 (`codex exec -s read-only`, findings at `codex-review.md`
alongside this log). One **Low** finding, no code findings:

- *"claude.md work-log is outside the spec's scope boundary — scope creep
  if committed."* **Triaged: spec-wording defect, not a code defect.** The
  work-log is mandated by the feature lifecycle and named in PR #29's
  expected diff; the spec's "Out" line simply failed to carve out
  `docs/agent-logs/<slug>/`. Fixed the scope line in both the repo and
  vault copies of `implementation.md` rather than dropping the file. No
  code change; verification gate unchanged (still green).

## 2026-07-11 — human-verify failure + fix (manifest prompt mirror)

rc4 manual step (c) failed in Claude Desktop: invoking `sommelier` returned
*"content validation failed. Rejecting response to prevent potential prompt
injection."*

**Diagnosis**: Claude Desktop validates `prompts/get` responses against the
prompt `text` declared in the MCPB manifest at install time — the manifest
is what the user reviewed, so runtime content that differs is rejected as
potential injection. The A117 code change updated `cmd/mcp.go` only; the
manifest `prompts` block still carried the pre-egg briefing, the old
description, and the deleted `game` prompt (which also explains a phantom
`game` entry in the prompt menu, i.e. manual step (b) would fail too).
An earlier finding hypothesis (user-role instruction-shaped text) was
wrong — the same text passes fine once the mirror matches.

**Fix** (this branch):

- `packaging/mcpb/manifest.template.json` — sommelier `text` now includes
  the egg paragraph (byte-identical to `sommelierBriefing`), description
  updated to the re-brief wording, `game` prompt entry removed.
- `cmd/mcp.go` — prompt description extracted to
  `sommelierPromptDescription` const so the test can compare it.
- `cmd/mcp_manifest_sync_test.go` — new `TestManifestPromptsMirrorServer`
  pins the mirror invariant (prompt set, text, description); drift now
  fails `go test`.
- Spec scope amended (`implementation.md`): manifest `prompts` block is in
  scope and load-bearing, no-fork contract extended to it.

**Verification**: `go build` / `go vet` / `go test ./...` green including
the new test. Manual re-verify needed: rebuild bundle, reinstall, re-run
steps (a)–(c) — (c) is the one this fixes; (b) should now also show the
`game` prompt gone from the menu.
