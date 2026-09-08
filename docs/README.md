# docs/

Development history, not user documentation. **[The top-level README](../README.md)
is the documentation** — start there if you want to install or use gamesome.

| Directory | What's in it |
|---|---|
| `../.mind-harness/specs/` | Per-feature design and implementation notes, written *before* the code |
| `../.mind-harness/agent-logs/` | Per-feature work logs, written *after* — what was built, what was decided, how it was verified |

## How to read them

These were written during development and mirrored here from a private planning
vault, which stays canonical. Two consequences worth knowing before you open one:

**The `A###` identifiers won't resolve.** They refer to entries in that vault's
action list — `A102`, `A126` and so on — and there's nothing in this repo they
point at. They're kept because the notes cross-reference each other by ID and
stripping them would make the reasoning harder to follow, not easier.

**They are historical, not maintained.** A spec describes the intent at the time
it was written; the code is what actually shipped. Where the two disagree, the
code is right and the spec is simply old. Nothing here is updated to track
changes made afterwards.

## Why they're kept

They record *why* things are shaped the way they are, which the code can't tell
you on its own. The Epic importer tries three sources in a fixed order; the GOG
importer routes by platform rather than using one path everywhere; installed
state is tracked separately from ownership instead of being inferred. Each of
those was a deliberate call made against a specific problem, and the note that
made the call is still here.
