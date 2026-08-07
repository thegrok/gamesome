# gamesome — Game Sommelier

Collected so many games you don't even know what to play anymore? Gamesome can help!

Ask Claude: *"what should I play tonight?"*

<!-- Bare URL on its own line — GitHub turns that into a player. Any markdown
wrapper, []() or ![](), downgrades it to a link. Asset uploaded via issue #41
(closing that issue does not remove it). The same file is committed at
docs/media/gamesome-demo.mp4 so it lives in version control too, but a relative
path to it will NOT render a player. -->

https://github.com/user-attachments/assets/8cf9ade0-7829-45bd-8241-d92591920379


As your game library's sommelier, Gamesome will mood- and context-fit picks from the games you
already own. It curates your Steam, GOG, Epic, and itch.io libraries, keeps track of what's actually installed, helping you choose from your real backlog based on energy, mood, and time, instead of just recommending universally "good" games you'll never start.

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

There's currently no officially release Claude Desktop on Linux, so Linux always takes the `tar.gz` —
even though a Linux `.mcpb` is also published for completeness.

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

See [Security](#security) for how that key is stored.

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
play?" — and the sommelier takes it from there, including offering
to refresh your library if it looks stale.

`gamesome enrich` is optional: it cross-references games to Steam app IDs and
fetches genre/tag metadata from the Steam Store, which helps the sommelier
reason about fit. Not required for basic use.

## Security

Your Steam Web API key (if you set one up) is stored in **plain text** in the
local `gamesome` database — there's no OS keychain integration. It's a
low-privilege, read-oriented key you can revoke or regenerate at any time at
[steamcommunity.com/dev/apikey](https://steamcommunity.com/dev/apikey), so
this is a deliberate, accepted trade-off rather than an oversight. The
database file itself is created with `0600` permissions (readable/writable
by you only) regardless of your OS's default umask.

The database lives in a platform-idiomatic data directory:

| OS | Path |
| --- | --- |
| Windows | `%LOCALAPPDATA%\gamesome\gamesome.db` |
| macOS | `~/Library/Application Support/gamesome/gamesome.db` |
| Linux | `~/.local/share/gamesome/gamesome.db` (or `$XDG_DATA_HOME/gamesome/gamesome.db` if set) |

## Updating

`.mcpb` bundles installed from a file don't auto-update — download the new
version and install it over the old one. If tool calls stop responding after
an update, toggle the extension off and on in Settings → Extensions.

Binary installs: download the new release tarball and replace the old one.

## Contributing

Bug reports and questions are welcome as
[issues](https://github.com/thegrok/gamesome/issues) — useful to me even when I
can't act on them quickly.

Pull requests are a different matter. This is a solo side project and I don't
have review capacity to promise, so an unsolicited PR may sit a long time or be
declined for reasons that have nothing to do with its quality. If there's
something you want changed, open an issue first. If you'd rather not wait: it's
AGPL, so fork it.

## License

[GNU Affero General Public License v3.0 or later](LICENSE) (AGPL-3.0-or-later).

In practice: run it, study it, change it, share it. If you distribute a modified
version — or run one as a service other people reach over a network — you have to
publish your source under the same license. Building a hosted game-recommender on
these importers is fine; keeping that version closed is not.

Third-party dependencies keep their own licenses, all permissive: the
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) (MIT/Apache-2.0),
[cobra](https://github.com/spf13/cobra) (Apache-2.0), and
[modernc.org/sqlite](https://modernc.org/sqlite) (BSD-3-Clause).
