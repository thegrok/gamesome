package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := OpenAt(filepath.Join(t.TempDir(), "gamesome.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func boolPtr(b bool) *bool { return &b }

func TestUpsertPersona_InsertAndUpdate(t *testing.T) {
	d := openTestDB(t)

	if err := UpsertPersona(d, "productivity_stance", "reduce guilt", boolPtr(true)); err != nil {
		t.Fatalf("insert: %v", err)
	}
	rows, err := GetPersona(d)
	if err != nil {
		t.Fatalf("GetPersona: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Dimension != "productivity_stance" || rows[0].Value != "reduce guilt" || !rows[0].Pinned || rows[0].Source != "user" {
		t.Errorf("unexpected row: %+v", rows[0])
	}
	if !PersonaConfigured(d) {
		t.Error("persona should be marked configured after a write")
	}

	// An adaptive (nil) write to a NEW dimension succeeds and is sourced "adaptive".
	if err := UpsertPersona(d, "session_preference", "short sessions", nil); err != nil {
		t.Fatalf("adaptive insert: %v", err)
	}
	rows, _ = GetPersona(d)
	if len(rows) != 2 || rows[0].Dimension != "productivity_stance" || rows[1].Dimension != "session_preference" {
		t.Fatalf("expected ordered [productivity_stance, session_preference], got %+v", rows)
	}
	if rows[1].Pinned || rows[1].Source != "adaptive" {
		t.Errorf("adaptive row should be unpinned/adaptive: %+v", rows[1])
	}
}

func TestUpsertPersona_PinGuard(t *testing.T) {
	d := openTestDB(t)

	if err := UpsertPersona(d, "productivity_stance", "reduce guilt", boolPtr(true)); err != nil {
		t.Fatalf("pin insert: %v", err)
	}

	// Adaptive (nil) overwrite of a pinned dimension is refused.
	if err := UpsertPersona(d, "productivity_stance", "push ambition", nil); err != ErrPersonaPinned {
		t.Fatalf("adaptive overwrite of pinned dim = %v, want ErrPersonaPinned", err)
	}
	rows, _ := GetPersona(d)
	if rows[0].Value != "reduce guilt" {
		t.Errorf("pinned value was changed by an adaptive write: %q", rows[0].Value)
	}

	// A deliberate (non-nil pinned) overwrite of a pinned dimension succeeds.
	if err := UpsertPersona(d, "productivity_stance", "push ambition", boolPtr(false)); err != nil {
		t.Fatalf("directed overwrite: %v", err)
	}
	rows, _ = GetPersona(d)
	if rows[0].Value != "push ambition" || rows[0].Pinned {
		t.Errorf("directed overwrite should update value + unpin: %+v", rows[0])
	}
}

func TestUpsertPersona_RequiresDimensionAndValue(t *testing.T) {
	d := openTestDB(t)
	if err := UpsertPersona(d, "  ", "x", nil); err == nil {
		t.Error("empty dimension should be rejected")
	}
	if err := UpsertPersona(d, "x", "  ", nil); err == nil {
		t.Error("empty value should be rejected")
	}
}

func TestResetPersona(t *testing.T) {
	d := openTestDB(t)
	_ = UpsertPersona(d, "a", "1", boolPtr(true))
	_ = UpsertPersona(d, "b", "2", nil)

	// Clear one dimension.
	n, err := ResetPersona(d, "a")
	if err != nil || n != 1 {
		t.Fatalf("ResetPersona(a) = %d, %v; want 1, nil", n, err)
	}
	rows, _ := GetPersona(d)
	if len(rows) != 1 || rows[0].Dimension != "b" {
		t.Fatalf("expected only [b] left, got %+v", rows)
	}

	// Clear all (the decline/reset-to-baseline path).
	n, err = ResetPersona(d, "")
	if err != nil || n != 1 {
		t.Fatalf("ResetPersona(all) = %d, %v; want 1, nil", n, err)
	}
	rows, _ = GetPersona(d)
	if len(rows) != 0 {
		t.Fatalf("expected empty persona, got %+v", rows)
	}
	if !PersonaConfigured(d) {
		t.Error("reset should keep the persona marked configured (don't re-nag)")
	}
}
