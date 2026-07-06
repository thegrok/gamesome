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

func TestResolveDBPath_MigratesLegacy(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "legacy")
	writeFile(t, filepath.Join(legacyDir, "gamesom.db"), "db")
	writeFile(t, filepath.Join(legacyDir, "gamesom.db-wal"), "wal")

	got := resolveDBPath(dir, legacyDir)

	want := filepath.Join(dir, "gamesom.db")
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
	writeFile(t, filepath.Join(dir, "gamesom.db"), "current")
	writeFile(t, filepath.Join(legacyDir, "gamesom.db"), "stale")

	got := resolveDBPath(dir, legacyDir)

	want := filepath.Join(dir, "gamesom.db")
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

	got := resolveDBPath(dir, legacyDir)

	want := filepath.Join(dir, "gamesom.db")
	if got != want {
		t.Fatalf("resolveDBPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Errorf("legacy dir should not be created as a side effect (stat err: %v)", err)
	}
}

func TestResolveDBPath_SamePath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "gamesom.db"), "db")

	got := resolveDBPath(dir, dir)

	want := filepath.Join(dir, "gamesom.db")
	if got != want {
		t.Fatalf("resolveDBPath = %q, want %q", got, want)
	}
	content, err := os.ReadFile(want)
	if err != nil || string(content) != "db" {
		t.Errorf("db must be untouched when dir == legacyDir (content %q, err %v)", content, err)
	}
}

func TestDataDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join("/", "custom", "share"))

	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	want := filepath.Join("/", "custom", "share", "gamesom")
	if got != want {
		t.Errorf("dataDir with XDG_DATA_HOME = %q, want %q", got, want)
	}
}
