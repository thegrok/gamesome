# gamesom MCP server

Exposes the local gamesom library database to Claude over MCP (stdio), so an
agent can act as game sommelier — browse and search the enriched library and
write taste profiles back into it.

## Prerequisites

- [`uv`](https://docs.astral.sh/uv/) on `PATH` (manages the venv + `mcp` dep from
  `pyproject.toml` automatically; no manual install step).
- A populated library database at `$XDG_DATA_HOME/gamesom/gamesom.db`
  (default `~/.local/share/gamesom/gamesom.db`), produced on this host by
  `gamesom import …` followed by `gamesom enrich`.

## Use with Claude Code

The repo ships a project-scoped `.mcp.json` at its root that registers a
`gamesom` server. Launch Claude Code **from the repo root**, approve the server
when prompted, and it runs:

```
uv run --directory mcp gamesom-mcp
```

## Surface

| Kind     | Name                         | Purpose                                  |
|----------|------------------------------|------------------------------------------|
| tool     | `list_games`                 | paginated / filtered library listing     |
| tool     | `search_games`               | title / text search                      |
| tool     | `get_game`                   | single game by id                        |
| tool     | `upsert_profile`             | write `sommelier_profile` rows           |
| resource | `gamesom://library/summary`  | library overview                         |

## Note on multi-host use

The server reads the **host-local** database via the XDG path above — it does not
reach across machines. Whichever host runs the sommelier agent (workstation,
moxie, …) must have its own populated `gamesom.db`.
