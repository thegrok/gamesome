package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

type nileLibraryFile struct {
	Library []nileLibraryEntry `json:"library"`
}

type nileLibraryEntry struct {
	AppName string `json:"app_name"`
	Title   string `json:"title"`
}

type nileInstalledEntry struct {
	AppName     string `json:"app_name"`
	InstallPath string `json:"install_path"`
}

// GOG imports GOG games with a Heroic-first fallback to GOG Galaxy direct.
// Tries Heroic's nile cache first (all platforms); falls back to GOG Galaxy
// SQLite on Windows/macOS if Heroic is not installed or has no GOG library.
func GOG(database *sql.DB) error {
	if err := HeroicGOG(database); err == nil {
		return nil
	}
	fmt.Println("Heroic GOG cache not found — trying GOG Galaxy direct...")
	return GOGGalaxy(database)
}

// HeroicGOG imports games from Heroic Launcher's GOG library cache (via nile).
func HeroicGOG(database *sql.DB) error {
	dir, err := heroicConfigDir()
	if err != nil {
		return err
	}

	library, err := readNileLibrary(dir)
	if err != nil {
		return fmt.Errorf("read nile library: %w", err)
	}

	installInfo, err := readNileInstalled(dir)
	if err != nil {
		log.Printf("warning: could not read nile installed.json: %v", err)
		installInfo = map[string]nileInstalledEntry{}
	}

	imported, installedCount := 0, 0
	for _, g := range library {
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
		if info, ok := installInfo[g.AppName]; ok {
			installedFlag = 1
			installPath = info.InstallPath
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
			LauncherURI:  fmt.Sprintf("nile://launch/%s", g.AppName),
		}
		if err := db.UpsertLibraryEntry(database, entry); err != nil {
			log.Printf("warning: upsert library entry %q: %v", g.Title, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d games from GOG via Heroic (%d installed)\n", imported, installedCount)
	return nil
}

func readNileLibrary(heroicDir string) ([]nileLibraryEntry, error) {
	path := filepath.Join(heroicDir, "store_cache", "nile_library.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	var f nileLibraryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse nile_library.json: %w", err)
	}
	return f.Library, nil
}

func readNileInstalled(heroicDir string) (map[string]nileInstalledEntry, error) {
	path := filepath.Join(heroicDir, "nileConfig", "nile", "installed.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	var info map[string]nileInstalledEntry
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse nile installed.json: %w", err)
	}
	return info, nil
}
