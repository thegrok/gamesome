package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/importer"
)

var enrichCmd = &cobra.Command{
	Use:   "enrich",
	Short: "Enrich game library with metadata and sommelier traits",
	Long: `Runs all enrichment steps in order:
  1. steam-ids  — cross-reference Epic games to Steam app IDs
  2. metadata   — fetch genres, description, tags from Steam Store API
  3. traits     — infer sommelier profile via LLM (requires ANTHROPIC_API_KEY)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		fmt.Println("Step 1/3: Resolving Steam IDs for unmatched games...")
		if err := importer.EnrichSteamIDs(database); err != nil {
			fmt.Fprintf(os.Stderr, "steam-ids: %v\n", err)
		}

		fmt.Println("\nStep 2/3: Fetching Steam Store metadata...")
		if err := importer.EnrichMetadata(database); err != nil {
			fmt.Fprintf(os.Stderr, "metadata: %v\n", err)
		}

		fmt.Println("\nStep 3/3: Inferring sommelier traits via LLM...")
		if err := importer.EnrichTraits(database); err != nil {
			fmt.Fprintf(os.Stderr, "traits: %v\n", err)
		}

		return nil
	},
}

var enrichSteamIDsCmd = &cobra.Command{
	Use:   "steam-ids",
	Short: "Cross-reference unresolved games to Steam app IDs",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()
		return importer.EnrichSteamIDs(database)
	},
}

var enrichMetadataCmd = &cobra.Command{
	Use:   "metadata",
	Short: "Fetch Steam Store metadata for games with a Steam app ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()
		return importer.EnrichMetadata(database)
	},
}

var enrichTraitsCmd = &cobra.Command{
	Use:   "traits",
	Short: "Infer sommelier profile traits via LLM (requires ANTHROPIC_API_KEY)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()
		return importer.EnrichTraits(database)
	},
}

func init() {
	enrichCmd.AddCommand(enrichSteamIDsCmd)
	enrichCmd.AddCommand(enrichMetadataCmd)
	enrichCmd.AddCommand(enrichTraitsCmd)
	rootCmd.AddCommand(enrichCmd)
}
