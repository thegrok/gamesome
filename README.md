# gamesome — Game Sommelier

Your game library's sommelier: mood- and context-fit picks from the games you
already own. Ask Claude: *"what should I play tonight?"*

`gamesome` imports your library from Steam, GOG, Epic, and itch.io into a
local database, keeps track of what's actually installed, and briefs Claude
as your personal game sommelier — helping you choose from your real backlog
based on tonight's energy, mood, and time, instead of recommending
universally "good" games you'll never start.

## Features

- **Cross-platform importers** (Linux / macOS / Windows):
  - **Steam** — local manifest scan, zero-config
  - **Epic** — Heroic cache → Legendary CLI → EGL manifests, first success wins
  - **GOG** — Linux via Heroic's nile cache, Windows/macOS via GOG Galaxy's own database directly
  - **itch.io** — reads butler's local database, resolves both default and custom install locations
- **Installed-state accuracy**, not just ownership
- **MCP server** (`gamesome mcp`, stdio transport) with:
  - Tools: `list_games`, `search_games`, `get_game`, `upsert_profile`, `mark_completed`, `set_steam_credentials`, `refresh_library`, `update_persona`, `reset_persona`
  - A `sommelier` re-brief prompt and a `gamesome://library/summary` resource
- **Configurable, adaptive sommelier persona** — a dimension-keyed profile the sommelier can propose adjustments to as it learns your taste, always confirm-gated
- **One-click Claude Desktop install** via an `.mcpb` bundle — no terminal, no hand-edited config

## Install

| Situation | Get |
|---|---|
| Claude Desktop, Windows/macOS | The `.mcpb` bundle from [Releases](https://github.com/thegrok/gamesome/releases) — double-click it, or Claude Desktop → Settings → Extensions → Install Extension |
| Linux, or any non-Desktop MCP client (Claude Code, etc.) | The `tar.gz`/`zip` binary from [Releases](https://github.com/thegrok/gamesome/releases) — extract and put `gamesome` on your `PATH` (or reference it by full path) |
| Building from source | See below |

There's no Claude Desktop on Linux, so Linux always takes the `tar.gz` —
even though a Linux `.mcpb` is also published for completeness, there's no
Desktop client on that platform to install it into.

### Building from source

```sh
git clone https://github.com/thegrok/gamesome.git
cd gamesome
go build -o gamesome .
```

Or directly with `go install`:

```sh
go install github.com/thegrok/gamesome@latest
```

## MCP config

`.mcpb` installs configure this automatically. If you installed the raw
binary (Linux, or a non-Desktop MCP client), point your client at it:

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

### Claude Code CLI

Register it globally, so it's available from any directory:

```sh
claude mcp add gamesome --scope user -- /path/to/gamesome mcp
```

Or add the JSON block above to a project's `.mcp.json` for a project-scoped
install instead.

## Importing your library

Ask the sommelier to "import my library" — it offers to do this on first run
— or run a specific store yourself:

```sh
gamesome import steam
gamesome import epic
gamesome import gog
gamesome import itch
```

### Steam

By default, `gamesome` scans your local Steam manifests for **installed**
games only — zero-config, no API key needed. To unlock your full owned
library (not just what's installed), ask the sommelier in conversation: it
will walk you through getting a Web API key from
[steamcommunity.com/dev/apikey](https://steamcommunity.com/dev/apikey) and
your SteamID64, then store them for you via `set_steam_credentials`.

The key is stored in plain text in your local `gamesome` database, and can
be revoked or regenerated at any time at the same URL.

### Epic

Tries, in order: Heroic's cache → the Legendary CLI → Epic Games Launcher's
own manifests. First one that succeeds wins.

### GOG

Platform-routed: on Linux, `gamesome` reads Heroic's nile cache (Heroic's
GOG backend). On Windows/macOS, it reads GOG Galaxy's own local database
directly, which is authoritative for both ownership and install state on
those platforms. Use `gamesome import gog-galaxy` to force the direct
Galaxy-database path on any platform.

### itch.io

Reads butler's local database (the same one the itch app uses) and resolves
both the default install location and any custom folder you've configured in
itch's own Preferences.

## Everyday use

```sh
gamesome status
```

shows your library stats. Otherwise, just talk to Claude — "what should I
play tonight?" — and the sommelier takes it from there, including offering
to refresh your library if it looks stale.

`gamesome enrich` is optional: it cross-references games to Steam app IDs and
fetches genre/tag metadata from the Steam Store, which helps the sommelier
reason about fit. Not required for basic use.

## Demo

<!-- asciinema embed goes here once A080 lands -->
*(asciinema recording coming — a full import run across all four stores,
followed by a few `gamesome status` / sommelier conversation examples)*

## Updating

`.mcpb` bundles installed from a file don't auto-update — download the new
version and install it over the old one. If tool calls stop responding after
an update, toggle the extension off and on in Settings → Extensions.

Binary installs: download the new release tarball and replace the old one.

## Contributing

Issues and pull requests welcome at
[github.com/thegrok/gamesome](https://github.com/thegrok/gamesome).
