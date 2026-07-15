# Work-log — persona-config (A099), Claude (implementer)

## What was built

Configurable + adaptive sommelier persona, per `docs/specs/persona-config/`.

- `internal/db/db.go`: new `persona` table in the schema const (additive
  `CREATE TABLE IF NOT EXISTS`, safe on existing DBs); `PersonaDimension` +
  `GetPersona` / `UpsertPersona` (pin-guard) / `ResetPersona` / `PersonaConfigured`;
  `MetaPersonaConfigured`, `ErrPersonaPinned`.
- `cmd/mcp.go`: `composeInstructions(db)` now feeds `ServerOptions.Instructions`
  (baseline + configured persona deltas + adaptation protocol + one-time onboarding);
  `personaProtocol` / `personaOnboarding` consts; `update_persona` + `reset_persona`
  tools; an intentional-fork doc comment on `sommelierBriefing`.
- `packaging/mcpb/manifest.template.json`: two new tool display entries.
- Tests: `internal/db/persona_test.go`, `cmd/mcp_persona_test.go`.

## Load-bearing choices

- **`sommelierBriefing` kept byte-stable.** Persona/protocol text lives only in the
  composed instructions string, never in the const — so the manifest mirror and
  `TestManifestPromptsMirrorServer` stay green (finding 006). The two delivery
  surfaces now intentionally fork (prompt = baseline; instructions = baseline +
  persona); documented in a comment on the const and in design.md.
- **Pin-guard is in the DB helper, not the handler**, so it's unit-testable.
  `UpsertPersona(_, _, _, pinned *bool)`: `pinned == nil` (adaptive/undirected)
  is refused with `ErrPersonaPinned` against a pinned row; `pinned != nil`
  (user-directed) may overwrite. The tool relays the refusal as a conversational
  `textResult`, not a JSON-RPC error, so the model asks the user rather than
  treating it as a crash.
- **`composeInstructions` never fails the server**: any DB error falls back to the
  bare baseline briefing.
- **`reset_persona` no-arg = decline/run-baseline path**, and both tools set
  `persona_configured` so onboarding stops nagging after any engagement.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` — all green.
- New unit tests pass; existing `TestManifestPromptsMirrorServer` still passes
  (baseline untouched).
- **End-to-end smoke test** (built binary, stdio JSON-RPC, temp `XDG_DATA_HOME`):
  - fresh DB → `initialize` instructions carry baseline + protocol + onboarding, no
    configured section; `tools/list` includes `update_persona` + `reset_persona`.
  - sequential tool calls: pinned write persists; an adaptive overwrite of the
    pinned dimension is refused (conversational message); an adaptive write to a new
    dimension succeeds.
  - a *fresh* launch reflects the persisted persona (pinned + adaptive dims rendered,
    `(pinned)` marker correct) and suppresses onboarding — confirming the
    "takes effect next launch" timing model.

## Codex cross-model review

Skipped — Codex quota exhausted ("You've hit your usage limit … try again at
Aug 3rd, 2026"). Best-effort/non-blocking per the /feature Stage 6 policy; no
subscription checkpoint applies. Verification was fully Claude-run (build/vet/test
+ end-to-end smoke test above).

## Residual / not in scope

- **`SQLITE_BUSY` under *concurrent* writes**: firing two `update_persona` calls
  without awaiting the first surfaced `database is locked`. Real MCP conversations
  issue tool calls sequentially (sequential probe is clean), and this is a
  pre-existing DB-config property (no `busy_timeout`, pooled connections) that
  affects every write path, not something persona introduces. Left as-is; a
  cross-cutting `busy_timeout`/`SetMaxOpenConns(1)` hardening is a possible
  follow-up, out of this feature's scope.
- Ambient delivery in Claude Desktop, the interview UX, and the confirm-gate
  behavior are client-side — human-verify steps (a)–(d) in implementation.md.
