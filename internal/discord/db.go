// Package discord loads and searches the database of Discord-detectable games.
package discord

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lamgopher/orbshacker/internal/config"
	"github.com/lamgopher/orbshacker/internal/netutil"
)

// Executable is a process Discord associates with a game.
type Executable struct {
	OS   string `json:"os"`
	Name string `json:"name"`
}

// SKU links a game to a store, e.g. {"distributor": "steam", "id": "366320"}.
type SKU struct {
	Distributor string `json:"distributor"`
	ID          string `json:"id"`
}

// Game is an entry of the detectable applications list.
type Game struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Aliases        []string     `json:"aliases"`
	Executables    []Executable `json:"executables"`
	ThirdPartySKUs []SKU        `json:"third_party_skus"`
}

// SteamAppID returns the Steam AppID of the game, if Discord knows it.
// Many games have no executables at all and are detected through Steam only.
func SteamAppID(g Game) (int, bool) {
	for _, s := range g.ThirdPartySKUs {
		if s.Distributor != "steam" {
			continue
		}
		if id, err := strconv.Atoi(s.ID); err == nil && id > 0 {
			return id, true
		}
	}
	return 0, false
}

// DB is an in-memory games database.
type DB struct {
	Games  []Game
	Source string
}

// Load fetches the games list from the Discord API, falling back to the GitHub archive.
func Load(ctx context.Context) (*DB, error) {
	var games []Game
	apiErr := netutil.FetchJSON(ctx, config.DiscordAPIURL, config.DiscordHeaders, nil, config.RequestTimeout, &games)
	if apiErr == nil {
		return &DB{Games: games, Source: "Discord Official API"}, nil
	}

	games = nil
	backupErr := netutil.FetchJSON(ctx, config.GitHubBackupURL, nil, nil, config.RequestTimeoutLong, &games)
	if backupErr == nil {
		return &DB{Games: games, Source: "GitHub Backup"}, nil
	}

	return nil, fmt.Errorf("could not load games database from any source: %w", errors.Join(apiErr, backupErr))
}

// Search finds games by name or alias: exact matches first, then partial ones.
func (db *DB) Search(query string, limit int) []Game {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	var exact, partial []Game
	seen := make(map[string]bool)
	for _, g := range db.Games {
		if seen[g.ID] {
			continue
		}
		name := strings.ToLower(g.Name)
		isExact, isPartial := q == name, strings.Contains(name, q)
		for _, a := range g.Aliases {
			a = strings.ToLower(a)
			isExact = isExact || q == a
			isPartial = isPartial || strings.Contains(a, q)
		}
		switch {
		case isExact:
			exact = append(exact, g)
		case isPartial:
			partial = append(partial, g)
		default:
			continue
		}
		seen[g.ID] = true
	}

	result := append(exact, partial...)
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// skipExePatterns mark helper binaries that are unlikely to be the game itself.
var skipExePatterns = []string{
	"_be.exe", "_eac.exe", "launcher", "unins",
	"crash", "report", "update", "setup", "install",
}

// PrimaryExecutable returns the most likely Windows game executable, or "" if none.
func PrimaryExecutable(g Game) string {
	exes := win32Executables(g, true)
	if len(exes) == 0 {
		return ""
	}
	return exes[0]
}

// AllExecutables returns every Windows executable of the game, primary first.
func AllExecutables(g Game) []string {
	primary := PrimaryExecutable(g)
	all := win32Executables(g, false)
	if primary == "" {
		return all
	}
	result := []string{primary}
	for _, e := range all {
		if e != primary {
			result = append(result, e)
		}
	}
	return result
}

func win32Executables(g Game, skipHelpers bool) []string {
	var result []string
	seen := make(map[string]bool)
	for _, e := range g.Executables {
		if e.OS != "win32" {
			continue
		}
		name := strings.ReplaceAll(strings.TrimPrefix(e.Name, ">"), `\`, "/")
		if name == "" || seen[name] {
			continue
		}
		if skipHelpers && isHelper(name) {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	return result
}

func isHelper(name string) bool {
	lower := strings.ToLower(name)
	for _, p := range skipExePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
