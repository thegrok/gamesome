---
feature: mcp-go
status: draft
created: 2026-06-20
---

# Design -- Go MCP server (A072)

## Problem

The MCP server lives in `mcp/` as a Python/uv package. This contradicts the Go
distribution story: a user who installs `gamesom` via a binary release has no Python
toolchain, so the MCP server is unreachable. The fix is to fold the MCP server into
the Go binary as a `gamesom mcp` subcommand.

## Approach

Replace `mcp/` with `cmd/mcp.go` using `github.com/mark3labs/mcp-go` (v0.55.0,
already added to go.mod). The subcommand:

1. Opens the gamesom SQLite database the same way every other subcommand does
2. Registers the same 5 tools + 1 resource the Python server exposed
3. Runs `server.ServeStdio(s)` -- stdio transport, same as the Python server

mcp.json becomes:

```json
{
  "mcpServers": {
    "gamesom": {
      "command": "gamesom",
      "args": ["mcp"]
    }
  }
}
```

## What changes

- CREATE `cmd/mcp.go`
- DELETE `mcp/` directory (Python package + pyproject.toml + uv.lock)
- No schema changes, no new DB logic -- all queries already exist

## Tools to port (1:1 from Python)

| Python tool | Go equivalent |
|-------------|---------------|
| `list_games` | `list_games` |
| `search_games` | `search_games` |
| `get_game` | `get_game` |
| `upsert_profile` | `upsert_profile` |
| `mark_completed` | `mark_completed` |
| `gamesom://library/summary` (resource) | same URI |
