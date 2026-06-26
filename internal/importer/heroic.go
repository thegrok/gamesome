package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func heroicConfigDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return filepath.Join(os.Getenv("HOME"), ".var/app/com.heroicgameslauncher.hgl/config/heroic"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "heroic"), nil
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appdata, "heroic"), nil
	default:
		return "", fmt.Errorf("heroic import not supported on %s", runtime.GOOS)
	}
}

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

// heroicInstalledEntry is one entry from legendary's installed.json, keyed by app_name.
type heroicInstalledEntry struct {
	AppName     string `json:"app_name"`
	InstallPath string `json:"install_path"`
	IsDLC       bool   `json:"is_dlc"`
}


func readHeroicLibrary() ([]heroicLibraryEntry, error) {
	dir, err := heroicConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "store_cache", "legendary_library.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	var f heroicLibraryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse legendary_library.json: %w", err)
	}
	return f.Library, nil
}

func readHeroicInstalled() (map[string]heroicInstalledEntry, error) {
	dir, err := heroicConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "legendaryConfig", "legendary", "installed.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	var info map[string]heroicInstalledEntry
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse installed.json: %w", err)
	}
	return info, nil
}
