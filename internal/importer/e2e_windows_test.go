//go:build windows && e2e

// Package importer e2e tests run against the REAL launcher installs on a Windows
// box (gamesome's primary platform). They are gated behind both the `windows` and
// `e2e` build tags, so they never run under the default `go test ./...`; invoke
// them explicitly with:
//
//	go test -tags e2e ./internal/importer/
//
// Installed-state ground truth only the human at the machine knows is
// declared per source via E2E_EXPECT_INSTALLED_<SOURCE> (STEAM, ITCHIO, GOG,
// EPIC): unset, a non-empty import reporting zero installed FAILS; set to N,
// the installed count must equal exactly N, e.g.
//
//	E2E_EXPECT_INSTALLED_GOG=0 go test -tags e2e ./internal/importer/
//
// They assert invariants that hold for ANY real library rather than a frozen
// expected answer, so they survive the maintainer installing/uninstalling games
// and never need re-baselining. This file is `package importer` (an internal
// test) on purpose: it reuses the unexported path resolvers and parsers
// (steamAppsDirectories, isSteamTool, parseACF) so the Steam count invariant is
// computed exactly the way the importer sees the filesystem.
package importer

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/thegrok/gamesome/internal/db"
	"github.com/thegrok/gamesome/internal/normalize"
)

// newTestDB opens a throwaway DB in a temp dir so a test run never touches the
// user's real gamesome.db.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenAt(filepath.Join(t.TempDir(), "e2e.db"))
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// assertLibraryInvariants checks the properties that must hold for every importer
// against a real, non-empty library.
func assertLibraryInvariants(t *testing.T, database *sql.DB, source string) {
	t.Helper()

	rows, err := database.Query(`
		SELECT le.source_game_id, le.source_title, le.installed, le.install_path,
		       g.canonical_title, g.normalized_title
		FROM library_entries le
		JOIN games g ON g.id = le.game_id
		WHERE le.source = ?`, source)
	if err != nil {
		t.Fatalf("[%s] query library entries: %v", source, err)
	}
	defer rows.Close()

	count := 0
	installedCount := 0
	for rows.Next() {
		var sourceGameID, sourceTitle, canonical, normalized string
		var installPath sql.NullString
		var installed int
		if err := rows.Scan(&sourceGameID, &sourceTitle, &installed, &installPath, &canonical, &normalized); err != nil {
			t.Fatalf("[%s] scan row: %v", source, err)
		}
		count++

		if sourceGameID == "" {
			t.Errorf("[%s] row %q has empty source_game_id", source, sourceTitle)
		}
		if sourceTitle == "" {
			t.Errorf("[%s] row with source_game_id %q has empty source_title", source, sourceGameID)
		}
		if canonical == "" {
			t.Errorf("[%s] row %q has empty canonical_title", source, sourceGameID)
		}
		if normalized == "" {
			t.Errorf("[%s] row %q has empty normalized_title", source, sourceGameID)
		}
		if installed == 1 {
			installedCount++
			if !installPath.Valid || installPath.String == "" {
				t.Errorf("[%s] row %q marked installed but install_path is empty", source, sourceTitle)
			} else if _, err := os.Stat(installPath.String); err != nil {
				t.Errorf("[%s] row %q install_path %q does not exist: %v", source, sourceTitle, installPath.String, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("[%s] row iteration: %v", source, err)
	}

	if count == 0 {
		t.Errorf("[%s] importer produced 0 library entries (expected a non-empty real library)", source)
	}

	// Installed-state accuracy: a source that silently under-reports (every
	// row installed=0) must be indistinguishable from nothing-installed only
	// when a human says so. E2E_EXPECT_INSTALLED_<SOURCE> pins the count the
	// human can see in the launcher; unset, zero installed on a non-empty
	// import is a loud failure (the A100/A102 class of bug).
	expectEnv := "E2E_EXPECT_INSTALLED_" + strings.ToUpper(source)
	declared := "undeclared"
	if v := os.Getenv(expectEnv); v != "" {
		want, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("[%s] %s=%q is not an integer", source, expectEnv, v)
		}
		declared = "declared=" + v
		if installedCount != want {
			t.Errorf("[%s] installed count = %d, but %s declares %d", source, installedCount, expectEnv, want)
		}
	} else if count > 0 && installedCount == 0 {
		t.Errorf("[%s] non-empty import (%d rows) reports 0 installed — an under-reporting importer is indistinguishable from an empty machine; if genuinely nothing is installed for this source, declare it: set %s=0", source, count, expectEnv)
	}

	t.Logf("[%s] rows=%d installed=%d %s=%s", source, count, installedCount, expectEnv, declared)

	// No duplicate (source, source_game_id). The DB UNIQUE constraint should make
	// this impossible; asserting it documents the invariant and guards a future
	// schema change.
	dupRows, err := database.Query(`
		SELECT source_game_id, COUNT(*) FROM library_entries
		WHERE source = ? GROUP BY source_game_id HAVING COUNT(*) > 1`, source)
	if err != nil {
		t.Fatalf("[%s] duplicate query: %v", source, err)
	}
	defer dupRows.Close()
	for dupRows.Next() {
		var sgid string
		var n int
		if err := dupRows.Scan(&sgid, &n); err != nil {
			t.Fatalf("[%s] duplicate scan: %v", source, err)
		}
		t.Errorf("[%s] duplicate source_game_id %q appears %d times", source, sgid, n)
	}
}

func TestE2E_Steam(t *testing.T) {
	// Local-scan only: force the web API path off so we exercise appmanifest
	// parsing, not api.steampowered.com (out of scope this pass).
	t.Setenv("STEAM_API_KEY", "")
	t.Setenv("STEAM_ID", "")

	database := newTestDB(t)
	if err := Steam(database); err != nil {
		t.Fatalf("Steam import: %v", err)
	}
	assertLibraryInvariants(t, database, "steam")

	// Strongest check: imported steam rows == distinct non-tool appmanifest
	// entries, computed with the same helpers the importer uses.
	expected := expectedSteamAppIDs(t)
	var got int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM library_entries WHERE source = 'steam'`).Scan(&got); err != nil {
		t.Fatalf("count steam rows: %v", err)
	}
	if got != len(expected) {
		t.Errorf("steam row count = %d, want %d (distinct non-tool appmanifests)", got, len(expected))
	}
}

// expectedSteamAppIDs replicates steamManifestScan's discovery + filters to
// produce the set of appids that should each yield exactly one library row.
func expectedSteamAppIDs(t *testing.T) map[string]struct{} {
	t.Helper()
	expected := make(map[string]struct{})
	for _, dir := range steamAppsDirectories() {
		matches, err := filepath.Glob(filepath.Join(dir, "appmanifest_*.acf"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		for _, path := range matches {
			fields, err := parseACF(path)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			appIDStr := fields["appid"]
			name := fields["name"]
			if appIDStr == "" || name == "" || isSteamTool(name) {
				continue
			}
			if _, err := strconv.ParseInt(appIDStr, 10, 64); err != nil {
				continue
			}
			if normalize.Title(name) == "" {
				continue
			}
			expected[appIDStr] = struct{}{}
		}
	}
	return expected
}

func TestE2E_Itch(t *testing.T) {
	database := newTestDB(t)
	if err := Itch(database); err != nil {
		t.Fatalf("Itch import: %v", err)
	}
	assertLibraryInvariants(t, database, "itchio")
}

func TestE2E_GOG(t *testing.T) {
	database := newTestDB(t)
	if err := GOG(database); err != nil {
		t.Fatalf("GOG import: %v", err)
	}
	assertLibraryInvariants(t, database, "gog")
}

func TestE2E_Epic(t *testing.T) {
	database := newTestDB(t)
	if err := Epic(database); err != nil {
		t.Fatalf("Epic import: %v", err)
	}
	assertLibraryInvariants(t, database, "epic")
}
