---
feature: persona-config
status: approved
created: 2026-07-15
author: claude
type: full   # new schema + new tool surface + a behavioral protocol
action: A099
---

# Design — configurable, adaptive sommelier persona (A099)

## Problem

The `sommelier` persona shipped as a single static const (`sommelierBriefing`,
A117/`persona-instructions`): guilt-reduction, anti-fantasy-self, backlog-first.
That stance is **Victor's personal default, not a universal instruction** for the
release audience (see `intent.md` § "General use case vs personal default"). The
general product's core is *mood/context-fit library rediscovery*; guilt-reduction
is **one persona a user can opt into**, not the only one.

A099 makes the persona configurable and adaptive while keeping the confirmed
personal default as the baseline everyone gets by default.

## Prior art this builds on (do not re-derive)

- **Two delivery surfaces** exist for the briefing (A117): `ServerOptions.Instructions`
  (ambient, delivered in the `initialize` result) and the `sommelier` **prompt**
  (`prompts/get`, a manual re-brief).
- **The manifest mirror is load-bearing** (finding 006): Claude Desktop validates
  a prompt's runtime content against the MCPB manifest's declared `text` at install
  time; drift is rejected as prompt injection. `TestManifestPromptsMirrorServer`
  pins prompt ⇔ manifest byte-equality. **Server instructions are *not* in the
  manifest** (verified: no `instructions` key) — so dynamic content is safe there,
  and *only* there.
- **Instructions are captured once**, at `mcp.NewServer` construction (go-sdk copies
  `opts.Instructions` into the initialize result; there is no later setter). So the
  persona is read from the DB **at server startup** and composed into the briefing.

## Decisions

### 1. Delivery — the adaptive persona rides `ServerOptions.Instructions` only

The composed briefing (baseline + persona delta + adaptation protocol) is passed to
`ServerOptions.Instructions`. This channel is not manifest-validated, so a
per-launch-dynamic string is safe.

The manifest-mirrored `sommelier` **prompt stays baseline-only**: it keeps returning
the static `sommelierBriefing` const, byte-identical to the manifest, so
`TestManifestPromptsMirrorServer` still passes untouched. It remains the belt-and-
suspenders re-brief that finding 006 rebuilt — for the baseline persona.

**This deliberately forks the A117 "the two delivery surfaces must never fork"
contract.** The fork is intentional and one-directional: instructions = baseline +
persona (dynamic); prompt = baseline only (static). A frozen manifest cannot mirror
a moving target, so the re-brief carries the part that *is* frozen. This is recorded
as an intentional divergence in code (comment on `sommelierBriefing`) and here.

### 2. Substrate — a new dimension-keyed `persona` table

Persona lives in a **new per-user table**, distinct from the per-game
`sommelier_profile` (that table is game traits; this is user stance).

```sql
CREATE TABLE IF NOT EXISTS persona (
    dimension  TEXT PRIMARY KEY,   -- e.g. 'productivity_stance', 'session_preference'
    value      TEXT NOT NULL,      -- the stance, in prose
    pinned     INTEGER NOT NULL DEFAULT 0,
    source     TEXT NOT NULL DEFAULT 'adaptive',  -- 'user' (interview / directed) | 'adaptive'
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

- **Open vocabulary**: dimension keys are free text. The briefing lists preferred
  keys for coherence, but the model may coin new ones as it learns. This lets
  adaptation grow new dimensions with no schema/code change.
- **Baseline is not stored here.** The table holds *deltas on top of* the static
  baseline const. Empty table ⇒ pure baseline. This is what makes "skip → baseline"
  and "reset → baseline" both mean "clear the deltas."
- **Deltas override baseline** where they conflict (stated in the composed briefing).

### 3. Configuration — an opt-in interview, populated via a tool

There is **no dedicated interview command**. The interview is a conversational
protocol carried in the briefing that culminates in `update_persona` calls
(`pinned: true`). Onboarding is offered **once**, only when the persona is
unconfigured, and framed as an offer — never a form. Declining runs the baseline.

Onboarding re-offer is suppressed by a `meta` flag `persona_configured` (set on any
`update_persona` or `reset_persona` call). Once set, the auto-offer stops; the user
can still start the interview on demand ("set up my sommelier persona") any time.

### 4. Adaptation — model-initiated, in-band, confirm-gated, never silent

MCP is request-driven (no cron), so "periodically revise the persona" =
**model-initiated in-band write-back**: as the sommelier accumulates a read of the
user across a conversation, it may refine the persona. The guardrail (the decision
Victor most cares about) is that **adaptation is confirm-gated and never silent**:

1. When the sommelier forms a new read of the user, it **surfaces that read back as
   feedback** — valuable in its own right, a mirror on the user's own patterns —
2. **paired with a question**: record this to your persona?
3. The `persona` row is written (via `update_persona`) **only on confirm**.

Two layers enforce "must not erase the guardrail":

- **Behavioral**: the protocol text above, plus the `update_persona` tool description
  ("call only after explicit user confirmation").
- **Structural (pin-guard)**: a dimension with `pinned = 1` **cannot be changed by an
  adaptive write**. `update_persona` refuses to overwrite a pinned row unless the call
  passes the `pinned` flag explicitly (i.e. a deliberate, user-directed re-pin or
  unpin). Adaptive write-back omits `pinned`, so it physically cannot touch a pinned
  dimension. Interview-set dimensions are pinned, so the guilt-reduction stance (when a
  user pins it) is safe from an adaptive read of in-the-moment behavior — which is
  exactly the case where revealed preference runs *counter* to the guardrail.

Reset-to-baseline stays available: `reset_persona` clears one dimension or all
(the "all" form is also the decline path). Reset clears pinned rows too — it is an
explicit, user-initiated act.

**Timing (honest limitation):** because instructions are captured at startup, a
confirmed change persists immediately but only *reshapes ambient behavior at the next
connector launch*. Within the current conversation the model already holds the read in
context, so behavior is right now; persistence is what carries to future sessions. The
protocol text tells the model to set this expectation ("I'll apply that from next
session"). A new conversation opened before the server restarts won't see the change —
accepted, not worked around (no setter exists).

## Tool surface (two new tools)

- **`update_persona`** — `{dimension, value, pinned?}`. Upsert one persona dimension.
  Used by both the interview (`pinned: true`) and confirmed adaptive write-back
  (`pinned` omitted). Enforces the pin-guard. Sets `persona_configured`.
- **`reset_persona`** — `{dimension?}`. Clear one dimension, or all (omit `dimension`)
  back to baseline. No-arg form doubles as the "run baseline / decline interview" path.
  Sets `persona_configured`.

No new prompt, no new resource. The current persona is visible to the model because it
is composed into the ambient instructions at startup; it does not also need a getter.

## Non-goals

- No change to the baseline persona *content* (`sommelierBriefing` stays byte-stable —
  the manifest mirror depends on it).
- No MCPB `user_config` persona field (A095 branch) — mechanism is DB-stored, decided.
- No MCP elicitation / server-driven confirmation UI — the confirm-gate is behavioral +
  the structural pin-guard. (Elicitation is a possible future hardening; its go-sdk /
  Claude Desktop support is unverified and out of scope.)
- No cross-user / multi-profile support — the local DB is single-user; "per-user table"
  means one user's persona.
- No live re-read of instructions mid-session — accepted timing limitation above.
