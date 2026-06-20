---
feature: mcp-go
status: draft
created: 2026-06-20
owner: Codex (implementation) / Claude (verify)
---

# Implementation -- Go MCP server (A072)

## Files to create / modify / delete

| File | Action |
|------|--------|
| `cmd/mcp.go` | CREATE -- `mcpCmd` cobra subcommand + all tool handlers |
| `cmd/root.go` | MODIFY -- add `rootCmd.AddCommand(mcpCmd)` in init() |
| `mcp/` | DELETE -- entire directory (Python package, no longer needed) |

Dependency `github.com/mark3labs/mcp-go v0.55.0` is already in `go.mod` and `go.sum`.

## `cmd/mcp.go`

### Imports

```go
package cmd

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "github.com/spf13/cobra"

    "github.com/thegrok/gamesom/internal/db"
)
```

### Server setup in RunE

```go
var mcpCmd = &cobra.Command{
    Use:   "mcp",
    Short: "Run the MCP server (stdio transport)",
    RunE: func(cmd *cobra.Command, args []string) error {
        database, err := db.Open()
        if err != nil {
            return fmt.Errorf("open db: %w", err)
        }
        defer database.Close()

        s := server.NewMCPServer("gamesom", "1.0.0",
            server.WithToolCapabilities(true),
            server.WithResourceCapabilities(false, false),
        )

        registerTools(s, database)
        registerResources(s, database)

        return server.ServeStdio(s)
    },
}

func init() {
    rootCmd.AddCommand(mcpCmd)
}
```

### Tool: `list_games`

Parameters: `installed_only` (bool, default false), `source` (string, optional),
`has_profile` (string: "true"/"false"/empty -- use string not bool so it's omittable),
`limit` (int, default 50), `offset` (int, default 0).

Handler runs the same JOIN query as the Python server:

```sql
SELECT
    g.id, g.canonical_title, g.genres, g.themes, g.summary,
    g.first_release_date, g.completed_at,
    MAX(le.installed) AS installed,
    SUM(le.playtime_minutes) AS playtime_minutes,
    MAX(le.last_played_at) AS last_played_at,
    GROUP_CONCAT(DISTINCT le.source) AS sources,
    sp.energy_required, sp.friction_level, sp.session_length_fit,
    sp.narrative_memory_load, sp.complexity_level,
    sp.mood_tags, sp.avoid_when, sp.best_when, sp.user_notes
FROM games g
LEFT JOIN library_entries le ON le.game_id = g.id
LEFT JOIN sommelier_profile sp ON sp.game_id = g.id
[WHERE ...]
GROUP BY g.id
ORDER BY g.canonical_title
LIMIT ? OFFSET ?
```

Build the WHERE clause dynamically from the params (same logic as Python).
Return JSON-marshalled rows as a single text content block.

### Tool: `search_games`

Parameters: `query` (string, required), `limit` (int, default 20).

Same query as Python: `WHERE g.canonical_title LIKE ?` with `%query%`.

### Tool: `get_game`

Parameters: `game_id` (int, required).

Three queries: `games WHERE id = ?`, `library_entries WHERE game_id = ?`,
`sommelier_profile WHERE game_id = ?`. Return combined JSON object.

### Tool: `upsert_profile`

Parameters (all optional except `game_id`):
- `game_id` (int, required)
- `energy_required`, `friction_level`, `session_length_fit`, `narrative_memory_load`,
  `complexity_level`, `mood_tags`, `avoid_when`, `best_when`, `user_notes` (all string)

Same `INSERT ... ON CONFLICT DO UPDATE SET ... = COALESCE(excluded.x, x)` query.
Return text: `"Profile saved for game_id=N"`.

### Tool: `mark_completed`

Parameters: `game_id` (int, required), `completed` (bool, default true).

If completed: `UPDATE games SET completed_at = CURRENT_TIMESTAMP WHERE id = ?`
Else: `UPDATE games SET completed_at = NULL WHERE id = ?`
Return text: `"Marked game_id=N completed"` or `"Cleared completion for game_id=N"`.

### Resource: `gamesom://library/summary`

URI: `gamesom://library/summary`, MIME: `text/plain`.

Handler runs three COUNT queries + source breakdown (same as Python), returns a
newline-joined string.

### Helper: result formatting

All tool handlers return `*mcp.CallToolResult`. The idiomatic way to return text:

```go
func textResult(s string) *mcp.CallToolResult {
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            mcp.TextContent{Type: "text", Text: s},
        },
    }
}
```

For structured data (list_games, search_games, get_game): JSON-marshal the result,
return as a text content block.

## `mcp/` deletion

Delete the entire directory. Nothing in the Go codebase imports it. After deletion,
update the mcp.json example in README.md if one exists.

## Load-bearing contract

- `server.ServeStdio` writes to stdout -- **no other code in the mcp subcommand may
  write to stdout** (it corrupts the JSON-RPC stream). All logging -> stderr.
- Tool names must be byte-for-byte identical to the Python server's tool names.
- `has_profile` param: use string ("true"/"false"/"") not bool -- empty string means
  "no filter". This avoids the optional-boolean problem in JSON schema.
- The database is opened once and shared across all tool calls.

## Scope boundary

- No new tools or resources beyond what the Python server exposed
- No HTTP transport (stdio only)
- No server version bumping in other files
- README update is optional

## Verification

After Codex edits:
- `go build ./...` -- Codex confirms compilation
- `go vet ./...` -- Claude runs
- `go test ./...` -- Claude runs (existing suite must stay green; no new tests required)
- Smoke test: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}' | gamesom mcp` should return an initialize response
