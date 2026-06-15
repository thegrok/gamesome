package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

type legendaryGame struct {
	AppName  string `json:"app_name"`
	Title    string `json:"title"`
	IsDLC    bool   `json:"is_dlc"`
	Metadata struct {
		ReleaseInfo []struct {
			DateAdded string `json:"dateAdded"`
		} `json:"releaseInfo"`
		ReleaseDate string `json:"releaseDate"`
	} `json:"metadata"`
}

type legendaryInstalled struct {
	AppName     string `json:"app_name"`
	Title       string `json:"title"`
	InstallPath string `json:"install_path"`
	IsDLC       bool   `json:"is_dlc"`
}

// Heroic imports games from Epic via the Legendary CLI.
func Heroic(database *sql.DB) error {
	owned, err := legendaryListOwned()
	if err != nil {
		return fmt.Errorf("legendary list: %w", err)
	}

	installed, err := legendaryListInstalled()
	if err != nil {
		return fmt.Errorf("legendary list-installed: %w", err)
	}

	installedMap := make(map[string]legendaryInstalled, len(installed))
	for _, g := range installed {
		installedMap[g.AppName] = g
	}

	imported := 0
	installedCount := 0
	for _, g := range owned {
		if g.IsDLC {
			continue
		}
		if g.Title == "" || g.AppName == "" {
			log.Printf("warning: skipping epic entry with empty title/app_name")
			continue
		}

		norm := normalize.Title(g.Title)
		if norm == "" {
			log.Printf("warning: normalized title empty for %q, skipping", g.Title)
			continue
		}

		gameID, err := db.UpsertGame(database, g.Title, norm)
		if err != nil {
			log.Printf("warning: upsert game %q: %v", g.Title, err)
			continue
		}

		inst, isInstalled := installedMap[g.AppName]
		installPath := ""
		installedFlag := 0
		if isInstalled {
			installPath = inst.InstallPath
			installedFlag = 1
			installedCount++
		}

		entry := db.LibraryEntry{
			GameID:       gameID,
			Source:       "epic",
			SourceGameID: g.AppName,
			SourceTitle:  g.Title,
			Owned:        1,
			Installed:    installedFlag,
			InstallPath:  installPath,
			LauncherURI:  fmt.Sprintf("legendary://launch/%s", g.AppName),
		}
		if err := db.UpsertLibraryEntry(database, entry); err != nil {
			log.Printf("warning: upsert library entry %q: %v", g.Title, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d games from Epic (%d installed)\n", imported, installedCount)
	return nil
}

func legendaryListOwned() ([]legendaryGame, error) {
	out, err := exec.Command("legendary", "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("exec legendary list: %w", err)
	}
	var games []legendaryGame
	if err := json.Unmarshal(out, &games); err != nil {
		return nil, fmt.Errorf("parse legendary list output: %w", err)
	}
	return games, nil
}

func legendaryListInstalled() ([]legendaryInstalled, error) {
	out, err := exec.Command("legendary", "list-installed", "--json", "--show-dirs").Output()
	if err != nil {
		return nil, fmt.Errorf("exec legendary list-installed: %w", err)
	}
	var games []legendaryInstalled
	if err := json.Unmarshal(out, &games); err != nil {
		return nil, fmt.Errorf("parse legendary list-installed output: %w", err)
	}
	return games, nil
}
