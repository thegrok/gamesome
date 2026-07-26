package db

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestRecordPlaytimeObservation_AppendsRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gamesome.db")
	database, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer database.Close()

	RecordPlaytimeObservation(database, "steam", "440", 100)
	RecordPlaytimeObservation(database, "steam", "440", 145)

	rows, err := database.Query(
		`SELECT playtime_minutes FROM library_observations WHERE source = ? AND source_game_id = ? ORDER BY id`,
		"steam", "440",
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []int
	for rows.Next() {
		var minutes int
		if err := rows.Scan(&minutes); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, minutes)
	}

	want := []int{100, 145}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("observations = %v, want %v (should append, not upsert)", got, want)
	}
}

func TestResolveDBPath_MigratesLegacy(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "legacy")
	writeFile(t, filepath.Join(legacyDir, "gamesom.db"), "db")
	writeFile(t, filepath.Join(legacyDir, "gamesom.db-wal"), "wal")

	got := resolveDBPath(dir, []string{legacyDir})

	want := filepath.Join(dir, "gamesome.db")
	if got != want {
		t.Fatalf("resolveDBPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("migrated db missing at %s: %v", want, err)
	}
	if _, err := os.Stat(want + "-wal"); err != nil {
		t.Errorf("migrated wal sidecar missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "gamesom.db")); !os.IsNotExist(err) {
		t.Errorf("legacy db still present after migration (stat err: %v)", err)
	}
}

func TestResolveDBPath_NewAlreadyExists(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "legacy")
	writeFile(t, filepath.Join(dir, "gamesome.db"), "current")
	writeFile(t, filepath.Join(legacyDir, "gamesom.db"), "stale")

	got := resolveDBPath(dir, []string{legacyDir})

	want := filepath.Join(dir, "gamesome.db")
	if got != want {
		t.Fatalf("resolveDBPath = %q, want %q", got, want)
	}
	content, err := os.ReadFile(filepath.Join(legacyDir, "gamesom.db"))
	if err != nil || string(content) != "stale" {
		t.Errorf("legacy db should be untouched when new db exists (content %q, err %v)", content, err)
	}
	content, err = os.ReadFile(want)
	if err != nil || string(content) != "current" {
		t.Errorf("existing new db must never be clobbered (content %q, err %v)", content, err)
	}
}

func TestResolveDBPath_NothingToMigrate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "legacy")

	got := resolveDBPath(dir, []string{legacyDir})

	want := filepath.Join(dir, "gamesome.db")
	if got != want {
		t.Fatalf("resolveDBPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Errorf("legacy dir should not be created as a side effect (stat err: %v)", err)
	}
}

func TestResolveDBPath_MigratesInPlace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "gamesom.db"), "db")

	got := resolveDBPath(dir, []string{dir})

	want := filepath.Join(dir, "gamesome.db")
	if got != want {
		t.Fatalf("resolveDBPath = %q, want %q", got, want)
	}
	content, err := os.ReadFile(want)
	if err != nil || string(content) != "db" {
		t.Errorf("db must be migrated in place when dir == legacyDir (content %q, err %v)", content, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gamesom.db")); !os.IsNotExist(err) {
		t.Errorf("legacy db still present after in-place migration (stat err: %v)", err)
	}
}

func TestResolveDBPath_ChecksLegacyDirsInOrder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "new")
	newerLegacy := filepath.Join(root, "legacy-newer")
	olderLegacy := filepath.Join(root, "legacy-older")
	writeFile(t, filepath.Join(olderLegacy, "gamesom.db"), "oldest")

	got := resolveDBPath(dir, []string{newerLegacy, olderLegacy})

	want := filepath.Join(dir, "gamesome.db")
	if got != want {
		t.Fatalf("resolveDBPath = %q, want %q", got, want)
	}
	content, err := os.ReadFile(want)
	if err != nil || string(content) != "oldest" {
		t.Errorf("should fall through to the second legacy dir when the first has nothing (content %q, err %v)", content, err)
	}
}

func TestDataDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join("/", "custom", "share"))

	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	want := filepath.Join("/", "custom", "share", "gamesome")
	if got != want {
		t.Errorf("dataDir with XDG_DATA_HOME = %q, want %q", got, want)
	}
}
