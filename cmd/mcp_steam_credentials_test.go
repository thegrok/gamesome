package cmd

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thegrok/gamesome/internal/db"
)

const (
	validTestKey     = "0123456789ABCDEF0123456789abcdef"
	validTestSteamID = "76561190000000001"
)

func steamCredentialsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenAt(filepath.Join(t.TempDir(), "gamesome.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestSetSteamCredentials_StoresValues(t *testing.T) {
	database := steamCredentialsTestDB(t)
	t.Setenv("STEAM_API_KEY", "")
	t.Setenv("STEAM_ID", "")

	message, err := setSteamCredentials(database, validTestKey, validTestSteamID)
	if err != nil {
		t.Fatalf("setSteamCredentials: %v", err)
	}
	if got := db.GetMeta(database, db.MetaSteamAPIKey); got != validTestKey {
		t.Errorf("stored api key = %q, want %q", got, validTestKey)
	}
	if got := db.GetMeta(database, db.MetaSteamID); got != validTestSteamID {
		t.Errorf("stored steam id = %q, want %q", got, validTestSteamID)
	}
	if !strings.Contains(message, "refresh_library") {
		t.Errorf("confirmation %q should suggest refresh_library", message)
	}
	if strings.Contains(message, "precedence") {
		t.Errorf("confirmation %q should not warn about env shadowing when env is empty", message)
	}
}

func TestSetSteamCredentials_TrimsWhitespace(t *testing.T) {
	database := steamCredentialsTestDB(t)
	t.Setenv("STEAM_API_KEY", "")
	t.Setenv("STEAM_ID", "")

	if _, err := setSteamCredentials(database, "  "+validTestKey+"\n", "\t"+validTestSteamID+" "); err != nil {
		t.Fatalf("setSteamCredentials: %v", err)
	}
	if got := db.GetMeta(database, db.MetaSteamAPIKey); got != validTestKey {
		t.Errorf("stored api key = %q, want trimmed %q", got, validTestKey)
	}
	if got := db.GetMeta(database, db.MetaSteamID); got != validTestSteamID {
		t.Errorf("stored steam id = %q, want trimmed %q", got, validTestSteamID)
	}
}

func TestSetSteamCredentials_RejectsBadKey(t *testing.T) {
	database := steamCredentialsTestDB(t)
	for _, bad := range []string{
		"0123456789ABCDEF0123456789abcde",  // 31 chars
		"0123456789ABCDEF0123456789abcdeg", // non-hex
		"",
	} {
		if _, err := setSteamCredentials(database, bad, validTestSteamID); err == nil {
			t.Errorf("key %q: want error, got nil", bad)
		}
	}
	if got := db.GetMeta(database, db.MetaSteamAPIKey); got != "" {
		t.Errorf("rejected key must not be stored, got %q", got)
	}
	if got := db.GetMeta(database, db.MetaSteamID); got != "" {
		t.Errorf("steam id must not be stored on rejection, got %q", got)
	}
}

func TestSetSteamCredentials_RejectsBadSteamID(t *testing.T) {
	database := steamCredentialsTestDB(t)
	for _, bad := range []string{
		"gaben",            // vanity name
		"7656119000000000", // 16 digits
		"",
	} {
		if _, err := setSteamCredentials(database, validTestKey, bad); err == nil {
			t.Errorf("steam id %q: want error, got nil", bad)
		}
	}
	if got := db.GetMeta(database, db.MetaSteamAPIKey); got != "" {
		t.Errorf("api key must not be stored on rejection, got %q", got)
	}
	if got := db.GetMeta(database, db.MetaSteamID); got != "" {
		t.Errorf("rejected steam id must not be stored, got %q", got)
	}
}

func TestSetSteamCredentials_EnvShadowWarning(t *testing.T) {
	database := steamCredentialsTestDB(t)
	t.Setenv("STEAM_API_KEY", "envkey")
	t.Setenv("STEAM_ID", "")

	message, err := setSteamCredentials(database, validTestKey, validTestSteamID)
	if err != nil {
		t.Fatalf("setSteamCredentials: %v", err)
	}
	if !strings.Contains(message, "precedence") {
		t.Errorf("confirmation %q should warn that env vars take precedence", message)
	}
}
