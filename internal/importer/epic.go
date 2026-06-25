package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

type epicManifest struct {
	AppName              string   `json:"AppName"`
	DisplayName          string   `json:"DisplayName"`
	InstallLocation      string   `json:"InstallLocation"`
	BIsIncompleteInstall bool     `json:"bIsIncompleteInstall"`
	AppCategories        []string `json:"AppCategories"`
}

func epicManifestsDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		pd := os.Getenv("PROGRAMDATA")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		return filepath.Join(pd, "Epic", "EpicGamesLauncher", "Data", "Manifests"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Epic", "EpicGamesLauncher", "Data", "Manifests"), nil
	default:
		return "", fmt.Errorf("epic import not supported on %s; use gamesom import heroic", runtime.GOOS)
	}
}

// EpicInstalledMap returns a map of AppName → manifest for all games that are
// fully installed in the Epic Games Launcher manifests directory.
func EpicInstalledMap() (map[string]epicManifest, error) {
	dir, err := epicManifestsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read epic manifests dir %s: %w", dir, err)
	}
	m := make(map[string]epicManifest)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".item") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			log.Printf("warning: read %s: %v", entry.Name(), err)
			continue
		}
		var manifest epicManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			log.Printf("warning: parse %s: %v", entry.Name(), err)
			continue
		}
		if manifest.AppName == "" || manifest.BIsIncompleteInstall {
			continue
		}
		m[manifest.AppName] = manifest
	}
	return m, nil
}

// Epic imports the full Epic Games library.
// It tries Legendary first for the complete owned library; if Legendary is
// unavailable and the user declines to install it, it falls back to reading
// only locally-installed games from the EGL manifests directory.
func Epic(database *sql.DB) error {
	bin, err := findLegendary()
	if err != nil {
		if installErr := promptInstallLegendary(); installErr != nil {
			if runtime.GOOS == "linux" {
				return installErr
			}
			fmt.Println("Skipping Legendary — importing installed games from Epic Games Launcher only.")
			return epicFromManifests(database)
		}
		bin, err = findLegendaryWithFreshPath(true)
		if err != nil {
			return fmt.Errorf("legendary not found after install; restart your terminal and try again")
		}
	}

	fmt.Println("Fetching Epic library via Legendary...")
	out, err := legendaryListGames(bin)
	if err != nil {
		return err
	}

	var games []legendaryGame
	if err := json.Unmarshal(out, &games); err != nil {
		return fmt.Errorf("parse legendary output: %w", err)
	}

	eglInstalled, err := EpicInstalledMap()
	if err != nil {
		log.Printf("warning: could not read EGL manifests for install status: %v", err)
	}

	imported, installedCount := 0, 0
	for _, g := range games {
		if g.AppTitle == "" || g.AppName == "" {
			continue
		}
		norm := normalize.Title(g.AppTitle)
		if norm == "" {
			continue
		}
		gameID, err := db.UpsertGame(database, g.AppTitle, norm)
		if err != nil {
			log.Printf("warning: upsert game %q: %v", g.AppTitle, err)
			continue
		}
		installedFlag := 0
		installPath := ""
		if manifest, ok := eglInstalled[g.AppName]; ok {
			installedFlag = 1
			installPath = manifest.InstallLocation
			installedCount++
		}
		e := db.LibraryEntry{
			GameID:       gameID,
			Source:       "epic",
			SourceGameID: g.AppName,
			SourceTitle:  g.AppTitle,
			Owned:        1,
			Installed:    installedFlag,
			InstallPath:  installPath,
			LauncherURI:  fmt.Sprintf("com.epicgames.launcher://apps/%s?action=launch", g.AppName),
		}
		if err := db.UpsertLibraryEntry(database, e); err != nil {
			log.Printf("warning: upsert library entry %q: %v", g.AppTitle, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d games from Epic via Legendary (%d installed)\n", imported, installedCount)
	return nil
}

// epicFromManifests imports only locally-installed games from EGL manifests.
func epicFromManifests(database *sql.DB) error {
	installed, err := EpicInstalledMap()
	if err != nil {
		return err
	}
	imported, installedCount := 0, 0
	for _, m := range installed {
		if !isEpicGame(m.AppCategories) {
			continue
		}
		if m.DisplayName == "" {
			continue
		}
		norm := normalize.Title(m.DisplayName)
		if norm == "" {
			continue
		}
		gameID, err := db.UpsertGame(database, m.DisplayName, norm)
		if err != nil {
			log.Printf("warning: upsert game %q: %v", m.DisplayName, err)
			continue
		}
		e := db.LibraryEntry{
			GameID:       gameID,
			Source:       "epic",
			SourceGameID: m.AppName,
			SourceTitle:  m.DisplayName,
			Owned:        1,
			Installed:    1,
			InstallPath:  m.InstallLocation,
			LauncherURI:  fmt.Sprintf("com.epicgames.launcher://apps/%s?action=launch", m.AppName),
		}
		if err := db.UpsertLibraryEntry(database, e); err != nil {
			log.Printf("warning: upsert library entry %q: %v", m.DisplayName, err)
			continue
		}
		imported++
		installedCount++
	}
	fmt.Printf("Imported %d games from Epic (%d installed)\n", imported, installedCount)
	return nil
}

func isEpicGame(categories []string) bool {
	return slices.Contains(categories, "games")
}
