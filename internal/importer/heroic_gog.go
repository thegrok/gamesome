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

var nileLibraryPath = filepath.Join(
	os.Getenv("HOME"),
	".var/app/com.heroicgameslauncher.hgl/config/heroic/store_cache/nile_library.json",
)

var nileInstalledPath = filepath.Join(
	os.Getenv("HOME"),
	".var/app/com.heroicgameslauncher.hgl/config/heroic/nileConfig/nile/installed.json",
)

type nileLibraryFile struct {
	Library []nileLibraryEntry `json:"library"`
}

type nileLibraryEntry struct {
	AppName     string `json:"app_name"`
	Title       string `json:"title"`
	IsInstalled bool   `json:"is_installed"`
}

type nileInstalledEntry struct {
	AppName     string `json:"app_name"`
	InstallPath string `json:"install_path"`
}

// HeroicGOG imports games from Heroic Launcher's GOG library cache (via nile).
func HeroicGOG(database *sql.DB) error {
	library, err := readNileLibrary()
	if err != nil {
		return fmt.Errorf("read nile library: %w", err)
	}

	installInfo, err := readNileInstalled()
	if err != nil {
		log.Printf("warning: could not read nile installed.json: %v", err)
		installInfo = map[string]nileInstalledEntry{}
	}

	imported := 0
	installedCount := 0
	for _, g := range library {
		if g.Title == "" || g.AppName == "" {
			log.Printf("warning: skipping gog entry with empty title/app_name")
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

		installPath := ""
		installedFlag := 0
		if info, ok := installInfo[g.AppName]; ok {
			installedFlag = 1
			installedCount++
			installPath = info.InstallPath
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

	fmt.Printf("Imported %d games from GOG (%d installed)\n", imported, installedCount)
	return nil
}

func readNileLibrary() ([]nileLibraryEntry, error) {
	data, err := os.ReadFile(nileLibraryPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", nileLibraryPath, err)
	}
	var f nileLibraryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse nile_library.json: %w", err)
	}
	return f.Library, nil
}

func readNileInstalled() (map[string]nileInstalledEntry, error) {
	data, err := os.ReadFile(nileInstalledPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", nileInstalledPath, err)
	}
	var info map[string]nileInstalledEntry
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse nile installed.json: %w", err)
	}
	return info, nil
}
