package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/importer"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import games from a launcher",
}

var importHeroicCmd = &cobra.Command{
	Use:   "heroic",
	Short: "Import from Epic Games via Legendary",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		if err := importer.Heroic(database); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		db.SetMeta(database, "last_import_heroic", time.Now().UTC().Format(time.RFC3339))
		return nil
	},
}

var importSteamCmd = &cobra.Command{
	Use:   "steam",
	Short: "Import from Steam (API + local manifests)",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		if err := importer.Steam(database); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		db.SetMeta(database, "last_import_steam", time.Now().UTC().Format(time.RFC3339))
		return nil
	},
}

var importSteamCollectionsCmd = &cobra.Command{
	Use:   "steam-collections",
	Short: "Import completed games from Steam collections",
	Long: `Import completed games from Steam collections.

Reads the Steam cloud storage JSON and marks games as completed based on
a named collection (default: "Completed"). Only marks games that aren't
already marked completed (idempotent).

Use --dry-run to preview matches without modifying the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		collectionName, _ := cmd.Flags().GetString("collection")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		result, err := importer.SteamCollectionsImport(database, collectionName, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Print results
		if !result.CollectionFound {
			fmt.Printf("Collection %q not found in Steam data\n", collectionName)
			os.Exit(1)
		}

		fmt.Printf("Collection: %q (ID: %s)\n", result.CollectionName, result.CollectionID)
		fmt.Printf("AppIDs in collection: %d\n", result.AppIDCount)

		if result.IsDynamic {
			fmt.Printf("⚠ Warning: this collection is dynamic (filter-based) with no explicit items\n")
		}

		fmt.Printf("Matched to library: %d\n", result.MatchedCount)

		if dryRun {
			fmt.Printf("Would mark as completed: %d (dry-run, no changes made)\n", result.NewlyMarkedCount)
		} else {
			fmt.Printf("Newly marked completed: %d\n", result.NewlyMarkedCount)
			if result.NewlyMarkedCount > 0 {
				db.SetMeta(database, "last_import_steam_collections", time.Now().UTC().Format(time.RFC3339))
			}
		}

		if len(result.UnmatchedAppIDs) > 0 {
			fmt.Printf("Unmatched AppIDs: %s\n", importer.FormatUnmatchedAppIDs(result.UnmatchedAppIDs, 10))
		}

		return nil
	},
}

var importGOGCmd = &cobra.Command{
	Use:   "gog",
	Short: "Import from GOG via Heroic/nile cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		if err := importer.HeroicGOG(database); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		db.SetMeta(database, "last_import_gog", time.Now().UTC().Format(time.RFC3339))
		return nil
	},
}

var importItchCmd = &cobra.Command{
	Use:   "itch",
	Short: "Import from itch.io (butler.db)",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		if err := importer.Itch(database); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		db.SetMeta(database, "last_import_itch", time.Now().UTC().Format(time.RFC3339))
		return nil
	},
}

func init() {
	importCmd.AddCommand(importHeroicCmd)
	importCmd.AddCommand(importGOGCmd)
	importCmd.AddCommand(importSteamCmd)
	importCmd.AddCommand(importSteamCollectionsCmd)
	importCmd.AddCommand(importItchCmd)

	importSteamCollectionsCmd.Flags().StringP("collection", "c", "Completed", "Name of the collection to import")
	importSteamCollectionsCmd.Flags().BoolP("dry-run", "d", false, "Preview changes without writing to database")

	rootCmd.AddCommand(importCmd)
}
