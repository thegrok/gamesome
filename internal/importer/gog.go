package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

func gogGalaxyDBPath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		pd := os.Getenv("PROGRAMDATA")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		return filepath.Join(pd, "GOG.com", "Galaxy", "storage", "galaxy-2.0.db"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "GOG.com", "Galaxy", "storage", "galaxy-2.0.db"), nil
	default:
		return "", fmt.Errorf("gog galaxy import not supported on %s; use gamesom import gog", runtime.GOOS)
	}
}

// GOG imports games from GOG Galaxy's local SQLite database.
func GOG(database *sql.DB) error {
	dbPath, err := gogGalaxyDBPath()
	if err != nil {
		return err
	}

	galaxyDB, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open galaxy db: %w", err)
	}
	defer galaxyDB.Close()

	// Resolve the gamePieceTypeId for 'title'
	var titleTypeID int
	err = galaxyDB.QueryRow(`SELECT id FROM GamePieceTypes WHERE type = 'title'`).Scan(&titleTypeID)
	if err != nil {
		return fmt.Errorf("GamePieceTypes lookup failed — GOG Galaxy schema may have changed: %w", err)
	}

	// Get owned GOG release keys from LibraryReleases
	rows, err := galaxyDB.Query(`SELECT releaseKey FROM LibraryReleases WHERE releaseKey LIKE 'gog_%'`)
	if err != nil {
		return fmt.Errorf("query LibraryReleases: %w", err)
	}
	defer rows.Close()

	// Get installed paths from InstalledBaseProducts (keyed by integer productId)
	installed := map[int64]string{}
	irows, err := galaxyDB.Query(`SELECT productId, installationPath FROM InstalledBaseProducts`)
	if err == nil {
		defer irows.Close()
		for irows.Next() {
			var pid int64
			var path string
			if irows.Scan(&pid, &path) == nil {
				installed[pid] = path
			}
		}
	}

	imported, installedCount := 0, 0
	for rows.Next() {
		var releaseKey string
		if err := rows.Scan(&releaseKey); err != nil {
			continue
		}

		// Parse numeric ID from releaseKey (e.g. "gog_1207658924" -> 1207658924)
		numericStr := strings.TrimPrefix(releaseKey, "gog_")
		numericID, err := strconv.ParseInt(numericStr, 10, 64)
		if err != nil {
			continue
		}

		// Get title from GamePieces
		var valueJSON string
		err = galaxyDB.QueryRow(
			`SELECT value FROM GamePieces WHERE releaseKey = ? AND gamePieceTypeId = ? LIMIT 1`,
			releaseKey, titleTypeID,
		).Scan(&valueJSON)
		if err != nil {
			continue
		}

		var titleObj struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(valueJSON), &titleObj); err != nil || titleObj.Title == "" {
			continue
		}

		norm := normalize.Title(titleObj.Title)
		if norm == "" {
			continue
		}

		gameID, err := db.UpsertGame(database, titleObj.Title, norm)
		if err != nil {
			continue
		}

		installPath, isInstalled := installed[numericID]
		installedFlag := 0
		if isInstalled {
			installedFlag = 1
			installedCount++
		}

		e := db.LibraryEntry{
			GameID:       gameID,
			Source:       "gog",
			SourceGameID: releaseKey,
			SourceTitle:  titleObj.Title,
			Owned:        1,
			Installed:    installedFlag,
			InstallPath:  installPath,
			LauncherURI:  fmt.Sprintf("goggalaxy://openGame/%d", numericID),
		}
		if err := db.UpsertLibraryEntry(database, e); err != nil {
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d games from GOG Galaxy (%d installed)\n", imported, installedCount)
	return nil
}
