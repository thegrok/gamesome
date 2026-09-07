package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Claude Desktop validates a prompt's runtime content (prompts/get) against
// the text declared in the MCPB manifest at install time; any mismatch is
// rejected as potential prompt injection. So the manifest's prompt block is
// not documentation — it must mirror what registerPrompts serves, byte for
// byte. This test pins that invariant (drift shipped in v0.0.2-rc4 broke the
// sommelier re-brief; see .mind-harness/agent-logs/persona-instructions/).
func TestManifestPromptsMirrorServer(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "packaging", "mcpb", "manifest.template.json"))
	if err != nil {
		t.Fatalf("read manifest template: %v", err)
	}

	var manifest struct {
		Prompts []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Text        string `json:"text"`
		} `json:"prompts"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse manifest template: %v", err)
	}

	if len(manifest.Prompts) != 1 {
		names := make([]string, 0, len(manifest.Prompts))
		for _, p := range manifest.Prompts {
			names = append(names, p.Name)
		}
		t.Fatalf("manifest declares prompts %v; server registers exactly [sommelier]", names)
	}

	p := manifest.Prompts[0]
	if p.Name != "sommelier" {
		t.Errorf("manifest prompt name = %q, want %q", p.Name, "sommelier")
	}
	if p.Description != sommelierPromptDescription {
		t.Errorf("manifest prompt description drifted from server:\nmanifest: %q\nserver:   %q", p.Description, sommelierPromptDescription)
	}
	if p.Text != sommelierBriefing {
		t.Errorf("manifest prompt text drifted from sommelierBriefing const:\nmanifest: %q\nserver:   %q", p.Text, sommelierBriefing)
	}
}
