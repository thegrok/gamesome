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

// heroicLibraryPath is where Heroic (Flatpak) stores the Epic library cache.
var heroicLibraryPath = filepath.Join(
	os.Getenv("HOME"),
	".var/app/com.heroicgameslauncher.hgl/config/heroic/store_cache/legendary_library.json",
)

var heroicInstallInfoPath = filepath.Join(
	os.Getenv("HOME"),
	".var/app/com.heroicgameslauncher.hgl/config/heroic/store_cache/legendary_install_info.json",
)

type heroicLibraryFile struct {
	Library []heroicLibraryEntry `json:"library"`
}

type heroicLibraryEntry struct {
	AppName     string `json:"app_name"`
	Title       string `json:"title"`
	IsInstalled bool   `json:"is_installed"`
	Install     struct {
		IsDLC bool `json:"is_dlc"`
	} `json:"install"`
}

// heroicInstallInfo is keyed by app_name.
type heroicInstallEntry struct {
	Game struct {
		IsDLC bool `json:"is_dlc"`
	} `json:"game"`
	Install *struct {
		InstallPath string `json:"install_path"`
	} `json:"install"`
}

// Heroic imports games from Heroic Launcher's Epic library cache.
func Heroic(database *sql.DB) error {
	library, err := readHeroicLibrary()
	if err != nil {
		return fmt.Errorf("read heroic library: %w", err)
	}

	installInfo, err := readHeroicInstallInfo()
	if err != nil {
		// Non-fatal: install paths just won't be populated
		log.Printf("warning: could not read install info: %v", err)
		installInfo = map[string]heroicInstallEntry{}
	}

	imported := 0
	installedCount := 0
	for _, g := range library {
		if g.Install.IsDLC {
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

		installPath := ""
		installedFlag := 0
		if g.IsInstalled {
			installedFlag = 1
			installedCount++
			if info, ok := installInfo[g.AppName]; ok && info.Install != nil {
				installPath = info.Install.InstallPath
			}
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

func readHeroicLibrary() ([]heroicLibraryEntry, error) {
	data, err := os.ReadFile(heroicLibraryPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", heroicLibraryPath, err)
	}
	var f heroicLibraryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse legendary_library.json: %w", err)
	}
	return f.Library, nil
}

func readHeroicInstallInfo() (map[string]heroicInstallEntry, error) {
	data, err := os.ReadFile(heroicInstallInfoPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", heroicInstallInfoPath, err)
	}
	var info map[string]heroicInstallEntry
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse legendary_install_info.json: %w", err)
	}
	return info, nil
}
