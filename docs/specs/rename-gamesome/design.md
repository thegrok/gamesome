---
feature: rename-gamesome
status: approved
created: 2026-07-10
author: claude
type: lite   # mechanical string rename + one migration hop; churn already enumerated in A112
---

# Design (lite) — rename gamesom → gamesome (A112)

## Problem

"gamesom" was a working name; "gamesome" reads better. The name shows up in
five places that need to move together: the Go module path, the CLI/binary
name, the MCPB bundle/manifest, the platform data directory + db filename,
and (Victor's manual step) the GitHub repo itself. A112 sequences this before
the v0.1.0 alpha cut (A081) so the release and README (A069) never mention
the old name.

## Decisions

**Vault project identity stays `game-sommelier`.** The mind-harness project
folder, manifest, and action tags are not renamed. Product branding (repo,
binary, module) and vault project identity are allowed to diverge — realigning
the vault folder breaks existing wiki-links across ~20 spec/finding files for
no functional benefit. (This closes the "decide whether that's fine or worth
aligning" question A112's action text left open.)

**Data dir gets a second migration hop, not a straight rename.** A096
(shipped 2026-07-06) already introduced one hop: pre-A096 hardcoded XDG path
→ A096 idiomatic-per-OS path (folder still named `gamesom`). This rename adds
a second hop on top: A096 idiomatic path (`gamesom`) → new idiomatic path
(`gamesome`). `resolveDBPath` generalizes from two candidate dirs (`dir`,
`legacyDir`) to a `dir` + ordered list of legacy candidates, checked newest
→ oldest, so a DB sitting at any of the three historical locations survives.
DB filename moves too: `gamesom.db` → `gamesome.db`.

**Everything else is a literal string rename**, no back-compat concern
because nothing else is persisted or externally addressed:
- Go module path (`go.mod` + all internal imports)
- CLI (`cmd.Use`), binary name, `version`/`status` command output strings
- MCP server `Implementation.Name`, the `gamesom://library/summary` resource
  URI (not persisted anywhere — recomputed fresh each read), log/help strings
  that reference the `gamesom` command by name
- MCPB `manifest.template.json` (`name`, `repository.url`), `build-mcpb.sh`
  (binary name, output filename), `.goreleaser.yaml` (ldflags path,
  explicit `project_name`/`binary` so archive/binary names don't depend on
  the local clone directory's name, which stays `gamesom` on disk)

**GitHub repo rename is Victor's manual step, sequencing-independent.**
GitHub redirects the old URL, so it can happen before or after this PR
merges. Recommend before the v0.1.0 tag so release links are live under the
final name, but it doesn't block the code changes.

## Out of scope

- Historical spec/agent-log files under `docs/specs/*` and
  `docs/agent-logs/*` — immutable records of what was built when it was
  still called gamesom; not touched.
- `packaging/mcpb/manifest.template.json`'s stray `game` prompt entry (a
  pre-existing drift from A117 cutting the `game` prompt in `cmd/mcp.go` but
  not updating the static manifest template) — out of A112's scope, flagged
  separately.
- README (A069) and asciinema demo (A080) — written under the final name,
  not touched here since they don't exist yet.
