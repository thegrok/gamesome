---
feature: persona-config
status: approved
created: 2026-07-15
author: claude
action: A099
---

# Implementation — configurable, adaptive sommelier persona (A099)

Grounded at repo head (`main`, commit `712fc69`, 2026-07-15). Changes span
`internal/db/db.go`, `cmd/mcp.go`, two test files, and the MCPB manifest.

## Load-bearing contract

1. **`sommelierBriefing` stays byte-stable.** It is the baseline persona *and* the
   manifest-mirrored `sommelier` prompt text. Do not edit its content — the manifest
   mirror + `TestManifestPromptsMirrorServer` depend on byte-equality. The persona and
   protocol additions live in a **separate composed string**, never in this const.
2. **The two delivery surfaces now intentionally fork** (reversing the A117 no-fork
   rule): `ServerOptions.Instructions` = `composeInstructions(db)` (baseline + persona
   + protocol, dynamic per launch); the `sommelier` prompt = raw `sommelierBriefing`
   (baseline, static, manifest-mirrored). Record this as a comment on the const.
3. **Instructions are read at startup only.** `composeInstructions(db)` runs once, at
   `mcp.NewServer`. Persona changes take effect at the *next* launch. It must never
   fail the server: any DB error inside it falls back to the bare baseline.
4. **Pin-guard is structural, not just advisory.** An adaptive `update_persona` call
   (no `pinned` arg) cannot overwrite a `pinned = 1` row. This lives in the DB helper
   so it is unit-testable, not only in the tool handler.

## File-level changes

### A. `internal/db/db.go`

**A1. Add the `persona` table to the `schema` const** (it is `CREATE TABLE IF NOT
EXISTS`, so it is applied on the next `Open`/`OpenAt` for existing DBs too — no
`migrateEnrichColumns` entry needed; that path is for `ALTER TABLE ADD COLUMN`):

```sql
CREATE TABLE IF NOT EXISTS persona (
    dimension  TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    pinned     INTEGER NOT NULL DEFAULT 0,
    source     TEXT NOT NULL DEFAULT 'adaptive',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**A2. Meta key + sentinel error** (beside `MetaSteamAPIKey`/`MetaSteamID`):

```go
const MetaPersonaConfigured = "persona_configured"

// ErrPersonaPinned is returned when an adaptive update (no explicit pin decision)
// tries to overwrite a pinned dimension. The pin-guard for A099's drift guardrail.
var ErrPersonaPinned = errors.New("dimension is pinned; an explicit pin decision is required to change it")
```
(add `errors` to imports)

**A3. Persona type + helpers:**

```go
type PersonaDimension struct {
    Dimension string `json:"dimension"`
    Value     string `json:"value"`
    Pinned    bool   `json:"pinned"`
    Source    string `json:"source"`
}

// GetPersona returns all persona dimensions ordered by dimension. Empty slice = baseline.
func GetPersona(db *sql.DB) ([]PersonaDimension, error) { … SELECT dimension,value,pinned,source FROM persona ORDER BY dimension … }

// UpsertPersona writes one dimension. pinned==nil means an adaptive/undirected write:
// it is refused (ErrPersonaPinned) if the existing row is pinned. A non-nil pinned is a
// deliberate user-directed pin decision and may overwrite. source is 'user' when pinned
// is non-nil, else 'adaptive'. Always marks the persona configured.
func UpsertPersona(db *sql.DB, dimension, value string, pinned *bool) error {
    // 1. look up existing pinned state
    // 2. if existing pinned==1 && pinned==nil -> return ErrPersonaPinned
    // 3. newPinned := existing (if pinned==nil) else *pinned
    //    source    := "adaptive" (if pinned==nil) else "user"
    // 4. INSERT ... ON CONFLICT(dimension) DO UPDATE SET value,pinned,source,updated_at=CURRENT_TIMESTAMP
    // 5. SetMeta(db, MetaPersonaConfigured, "1")
}

// ResetPersona deletes one dimension (dimension != "") or all (dimension == "") and
// marks the persona configured (so a decline/reset doesn't re-trigger onboarding).
// Returns rows deleted.
func ResetPersona(db *sql.DB, dimension string) (int64, error) { … }

// PersonaConfigured reports whether the user has engaged persona setup at all.
func PersonaConfigured(db *sql.DB) bool { return GetMeta(db, MetaPersonaConfigured) == "1" }
```

`dimension` should be trimmed/lowercased-for-key? **No** — keep it as the model sends
it (open vocab); just `strings.TrimSpace`. Empty `dimension` or `value` after trim →
return a plain validation error from `UpsertPersona` (guards a malformed tool call).

### B. `cmd/mcp.go`

**B1. Server construction (line 45–46)** — compose instead of the bare const:

```go
s := mcp.NewServer(&mcp.Implementation{Name: "gamesome", Version: "v1.0.0"},
    &mcp.ServerOptions{Instructions: composeInstructions(database)})
```

**B2. `composeInstructions(db *sql.DB) string`** — baseline + persona section (if any)
+ protocol; onboarding sentence only when unconfigured & empty. DB errors → return bare
`sommelierBriefing`:

```go
func composeInstructions(db *sql.DB) string {
    rows, err := dbpkg.GetPersona(db)
    if err != nil { return sommelierBriefing }        // never fail the server over persona
    var b strings.Builder
    b.WriteString(sommelierBriefing)
    if len(rows) > 0 {
        b.WriteString("\n\n--- Your configured persona ---\n")
        b.WriteString("These preferences override the defaults above where they conflict:\n")
        for _, r := range rows {
            pin := ""; if r.Pinned { pin = " (pinned)" }
            b.WriteString(fmt.Sprintf("- %s%s: %s\n", r.Dimension, pin, r.Value))
        }
    }
    b.WriteString("\n\n")
    b.WriteString(personaProtocol)
    if len(rows) == 0 && !dbpkg.PersonaConfigured(db) {
        b.WriteString("\n\n")
        b.WriteString(personaOnboarding)
    }
    return b.String()
}
```

**B3. Two new text consts** (near `sommelierBriefing`): `personaProtocol` (always) and
`personaOnboarding` (conditional). Content per design.md §4/§3 — the confirm-gate, the
pin semantics, the "takes effect next session" note, and the one-time offer. Keep them
*out* of `sommelierBriefing` (contract #1).

**B4. Register `update_persona` + `reset_persona`** in `registerTools` (they need
`database`, which `registerTools` already has):

- `update_persona` — InputSchema `{dimension(req, string), value(req, string),
  pinned(bool, optional)}`. Handler: read args; `pinned` present → `&b`, else `nil`;
  call `db.UpsertPersona`; on `ErrPersonaPinned` return a `textResult` explaining the
  dimension is pinned (a conversational refusal, not a Go error — the model should relay
  it, not treat it as a crash); on other error return the error. Description pins the
  confirm-gate and the pin semantics.
- `reset_persona` — InputSchema `{dimension(optional, string)}`. Handler: call
  `db.ResetPersona`; return a `textResult` naming what was cleared (all vs one).
  Description names the decline/run-baseline role of the no-arg form.

**B5. `sommelier` prompt + manifest** — **unchanged**. It still serves
`sommelierBriefing`. Add a short comment on the const documenting the intentional fork
(contract #2) so a future reader doesn't "fix" the divergence.

Note the import: `cmd` already imports the db package as `"github.com/thegrok/gamesome/internal/db"`
(aliased `db` in-file). The pseudocode above says `dbpkg`/`db` — use the existing
in-file name (`db`), matching current usage like `db.SetMeta`.

### C. MCPB manifest — `packaging/mcpb/manifest.template.json`

Add two entries to the `tools` array (name + one-line description), mirroring the
install-time tool list to what the server registers. There is no tools byte-mirror
test (only prompts are validated), and tool *responses* are dynamic by nature, so
this is a display/consistency change, not a validation gate — but leaving packaging
out of scope is exactly what bit finding 006, so it is **in scope** here:

```json
{ "name": "update_persona", "description": "Record or update one dimension of the user's sommelier persona (confirm-gated)." },
{ "name": "reset_persona",  "description": "Clear the user's configured persona (one dimension or all) back to the baseline default." }
```

## Tests

- **`internal/db/db_test.go`** (add): `UpsertPersona` insert/update round-trip;
  `pinned` insert then adaptive (nil) overwrite → `ErrPersonaPinned`; directed
  (non-nil pinned) overwrite of a pinned row → succeeds; `GetPersona` ordering;
  `ResetPersona("")` clears all + `PersonaConfigured` true; `ResetPersona("dim")`
  clears one; empty dimension/value → validation error. Use `OpenAt(t.TempDir()+"/x.db")`
  (hermetic, per existing tests).
- **`cmd/mcp_persona_test.go`** (new): `composeInstructions` — empty persona ⇒ starts
  with `sommelierBriefing` and includes onboarding; configured/non-empty ⇒ includes the
  persona section + each dimension + "(pinned)" marker + protocol and **omits**
  onboarding; DB-error path ⇒ bare baseline. (Compose is the load-bearing surface;
  test it directly with a temp DB.)
- **`TestManifestPromptsMirrorServer`** must still pass unchanged (baseline const +
  single `sommelier` prompt untouched) — a regression check, not a new test.

## Sequencing

Single logical change; no data migration beyond the additive `CREATE TABLE IF NOT
EXISTS` (safe on existing DBs). Order: db.go (schema + helpers) → mcp.go (compose +
tools) → manifest → tests.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` (no repo `AGENTS.md`; standard Go
  gate). New unit tests above must pass; existing manifest-mirror test must stay green.
- **Agent-verifiable beyond compile:** `composeInstructions` and the pin-guard are
  pure/DB-level — covered by the unit tests above (real observable behavior, not just
  compile).
- **Manual (human, Claude Desktop), after rebuild/reinstall:**
  (a) fresh conversation, connector enabled, unconfigured persona → sommelier offers
  the opt-in interview once; declining and asking for a pick runs the baseline stance.
  (b) run the interview; confirm a dimension is recorded (pinned); restart the
  connector / new session → the tailored stance is ambient without invoking any prompt.
  (c) confirm-gate: prompt the model toward a stance change; it surfaces the read +
  asks before writing; on confirm it persists; a *pinned* dimension is not silently
  changed.
  (d) `reset_persona` (all) returns to baseline; the `sommelier` re-brief prompt still
  works (baseline) and is not rejected by Claude Desktop (manifest still mirrors).

## Scope boundary

**In:** the `persona` table + db helpers; `composeInstructions` + the two protocol
consts; the `update_persona` / `reset_persona` tools; the manifest `tools` entries;
the tests above; the intentional-fork comment on `sommelierBriefing`.

**Out:** any edit to `sommelierBriefing` *content* (byte-stable — contract #1); any
new prompt/resource; MCP elicitation; MCPB `user_config`; multi-user/profile support;
live mid-session re-read of instructions; docs outside
`docs/specs/persona-config/` and `docs/agent-logs/persona-config/`.
