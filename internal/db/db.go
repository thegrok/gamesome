package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS games (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    canonical_title TEXT NOT NULL,
    normalized_title TEXT NOT NULL UNIQUE,
    igdb_id INTEGER,
    steam_appid INTEGER,
    first_release_date INTEGER,
    genres TEXT,
    themes TEXT,
    summary TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS library_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id INTEGER NOT NULL REFERENCES games(id),
    source TEXT NOT NULL,
    source_game_id TEXT,
    source_title TEXT NOT NULL,
    owned INTEGER NOT NULL DEFAULT 1,
    installed INTEGER NOT NULL DEFAULT 0,
    install_path TEXT,
    launcher_uri TEXT,
    playtime_minutes INTEGER DEFAULT 0,
    last_played_at DATETIME,
    imported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source, source_game_id)
);

CREATE TABLE IF NOT EXISTS sommelier_profile (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id INTEGER NOT NULL UNIQUE REFERENCES games(id),
    energy_required TEXT,
    friction_level TEXT,
    session_length_fit TEXT,
    narrative_memory_load TEXT,
    complexity_level TEXT,
    mood_tags TEXT,
    avoid_when TEXT,
    best_when TEXT,
    user_notes TEXT
);

CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS persona (
    dimension  TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    pinned     INTEGER NOT NULL DEFAULT 0,
    source     TEXT NOT NULL DEFAULT 'adaptive',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Append-only: a row per import observation, never updated or upserted.
-- Diffing consecutive rows for a (source, source_game_id) yields
-- minutes-played-in-the-interval, since Steam's playtime_forever is
-- cumulative (A162, Phase 5a). No FK to library_entries — keyed by the same
-- (source, source_game_id) pair library_entries' own UNIQUE constraint uses.
CREATE TABLE IF NOT EXISTS library_observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    source_game_id TEXT NOT NULL,
    observed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    playtime_minutes INTEGER NOT NULL
);
`

// Meta keys for the Steam Web API credentials stored in-conversation via the
// set_steam_credentials MCP tool. Env vars STEAM_API_KEY/STEAM_ID take
// precedence over these at import time.
const (
	MetaSteamAPIKey = "steam_api_key"
	MetaSteamID     = "steam_id"
)

// MetaPersonaConfigured records that the user has engaged persona setup at all
// (via update_persona or reset_persona). Once "1", the sommelier stops auto-
// offering the opt-in interview — the user can still start it on demand. See
// A099 (game-sommelier/spec/features/persona-config).
const MetaPersonaConfigured = "persona_configured"

// ErrPersonaPinned is returned by UpsertPersona when an adaptive write (one that
// carries no explicit pin decision) tries to overwrite a pinned dimension. This
// is the structural half of A099's drift guardrail: the sommelier's in-band,
// confirm-gated adaptation physically cannot erase a dimension the user pinned.
var ErrPersonaPinned = errors.New("dimension is pinned; an explicit pin decision is required to change it")

// Open returns a ready-to-use DB at the default data location, creating the data
// directory and schema if needed.
func Open() (*sql.DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	legacyIdiomatic, err := legacyIdiomaticDataDir()
	if err != nil {
		return nil, err
	}
	legacyPreA096, err := legacyDataDir()
	if err != nil {
		return nil, err
	}
	return OpenAt(resolveDBPath(dir, []string{legacyIdiomatic, legacyPreA096}))
}

// OpenAt returns a ready-to-use DB at the given path, creating the parent
// directory and applying the schema + enrich migrations. Open is OpenAt at the
// default data location; tests use OpenAt with a throwaway path so they never
// touch the user's real database.
func OpenAt(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	// The db carries the Steam Web API key in plain text (A126) — restrict it
	// regardless of umask. Best-effort: a permission error here shouldn't stop
	// the app from starting.
	if err := os.Chmod(path, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not restrict db file permissions: %v\n", err)
	}

	migrateEnrichColumns(db)
	return db, nil
}

// migrateEnrichColumns adds enrichment columns idempotently.
func migrateEnrichColumns(db *sql.DB) {
	for _, stmt := range []string{
		"ALTER TABLE games ADD COLUMN metacritic_score INTEGER",
		"ALTER TABLE games ADD COLUMN linux_native INTEGER",
		"ALTER TABLE games ADD COLUMN enriched_at DATETIME",
		"ALTER TABLE games ADD COLUMN completed_at DATETIME",
	} {
		db.Exec(stmt) // ignore error — "duplicate column" is expected on subsequent runs
	}
}

// SetSteamAppID sets the steam_appid for a game row.
func SetSteamAppID(db *sql.DB, gameID, appID int64) error {
	_, err := db.Exec(`UPDATE games SET steam_appid = ? WHERE id = ?`, appID, gameID)
	return err
}

// GameStub is a minimal game row used for enrichment queries.
type GameStub struct {
	ID              int64
	NormalizedTitle string
	CanonicalTitle  string
}

// GamesWithoutSteamAppID returns all games that have no steam_appid set.
func GamesWithoutSteamAppID(db *sql.DB) ([]GameStub, error) {
	rows, err := db.Query(`SELECT id, normalized_title, canonical_title FROM games WHERE steam_appid IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameStub
	for rows.Next() {
		var g GameStub
		if err := rows.Scan(&g.ID, &g.NormalizedTitle, &g.CanonicalTitle); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// GameWithAppID is a game row that has a steam_appid.
type GameWithAppID struct {
	ID         int64
	SteamAppID int64
	Title      string
}

// GamesWithSteamAppID returns all games that have a steam_appid and haven't been enriched yet.
func GamesWithSteamAppID(db *sql.DB) ([]GameWithAppID, error) {
	rows, err := db.Query(
		`SELECT id, steam_appid, canonical_title FROM games WHERE steam_appid IS NOT NULL AND enriched_at IS NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameWithAppID
	for rows.Next() {
		var g GameWithAppID
		if err := rows.Scan(&g.ID, &g.SteamAppID, &g.Title); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// GameForTraits is a game row with metadata needed for LLM trait inference.
type GameForTraits struct {
	ID     int64
	Title  string
	Genres string
	Themes string
	Summary string
}

// GamesNeedingTraits returns games that have no sommelier_profile row yet.
func GamesNeedingTraits(db *sql.DB) ([]GameForTraits, error) {
	rows, err := db.Query(`
		SELECT g.id, g.canonical_title,
		       COALESCE(g.genres, ''), COALESCE(g.themes, ''), COALESCE(g.summary, '')
		FROM games g
		WHERE NOT EXISTS (SELECT 1 FROM sommelier_profile sp WHERE sp.game_id = g.id)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameForTraits
	for rows.Next() {
		var g GameForTraits
		if err := rows.Scan(&g.ID, &g.Title, &g.Genres, &g.Themes, &g.Summary); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// UpdateGameMetadata stores Steam Store enrichment results on a game row.
func UpdateGameMetadata(db *sql.DB, gameID int64, genres, themes, summary string, metacritic *int, releaseDate *int64, linuxNative *bool) error {
	_, err := db.Exec(`
		UPDATE games SET
			genres = CASE WHEN ? != '' THEN ? ELSE genres END,
			themes = CASE WHEN ? != '' THEN ? ELSE themes END,
			summary = CASE WHEN ? != '' THEN ? ELSE summary END,
			metacritic_score = COALESCE(?, metacritic_score),
			first_release_date = COALESCE(?, first_release_date),
			linux_native = COALESCE(?, linux_native),
			enriched_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, genres, genres, themes, themes, summary, summary, metacritic, releaseDate, linuxNativeInt(linuxNative), gameID)
	return err
}

// MarkEnrichedAt sets enriched_at without updating any metadata (for failed/delisted games).
func MarkEnrichedAt(db *sql.DB, gameID int64) error {
	_, err := db.Exec(`UPDATE games SET enriched_at = CURRENT_TIMESTAMP WHERE id = ?`, gameID)
	return err
}

// UpsertSommelierProfile inserts or replaces the sommelier_profile for a game.
func UpsertSommelierProfile(db *sql.DB, gameID int64, p SommelierProfile) error {
	_, err := db.Exec(`
		INSERT INTO sommelier_profile
			(game_id, energy_required, friction_level, session_length_fit,
			 narrative_memory_load, complexity_level, mood_tags, avoid_when, best_when)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(game_id) DO UPDATE SET
			energy_required      = excluded.energy_required,
			friction_level       = excluded.friction_level,
			session_length_fit   = excluded.session_length_fit,
			narrative_memory_load= excluded.narrative_memory_load,
			complexity_level     = excluded.complexity_level,
			mood_tags            = excluded.mood_tags,
			avoid_when           = excluded.avoid_when,
			best_when            = excluded.best_when
	`, gameID, p.EnergyRequired, p.FrictionLevel, p.SessionLengthFit,
		p.NarrativeMemoryLoad, p.ComplexityLevel, p.MoodTags, p.AvoidWhen, p.BestWhen)
	return err
}

// SommelierProfile holds inferred trait data for a game.
type SommelierProfile struct {
	EnergyRequired      string `json:"energy_required"`
	FrictionLevel       string `json:"friction_level"`
	SessionLengthFit    string `json:"session_length_fit"`
	NarrativeMemoryLoad string `json:"narrative_memory_load"`
	ComplexityLevel     string `json:"complexity_level"`
	MoodTags            string `json:"mood_tags"`
	AvoidWhen           string `json:"avoid_when"`
	BestWhen            string `json:"best_when"`
}

func linuxNativeInt(b *bool) *int {
	if b == nil {
		return nil
	}
	v := 0
	if *b {
		v = 1
	}
	return &v
}

// UpsertGame inserts a game by normalized title or returns the existing id.
// Never overwrites canonical info already set by another source.
func UpsertGame(db *sql.DB, canonicalTitle, normalizedTitle string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO games (canonical_title, normalized_title) VALUES (?, ?)
         ON CONFLICT(normalized_title) DO NOTHING`,
		canonicalTitle, normalizedTitle,
	)
	if err != nil {
		return 0, err
	}

	id, _ := res.LastInsertId()
	if id > 0 {
		return id, nil
	}

	// Already existed — fetch the id.
	var existing int64
	err = db.QueryRow(`SELECT id FROM games WHERE normalized_title = ?`, normalizedTitle).Scan(&existing)
	return existing, err
}

// UpsertLibraryEntry inserts or updates a library entry.
func UpsertLibraryEntry(db *sql.DB, e LibraryEntry) error {
	_, err := db.Exec(`
		INSERT INTO library_entries
			(game_id, source, source_game_id, source_title, owned, installed, install_path, launcher_uri, playtime_minutes, last_played_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source, source_game_id) DO UPDATE SET
			installed      = excluded.installed,
			install_path   = excluded.install_path,
			playtime_minutes = excluded.playtime_minutes,
			last_played_at = excluded.last_played_at,
			imported_at    = CURRENT_TIMESTAMP
	`,
		e.GameID, e.Source, e.SourceGameID, e.SourceTitle,
		e.Owned, e.Installed, e.InstallPath, e.LauncherURI,
		e.PlaytimeMinutes, e.LastPlayedAt,
	)
	return err
}

// RecordPlaytimeObservation appends a playtime snapshot for later diffing
// (A162, Phase 5a). Best-effort: an error here must never fail the import
// that triggered it — this is instrumentation, not import correctness.
func RecordPlaytimeObservation(db *sql.DB, source, sourceGameID string, playtimeMinutes int) {
	if _, err := db.Exec(
		`INSERT INTO library_observations (source, source_game_id, playtime_minutes) VALUES (?, ?, ?)`,
		source, sourceGameID, playtimeMinutes,
	); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record playtime observation for %s/%s: %v\n", source, sourceGameID, err)
	}
}

// UpdateInstalledBySteamAppID marks a steam entry as installed by appid.
func UpdateInstalledBySteamAppID(db *sql.DB, appID int64, installPath string) error {
	_, err := db.Exec(`
		UPDATE library_entries SET installed = 1, install_path = ?, imported_at = CURRENT_TIMESTAMP
		WHERE source = 'steam' AND source_game_id = ?
	`, installPath, fmt.Sprintf("%d", appID))
	return err
}

// SetMeta stores a key/value in the meta table.
func SetMeta(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// GetMeta retrieves a value from meta; returns "" if not found.
func GetMeta(db *sql.DB, key string) string {
	var v string
	db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	return v
}

// PersonaDimension is one configured stance of the sommelier persona: a delta on
// top of the static baseline briefing. See A099.
type PersonaDimension struct {
	Dimension string `json:"dimension"`
	Value     string `json:"value"`
	Pinned    bool   `json:"pinned"`
	Source    string `json:"source"`
}

// GetPersona returns all configured persona dimensions, ordered by dimension.
// An empty slice means the user runs the baseline default (no deltas).
func GetPersona(db *sql.DB) ([]PersonaDimension, error) {
	rows, err := db.Query(`SELECT dimension, value, pinned, source FROM persona ORDER BY dimension`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PersonaDimension
	for rows.Next() {
		var p PersonaDimension
		var pinned int
		if err := rows.Scan(&p.Dimension, &p.Value, &pinned, &p.Source); err != nil {
			return nil, err
		}
		p.Pinned = pinned == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpsertPersona writes one persona dimension. The pinned argument encodes intent:
//   - pinned == nil is an adaptive / undirected write. It is REFUSED with
//     ErrPersonaPinned if the existing row is pinned, and stored with source
//     "adaptive" otherwise.
//   - pinned != nil is a deliberate user-directed pin decision (interview or an
//     explicit change) and may overwrite a pinned row; stored with source "user".
//
// Any successful write marks the persona configured, so onboarding stops nagging.
func UpsertPersona(db *sql.DB, dimension, value string, pinned *bool) error {
	dimension = strings.TrimSpace(dimension)
	value = strings.TrimSpace(value)
	if dimension == "" || value == "" {
		return errors.New("persona dimension and value are both required")
	}

	var existingPinned int
	err := db.QueryRow(`SELECT pinned FROM persona WHERE dimension = ?`, dimension).Scan(&existingPinned)
	switch {
	case err == sql.ErrNoRows:
		// new dimension — no guard applies
	case err != nil:
		return err
	case existingPinned == 1 && pinned == nil:
		return ErrPersonaPinned
	}

	newPinned := 0
	source := "adaptive"
	if pinned != nil {
		source = "user"
		if *pinned {
			newPinned = 1
		}
	} else if existingPinned == 1 {
		newPinned = 1 // unreachable given the guard above, but keeps the value honest
	}

	if _, err := db.Exec(`
		INSERT INTO persona (dimension, value, pinned, source, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(dimension) DO UPDATE SET
			value      = excluded.value,
			pinned     = excluded.pinned,
			source     = excluded.source,
			updated_at = CURRENT_TIMESTAMP`,
		dimension, value, newPinned, source); err != nil {
		return err
	}
	return SetMeta(db, MetaPersonaConfigured, "1")
}

// ResetPersona clears the configured persona back to baseline: one dimension when
// dimension != "", or the whole persona when dimension == "". It marks the persona
// configured (a deliberate reset / interview decline shouldn't re-trigger the
// onboarding offer). Returns the number of dimensions cleared.
func ResetPersona(db *sql.DB, dimension string) (int64, error) {
	dimension = strings.TrimSpace(dimension)
	var res sql.Result
	var err error
	if dimension == "" {
		res, err = db.Exec(`DELETE FROM persona`)
	} else {
		res, err = db.Exec(`DELETE FROM persona WHERE dimension = ?`, dimension)
	}
	if err != nil {
		return 0, err
	}
	if err := SetMeta(db, MetaPersonaConfigured, "1"); err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PersonaConfigured reports whether the user has engaged persona setup at all.
func PersonaConfigured(db *sql.DB) bool {
	return GetMeta(db, MetaPersonaConfigured) == "1"
}

// LibraryEntry is the data needed for an upsert.
type LibraryEntry struct {
	GameID          int64
	Source          string
	SourceGameID    string
	SourceTitle     string
	Owned           int
	Installed       int
	InstallPath     string
	LauncherURI     string
	PlaytimeMinutes int
	LastPlayedAt    *string
}

// dataDir returns the platform-idiomatic gamesome data directory.
// XDG_DATA_HOME overrides on every OS — it's also the hermetic-test hook.
func dataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gamesome"), nil
	}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA not set")
		}
		return filepath.Join(localAppData, "gamesome"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "gamesome"), nil
	default: // linux and friends
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "gamesome"), nil
	}
}

// DataDir exposes the platform data directory for siblings that store
// operational files (e.g. the mcp --debug-env dump) beside the DB.
func DataDir() (string, error) { return dataDir() }

// legacyIdiomaticDataDir returns the A096-era idiomatic-per-OS path, still
// named "gamesom" (pre-A112 rename). Migration source only.
func legacyIdiomaticDataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gamesom"), nil
	}
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA not set")
		}
		return filepath.Join(localAppData, "gamesom"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "gamesom"), nil
	default: // linux and friends
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "gamesom"), nil
	}
}

// legacyDataDir is the pre-A096 data location on every OS: XDG semantics
// hardcoded, so Windows/macOS DBs landed under ~/.local/share/gamesom. The
// oldest of the two legacy candidates resolveDBPath checks.
func legacyDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "gamesom"), nil
}

// resolveDBPath returns the DB path under dir, first moving a legacy DB
// (and its -wal/-shm sidecars) from the first legacyDirs entry that has
// one — callers pass newest-legacy-first so a DB already sitting at a more
// recent legacy location isn't skipped in favor of an older one. If the
// move fails, it returns the legacy path — opening data where it lives
// beats losing it to an idiomatic location.
func resolveDBPath(dir string, legacyDirs []string) string {
	newPath := filepath.Join(dir, "gamesome.db")
	if _, err := os.Stat(newPath); err == nil {
		return newPath // idiomatic DB already exists; legacy (if any) is stale
	}
	for _, legacyDir := range legacyDirs {
		legacyPath := filepath.Join(legacyDir, "gamesom.db")
		if _, err := os.Stat(legacyPath); err != nil {
			continue // nothing here, try the next candidate
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Printf("warning: create data dir %s failed (%v); using legacy %s", dir, err, legacyPath)
			return legacyPath
		}
		if err := os.Rename(legacyPath, newPath); err != nil {
			log.Printf("warning: migrate db %s → %s failed (%v); using legacy path", legacyPath, newPath, err)
			return legacyPath
		}
		for _, ext := range []string{"-wal", "-shm"} {
			if _, err := os.Stat(legacyPath + ext); err == nil {
				if err := os.Rename(legacyPath+ext, newPath+ext); err != nil {
					log.Printf("warning: migrate sidecar %s failed: %v", legacyPath+ext, err)
				}
			}
		}
		log.Printf("migrated db: %s → %s", legacyPath, newPath)
		return newPath
	}
	return newPath
}
