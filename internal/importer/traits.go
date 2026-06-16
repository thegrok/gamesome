package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	"github.com/thegrok/gamesom/internal/db"
)

const traitsSystemPrompt = `You are a game library analyst. Given a game's title and metadata, infer how it fits into a sommelier recommendation system. Respond ONLY with valid JSON, no other text.`

const traitsUserPromptTemplate = `Title: %s
Genres: %s
Categories: %s
Description: %s

Return JSON with these fields (all strings, keep values short):
{
  "energy_required": "low|medium|high",
  "friction_level": "low|medium|high",
  "session_length_fit": "5-15min|15-45min|45min-2h|2h+|variable",
  "narrative_memory_load": "none|low|medium|high",
  "complexity_level": "minimal|low|medium|high|very high",
  "mood_tags": "comma-separated mood words (e.g. relaxing, tense, cozy, dark)",
  "avoid_when": "short phrase (e.g. tired, foggy, low time)",
  "best_when": "short phrase (e.g. want quick wins, cozy evening, focused)"
}`

// EnrichTraits calls Claude Haiku for each game without a sommelier_profile and stores the result.
func EnrichTraits(database *sql.DB) error {
	games, err := db.GamesNeedingTraits(database)
	if err != nil {
		return fmt.Errorf("query games needing traits: %w", err)
	}

	if len(games) == 0 {
		fmt.Println("All games already have traits inferred.")
		return nil
	}

	client := anthropic.NewClient()
	ctx := context.Background()

	total := len(games)
	succeeded, skipped, failed := 0, 0, 0

	for i, g := range games {
		if i > 0 && i%50 == 0 {
			fmt.Printf("Inferred traits for %d/%d...\n", i, total)
		}

		userPrompt := fmt.Sprintf(traitsUserPromptTemplate,
			g.Title,
			g.Genres,
			g.Themes,
			truncate(g.Summary, 400),
		)

		msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeHaiku4_5_20251001,
			MaxTokens: 400,
			System: []anthropic.TextBlockParam{
				{Text: traitsSystemPrompt},
			},
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
			},
		})
		if err != nil {
			log.Printf("warning: trait inference for %q: %v", g.Title, err)
			failed++
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if len(msg.Content) == 0 {
			log.Printf("warning: empty response for %q", g.Title)
			skipped++
			time.Sleep(100 * time.Millisecond)
			continue
		}

		rawJSON := strings.TrimSpace(msg.Content[0].AsText().Text)

		// Strip markdown code fences if present.
		rawJSON = strings.TrimPrefix(rawJSON, "```json")
		rawJSON = strings.TrimPrefix(rawJSON, "```")
		rawJSON = strings.TrimSuffix(rawJSON, "```")
		rawJSON = strings.TrimSpace(rawJSON)

		var profile db.SommelierProfile
		if err := json.Unmarshal([]byte(rawJSON), &profile); err != nil {
			log.Printf("warning: parse traits JSON for %q: %v\nraw: %s", g.Title, err, rawJSON)
			skipped++
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if err := db.UpsertSommelierProfile(database, g.ID, profile); err != nil {
			log.Printf("warning: store traits for %q: %v", g.Title, err)
			failed++
		} else {
			succeeded++
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("Inferred traits for %d games (%d skipped, %d failed)\n", succeeded, skipped, failed)
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
