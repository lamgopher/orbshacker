// Package steam implements Steam Quest Mode helpers: locating Steam,
// querying the store and SteamCMD APIs and generating a fake appmanifest.
package steam

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lamgopher/orbshacker/internal/config"
	"github.com/lamgopher/orbshacker/internal/netutil"
)

// steamID64Base converts a 32-bit account ID into a SteamID64.
const steamID64Base = 76561197960265728

// StoreItem is a Steam store search result.
type StoreItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// AppInfo describes what is needed to fake a Steam installation.
type AppInfo struct {
	AppID      int
	Name       string
	InstallDir string
	// Executable is relative to InstallDir, with forward slashes.
	Executable string
	// DepotID is empty when no depot is known.
	DepotID string
}

// FindPath returns the Steam installation directory, or "" if not found.
func FindPath() string {
	if p := registrySteamPath(); p != "" {
		if dirExists(p) {
			return filepath.Clean(p)
		}
	}
	if fallback := `C:\Program Files (x86)\Steam`; dirExists(fallback) {
		return fallback
	}
	return ""
}

// UserID returns the SteamID64 of the logged-in user, or "0" if unknown.
func UserID() string {
	accountID, ok := registryActiveUser()
	if !ok || accountID == 0 {
		return "0"
	}
	return strconv.FormatUint(uint64(accountID)+steamID64Base, 10)
}

// SearchStore searches the Steam store by game name.
func SearchStore(ctx context.Context, query string) ([]StoreItem, error) {
	var resp struct {
		Items []StoreItem `json:"items"`
	}
	params := url.Values{"term": {query}, "l": {"english"}, "cc": {"US"}}
	if err := netutil.FetchJSON(ctx, config.SteamStoreSearchURL, nil, params, config.RequestTimeout, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

type steamCMDResponse struct {
	Data map[string]steamCMDApp `json:"data"`
}

type steamCMDApp struct {
	Common struct {
		Name string `json:"name"`
	} `json:"common"`
	Config struct {
		InstallDir string                    `json:"installdir"`
		Launch     map[string]steamCMDLaunch `json:"launch"`
	} `json:"config"`
	Depots map[string]any `json:"depots"`
}

type steamCMDLaunch struct {
	Executable string `json:"executable"`
	Config     struct {
		OSList *string `json:"oslist"`
	} `json:"config"`
}

// FetchAppInfo queries the SteamCMD API for install details of an app.
func FetchAppInfo(ctx context.Context, appID int) (AppInfo, error) {
	app, err := fetchApp(ctx, appID)
	if err != nil {
		return AppInfo{}, err
	}
	return appInfoFrom(appID, app), nil
}

// FetchWindowsExecutables returns the Windows executables from the app's Steam
// launch options, relative to its install folder. It reads public metadata only
// and does not need the game to be owned.
func FetchWindowsExecutables(ctx context.Context, appID int) ([]string, error) {
	app, err := fetchApp(ctx, appID)
	if err != nil {
		return nil, err
	}
	return windowsExes(app.Config.Launch), nil
}

func fetchApp(ctx context.Context, appID int) (steamCMDApp, error) {
	var resp steamCMDResponse
	rawURL := fmt.Sprintf("%s/%d", config.SteamCMDAPIURL, appID)
	if err := netutil.FetchJSON(ctx, rawURL, nil, nil, config.RequestTimeout, &resp); err != nil {
		return steamCMDApp{}, err
	}
	app, ok := resp.Data[strconv.Itoa(appID)]
	if !ok {
		return steamCMDApp{}, fmt.Errorf("SteamCMD has no data for app %d", appID)
	}
	return app, nil
}

func appInfoFrom(appID int, app steamCMDApp) AppInfo {
	info := AppInfo{AppID: appID, Name: app.Common.Name}
	if info.Name == "" {
		info.Name = fmt.Sprintf("App %d", appID)
	}
	info.InstallDir = app.Config.InstallDir
	if info.InstallDir == "" {
		info.InstallDir = info.Name
	}
	if exes := windowsExes(app.Config.Launch); len(exes) > 0 {
		info.Executable = exes[0]
	} else {
		info.Executable = path.Base(filepath.ToSlash(info.InstallDir)) + ".exe"
	}
	info.DepotID = firstDepot(app.Depots)
	return info
}

// windowsExes returns the distinct Windows .exe launch entries in key order.
func windowsExes(launch map[string]steamCMDLaunch) []string {
	keys := make([]string, 0, len(launch))
	for k := range launch {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result []string
	seen := make(map[string]bool)
	for _, k := range keys {
		entry := launch[k]
		osList := "windows"
		if entry.Config.OSList != nil {
			osList = *entry.Config.OSList
		}
		if osList != "" && !strings.Contains(osList, "windows") {
			continue
		}
		exe := path.Clean(strings.ReplaceAll(strings.TrimSpace(entry.Executable), `\`, "/"))
		if !strings.HasSuffix(strings.ToLower(exe), ".exe") || strings.HasPrefix(exe, "../") || seen[exe] {
			continue
		}
		seen[exe] = true
		result = append(result, exe)
	}
	return result
}

// firstDepot returns the lowest numeric depot ID that belongs to the app itself.
// Shared depots borrowed from other apps (e.g. VC++ redistributables) are used
// only when nothing else is available.
func firstDepot(depots map[string]any) string {
	own, shared := -1, -1
	for k, v := range depots {
		id, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		depot, isObject := v.(map[string]any)
		if !isObject {
			continue
		}
		_, fromApp := depot["depotfromapp"]
		_, sharedInstall := depot["sharedinstall"]
		if fromApp || sharedInstall {
			if shared < 0 || id < shared {
				shared = id
			}
		} else if own < 0 || id < own {
			own = id
		}
	}
	switch {
	case own >= 0:
		return strconv.Itoa(own)
	case shared >= 0:
		return strconv.Itoa(shared)
	}
	return ""
}

// ManifestPath returns the appmanifest location for the app.
func ManifestPath(steamPath string, appID int) string {
	return filepath.Join(steamPath, "steamapps", fmt.Sprintf("appmanifest_%d.acf", appID))
}

// FakeExePath returns where the fake executable goes inside steamapps/common.
func FakeExePath(steamPath string, info AppInfo) string {
	return filepath.Join(steamPath, "steamapps", "common", filepath.FromSlash(info.InstallDir), filepath.FromSlash(info.Executable))
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
