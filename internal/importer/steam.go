package importer

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

type steamAPIResponse struct {
	Response struct {
		Games []struct {
			AppID           int64  `json:"appid"`
			Name            string `json:"name"`
			PlaytimeForever int    `json:"playtime_forever"`
			RtimeLastPlayed int64  `json:"rtime_last_played"`
		} `json:"games"`
	} `json:"response"`
}

// Steam imports games from Steam via Web API (if keys present) then local manifests.
func Steam(database *sql.DB) error {
	apiKey := os.Getenv("STEAM_API_KEY")
	steamID := os.Getenv("STEAM_ID")

	apiImported := 0
	var apiErr error

	if apiKey != "" && steamID != "" {
		apiImported, apiErr = steamAPIImport(database, apiKey, steamID)
		if apiErr != nil {
			log.Printf("warning: Steam API import failed: %v", apiErr)
		}
	} else {
		fmt.Println("Steam API skipped (no STEAM_API_KEY/STEAM_ID)")
	}

	manifestImported, manifestInstalled := steamManifestScan(database, apiImported > 0)

	if apiKey != "" && steamID != "" && apiErr == nil {
		fmt.Printf("Imported %d games from Steam (%d from API, %d installed from local manifests)\n",
			apiImported+manifestImported, apiImported, manifestInstalled)
	} else {
		fmt.Printf("Steam API skipped (no STEAM_API_KEY/STEAM_ID); found %d installed via local manifests\n",
			manifestInstalled)
	}
	return nil
}

func steamAPIImport(database *sql.DB, apiKey, steamID string) (int, error) {
	url := fmt.Sprintf(
		"https://api.steampowered.com/IPlayerService/GetOwnedGames/v0001/?key=%s&steamid=%s&format=json&include_appinfo=1&include_played_free_games=1",
		apiKey, steamID,
	)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	var result steamAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	imported := 0
	for _, g := range result.Response.Games {
		if g.Name == "" {
			continue
		}

		norm := normalize.Title(g.Name)
		if norm == "" {
			continue
		}

		gameID, err := db.UpsertGame(database, g.Name, norm)
		if err != nil {
			log.Printf("warning: upsert game %q: %v", g.Name, err)
			continue
		}

		// Update steam_appid on the games row if not set.
		_, _ = database.Exec(
			`UPDATE games SET steam_appid = ? WHERE id = ? AND steam_appid IS NULL`,
			g.AppID, gameID,
		)

		var lastPlayed *string
		if g.RtimeLastPlayed > 0 {
			t := time.Unix(g.RtimeLastPlayed, 0).UTC().Format(time.RFC3339)
			lastPlayed = &t
		}

		entry := db.LibraryEntry{
			GameID:          gameID,
			Source:          "steam",
			SourceGameID:    strconv.FormatInt(g.AppID, 10),
			SourceTitle:     g.Name,
			Owned:           1,
			Installed:       0,
			PlaytimeMinutes: g.PlaytimeForever,
			LastPlayedAt:    lastPlayed,
		}
		if err := db.UpsertLibraryEntry(database, entry); err != nil {
			log.Printf("warning: upsert library entry %q: %v", g.Name, err)
			continue
		}
		imported++
	}
	return imported, nil
}

// steamManifestScan reads local ACF manifests to detect installed games.
// apiRan=true → just update installed state for existing entries.
// apiRan=false → insert as owned+installed.
// Returns (newInserts, installedCount).
func steamManifestScan(database *sql.DB, apiRan bool) (int, int) {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".steam", "steam", "steamapps"),
		filepath.Join(os.Getenv("HOME"), ".local", "share", "Steam", "steamapps"),
	}

	newInserts := 0
	installedCount := 0

	for _, dir := range candidates {
		matches, err := filepath.Glob(filepath.Join(dir, "appmanifest_*.acf"))
		if err != nil || len(matches) == 0 {
			continue
		}

		for _, path := range matches {
			fields, err := parseACF(path)
			if err != nil {
				log.Printf("warning: parse %s: %v", path, err)
				continue
			}

			appIDStr := fields["appid"]
			name := fields["name"]
			installDir := fields["installdir"]

			if appIDStr == "" || name == "" {
				continue
			}

			appID, err := strconv.ParseInt(appIDStr, 10, 64)
			if err != nil {
				continue
			}

			installPath := filepath.Join(dir, "common", installDir)
			installedCount++

			if apiRan {
				if err := db.UpdateInstalledBySteamAppID(database, appID, installPath); err != nil {
					log.Printf("warning: update installed for appid %d: %v", appID, err)
				}
				continue
			}

			// No API run — insert as owned+installed.
			norm := normalize.Title(name)
			if norm == "" {
				continue
			}

			gameID, err := db.UpsertGame(database, name, norm)
			if err != nil {
				log.Printf("warning: upsert game %q: %v", name, err)
				continue
			}

			_, _ = database.Exec(
				`UPDATE games SET steam_appid = ? WHERE id = ? AND steam_appid IS NULL`,
				appID, gameID,
			)

			entry := db.LibraryEntry{
				GameID:       gameID,
				Source:       "steam",
				SourceGameID: appIDStr,
				SourceTitle:  name,
				Owned:        1,
				Installed:    1,
				InstallPath:  installPath,
			}
			if err := db.UpsertLibraryEntry(database, entry); err != nil {
				log.Printf("warning: upsert library entry %q: %v", name, err)
				continue
			}
			newInserts++
		}
	}

	return newInserts, installedCount
}

// parseACF reads a Valve VDF app manifest and returns top-level key/value pairs.
func parseACF(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fields := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Match lines like:  "key"  "value"
		parts := splitVDFLine(line)
		if len(parts) == 2 {
			fields[strings.ToLower(parts[0])] = parts[1]
		}
	}
	return fields, scanner.Err()
}

// splitVDFLine extracts the two quoted tokens from a VDF key-value line.
func splitVDFLine(line string) []string {
	var tokens []string
	inQuote := false
	var cur strings.Builder

	for _, r := range line {
		switch {
		case r == '"' && !inQuote:
			inQuote = true
		case r == '"' && inQuote:
			tokens = append(tokens, cur.String())
			cur.Reset()
			inQuote = false
		case inQuote:
			cur.WriteRune(r)
		}
		if len(tokens) == 2 {
			break
		}
	}
	return tokens
}
