package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thegrok/gamesome/internal/db"
	"github.com/thegrok/gamesome/internal/importer"
)

var enrichCmd = &cobra.Command{
	Use:   "enrich",
	Short: "Enrich game library with metadata from Steam",
	Long: `Runs all enrichment steps in order:
  1. steam-ids  — cross-reference Epic games to Steam app IDs
  2. metadata   — fetch genres, description, tags from Steam Store API

Sommelier profile traits are inferred lazily during Claude sommelier
conversations and written back to the database via MCP.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		fmt.Println("Step 1/2: Resolving Steam IDs for unmatched games...")
		if err := importer.EnrichSteamIDs(database); err != nil {
			fmt.Fprintf(os.Stderr, "steam-ids: %v\n", err)
		}

		fmt.Println("\nStep 2/2: Fetching Steam Store metadata...")
		if err := importer.EnrichMetadata(database); err != nil {
			fmt.Fprintf(os.Stderr, "metadata: %v\n", err)
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

func init() {
	enrichCmd.AddCommand(enrichSteamIDsCmd)
	enrichCmd.AddCommand(enrichMetadataCmd)
	rootCmd.AddCommand(enrichCmd)
}
