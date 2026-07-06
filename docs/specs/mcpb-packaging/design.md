---
project: game-sommelier
feature: mcpb-packaging
status: draft
kind: design-lite
created: 2026-07-04
---

# MCPB one-click packaging — design note

## Problem

The current install path for a Claude Desktop user is: download a binary, put
it on PATH, hand-edit `claude_desktop_config.json`, fully restart the app
(including the non-obvious tray-quit). Live evidence from 2026-07-04: Victor —
a senior engineer — needed multiple restart cycles, hit the tray-icon gotcha,
and had the npx-spawned filesystem servers wedge mid-session twice. The release
audience (gamers who are not agent-savvy) will not survive this path, and the
known Windows MSIX bug where "Edit Config" opens a different
`claude_desktop_config.json` than the app reads makes it actively hostile.

## Approach: ship `gamesom.mcpb`

Package the server as an **MCP Bundle** — facts verified 2026-07-04 against
the MCPB repo/docs and Anthropic's documentation:

- An `.mcpb` is a zip containing the MCP server + a `manifest.json`; it
  installs in Claude Desktop with a single click (or Settings → Extensions →
  Install Extension), like a browser extension — no terminal, no JSON, no
  restart ritual
- **Binary servers are supported** (alongside Node/Python); a static Go binary
  needs no bundled runtime at all
- `user_config` in the manifest auto-generates a settings UI; fields marked
  `"sensitive": true` are encrypted via the OS keychain (Credential Manager on
  Windows)
- The format now lives in the MCP project itself (`modelcontextprotocol/mcpb`),
  works across Claude Desktop, Claude Code, and MCP for Windows
- Distribution: attach per-platform `.mcpb` files to GitHub releases;
  privately-distributed bundles do **not** auto-update (directory-installed
  ones do) — release notes must say "download the new bundle to update"

### Deliverables

1. **`manifest.json`** — name/description (the description is an onboarding
   surface: say what the sommelier does and that the first ask should be
   "what should I play tonight?"), platform binary entry points, and
   `user_config` with `steam_api_key` (optional, sensitive; pending the
   A097 decision)
2. **GoReleaser integration** — emit one `.mcpb` per OS/arch alongside the
   existing archives (`mcpb pack`, or a plain zip assembled to the MANIFEST.md
   spec — the format is just zip + manifest)
3. **Platform-idiomatic data dir** (A096) —
   `dataDir()` currently hardcodes XDG/`~/.local/share/gamesom`, which on
   Windows lands at `C:\Users\<u>\.local\share\gamesom` (verified in
   `internal/db/db.go` 2026-07-04). Move to `%LOCALAPPDATA%\gamesom` (Windows)
   / `~/Library/Application Support/gamesom` (macOS), keep XDG on Linux and the
   `XDG_DATA_HOME` override everywhere, and read-or-migrate the legacy path so
   existing DBs survive. Bundle users should never see a Unix dotpath on
   Windows.

## Scope

**In:** manifest, GoReleaser artifact, data-dir fix, release-page install
instructions ("download, double-click, ask Claude what to play").

**Out:** Connectors Directory submission (post-alpha follow-up — MCPB is the
accepted secondary path for the directory; remote connectors are the preferred
listing form), code signing, a remote-connector variant, auto-update machinery.

## Constraints / honesty notes

- Gate on sommelier-layer:
  a bundle that installs in one click but exposes only raw DB tools ships the
  friction problem one layer deeper. The persona travels in the bundle.
- MCPB install ≠ MCPB works: the local-MCP failure modes observed today
  (server wedging, tool calls timing out) are client/runtime-side and won't be
  fixed by packaging — the README needs a "toggle the extension off/on"
  troubleshooting line.
- Sequencing vs A081 (v0.1.0 alpha cut) is
  Victor's call: the alpha can stay CLI-shaped with MCPB as v0.2, or the alpha
  waits for this — the second option makes the first public impression the
  one-click path.
