package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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
`

// Open returns a ready-to-use DB, creating the data directory and schema if needed.
func Open() (*sql.DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	path := filepath.Join(dir, "gamesom.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return db, nil
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

func dataDir() (string, error) {
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg != "" {
		return filepath.Join(xdg, "gamesom"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "gamesom"), nil
}
