---
feature: readme
status: approved
created: 2026-07-25
author: claude
type: implementation
actions: A069
---

# Implementation — gamesome README (A069)

Ground truth pulled from repo head `aa501b8` (2026-07-25): `cmd/root.go`,
`cmd/import.go`, `cmd/enrich.go`, `cmd/status.go`, `cmd/mcp.go`,
`packaging/mcpb/manifest.template.json`, `scripts/build-mcpb.sh`,
`.goreleaser.yaml`, and the as-built specs listed in design.md.

## File

Single new `README.md` at repo root. No other files change (the stale
`.mcp.json` is flagged in the work-log, not touched — out of scope).

## Section-by-section content plan

1. **Title + one-line hook.** "Game Sommelier" / `gamesome` — pick what to
   play tonight from the backlog you already own, not a universally-good-games
   list. Pull the hook from the MCPB manifest's `description` field (already
   decided copy) rather than writing new marketing language.

2. **What it does (short).** Imports your library (Steam/GOG/Epic/itch) into
   a local SQLite db, tracks installed state, and briefs Claude as an MCP
   server that judges fit-to-moment instead of "objectively good" recs.

3. **Feature list.**
   - Cross-platform importers: Steam, Epic (Heroic → Legendary → EGL
     fallback chain), GOG (Linux: Heroic nile cache; Windows/macOS: GOG
     Galaxy SQLite direct), itch.io (butler db, resolves custom install
     locations)
   - Installed-state accuracy (not just ownership)
   - MCP server (`gamesome mcp`, stdio transport) — 9 tools: `list_games`,
     `search_games`, `get_game`, `upsert_profile`, `mark_completed`,
     `set_steam_credentials`, `refresh_library`, `update_persona`,
     `reset_persona` — plus a `sommelier` re-brief prompt and a
     `gamesome://library/summary` resource
   - Configurable, adaptive sommelier persona (dimension-keyed, confirm-gated
     — A099)
   - One-click Claude Desktop install via `.mcpb` bundle

4. **Install** — three paths, as a short table + one paragraph each:

   | Situation | Get |
   |---|---|
   | Claude Desktop, Windows/macOS | `.mcpb` bundle — double-click, or Settings → Extensions → Install Extension |
   | Linux, or any non-Desktop MCP client (Claude Code, etc.) | `tar.gz`/`zip` binary — extract, put on PATH or reference by full path |
   | Building from source | `go build` |

   State plainly: **no Claude Desktop on Linux**, so Linux always takes the
   tar.gz even though a linux `.mcpb` is also published (some other MCPB-aware
   client could theoretically use it, but Desktop can't).

   Build-from-source: `git clone`, `go build -o gamesome .` (module is
   `github.com/thegrok/gamesome`, confirmed `go.mod`), or `go install
   github.com/thegrok/gamesome@latest`.

5. **MCP config** — the actual snippet the release ships, not the stale repo
   `.mcp.json`:
   ```json
   {
     "mcpServers": {
       "gamesome": {
         "command": "/path/to/gamesome",
         "args": ["mcp"]
       }
     }
   }
   ```
   One line noting `.mcpb` installs configure this automatically — the
   snippet is only needed for the tar.gz/build-from-source path.

6. **Per-platform import guide** — one subsection per store, table of
   platform → source, called out from the as-built specs:
   - **Steam**: local manifest scan by default (installed games only,
     zero-config). Full owned-library coverage is an in-conversation upgrade
     — ask the sommelier, it walks through getting a Web API key
     (steamcommunity.com/dev/apikey) + SteamID64 and calls
     `set_steam_credentials`. State the disclosure verbatim from the shipped
     `sommelier` prompt: key stored in plain text in the local gamesome
     database, revocable/regenerable any time at the same URL. (This is
     A069-scope documentation of already-shipped copy, not A126's file-mode
     hardening — don't conflate the two in the text.)
   - **Epic**: Heroic cache → Legendary CLI → EGL manifests fallback chain,
     first success wins (Heroic-first because legendary-gl is a pain on
     macOS).
   - **GOG**: platform-routed — Linux via Heroic's nile cache
     (`store_cache/gog_library.json`), Windows/macOS via GOG Galaxy's own
     SQLite db directly (`galaxy-2.0.db`, authoritative for ownership +
     install state on those platforms). Note `import gog-galaxy` as the
     direct-override alias.
   - **itch.io**: reads butler's local db (`caves` + `install_locations`),
     resolves default and custom install folders both — not just the default
     `apps/` location.
   - One line: `gamesome import <store>` for a single store, or ask the
     sommelier to "import my library" (it offers on first run per the shipped
     prompt).

7. **Everyday use.** `gamesome status` (library stats), then talk to Claude:
   "what should I play tonight?" — the sommelier prompt handles the rest.
   Mention `gamesome enrich` (Steam ID cross-reference + Store metadata) as
   optional, not required for basic use.

8. **Demo.** Section present with the intended embed left as a commented-out
   placeholder plus a plain-text line, e.g.:
   ```html
   <!-- asciinema embed goes here once A080 lands -->
   ```
   `*(asciinema recording coming — full import run + a few status/search
   queries)*`
   No fabricated cast/link. This keeps the section slot stable so landing
   A080 is a one-line swap.

9. **Updating.** `.mcpb`-installed bundles don't auto-update (already-decided
   copy from `.goreleaser.yaml`'s release footer) — download the new bundle
   and install over the old one; if tool calls stop responding, toggle the
   extension off/on in Settings → Extensions. Binary installs: re-download
   the new release tarball.

10. **License / contributing** — keep minimal (one line pointing at the repo
    issues if Victor wants one; do not invent a license file or claim one
    exists — check `find . -iname "license*"` first and only reference it if
    present).

## Load-bearing contract

- Every factual claim (fallback chains, storage paths, tool names, disclosure
  language) must trace to a file actually read in this repo at head, not
  recalled from the vault BRIEF — the BRIEF is a summary, the code + as-built
  specs are ground truth.
- The MCP config snippet must match what `cmd/mcp.go` + the MCPB manifest
  actually register today, not the stale root `.mcp.json`.
- Don't claim Steam/GOG/Epic/itch parity across OSes where the specs say
  otherwise (e.g. GOG is platform-routed, not "works the same everywhere").

## Verification

No build/vet/test surface — README is prose, not code. "Verify" here means:
read the rendered file back for accuracy against the source files cited
above, and confirm `go build ./...` still passes untouched (sanity that
adding a root-level markdown file didn't disturb anything — it won't, but
cheap to confirm).

## Scope boundary

In: `README.md` only. Out: `.mcp.json` fix, A080 recording, A126 code change,
any CLI/MCP behavior change, license file creation.
