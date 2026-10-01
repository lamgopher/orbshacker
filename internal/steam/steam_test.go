package steam

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

const steamCMDSample = `{
  "data": {
    "730": {
      "common": {"name": "Counter-Strike 2"},
      "config": {
        "installdir": "Counter-Strike Global Offensive",
        "launch": {
          "1": {"executable": "game\\bin\\linuxsteamrt64\\cs2", "config": {"oslist": "linux"}},
          "0": {"executable": "game\\bin\\win64\\cs2.exe", "config": {"oslist": "windows"}}
        }
      },
      "depots": {"branches": {}, "228989": {"depotfromapp": "228980", "sharedinstall": "1"}, "2347771": {}, "2347770": {}, "baselanguages": "english"}
    }
  }
}`

func TestAppInfoFrom(t *testing.T) {
	var resp steamCMDResponse
	if err := json.Unmarshal([]byte(steamCMDSample), &resp); err != nil {
		t.Fatal(err)
	}
	got := appInfoFrom(730, resp.Data["730"])
	want := AppInfo{
		AppID:      730,
		Name:       "Counter-Strike 2",
		InstallDir: "Counter-Strike Global Offensive",
		Executable: "game/bin/win64/cs2.exe",
		DepotID:    "2347770",
	}
	if got != want {
		t.Fatalf("appInfoFrom() = %+v, want %+v", got, want)
	}
}

func TestAppInfoFromFallbacks(t *testing.T) {
	got := appInfoFrom(42, steamCMDApp{})
	if got.Name != "App 42" || got.InstallDir != "App 42" || got.Executable != "App 42.exe" || got.DepotID != "" {
		t.Fatalf("unexpected fallbacks: %+v", got)
	}
}

func TestWindowsExes(t *testing.T) {
	windows, linux, empty := "windows", "linux", ""
	launch := map[string]steamCMDLaunch{
		"0": {Executable: "Game.exe"},
		"1": {Executable: `.\bin\Tool.EXE`, Config: launchConfig(&empty)},
		"2": {Executable: "game.sh", Config: launchConfig(&linux)},
		"3": {Executable: "Game.exe", Config: launchConfig(&windows)},
		"4": {Executable: "../outside.exe"},
	}
	got := windowsExes(launch)
	want := []string{"Game.exe", "bin/Tool.EXE"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windowsExes() = %v, want %v", got, want)
	}
}

func launchConfig(osList *string) (c struct {
	OSList *string `json:"oslist"`
}) {
	c.OSList = osList
	return c
}

func TestRenderManifest(t *testing.T) {
	info := AppInfo{AppID: 10, Name: `Say "Hi"`, InstallDir: "Hi", DepotID: "11"}
	got := RenderManifest(info, `C:\Steam`, "7656")
	for _, want := range []string{
		`"appid"		"10"`,
		`"LauncherPath"		"C:\\Steam\\steam.exe"`,
		`"name"		"Say \"Hi\""`,
		`"LastOwner"		"7656"`,
		`"StateFlags"		"1026"`,
		"\t\t\"11\"\n\t\t{",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("manifest does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteManifestRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	info := AppInfo{AppID: 5, Name: "X", InstallDir: "X"}
	p, err := WriteManifest(info, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteManifest(info, dir); !errors.Is(err, ErrManifestExists) {
		t.Fatalf("second write error = %v, want ErrManifestExists", err)
	}
}
