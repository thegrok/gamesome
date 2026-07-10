package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thegrok/gamesome/internal/db"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show library stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		var totalGames int
		database.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&totalGames)

		rows, err := database.Query(`SELECT source, COUNT(*) FROM library_entries GROUP BY source ORDER BY source`)
		if err != nil {
			return err
		}
		defer rows.Close()

		type sourceStat struct {
			source string
			count  int
		}
		var stats []sourceStat
		for rows.Next() {
			var s sourceStat
			rows.Scan(&s.source, &s.count)
			stats = append(stats, s)
		}

		var installed int
		database.QueryRow(`SELECT COUNT(*) FROM library_entries WHERE installed = 1`).Scan(&installed)

		// Build source breakdown string
		breakdown := ""
		for i, s := range stats {
			if i > 0 {
				breakdown += ", "
			}
			breakdown += fmt.Sprintf("%d %s", s.count, s.source)
		}
		if breakdown != "" {
			breakdown = " (" + breakdown + ")"
		}

		fmt.Printf("gamesome library: %d games%s\n", totalGames, breakdown)
		fmt.Printf("Installed: %d games\n", installed)

		lastHeroic := db.GetMeta(database, "last_import_heroic")
		lastSteam := db.GetMeta(database, "last_import_steam")

		if lastHeroic != "" || lastSteam != "" {
			fmt.Print("Last import:")
			sep := " "
			if lastHeroic != "" {
				fmt.Printf("%s%s (heroic)", sep, lastHeroic)
				sep = ", "
			}
			if lastSteam != "" {
				fmt.Printf("%s%s (steam)", sep, lastSteam)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
