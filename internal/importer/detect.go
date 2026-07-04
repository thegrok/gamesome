package importer

import (
	"os"
	"path/filepath"
	"runtime"
)

// DetectedSources returns the subset of known import sources whose launcher
// data is present on this machine, in a stable order (steam, itchio, gog, epic).
// Values match the library_entries.source strings the importers write
// (itch.io's is "itchio", not "itch" — see internal/importer/itch.go).
// It only checks for presence — it does not run any importer.
func DetectedSources() []string {
	var sources []string
	if steamPresent() {
		sources = append(sources, "steam")
	}
	if itchPresent() {
		sources = append(sources, "itchio")
	}
	if gogPresent() {
		sources = append(sources, "gog")
	}
	if epicPresent() {
		sources = append(sources, "epic")
	}
	return sources
}

func steamPresent() bool {
	for _, dir := range steamAppsDirectories() {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}
	return false
}

func itchPresent() bool {
	dir, err := itchConfigDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "db", "butler.db"))
	return err == nil
}

func gogPresent() bool {
	if runtime.GOOS == "linux" {
		dir, err := heroicConfigDir()
		if err != nil {
			return false
		}
		_, err = os.Stat(filepath.Join(dir, "store_cache", "gog_library.json"))
		return err == nil
	}
	path, err := gogGalaxyDBPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func epicPresent() bool {
	dir, err := epicManifestsDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(dir)
	return err == nil
}
