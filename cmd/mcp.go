package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/thegrok/gamesom/internal/db"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the MCP server (stdio transport)",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		s := mcp.NewServer(&mcp.Implementation{Name: "gamesom", Version: "v1.0.0"}, nil)
		registerTools(s, database)
		registerResources(s, database)
		return s.Run(cmd.Context(), &mcp.StdioTransport{})
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func jsonResult(value any) (*mcp.CallToolResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return textResult(string(encoded)), nil
}

func argMap(req *mcp.CallToolRequest) map[string]any {
	var m map[string]any
	if req.Params.Arguments != nil {
		_ = json.Unmarshal(req.Params.Arguments, &m)
	}
	return m
}

func getBool(m map[string]any, key string, def bool) bool {
	v, ok := m[key]
	if !ok {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func getString(m map[string]any, key string, def string) string {
	v, ok := m[key]
	if !ok {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func getInt(m map[string]any, key string, def int) int {
	v, ok := m[key]
	if !ok {
		return def
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return def
}

func registerTools(s *mcp.Server, database *sql.DB) {
	s.AddTool(&mcp.Tool{
		Name:        "list_games",
		Description: "List games from the library with optional filters.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"installed_only": {"type": "boolean", "description": "Only return installed games.", "default": false},
				"source": {"type": "string", "description": "Filter by import source."},
				"has_profile": {"type": "string", "description": "Filter by profile presence: \"true\", \"false\", or empty.", "enum": ["true", "false", ""]},
				"limit": {"type": "integer", "description": "Maximum number of results.", "default": 50},
				"offset": {"type": "integer", "description": "Pagination offset.", "default": 0}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		var wheres []string
		var params []any
		if getBool(args, "installed_only", false) {
			wheres = append(wheres, "EXISTS (SELECT 1 FROM library_entries le2 WHERE le2.game_id = g.id AND le2.installed = 1)")
		}
		if source := getString(args, "source", ""); source != "" {
			wheres = append(wheres, "EXISTS (SELECT 1 FROM library_entries le2 WHERE le2.game_id = g.id AND le2.source = ?)")
			params = append(params, source)
		}
		switch getString(args, "has_profile", "") {
		case "true":
			wheres = append(wheres, "EXISTS (SELECT 1 FROM sommelier_profile sp2 WHERE sp2.game_id = g.id)")
		case "false":
			wheres = append(wheres, "NOT EXISTS (SELECT 1 FROM sommelier_profile sp2 WHERE sp2.game_id = g.id)")
		}

		whereClause := ""
		if len(wheres) > 0 {
			whereClause = "WHERE " + strings.Join(wheres, " AND ")
		}
		params = append(params, getInt(args, "limit", 50), getInt(args, "offset", 0))

		rows, err := queryRows(ctx, database, `
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
			`+whereClause+`
			GROUP BY g.id
			ORDER BY g.canonical_title
			LIMIT ? OFFSET ?`, params...)
		if err != nil {
			return nil, fmt.Errorf("list games: %w", err)
		}
		return jsonResult(rows)
	})

	s.AddTool(&mcp.Tool{
		Name:        "search_games",
		Description: "Search games by title using a case-insensitive substring match.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["query"],
			"properties": {
				"query": {"type": "string", "description": "Title substring to search for."},
				"limit": {"type": "integer", "description": "Maximum number of results.", "default": 20}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		rows, err := queryRows(ctx, database, `
			SELECT
				g.id, g.canonical_title, g.genres, g.summary, g.completed_at,
				MAX(le.installed) AS installed,
				SUM(le.playtime_minutes) AS playtime_minutes,
				GROUP_CONCAT(DISTINCT le.source) AS sources,
				sp.energy_required, sp.friction_level, sp.session_length_fit,
				sp.mood_tags, sp.avoid_when, sp.best_when
			FROM games g
			LEFT JOIN library_entries le ON le.game_id = g.id
			LEFT JOIN sommelier_profile sp ON sp.game_id = g.id
			WHERE g.canonical_title LIKE ?
			GROUP BY g.id
			ORDER BY g.canonical_title
			LIMIT ?`, "%"+getString(args, "query", "")+"%", getInt(args, "limit", 20))
		if err != nil {
			return nil, fmt.Errorf("search games: %w", err)
		}
		return jsonResult(rows)
	})

	s.AddTool(&mcp.Tool{
		Name:        "get_game",
		Description: "Get a game with all library entries and its sommelier profile.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["game_id"],
			"properties": {
				"game_id": {"type": "integer", "description": "The game's database ID."}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		gameID := getInt(args, "game_id", 0)
		games, err := queryRows(ctx, database, "SELECT * FROM games WHERE id = ?", gameID)
		if err != nil {
			return nil, fmt.Errorf("get game: %w", err)
		}
		if len(games) == 0 {
			return jsonResult(nil)
		}
		entries, err := queryRows(ctx, database, "SELECT * FROM library_entries WHERE game_id = ?", gameID)
		if err != nil {
			return nil, fmt.Errorf("get library entries: %w", err)
		}
		profiles, err := queryRows(ctx, database, "SELECT * FROM sommelier_profile WHERE game_id = ?", gameID)
		if err != nil {
			return nil, fmt.Errorf("get sommelier profile: %w", err)
		}
		var profile any
		if len(profiles) > 0 {
			profile = profiles[0]
		}
		return jsonResult(map[string]any{
			"game":              games[0],
			"library_entries":   entries,
			"sommelier_profile": profile,
		})
	})

	s.AddTool(&mcp.Tool{
		Name:        "upsert_profile",
		Description: "Write or partially update the sommelier profile for a game.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["game_id"],
			"properties": {
				"game_id": {"type": "integer", "description": "The game's database ID."},
				"energy_required": {"type": "string"},
				"friction_level": {"type": "string"},
				"session_length_fit": {"type": "string"},
				"narrative_memory_load": {"type": "string"},
				"complexity_level": {"type": "string"},
				"mood_tags": {"type": "string"},
				"avoid_when": {"type": "string"},
				"best_when": {"type": "string"},
				"user_notes": {"type": "string"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		gameID := getInt(args, "game_id", 0)
		optionalString := func(key string) any {
			v, ok := args[key]
			if !ok {
				return nil
			}
			return v
		}
		_, err := database.ExecContext(ctx, `
			INSERT INTO sommelier_profile
				(game_id, energy_required, friction_level, session_length_fit,
				 narrative_memory_load, complexity_level, mood_tags, avoid_when,
				 best_when, user_notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(game_id) DO UPDATE SET
				energy_required       = COALESCE(excluded.energy_required, energy_required),
				friction_level        = COALESCE(excluded.friction_level, friction_level),
				session_length_fit    = COALESCE(excluded.session_length_fit, session_length_fit),
				narrative_memory_load = COALESCE(excluded.narrative_memory_load, narrative_memory_load),
				complexity_level      = COALESCE(excluded.complexity_level, complexity_level),
				mood_tags             = COALESCE(excluded.mood_tags, mood_tags),
				avoid_when            = COALESCE(excluded.avoid_when, avoid_when),
				best_when             = COALESCE(excluded.best_when, best_when),
				user_notes            = COALESCE(excluded.user_notes, user_notes)`,
			gameID,
			optionalString("energy_required"), optionalString("friction_level"),
			optionalString("session_length_fit"), optionalString("narrative_memory_load"),
			optionalString("complexity_level"), optionalString("mood_tags"),
			optionalString("avoid_when"), optionalString("best_when"), optionalString("user_notes"),
		)
		if err != nil {
			return nil, fmt.Errorf("upsert profile: %w", err)
		}
		return textResult(fmt.Sprintf("Profile saved for game_id=%d", gameID)), nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "mark_completed",
		Description: "Mark a game completed or clear its completion timestamp.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["game_id"],
			"properties": {
				"game_id": {"type": "integer", "description": "The game's database ID."},
				"completed": {"type": "boolean", "description": "Whether the game is completed.", "default": true}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		gameID := getInt(args, "game_id", 0)
		completed := getBool(args, "completed", true)
		query := "UPDATE games SET completed_at = CURRENT_TIMESTAMP WHERE id = ?"
		message := fmt.Sprintf("Marked game_id=%d completed", gameID)
		if !completed {
			query = "UPDATE games SET completed_at = NULL WHERE id = ?"
			message = fmt.Sprintf("Cleared completion for game_id=%d", gameID)
		}
		if _, err := database.ExecContext(ctx, query, gameID); err != nil {
			return nil, fmt.Errorf("mark completed: %w", err)
		}
		return textResult(message), nil
	})
}

func registerResources(s *mcp.Server, database *sql.DB) {
	const uri = "gamesom://library/summary"
	s.AddResource(&mcp.Resource{URI: uri, Name: "Library Summary", MIMEType: "text/plain"},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			var total, installed, withProfile int
			if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM games").Scan(&total); err != nil {
				return nil, fmt.Errorf("count games: %w", err)
			}
			if err := database.QueryRowContext(ctx, "SELECT COUNT(DISTINCT game_id) FROM library_entries WHERE installed = 1").Scan(&installed); err != nil {
				return nil, fmt.Errorf("count installed games: %w", err)
			}
			if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM sommelier_profile").Scan(&withProfile); err != nil {
				return nil, fmt.Errorf("count profiles: %w", err)
			}

			rows, err := database.QueryContext(ctx, "SELECT source, COUNT(*) FROM library_entries GROUP BY source ORDER BY COUNT(*) DESC")
			if err != nil {
				return nil, fmt.Errorf("count sources: %w", err)
			}
			defer rows.Close()
			lines := []string{
				fmt.Sprintf("Total games: %d", total),
				fmt.Sprintf("Installed: %d", installed),
				fmt.Sprintf("With sommelier profile: %d", withProfile),
				"",
				"By source:",
			}
			for rows.Next() {
				var source string
				var count int
				if err := rows.Scan(&source, &count); err != nil {
					return nil, fmt.Errorf("scan source count: %w", err)
				}
				lines = append(lines, fmt.Sprintf("  %s: %d", source, count))
			}
			if err := rows.Err(); err != nil {
				return nil, fmt.Errorf("read source counts: %w", err)
			}

			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/plain", Text: strings.Join(lines, "\n")}},
			}, nil
		})
}

func queryRows(ctx context.Context, database *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			if value, ok := values[i].([]byte); ok {
				row[column] = string(value)
			} else {
				row[column] = values[i]
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
