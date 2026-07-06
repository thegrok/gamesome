package importer

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

func itchConfigDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
		return filepath.Join(appdata, "itch"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "itch"), nil
	default: // linux
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "itch"), nil
	}
}

type itchGame struct {
	ID    int64
	Title string
	URL   string
}

type itchDownloadKey struct {
	GameID int64
	Game   itchGame
}

// itchCavePaths maps game_id → resolved install path for every cave in
// butler's DB. Butler records where a cave actually lives: a custom folder
// (full path, set via itch's Preferences), or an install-location row joined
// by id. The legacy configDir/apps guess is only a last resort — and the
// whole-query fallback below keeps older butler schemas importing exactly as
// before this fix.
func itchCavePaths(butlerDB *sql.DB, configDir string) (map[int64]string, error) {
	rows, err := butlerDB.Query(`
		SELECT c.game_id, c.install_folder_name, c.custom_install_folder, l.path
		FROM caves c
		LEFT JOIN install_locations l ON c.install_location_id = l.id`)
	if err != nil {
		log.Printf("warning: itch caves/install_locations query failed (%v); falling back to legacy apps-dir assumption", err)
		return itchCavePathsLegacy(butlerDB, configDir)
	}
	defer rows.Close()

	paths := make(map[int64]string)
	for rows.Next() {
		var gameID int64
		var folderName, customFolder, locationPath sql.NullString
		if err := rows.Scan(&gameID, &folderName, &customFolder, &locationPath); err != nil {
			return nil, fmt.Errorf("scan itch cave: %w", err)
		}
		switch {
		case customFolder.String != "":
			paths[gameID] = customFolder.String
		case locationPath.String != "":
			paths[gameID] = filepath.Join(locationPath.String, folderName.String)
		default:
			paths[gameID] = filepath.Join(configDir, "apps", folderName.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read itch caves: %w", err)
	}
	return paths, nil
}

// itchCavePathsLegacy is the pre-install_locations behavior: bare folder
// names from caves, assumed to live under configDir/apps.
func itchCavePathsLegacy(butlerDB *sql.DB, configDir string) (map[int64]string, error) {
	rows, err := butlerDB.Query(`SELECT game_id, install_folder_name FROM caves`)
	if err != nil {
		return nil, fmt.Errorf("query itch caves: %w", err)
	}
	defer rows.Close()

	paths := make(map[int64]string)
	for rows.Next() {
		var gameID int64
		var folderName sql.NullString
		if err := rows.Scan(&gameID, &folderName); err != nil {
			return nil, fmt.Errorf("scan itch cave: %w", err)
		}
		paths[gameID] = filepath.Join(configDir, "apps", folderName.String)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read itch caves: %w", err)
	}
	return paths, nil
}

// Itch imports owned and installed games from itch.io's butler database.
func Itch(database *sql.DB) error {
	configDir, err := itchConfigDir()
	if err != nil {
		return err
	}
	butlerDBPath := filepath.Join(configDir, "db", "butler.db")
	butlerDB, err := sql.Open("sqlite", butlerDBPath+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open itch butler database %s: %w", butlerDBPath, err)
	}
	defer butlerDB.Close()

	downloadRows, err := butlerDB.Query(`
		SELECT dk.game_id, g.title, g.url
		FROM download_keys dk
		JOIN games g ON g.id = dk.game_id
		WHERE g.classification = 'game'`)
	if err != nil {
		return fmt.Errorf("query itch download keys: %w", err)
	}

	var downloadKeys []itchDownloadKey
	for downloadRows.Next() {
		var key itchDownloadKey
		var title, url sql.NullString
		if err := downloadRows.Scan(&key.GameID, &title, &url); err != nil {
			downloadRows.Close()
			return fmt.Errorf("scan itch download key: %w", err)
		}
		key.Game = itchGame{ID: key.GameID, Title: title.String, URL: url.String}
		downloadKeys = append(downloadKeys, key)
	}
	if err := downloadRows.Err(); err != nil {
		downloadRows.Close()
		return fmt.Errorf("read itch download keys: %w", err)
	}
	downloadRows.Close()

	caves, err := itchCavePaths(butlerDB, configDir)
	if err != nil {
		return err
	}

	imported := 0
	installedCount := 0
	for _, key := range downloadKeys {
		game := key.Game
		if game.Title == "" {
			continue
		}

		norm := normalize.Title(game.Title)
		if norm == "" {
			log.Printf("warning: normalized title empty for %q, skipping", game.Title)
			continue
		}

		gameID, err := db.UpsertGame(database, game.Title, norm)
		if err != nil {
			log.Printf("warning: upsert game %q: %v", game.Title, err)
			continue
		}

		installed := 0
		installPath := ""
		if p, ok := caves[key.GameID]; ok {
			if _, err := os.Stat(p); err == nil {
				installed = 1
				installPath = p
			}
		}

		entry := db.LibraryEntry{
			GameID:       gameID,
			Source:       "itchio",
			SourceGameID: strconv.FormatInt(key.GameID, 10),
			SourceTitle:  game.Title,
			Owned:        1,
			Installed:    installed,
			InstallPath:  installPath,
		}
		if err := db.UpsertLibraryEntry(database, entry); err != nil {
			log.Printf("warning: upsert library entry %q: %v", game.Title, err)
			continue
		}

		imported++
		installedCount += installed
	}

	fmt.Printf("Imported %d games from itch.io (%d installed)\n", imported, installedCount)
	return nil
}

