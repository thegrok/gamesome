package cmd

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thegrok/gamesome/internal/db"
)

func personaTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenAt(filepath.Join(t.TempDir(), "gamesome.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestComposeInstructions_BaselineOffersOnboarding(t *testing.T) {
	database := personaTestDB(t)

	got := composeInstructions(database)

	if !strings.HasPrefix(got, sommelierBriefing) {
		t.Error("composed instructions must start with the baseline briefing")
	}
	if !strings.Contains(got, personaProtocol) {
		t.Error("adaptation protocol should always be present")
	}
	if !strings.Contains(got, personaOnboarding) {
		t.Error("unconfigured persona should include the onboarding offer")
	}
	if strings.Contains(got, "--- Your configured persona ---") {
		t.Error("empty persona should not render a configured-persona section")
	}
}

func TestComposeInstructions_ConfiguredRendersDeltasNoOnboarding(t *testing.T) {
	database := personaTestDB(t)
	if err := db.UpsertPersona(database, "productivity_stance", "reduce guilt", boolPtrCmd(true)); err != nil {
		t.Fatalf("seed pinned dim: %v", err)
	}
	if err := db.UpsertPersona(database, "session_preference", "short sessions", nil); err != nil {
		t.Fatalf("seed adaptive dim: %v", err)
	}

	got := composeInstructions(database)

	if !strings.Contains(got, "--- Your configured persona ---") {
		t.Error("configured persona should render its section")
	}
	if !strings.Contains(got, "- productivity_stance (pinned): reduce guilt") {
		t.Errorf("pinned dimension not rendered with (pinned) marker:\n%s", got)
	}
	if !strings.Contains(got, "- session_preference: short sessions") {
		t.Errorf("adaptive dimension not rendered:\n%s", got)
	}
	if strings.Contains(got, "- session_preference (pinned)") {
		t.Error("unpinned dimension should not carry the (pinned) marker")
	}
	if strings.Contains(got, personaOnboarding) {
		t.Error("a configured persona should not re-offer onboarding")
	}
	if !strings.Contains(got, personaProtocol) {
		t.Error("adaptation protocol should still be present when configured")
	}
}

func TestComposeInstructions_ResetSuppressesOnboarding(t *testing.T) {
	database := personaTestDB(t)
	// User declined setup: reset with no dimension marks configured but leaves persona empty.
	if _, err := db.ResetPersona(database, ""); err != nil {
		t.Fatalf("reset: %v", err)
	}

	got := composeInstructions(database)

	if strings.Contains(got, personaOnboarding) {
		t.Error("after a decline/reset the onboarding offer must not reappear")
	}
	if strings.Contains(got, "--- Your configured persona ---") {
		t.Error("empty persona should not render a configured-persona section")
	}
}

func boolPtrCmd(b bool) *bool { return &b }
