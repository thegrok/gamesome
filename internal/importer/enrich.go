package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

// steamAppListCacheKey is the meta key for the cached Steam app list JSON.
const steamAppListCacheKey = "steam_app_list"
const steamAppListCachedAtKey = "steam_app_list_cached_at"
const steamAppListCacheTTL = 7 * 24 * time.Hour

type steamAppList struct {
	Applist struct {
		Apps []struct {
			Appid int64  `json:"appid"`
			Name  string `json:"name"`
		} `json:"apps"`
	} `json:"applist"`
}

// EnrichSteamIDs downloads the Steam app list and fills in missing steam_appid values
// by matching normalized titles.
func EnrichSteamIDs(database *sql.DB) error {
	appMap, err := loadSteamAppList(database)
	if err != nil {
		return fmt.Errorf("load steam app list: %w", err)
	}

	games, err := db.GamesWithoutSteamAppID(database)
	if err != nil {
		return fmt.Errorf("query games without steam_appid: %w", err)
	}

	resolved := 0
	for _, g := range games {
		if appid, ok := appMap[g.NormalizedTitle]; ok {
			if err := db.SetSteamAppID(database, g.ID, appid); err != nil {
				log.Printf("warning: set steam_appid for game %d: %v", g.ID, err)
				continue
			}
			resolved++
		}
	}

	unresolved := len(games) - resolved
	fmt.Printf("Resolved %d new Steam IDs (%d games remain unresolved)\n", resolved, unresolved)
	return nil
}

func loadSteamAppList(database *sql.DB) (map[string]int64, error) {
	cachedAt := db.GetMeta(database, steamAppListCachedAtKey)
	if cachedAt != "" {
		t, err := time.Parse(time.RFC3339, cachedAt)
		if err == nil && time.Since(t) < steamAppListCacheTTL {
			cached := db.GetMeta(database, steamAppListCacheKey)
			if cached != "" {
				return parseSteamAppListJSON([]byte(cached))
			}
		}
	}

	resp, err := http.Get("https://api.steampowered.com/ISteamApps/GetAppList/v2/")
	if err != nil {
		return nil, fmt.Errorf("fetch steam app list: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read steam app list: %w", err)
	}

	appMap, err := parseSteamAppListJSON(data)
	if err != nil {
		return nil, err
	}

	// Cache the raw JSON and timestamp.
	_ = db.SetMeta(database, steamAppListCacheKey, string(data))
	_ = db.SetMeta(database, steamAppListCachedAtKey, time.Now().UTC().Format(time.RFC3339))

	return appMap, nil
}

func parseSteamAppListJSON(data []byte) (map[string]int64, error) {
	var list steamAppList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse steam app list: %w", err)
	}
	m := make(map[string]int64, len(list.Applist.Apps))
	for _, app := range list.Applist.Apps {
		if app.Name == "" {
			continue
		}
		norm := normalize.Title(app.Name)
		if norm != "" {
			m[norm] = app.Appid
		}
	}
	return m, nil
}

// steamStoreResponse is the appdetails API shape.
type steamStoreResponse map[string]struct {
	Success bool `json:"success"`
	Data    *struct {
		ShortDescription string `json:"short_description"`
		Genres           []struct {
			Description string `json:"description"`
		} `json:"genres"`
		Categories []struct {
			Description string `json:"description"`
		} `json:"categories"`
		Metacritic *struct {
			Score int `json:"score"`
		} `json:"metacritic"`
		ReleaseDate *struct {
			Date string `json:"date"`
		} `json:"release_date"`
		Platforms *struct {
			Linux bool `json:"linux"`
		} `json:"platforms"`
	} `json:"data"`
}

// releaseDateFormats are tried in order when parsing Steam's date strings.
var releaseDateFormats = []string{
	"2 Jan, 2006",
	"Jan 2, 2006",
	"2 January, 2006",
	"January 2, 2006",
	"Jan 2006",
	"January 2006",
	"2006",
}

// EnrichMetadata fetches Steam Store metadata for all games with a steam_appid.
func EnrichMetadata(database *sql.DB) error {
	games, err := db.GamesWithSteamAppID(database)
	if err != nil {
		return fmt.Errorf("query games with steam_appid: %w", err)
	}

	total := len(games)
	enriched, skipped, failed := 0, 0, 0
	client := &http.Client{Timeout: 15 * time.Second}

	for i, g := range games {
		if i > 0 && i%50 == 0 {
			fmt.Printf("Enriched %d/%d...\n", i, total)
		}

		url := fmt.Sprintf(
			"https://store.steampowered.com/api/appdetails?appids=%d&l=english",
			g.SteamAppID,
		)

		resp, err := client.Get(url)
		if err != nil {
			log.Printf("warning: fetch metadata for %q (appid %d): %v", g.Title, g.SteamAppID, err)
			failed++
			time.Sleep(time.Second)
			continue
		}

		var result steamStoreResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if decodeErr != nil {
			log.Printf("warning: decode metadata for %q: %v", g.Title, decodeErr)
			failed++
			time.Sleep(time.Second)
			continue
		}

		entry, ok := result[strconv.FormatInt(g.SteamAppID, 10)]
		if !ok || !entry.Success || entry.Data == nil {
			// Delisted or not a game — mark as done so we don't retry.
			_ = db.MarkEnrichedAt(database, g.ID)
			skipped++
			time.Sleep(time.Second)
			continue
		}

		d := entry.Data

		var genreStrs, catStrs []string
		for _, genre := range d.Genres {
			genreStrs = append(genreStrs, genre.Description)
		}
		for _, cat := range d.Categories {
			catStrs = append(catStrs, cat.Description)
		}

		var metacritic *int
		if d.Metacritic != nil {
			metacritic = &d.Metacritic.Score
		}

		var releaseDate *int64
		if d.ReleaseDate != nil && d.ReleaseDate.Date != "" {
			if ts := parseSteamDate(d.ReleaseDate.Date); ts != 0 {
				releaseDate = &ts
			}
		}

		var linuxNative *bool
		if d.Platforms != nil {
			v := d.Platforms.Linux
			linuxNative = &v
		}

		if err := db.UpdateGameMetadata(
			database, g.ID,
			strings.Join(genreStrs, ", "),
			strings.Join(catStrs, ", "),
			d.ShortDescription,
			metacritic, releaseDate, linuxNative,
		); err != nil {
			log.Printf("warning: store metadata for %q: %v", g.Title, err)
			failed++
		} else {
			enriched++
		}

		time.Sleep(time.Second)
	}

	fmt.Printf("Enriched %d games via Steam Store API (%d skipped, %d failed)\n", enriched, skipped, failed)
	return nil
}

func parseSteamDate(s string) int64 {
	s = strings.TrimSpace(s)
	for _, format := range releaseDateFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}
