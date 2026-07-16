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

func TestComposePersona_BaselineOffersOnboarding(t *testing.T) {
	database := personaTestDB(t)

	got := composePersona(database)

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

func TestComposePersona_ConfiguredRendersDeltasNoOnboarding(t *testing.T) {
	database := personaTestDB(t)
	if err := db.UpsertPersona(database, "productivity_stance", "reduce guilt", boolPtrCmd(true)); err != nil {
		t.Fatalf("seed pinned dim: %v", err)
	}
	if err := db.UpsertPersona(database, "session_preference", "short sessions", nil); err != nil {
		t.Fatalf("seed adaptive dim: %v", err)
	}

	got := composePersona(database)

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

func TestComposePersona_ResetSuppressesOnboarding(t *testing.T) {
	database := personaTestDB(t)
	// User declined setup: reset with no dimension marks configured but leaves persona empty.
	if _, err := db.ResetPersona(database, ""); err != nil {
		t.Fatalf("reset: %v", err)
	}

	got := composePersona(database)

	if strings.Contains(got, personaOnboarding) {
		t.Error("after a decline/reset the onboarding offer must not reappear")
	}
	if strings.Contains(got, "--- Your configured persona ---") {
		t.Error("empty persona should not render a configured-persona section")
	}
}

// The persona is only useful if it reaches the model. Claude Desktop discards
// ServerOptions.Instructions (finding 007), so tool descriptions are the delivery
// channel — this pins that the persona actually rides list_games' description, and
// that a configured persona reaches it. Without this, the persona can be perfectly
// composed and still be delivered nowhere: exactly the A117/A099 failure.
func TestListGamesDescription_CarriesPersona(t *testing.T) {
	database := personaTestDB(t)

	got := listGamesDescription(database)

	if !strings.HasPrefix(got, listGamesBaseDescription) {
		t.Error("the tool description must still describe the tool first")
	}
	if !strings.Contains(got, sommelierBriefing) {
		t.Error("list_games description must carry the baseline briefing — it is the only channel Desktop honours")
	}
	if !strings.Contains(got, personaProtocol) {
		t.Error("list_games description must carry the adaptation protocol")
	}
	if !strings.Contains(got, personaOnboarding) {
		t.Error("unconfigured persona should carry the onboarding offer into the tool description")
	}

	// A configured persona must reach the same channel.
	if err := db.UpsertPersona(database, "productivity_stance", "reduce guilt", boolPtrCmd(true)); err != nil {
		t.Fatalf("seed dim: %v", err)
	}
	got = listGamesDescription(database)
	if !strings.Contains(got, "- productivity_stance (pinned): reduce guilt") {
		t.Error("configured persona dimensions must reach the tool description")
	}
	if strings.Contains(got, personaOnboarding) {
		t.Error("a configured persona should not re-offer onboarding via the tool description")
	}
}

func boolPtrCmd(b bool) *bool { return &b }
