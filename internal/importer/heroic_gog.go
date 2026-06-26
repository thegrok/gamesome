package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

type gogHeroicLibraryFile struct {
	Games []gogHeroicEntry `json:"games"`
}

type gogHeroicEntry struct {
	AppName     string `json:"app_name"`
	Runner      string `json:"runner"`
	Title       string `json:"title"`
	IsInstalled bool   `json:"is_installed"`
	Install     struct {
		IsDLC       bool   `json:"is_dlc"`
		InstallPath string `json:"install_path"`
	} `json:"install"`
}

// GOG imports GOG games. On Linux, uses Heroic's gogdl cache (no GOG Galaxy client
// exists for Linux). On Windows/macOS, goes directly to GOG Galaxy which is
// authoritative for both ownership and install status.
func GOG(database *sql.DB) error {
	if runtime.GOOS == "linux" {
		return HeroicGOG(database)
	}
	return GOGGalaxy(database)
}

// HeroicGOG imports games from Heroic Launcher's GOG library cache (via gogdl).
// Linux only — on Windows/macOS use GOGGalaxy instead.
func HeroicGOG(database *sql.DB) error {
	dir, err := heroicConfigDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "store_cache", "gog_library.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	var f gogHeroicLibraryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse gog_library.json: %w", err)
	}

	imported, installedCount := 0, 0
	for _, g := range f.Games {
		if g.Runner != "gog" || g.Install.IsDLC {
			continue
		}
		if g.Title == "" || g.AppName == "" {
			continue
		}
		norm := normalize.Title(g.Title)
		if norm == "" {
			continue
		}
		gameID, err := db.UpsertGame(database, g.Title, norm)
		if err != nil {
			log.Printf("warning: upsert game %q: %v", g.Title, err)
			continue
		}
		installedFlag := 0
		installPath := ""
		if g.IsInstalled {
			installedFlag = 1
			installPath = g.Install.InstallPath
			installedCount++
		}
		entry := db.LibraryEntry{
			GameID:       gameID,
			Source:       "gog",
			SourceGameID: g.AppName,
			SourceTitle:  g.Title,
			Owned:        1,
			Installed:    installedFlag,
			InstallPath:  installPath,
			LauncherURI:  fmt.Sprintf("goggalaxy://openGame/%s", g.AppName),
		}
		if err := db.UpsertLibraryEntry(database, entry); err != nil {
			log.Printf("warning: upsert library entry %q: %v", g.Title, err)
			continue
		}
		imported++
	}

	if imported == 0 {
		return fmt.Errorf("no GOG games found in Heroic gog_library.json")
	}
	fmt.Printf("Imported %d games from GOG via Heroic (%d installed)\n", imported, installedCount)
	return nil
}
