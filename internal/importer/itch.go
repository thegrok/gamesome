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

type itchCave struct {
	GameID            int64
	InstallFolderName string
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

	caveRows, err := butlerDB.Query(`SELECT game_id, install_folder_name FROM caves`)
	if err != nil {
		return fmt.Errorf("query itch caves: %w", err)
	}

	caves := make(map[int64]string)
	for caveRows.Next() {
		var cave itchCave
		if err := caveRows.Scan(&cave.GameID, &cave.InstallFolderName); err != nil {
			caveRows.Close()
			return fmt.Errorf("scan itch cave: %w", err)
		}
		caves[cave.GameID] = cave.InstallFolderName
	}
	if err := caveRows.Err(); err != nil {
		caveRows.Close()
		return fmt.Errorf("read itch caves: %w", err)
	}
	caveRows.Close()

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
		if caveFolder, ok := caves[key.GameID]; ok {
			p := filepath.Join(configDir, "apps", caveFolder)
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

