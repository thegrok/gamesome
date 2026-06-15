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

func init() {
	importCmd.AddCommand(importHeroicCmd)
	importCmd.AddCommand(importSteamCmd)
	rootCmd.AddCommand(importCmd)
}
