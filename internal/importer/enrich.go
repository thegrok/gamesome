package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thegrok/gamesom/internal/db"
	"github.com/thegrok/gamesom/internal/normalize"
)

type steamSearchResponse struct {
	Items []struct {
		Type string `json:"type"`
		Name string `json:"name"`
		ID   int64  `json:"id"`
	} `json:"items"`
}

// EnrichSteamIDs searches the Steam store by title for each game without a steam_appid.
func EnrichSteamIDs(database *sql.DB) error {
	games, err := db.GamesWithoutSteamAppID(database)
	if err != nil {
		return fmt.Errorf("query games without steam_appid: %w", err)
	}

	if len(games) == 0 {
		fmt.Println("All games already have Steam IDs resolved.")
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resolved, skipped := 0, 0

	for i, g := range games {
		if i > 0 && i%50 == 0 {
			fmt.Printf("Searched %d/%d...\n", i, len(games))
		}

		appid, err := searchSteamByTitle(client, g.NormalizedTitle, g.CanonicalTitle)
		if err != nil {
			log.Printf("warning: steam search for %q: %v", g.CanonicalTitle, err)
			skipped++
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if appid == 0 {
			skipped++
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err := db.SetSteamAppID(database, g.ID, appid); err != nil {
			log.Printf("warning: set steam_appid for %q: %v", g.CanonicalTitle, err)
		} else {
			resolved++
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("Resolved %d new Steam IDs (%d unmatched)\n", resolved, skipped)
	return nil
}

// searchSteamByTitle queries the Steam store search API and returns the appid of
// the first result whose normalized name matches the game's normalized title.
func searchSteamByTitle(client *http.Client, normalizedTitle, canonicalTitle string) (int64, error) {
	u := "https://store.steampowered.com/api/storesearch/?term=" +
		url.QueryEscape(canonicalTitle) + "&l=english&cc=US"

	resp, err := client.Get(u)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result steamSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	for _, item := range result.Items {
		if item.Type != "app" {
			continue
		}
		if normalize.Title(item.Name) == normalizedTitle {
			return item.ID, nil
		}
	}
	return 0, nil
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
