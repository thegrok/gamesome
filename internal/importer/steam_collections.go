package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SteamCollection represents a parsed Steam collection from cloud storage.
type SteamCollection struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Added     []int64 `json:"added"`
	Removed   []int64 `json:"removed"`
	FilterSpec interface{} `json:"filterSpec,omitempty"`
}

// SteamCollectionsImportResult holds the results of a collections import.
type SteamCollectionsImportResult struct {
	CollectionFound bool
	CollectionName  string
	CollectionID    string
	IsDynamic       bool
	AppIDCount      int
	MatchedCount    int
	NewlyMarkedCount int
	UnmatchedAppIDs []int64
	Error           string
}

// SteamCollectionsImport reads the Steam collections from cloud storage and marks
// games as completed based on a named collection. If dryRun is true, no database
// writes are performed.
func SteamCollectionsImport(database *sql.DB, collectionName string, dryRun bool) (SteamCollectionsImportResult, error) {
	result := SteamCollectionsImportResult{}

	// Find the cloud storage JSON file
	collectionData, _, err := findSteamCollection(collectionName)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	if collectionData == nil {
		result.Error = fmt.Sprintf("collection %q not found", collectionName)
		return result, nil
	}

	result.CollectionFound = true
	result.CollectionName = collectionData.Name
	result.CollectionID = collectionData.ID
	result.AppIDCount = len(collectionData.Added)

	// Check if it's a dynamic collection
	if collectionData.FilterSpec != nil && len(collectionData.Added) == 0 {
		result.IsDynamic = true
		// Still process, but warn
	}

	// Build a map of steam_appid -> game_id for quick lookup
	appIDToGameID := make(map[int64]int64)
	rows, err := database.Query(`SELECT id, steam_appid FROM games WHERE steam_appid IS NOT NULL`)
	if err != nil {
		return result, fmt.Errorf("query games: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var gameID, appID int64
		if err := rows.Scan(&gameID, &appID); err != nil {
			return result, fmt.Errorf("scan game: %w", err)
		}
		appIDToGameID[appID] = gameID
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	// Process each AppID in the collection
	now := time.Now().UTC().Format(time.RFC3339)
	unmatched := make([]int64, 0)
	matched := make([]int64, 0)

	for _, appID := range collectionData.Added {
		gameID, found := appIDToGameID[appID]
		if !found {
			unmatched = append(unmatched, appID)
			continue
		}

		matched = append(matched, appID)

		if !dryRun {
			// Update completed_at only if not already set (idempotent)
			res, err := database.Exec(
				`UPDATE games SET completed_at = ? WHERE id = ? AND completed_at IS NULL`,
				now, gameID,
			)
			if err != nil {
				log.Printf("warning: mark completed appid %d (game %d): %v", appID, gameID, err)
				continue
			}

			if affected, err := res.RowsAffected(); err == nil && affected > 0 {
				result.NewlyMarkedCount++
			}
		} else {
			// In dry-run mode, assume it would be updated
			result.NewlyMarkedCount++
		}
	}

	result.MatchedCount = len(matched)
	result.UnmatchedAppIDs = unmatched

	if !dryRun {
		// Log the import for auditing
		logImportMsg := fmt.Sprintf(
			"Imported %s collection: %d matched, %d newly marked, %d unmatched",
			collectionName, result.MatchedCount, result.NewlyMarkedCount, len(unmatched),
		)
		log.Printf(logImportMsg)
	}

	return result, nil
}

// findSteamCollection searches for a Steam collection in the cloud storage JSON file.
// The collection data is stored in Steam's cloud storage metadata, not in the LevelDB
// Local Storage cache, so it's always accessible regardless of Steam process state.
// Returns the collection data, the user ID, or an error if not found.
func findSteamCollection(collectionName string) (*SteamCollection, string, error) {
	// Try both standard locations
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".local/share/Steam/userdata"),
		filepath.Join(os.Getenv("HOME"), ".steam/steam/userdata"),
	}

	for _, baseDir := range candidates {
		if _, err := os.Stat(baseDir); err != nil {
			continue
		}

		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		// Each subdirectory is a user ID
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			userID := entry.Name()
			cloudStorageFile := filepath.Join(baseDir, userID, "config/cloudstorage/cloud-storage-namespace-1.json")

			data, err := os.ReadFile(cloudStorageFile)
			if err != nil {
				continue
			}

			// Parse the JSON array format
			collection, err := parseCloudStorageJSON(data, collectionName)
			if err != nil {
				log.Printf("warning: parse cloud storage for user %s: %v", userID, err)
				continue
			}

			if collection != nil {
				return collection, userID, nil
			}
		}
	}

	return nil, "", fmt.Errorf("steam collection data not found in standard locations")
}

// parseCloudStorageJSON parses Steam's cloud storage JSON and extracts a named collection.
// The format is a JSON array of [key, value] pairs where value is an object with a "value" field
// containing a JSON string.
func parseCloudStorageJSON(data []byte, collectionName string) (*SteamCollection, error) {
	// Steam stores it as a JSON array of [key, {value: "...", ...}] pairs
	var entries [][]interface{}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}

	// Case-insensitive matching
	lowerName := strings.ToLower(collectionName)

	for _, entry := range entries {
		if len(entry) < 2 {
			continue
		}

		// First element should be the key
		key, ok := entry[0].(string)
		if !ok {
			continue
		}

		// Look for user-collections entries
		if !strings.HasPrefix(key, "user-collections.") {
			continue
		}

		// Second element is the value object
		valueObj, ok := entry[1].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract the "value" field which contains the JSON string
		valueStr, ok := valueObj["value"].(string)
		if !ok || valueStr == "" {
			continue
		}

		// Parse the inner JSON
		var collection SteamCollection
		if err := json.Unmarshal([]byte(valueStr), &collection); err != nil {
			continue
		}

		// Check if this matches the desired collection name
		if strings.EqualFold(collection.Name, lowerName) {
			return &collection, nil
		}
	}

	return nil, nil
}

// FormatUnmatchedAppIDs formats the unmatched appids for display
func FormatUnmatchedAppIDs(appids []int64, maxDisplay int) string {
	if len(appids) == 0 {
		return ""
	}

	// Sort for consistent display
	sort.Slice(appids, func(i, j int) bool { return appids[i] < appids[j] })

	if len(appids) <= maxDisplay {
		return fmt.Sprintf("%v", appids)
	}

	// Show first N and count remaining
	displayed := appids[:maxDisplay]
	return fmt.Sprintf("%v... (+%d more)", displayed, len(appids)-maxDisplay)
}
