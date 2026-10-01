// Package config holds application settings.
//
// User-editable values can be overridden by an optional settings.json placed
// next to the executable (or in the working directory). Internal constants
// such as API endpoints are not configurable.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Version is set at build time from the git tag:
//
//	go build -ldflags "-X github.com/lamgopher/orbshacker/internal/config.Version=v1.0.0" ./cmd/orbshacker
var Version = "dev"

const (
	// Developer is the author of the original orbshacker.
	Developer = "Strykey"
	// PortAuthor is the author of this Go version.
	PortAuthor      = "lamgopher"
	RepoURL         = "https://github.com/lamgopher/orbshacker"
	OriginalRepoURL = "https://github.com/Strykey/orbshacker"

	DiscordAPIURL       = "https://discord.com/api/v9/applications/detectable"
	GitHubBackupURL     = "https://gist.githubusercontent.com/Cynosphere/c1e77f77f0e565ddaac2822977961e76/raw/gameslist.json"
	SteamCMDAPIURL      = "https://api.steamcmd.net/v1/info"
	SteamStoreSearchURL = "https://store.steampowered.com/api/storesearch"

	RequestTimeout     = 10 * time.Second
	RequestTimeoutLong = 20 * time.Second

	// TimerMinutes is how long the fake game process counts down.
	TimerMinutes = 15
)

// DiscordHeaders mimic a browser so the Discord API answers normally.
var DiscordHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Accept":          "application/json",
	"Accept-Language": "en-US,en;q=0.9",
	"Referer":         "https://discord.com/",
	"Origin":          "https://discord.com",
}

// Settings are the user-editable values.
type Settings struct {
	// FakeExeDir is the folder on the Desktop where fake executables are created.
	FakeExeDir string `json:"fake_exe_dir"`
	// MaxSearchResults limits the Discord database search output.
	MaxSearchResults int `json:"max_search_results"`
}

// Default returns the built-in settings.
func Default() Settings {
	return Settings{
		FakeExeDir:       "Win64",
		MaxSearchResults: 20,
	}
}

// Load returns the default settings overridden by the first settings.json found
// next to the executable or in the working directory.
func Load() Settings {
	s := Default()
	for _, path := range candidatePaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var override Settings
		if json.Unmarshal(data, &override) != nil {
			continue
		}
		if override.FakeExeDir != "" {
			s.FakeExeDir = override.FakeExeDir
		}
		if override.MaxSearchResults > 0 {
			s.MaxSearchResults = override.MaxSearchResults
		}
		break
	}
	return s
}

func candidatePaths() []string {
	var paths []string
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "settings.json"))
	}
	return append(paths, "settings.json")
}
