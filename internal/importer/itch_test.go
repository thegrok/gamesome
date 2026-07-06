package importer

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// fixtureButlerDB creates a minimal butler.db with just the columns
// itchOwnedGames reads. The modernc sqlite driver is registered via the
// internal/db import.
func fixtureButlerDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "butler.db")
	butlerDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture butler db: %v", err)
	}
	t.Cleanup(func() { butlerDB.Close() })

	schema := `
		CREATE TABLE games (id INTEGER PRIMARY KEY, title TEXT, url TEXT, classification TEXT);
		CREATE TABLE download_keys (id INTEGER PRIMARY KEY, game_id INTEGER);`
	if _, err := butlerDB.Exec(schema); err != nil {
		t.Fatalf("create fixture schema: %v", err)
	}
	return butlerDB
}

func TestItchOwnedGames_DedupesMultipleKeys(t *testing.T) {
	butlerDB := fixtureButlerDB(t)

	inserts := `
		INSERT INTO games VALUES
			(118243, 'Bundle Game', 'https://example.itch.io/bundle-game', 'game'),
			(200, 'Single Key Game', 'https://example.itch.io/single', 'game'),
			(300, 'Some Soundtrack', 'https://example.itch.io/ost', 'soundtrack');
		INSERT INTO download_keys (game_id) VALUES
			(118243), (118243),
			(200),
			(300);`
	if _, err := butlerDB.Exec(inserts); err != nil {
		t.Fatalf("insert fixture rows: %v", err)
	}

	games, err := itchOwnedGames(butlerDB)
	if err != nil {
		t.Fatalf("itchOwnedGames: %v", err)
	}

	if len(games) != 2 {
		t.Fatalf("got %d games, want 2 (multi-key game deduped, non-game excluded): %+v", len(games), games)
	}
	seen := make(map[int64]int)
	for _, g := range games {
		seen[g.ID]++
	}
	if seen[118243] != 1 {
		t.Errorf("game 118243 (two download keys) appeared %d times, want exactly 1", seen[118243])
	}
	if seen[200] != 1 {
		t.Errorf("game 200 appeared %d times, want exactly 1", seen[200])
	}
	if seen[300] != 0 {
		t.Errorf("game 300 (classification 'soundtrack') should be excluded, appeared %d times", seen[300])
	}
}
