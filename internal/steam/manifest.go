package steam

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrManifestExists is returned when an appmanifest for the app is already present,
// which usually means the game is really installed or being downloaded.
var ErrManifestExists = errors.New("appmanifest already exists")

const acfTemplate = `"AppState"
{
	"appid"		"%[1]d"
	"universe"		"1"
	"LauncherPath"		"%[2]s"
	"name"		"%[3]s"
	"StateFlags"		"1026"
	"installdir"		"%[4]s"
	"LastUpdated"		"0"
	"LastPlayed"		"0"
	"SizeOnDisk"		"0"
	"StagingSize"		"1073741824"
	"buildid"		"0"
	"LastOwner"		"%[5]s"
	"DownloadType"		"1"
	"UpdateResult"		"4"
	"BytesToDownload"		"1073741824"
	"BytesDownloaded"		"27262976"
	"BytesToStage"		"1073741824"
	"BytesStaged"		"27262976"
	"TargetBuildID"		"0"
	"AutoUpdateBehavior"		"0"
	"AllowOtherDownloadsWhileRunning"		"0"
	"ScheduledAutoUpdate"		"0"
	"InstalledDepots"
	{
	}
	"StagedDepots"
	{%[6]s
	}
	"UserConfig"
	{
	}
	"MountedConfig"
	{
	}
}
`

const stagedDepotTemplate = `
		"%s"
		{
			"manifest"		"0"
			"size"		"1073741824"
			"dlcappid"		"0"
		}`

// RenderManifest builds the content of a "download in progress" appmanifest (StateFlags 1026).
func RenderManifest(info AppInfo, steamPath, ownerID string) string {
	staged := ""
	if info.DepotID != "" {
		staged = fmt.Sprintf(stagedDepotTemplate, vdfEscape(info.DepotID))
	}
	launcher := filepath.Join(steamPath, "steam.exe")
	return fmt.Sprintf(acfTemplate,
		info.AppID,
		vdfEscape(launcher),
		vdfEscape(info.Name),
		vdfEscape(info.InstallDir),
		vdfEscape(ownerID),
		staged,
	)
}

// WriteManifest creates the appmanifest file. It refuses to overwrite an existing one.
func WriteManifest(info AppInfo, steamPath string) (string, error) {
	p := ManifestPath(steamPath, info.AppID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%w: %s", ErrManifestExists, p)
	}
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(RenderManifest(info, steamPath, UserID()))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(p)
		return "", err
	}
	return p, nil
}

var vdfReplacer = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func vdfEscape(s string) string {
	return vdfReplacer.Replace(s)
}
