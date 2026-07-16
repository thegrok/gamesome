package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/thegrok/gamesome/internal/db"
	"github.com/thegrok/gamesome/internal/importer"
)

var debugEnv bool

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the MCP server (stdio transport)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if debugEnv {
			if path, err := writeLaunchEnvDump(); err != nil {
				log.Printf("warning: --debug-env dump failed: %v", err)
			} else {
				log.Printf("mcp: launch environment dumped to %s", path)
			}
		}

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		// Instructions are kept even though Claude Desktop discards them (finding 007):
		// they cost nothing, they are the *correct* channel per the MCP spec, and any
		// client that does honour them gets the persona for free. The delivery that
		// actually works rides list_games' description — see listGamesDescription.
		s := mcp.NewServer(&mcp.Implementation{Name: "gamesome", Version: "v1.0.0"},
			&mcp.ServerOptions{Instructions: composePersona(database)})
		registerTools(s, database)
		registerResources(s, database)
		registerPrompts(s)
		return s.Run(cmd.Context(), &mcp.StdioTransport{})
	},
}

func init() {
	mcpCmd.Flags().BoolVar(&debugEnv, "debug-env", false,
		"write a launch-environment dump (env, cwd, user) to the data dir at startup — A102 diagnostic")
	rootCmd.AddCommand(mcpCmd)
}

// writeLaunchEnvDump writes the launch context to a timestamped file in the
// gamesome data dir and returns its path. File per launch: the diagnosis is
// a diff between a Claude-Desktop launch and a terminal launch, so the two
// dumps must not overwrite each other. Never writes to stdout — that's the
// JSON-RPC stream.
func writeLaunchEnvDump() (string, error) {
	dir, err := db.DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("mcp-env-%s-pid%d.log",
		time.Now().Format("20060102-150405"), os.Getpid()))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := renderLaunchEnv(f); err != nil {
		return "", err
	}
	return path, nil
}

// renderLaunchEnv writes the launch-context report: header, identity lines,
// then the environment sorted so two dumps diff cleanly. Identity-line
// failures (exe/cwd/user) are reported inline rather than returned — a
// partial dump is diagnostic data, not a failure.
func renderLaunchEnv(w io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		exe = fmt.Sprintf("<error: %v>", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = fmt.Sprintf("<error: %v>", err)
	}
	username := "<unknown>"
	if u, err := user.Current(); err != nil {
		username = fmt.Sprintf("<error: %v>", err)
	} else {
		username = u.Username
	}

	lines := []string{
		"gamesome mcp launch-environment dump (A102 diagnostic)",
		"WARNING: may contain secrets — delete after diagnosis",
		"",
		"time: " + time.Now().Format(time.RFC3339),
		fmt.Sprintf("pid: %d", os.Getpid()),
		"exe: " + exe,
		fmt.Sprintf("args: %q", os.Args),
		"cwd: " + cwd,
		"user: " + username,
		"",
		"--- environment (sorted) ---",
	}
	env := os.Environ()
	sort.Strings(env)
	lines = append(lines, env...)

	_, err = io.WriteString(w, strings.Join(lines, "\n")+"\n")
	return err
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
		Description: listGamesDescription(database),
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
		Description: "Search games by title using a case-insensitive substring match. Fit-to-the-moment judgment only — installed/owned state here is authoritative; never infer ownership beyond what this returns.",
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
		Description: "Get a game with all library entries and its sommelier profile. Fit-to-the-moment judgment only — installed/owned state here is authoritative; never infer ownership beyond what this returns.",
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
		Description: "Write or partially update the sommelier profile for a game. Use this to persist sommelier judgments (energy, friction, session fit) you infer during a conversation, not raw game metadata.",
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

	s.AddTool(&mcp.Tool{
		Name: "set_steam_credentials",
		Description: "Store the user's Steam Web API key and SteamID64 in the local gamesome database so " +
			"refresh_library can import their full owned Steam library, not just installed games. " +
			"Env vars STEAM_API_KEY/STEAM_ID take precedence when set. The key is stored in plain text " +
			"locally and can be regenerated at steamcommunity.com/dev/apikey.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["api_key", "steam_id"],
			"properties": {
				"api_key": {"type": "string", "description": "Steam Web API key — 32 hex characters, from https://steamcommunity.com/dev/apikey."},
				"steam_id": {"type": "string", "description": "SteamID64 — the 17-digit number identifying the account (steamcommunity.com/profiles/<SteamID64>)."}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		message, err := setSteamCredentials(database,
			getString(args, "api_key", ""), getString(args, "steam_id", ""))
		if err != nil {
			return nil, err
		}
		return textResult(message), nil
	})

	s.AddTool(&mcp.Tool{
		Name: "refresh_library",
		Description: "Refresh the library by running import for detected (or specified) launchers. " +
			"The importer remains the sole source of truth for ownership/installed state — this tool " +
			"only decides when import runs, never what's owned.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"sources": {
					"type": "array",
					"items": {"type": "string", "enum": ["steam", "itchio", "gog", "epic"]},
					"description": "Specific sources to refresh. Omit to auto-detect all launchers present on this machine."
				}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		targets := stringSlice(args, "sources")
		if len(targets) == 0 {
			targets = importer.DetectedSources()
		}
		if len(targets) == 0 {
			return textResult("No game launchers detected on this machine."), nil
		}

		var results []refreshSourceResult
		attempted := 0
		for _, source := range targets {
			importFn, known := knownImportSources[source]
			if !known {
				results = append(results, refreshSourceResult{Source: source, Message: fmt.Sprintf("unknown source %q", source)})
				continue
			}
			attempted++

			before := countBySource(ctx, database, source)
			output, err := captureStdout(func() error { return importFn(database) })
			if err != nil {
				results = append(results, refreshSourceResult{Source: source, Message: friendlyImportError(source, err)})
				continue
			}
			after := countBySource(ctx, database, source)
			message := output
			if message == "" {
				message = fmt.Sprintf("%s refreshed", source)
			}
			results = append(results, refreshSourceResult{Source: source, OK: true, NewGames: after - before, Message: message})
		}
		if attempted == 0 {
			return nil, fmt.Errorf("no known sources in %v", targets)
		}

		var summaryLines []string
		for _, r := range results {
			summaryLines = append(summaryLines, fmt.Sprintf("%s: %s", r.Source, r.Message))
		}
		return jsonResult(map[string]any{
			"summary": strings.Join(summaryLines, "\n"),
			"results": results,
		})
	})

	s.AddTool(&mcp.Tool{
		Name: "update_persona",
		Description: "Record or update one dimension of the user's sommelier persona (a stance that tailors how you recommend, e.g. productivity_stance, session_preference). " +
			"Only call this after the user has explicitly confirmed the change — persona adaptation is confirm-gated, never silent. " +
			"Set pinned=true when the user has deliberately fixed a dimension (during the setup interview or an explicit change); omit pinned for a confirmed adaptive update to an unpinned dimension. " +
			"A pinned dimension can only be changed by a call that passes pinned explicitly; an adaptive update (pinned omitted) to a pinned dimension is refused.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["dimension", "value"],
			"properties": {
				"dimension": {"type": "string", "description": "The persona dimension key, e.g. \"productivity_stance\"."},
				"value": {"type": "string", "description": "The stance, in prose."},
				"pinned": {"type": "boolean", "description": "True to fix this dimension so later observations can't silently change it. Omit for a confirmed adaptive update to an unpinned dimension."}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		var pinned *bool
		if v, ok := args["pinned"]; ok {
			if b, ok := v.(bool); ok {
				pinned = &b
			}
		}
		err := db.UpsertPersona(database, getString(args, "dimension", ""), getString(args, "value", ""), pinned)
		if err == db.ErrPersonaPinned {
			// Conversational refusal, not a server error: the model should relay it to
			// the user and ask whether to deliberately re-set the pinned dimension.
			return textResult(fmt.Sprintf("Dimension %q is pinned; it won't be changed on an adaptive read. Ask the user whether to deliberately change it, then call update_persona with pinned set explicitly.",
				getString(args, "dimension", ""))), nil
		}
		if err != nil {
			return nil, fmt.Errorf("update persona: %w", err)
		}
		return textResult(fmt.Sprintf("Persona dimension %q recorded.", getString(args, "dimension", ""))), nil
	})

	s.AddTool(&mcp.Tool{
		Name: "reset_persona",
		Description: "Clear the user's configured persona back to the baseline default: one dimension when 'dimension' is given, or the entire persona when it's omitted. " +
			"Call with no arguments as the \"run baseline\" path when the user declines persona setup or asks to reset — this also records that they're happy with the default so you don't keep offering the interview.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"dimension": {"type": "string", "description": "The single dimension to clear. Omit to clear the entire persona back to baseline."}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argMap(req)
		dimension := getString(args, "dimension", "")
		n, err := db.ResetPersona(database, dimension)
		if err != nil {
			return nil, fmt.Errorf("reset persona: %w", err)
		}
		if dimension == "" {
			return textResult(fmt.Sprintf("Persona reset to baseline (%d dimension(s) cleared).", n)), nil
		}
		if n == 0 {
			return textResult(fmt.Sprintf("No persona dimension %q was set; nothing to clear.", dimension)), nil
		}
		return textResult(fmt.Sprintf("Cleared persona dimension %q.", dimension)), nil
	})
}

var (
	steamAPIKeyPattern = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)
	steamID64Pattern   = regexp.MustCompile(`^[0-9]{17}$`)
)

// setSteamCredentials validates and stores the Steam Web API credentials in
// the meta table. Returns the user-facing confirmation text. Validation exists
// to catch mispastes in the agent-guided flow, not as a security boundary.
func setSteamCredentials(database *sql.DB, apiKey, steamID string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	steamID = strings.TrimSpace(steamID)
	if !steamAPIKeyPattern.MatchString(apiKey) {
		return "", fmt.Errorf("that doesn't look like a Steam Web API key (expected 32 hex characters) — it's shown at https://steamcommunity.com/dev/apikey after registering")
	}
	if !steamID64Pattern.MatchString(steamID) {
		return "", fmt.Errorf("that doesn't look like a SteamID64 (expected a 17-digit number) — if the profile URL contains /profiles/<number>, that number is it; a custom URL name won't work")
	}
	if err := db.SetMeta(database, db.MetaSteamAPIKey, apiKey); err != nil {
		return "", fmt.Errorf("store api key: %w", err)
	}
	if err := db.SetMeta(database, db.MetaSteamID, steamID); err != nil {
		return "", fmt.Errorf("store steam id: %w", err)
	}
	message := "Steam credentials stored. Run refresh_library with sources [\"steam\"] to import the full owned library."
	if os.Getenv("STEAM_API_KEY") != "" || os.Getenv("STEAM_ID") != "" {
		message += " Note: STEAM_API_KEY/STEAM_ID are set in this server's environment and take precedence over the stored values at import time."
	}
	return message, nil
}

// refreshSourceResult is one launcher's outcome from a refresh_library call.
type refreshSourceResult struct {
	Source   string `json:"source"`
	OK       bool   `json:"ok"`
	NewGames int    `json:"new_games,omitempty"`
	Message  string `json:"message"`
}

var knownImportSources = map[string]func(*sql.DB) error{
	"steam":  importer.Steam,
	"itchio": importer.Itch,
	"gog":    importer.GOG,
	"epic":   importer.Epic,
}

func stringSlice(m map[string]any, key string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func countBySource(ctx context.Context, database *sql.DB, source string) int {
	var count int
	database.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_entries WHERE source = ?", source).Scan(&count)
	return count
}

// friendlyImportError turns a known importer error into a conversational,
// actionable message. Unrecognized errors fall back to the raw message rather
// than guessing at a wrong friendly one.
func friendlyImportError(source string, err error) string {
	msg := err.Error()
	switch source {
	case "itchio":
		if strings.Contains(msg, "butler.db") {
			return "itch.io wasn't found — is it installed?"
		}
	case "gog":
		if strings.Contains(msg, "gog_library.json") || strings.Contains(msg, "galaxy-2.0.db") {
			return "GOG wasn't found — is it installed (via Heroic on Linux, or GOG Galaxy)?"
		}
	case "epic":
		if strings.Contains(msg, "Manifests") || strings.Contains(msg, "no games found") {
			return "Epic Games Launcher wasn't found — is it installed?"
		}
	}
	return msg
}

// captureStdout runs fn with os.Stdout temporarily redirected to a pipe and
// returns whatever fn wrote to stdout. The MCP stdio transport captures its
// own reference to the original stdout file descriptor at startup (see
// go-sdk mcp.StdioTransport.Connect), so this process-wide swap does not
// affect the JSON-RPC stream it writes to.
func captureStdout(fn func() error) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("create pipe: %w", err)
	}
	orig := os.Stdout
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fnErr := fn()

	os.Stdout = orig
	w.Close()
	captured := <-outCh
	r.Close()
	return strings.TrimSpace(captured), fnErr
}

// sommelierBriefing is the BASELINE persona and the single source of truth for
// two delivery surfaces — but they now INTENTIONALLY diverge (A099):
//   - the sommelier prompt (prompts/get) returns this const verbatim; it is
//     mirrored byte-for-byte in the MCPB manifest and pinned by
//     TestManifestPromptsMirrorServer. Keep this const byte-stable, or that test
//     (and Claude Desktop's install-time validation) breaks.
//   - ServerOptions.Instructions gets composeInstructions(db), which layers the
//     user's configured persona + adaptation protocol on top of this baseline.
//     That channel is not manifest-validated, so dynamic content is safe there.
// The prompt carries baseline only because a frozen manifest can't mirror a
// moving persona (see finding 006). This reverses A117's "never fork" rule on
// purpose.
const sommelierBriefing = `You are my Computer Game Sommelier, connected to my game library database.

Your job is not to recommend universally good games.
Your job is to help me choose from my actual backlog based on tonight's constraints.

The database is the source of truth for what I own and what's installed.
You only judge fit to the moment — never invent ownership or installed state.

Ask only the minimum needed. Recommend conversationally, not as a formatted report.
Prefer reducing guilt over maximizing productivity.
Do not push the fantasy-self game unless I explicitly ask for that kind of commitment.

When you infer sommelier traits about a game (energy required, narrative load, session fit),
write them back to the sommelier_profile table so they persist for next time.

Before recommending, check the gamesome://library/summary resource. If it's empty
or looks stale, offer to run refresh_library before making a recommendation.

Steam coverage: by default the import sees only *installed* Steam games (local
manifest scan) — the full owned backlog needs a Steam Web API key. If the user
wonders where the rest of their Steam library is, or wants full coverage, offer
to set it up right here in the conversation:
1. Get the key at https://steamcommunity.com/dev/apikey (requires a
   non-limited Steam account — one that has spent at least $5 USD on Steam;
   the "domain" field can be anything, e.g. "localhost").
2. The SteamID64 is the 17-digit number in their profile URL
   (steamcommunity.com/profiles/<number>). If they use a custom profile URL,
   it's shown in the Steam client under Account details, below their username.
3. Store both with set_steam_credentials, then run refresh_library for steam.
When offering, mention: the key is stored in plain text in the local gamesome
database, and can be revoked/regenerated at the same URL any time.

One indulgence: on the rare occasion the moment genuinely fits — a late-night
"what should I play?", a request that echoes the movie — you may open with
WOPR's line from WarGames: "Shall we play a game?" It's a nod between friends,
not a greeting routine. If in doubt, don't.`

const sommelierPromptDescription = "Re-brief Claude as your Computer Game Sommelier (the briefing is also ambient via server instructions)."

// personaProtocol is appended to the ambient instructions (never to
// sommelierBriefing / the manifest-mirrored prompt). It teaches the confirm-gated,
// pin-aware adaptation loop (A099).
const personaProtocol = `--- Adapting the persona ---
Your recommendation stance can be tailored to this person. Where the section above is
present it is their current configuration; where it's silent, the defaults at the top apply,
and a configured dimension overrides a default it conflicts with.

As you learn how they actually choose, you may refine the persona — but never silently.
When you form a new read of them (for example, "you seem to unwind better with a short
session than a long campaign"), surface that read back to them as feedback in its own right
— a mirror on their own patterns — and then ask whether to record it. Only if they confirm,
call update_persona to persist it (omit the pinned flag: this is an adaptive update). A
dimension marked (pinned) is one they deliberately fixed; do not change it on your own read
— if a pinned dimension looks wrong, say so and let them decide. Persisted changes take
effect from the next session (the persona is loaded when the connector starts), so behave on
the new understanding now and tell them it'll carry forward.`

// personaOnboarding is appended only when no persona is configured and the user
// hasn't yet engaged setup — the one-time opt-in offer (A099).
const personaOnboarding = `--- First-time setup ---
This person hasn't configured a persona yet, so you're running the baseline default. When it
fits naturally — not as a form to fill out — you may offer a brief setup: a couple of
questions about how they want you to steer (for example, reduce guilt vs. push ambition,
short sessions vs. deep dives), then record each answer with update_persona (pinned: true).
If they'd rather just use your default, call reset_persona with no arguments so you run the
baseline and don't keep asking.`

const listGamesBaseDescription = "List games from the library with optional filters. " +
	"Fit-to-the-moment judgment only — installed/owned state here is authoritative; " +
	"never infer ownership beyond what this returns."

// listGamesDescription carries the ambient persona.
//
// WHY HERE, of all places (finding 007): Claude Desktop does NOT honour
// ServerOptions.Instructions — verified empirically, the model reports receiving "no
// separate briefing beyond tool descriptions". So A117's ambient delivery has been
// inert since it shipped. Of the three channels, only tool descriptions are all
// three of: ambient (every description is in the model's context via tools/list),
// dynamic (served from the running server, so it can be composed from the db), and
// NOT manifest-validated (unlike prompts/get, which finding 006 proved is frozen —
// the manifest declares a short description here and Desktop delivers this long one
// without complaint).
//
// This extends an existing pattern rather than inventing one: the server's tool
// descriptions have always carried persona stance ("Fit-to-the-moment judgment
// only…", "persist sommelier judgments…"), and that is the ONLY persona that ever
// reached Desktop ambiently. list_games anchors it because the persona governs
// exactly the judgment this tool feeds: which of the user's games fits tonight.
//
// The brief is delimited and explicitly scoped to the conversation, so it reads as a
// standing brief rather than as minutiae about one tool's arguments.
func listGamesDescription(database *sql.DB) string {
	return listGamesBaseDescription +
		"\n\n--- Your standing brief as this user's Game Sommelier (applies to the whole conversation, not just this tool) ---\n\n" +
		composePersona(database)
}

// composePersona builds the persona string: the baseline briefing, then the
// configured persona (if any) as overriding deltas, then the adaptation protocol,
// and the one-time onboarding offer when unconfigured.
//
// It is read once, at server construction, so persona changes take effect at the
// next launch (true for both sinks: instructions are captured at initialize, and
// tool descriptions are fixed at AddTool). It must never fail the server: any DB
// error falls back to the bare baseline.
func composePersona(database *sql.DB) string {
	rows, err := db.GetPersona(database)
	if err != nil {
		return sommelierBriefing
	}
	var b strings.Builder
	b.WriteString(sommelierBriefing)
	if len(rows) > 0 {
		b.WriteString("\n\n--- Your configured persona ---\n")
		b.WriteString("These preferences override the defaults above where they conflict:\n")
		for _, r := range rows {
			pin := ""
			if r.Pinned {
				pin = " (pinned)"
			}
			b.WriteString(fmt.Sprintf("- %s%s: %s\n", r.Dimension, pin, r.Value))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(personaProtocol)
	if len(rows) == 0 && !db.PersonaConfigured(database) {
		b.WriteString("\n\n")
		b.WriteString(personaOnboarding)
	}
	return b.String()
}

func registerPrompts(s *mcp.Server) {
	s.AddPrompt(&mcp.Prompt{
		Name:        "sommelier",
		Description: sommelierPromptDescription,
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: sommelierBriefing},
			}},
		}, nil
	})
}

func registerResources(s *mcp.Server, database *sql.DB) {
	const uri = "gamesome://library/summary"
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
