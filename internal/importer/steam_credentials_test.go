package importer

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/thegrok/gamesom/internal/db"
)

func credentialsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenAt(filepath.Join(t.TempDir(), "gamesom.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func setMetaCredentials(t *testing.T, database *sql.DB, apiKey, steamID string) {
	t.Helper()
	if err := db.SetMeta(database, db.MetaSteamAPIKey, apiKey); err != nil {
		t.Fatalf("set meta api key: %v", err)
	}
	if err := db.SetMeta(database, db.MetaSteamID, steamID); err != nil {
		t.Fatalf("set meta steam id: %v", err)
	}
}

func TestSteamCredentials_EnvWins(t *testing.T) {
	database := credentialsTestDB(t)
	setMetaCredentials(t, database, "metakey", "76561190000000001")
	t.Setenv("STEAM_API_KEY", "envkey")
	t.Setenv("STEAM_ID", "76561190000000002")

	apiKey, steamID := steamCredentials(database)
	if apiKey != "envkey" || steamID != "76561190000000002" {
		t.Errorf("got (%q, %q), want env values to win", apiKey, steamID)
	}
}

func TestSteamCredentials_MetaFallback(t *testing.T) {
	database := credentialsTestDB(t)
	setMetaCredentials(t, database, "metakey", "76561190000000001")
	t.Setenv("STEAM_API_KEY", "")
	t.Setenv("STEAM_ID", "")

	apiKey, steamID := steamCredentials(database)
	if apiKey != "metakey" || steamID != "76561190000000001" {
		t.Errorf("got (%q, %q), want meta values", apiKey, steamID)
	}
}

func TestSteamCredentials_PerValueMix(t *testing.T) {
	database := credentialsTestDB(t)
	setMetaCredentials(t, database, "metakey", "76561190000000001")
	t.Setenv("STEAM_API_KEY", "envkey")
	t.Setenv("STEAM_ID", "")

	apiKey, steamID := steamCredentials(database)
	if apiKey != "envkey" || steamID != "76561190000000001" {
		t.Errorf("got (%q, %q), want env key + meta id", apiKey, steamID)
	}
}

func TestSteamCredentials_EmptyWhenNeither(t *testing.T) {
	database := credentialsTestDB(t)
	t.Setenv("STEAM_API_KEY", "")
	t.Setenv("STEAM_ID", "")

	apiKey, steamID := steamCredentials(database)
	if apiKey != "" || steamID != "" {
		t.Errorf("got (%q, %q), want empty values", apiKey, steamID)
	}
}
