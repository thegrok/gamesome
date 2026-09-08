---
feature: mcpb-packaging
status: ready
created: 2026-07-06
author: claude
repo: git@github.com:thegrok/gamesom.git
base-branch: feature/mcp-env-dump   # stacked, PR 6 (top) of the 2026-07-06 run
---

# Implementation — MCPB one-click packaging (A095)

Design: `design.md` in this folder (2026-07-04, Victor-reviewed). This doc is
the build plan. **PR stays draft until the A097 Steam-onboarding decision** —
the `user_config` Steam fields below are the pending-A097 shape and may be
cut or reworded before merge.

## Load-bearing contract

Every GoReleaser release must emit one `gamesom_<version>_<os>_<arch>.mcpb`
per built binary, attached to the GitHub release beside the existing
archives: a zip whose root holds a valid `manifest.json`
(manifest_version 0.3, `server.type: "binary"`) and `server/gamesom(.exe)`.
Install = download → double-click in Claude Desktop. The existing archives,
checksums, and changelog behavior are untouched.

## Files changed

### `packaging/mcpb/manifest.template.json` (new)

Manifest per the MCPB spec (verified against
`modelcontextprotocol/mcpb/MANIFEST.md`, fetched 2026-07-06). Placeholders
`__VERSION__`, `__BIN__` (`gamesom` / `gamesom.exe`), `__PLATFORM__`
(`linux` / `darwin` / `win32`) are substituted by the build script — one
single-platform bundle per binary rather than one manifest with
`platform_overrides`, because the binaries are per-arch zips anyway.

Key fields:

- `description` is the onboarding surface (per design): what the sommelier
  does + "ask: what should I play tonight?". `long_description` carries the
  fuller store copy including the toggle-off/on troubleshooting line.
- `server.mcp_config`: `command: "${__dirname}/server/__BIN__"`,
  `args: ["mcp"]` — absolute via `${__dirname}`, never cwd-relative (cwd
  ambiguity is literally the A102 suspect).
- `user_config` (**pending A097**): `steam_api_key` (`sensitive: true` → OS
  keychain, optional) and `steam_id` (optional — the web-API path in
  `steam.go` needs both), wired into `mcp_config.env` as
  `${user_config.steam_api_key}` / `${user_config.steam_id}`. Left empty
  they substitute to empty strings, which `steam.go:34-46` already treats
  as skip-the-web-API.
- `tools` lists the six real tools (`list_games`, `search_games`,
  `get_game`, `upsert_profile`, `mark_completed`, `refresh_library`),
  `prompts` lists `sommelier` — descriptions match `cmd/mcp.go`.
- `compatibility.platforms`: the single bundle platform.

### `scripts/build-mcpb.sh` (new, POSIX sh)

`build-mcpb.sh <binary-path> <goos> <goarch> <version>`:

1. Map `windows→win32/gamesom.exe`, `darwin→darwin/gamesom`,
   `linux→linux/gamesom`; reject anything else.
2. Stage `dist/mcpb-stage/<goos>_<goarch>/` with `server/<bin>` (cp keeps
   the exec bit) + `manifest.json` from the template via `sed`.
3. Zip stage-root-relative into `dist/mcpb/gamesom_<version>_<goos>_<goarch>.mcpb`
   — `zip -qr` when available (ubuntu-latest CI), else a `python3` zipfile
   fallback that preserves file modes (keeps the script runnable in
   sandboxes without zip).

### `.goreleaser.yaml`

- `builds[0].hooks.post`:
  `./scripts/build-mcpb.sh "{{ .Path }}" "{{ .Os }}" "{{ .Arch }}" "{{ .Version }}"`
  — runs once per built binary with GoReleaser's artifact template fields.
- `release.extra_files`: glob `dist/mcpb/*.mcpb` so bundles attach to the
  GitHub release.
- `release.footer`: install instructions ("download the `.mcpb` for your
  platform, double-click / Settings → Extensions → Install Extension; to
  update, download the new bundle — privately-distributed bundles don't
  auto-update; if tools wedge, toggle the extension off/on").

## Integration points

- A096 (PR #22, below in the stack): bundle users get
  `%LOCALAPPDATA%\gamesom` from day one — the design's deliverable 3.
- Sommelier layer (merged, A094): the persona travels in the bundle via the
  `sommelier` prompt — the design's gate, cleared.
- No Go code changes at all; `gamesom mcp` is already the entry point.

## Sequencing

PR 6 (top) of the 2026-07-06 stack, based on `feature/mcp-env-dump`
(PR #24). **Stays draft** until: (1) A097 decides the Steam onboarding
story → fields confirmed/cut; (2) a real tag exercises the pipeline. Merge
order: #20 → #21 → #22 → #23 → #24 → this.

## Verification

- Sandbox: build a linux binary, run `scripts/build-mcpb.sh` against it,
  then assert — zip lists `manifest.json` + `server/gamesom`, manifest
  parses as JSON with no `__PLACEHOLDER__` remnants, binary mode preserved.
- `goreleaser check` on the config if installable here; otherwise the hook
  syntax is verified against GoReleaser docs and the real check is the
  first tag push (CI has goreleaser + zip).
- What can't be verified here: an actual double-click install in Claude
  Desktop (Victor, on the Windows box, post-A097) and a real
  `goreleaser release` (needs the tag). Hence draft.

## Scope boundary

Packaging only: template + script + GoReleaser wiring. No Connectors
Directory submission (post-alpha), no code signing, no auto-update
machinery, no README rewrite (A069), no Go code changes.
