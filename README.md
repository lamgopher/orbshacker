# orbshacker (Go TUI)

Standalone terminal UI version of [orbshacker](https://github.com/Strykey/orbshacker) —
a Windows tool that creates fake game processes for Discord Orb quests.
Single self-contained `.exe`, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
No Python required.

> **Educational purposes only.** Use at your own risk and in compliance with Discord's Terms of Service.

## Download

Prebuilt `orbshacker.exe` is attached to every [release](https://github.com/lamgopher/orbshacker/releases).

## Build

Requires Go 1.26+.

```bash
go build -o bin/orbshacker.exe ./cmd/orbshacker
```

Local builds report version `dev`. To stamp a version:

```bash
go build -ldflags "-X github.com/lamgopher/orbshacker/internal/config.Version=v1.0.0" -o bin/orbshacker.exe ./cmd/orbshacker
```

## Releases

Pushing a tag `v*` runs `.github/workflows/release.yml`: tests, builds `orbshacker.exe`
with the tag as its version and publishes a GitHub Release.

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Usage

Run `bin/orbshacker.exe`. Discord must be running.

1. **Search Discord database** — live search by name or alias, choose which executable to fake.
   If Discord lists no executables for a game but knows its Steam AppID, executable names are
   looked up in public SteamCMD metadata (no license needed); the fake still runs as a normal process.
2. **Manual mode** — type an exact process name (subfolders allowed: `bin/win64/Game.exe`).
3. **Steam Quest Mode** — search Steam, fetch app info from SteamCMD, review/edit name, install dir
   and exe, then create `appmanifest_<appid>.acf` + fake exe in `steamapps/common/<installdir>/`.
   Existing manifests and executables are never overwritten.
4. **Running fakes** — launched fakes with PID, status and uptime: `x` stop, `d` stop & delete files,
   `D` the same for all.
5. **Credits & info**
6. **Exit** — if fakes exist, choose to stop & clean everything or leave them running.

Each fake is a copy of `orbshacker.exe` renamed to the game's executable and started with
`--timer-mode` in its own console window. The window shows the game name, a 15-minute countdown,
elapsed time and end time. Press `q` or close the window to stop the fake.

## Settings

Optional `settings.json` next to the exe:

```json
{ "fake_exe_dir": "Win64", "max_search_results": 20 }
```

## Project layout

```
cmd/orbshacker/     entry point (main UI or --timer-mode)
internal/config/    settings and API endpoints
internal/discord/   detectable games database: load, search, executables
internal/steam/     Steam registry, Store/SteamCMD APIs, appmanifest generation
internal/faker/     copy, launch, track, stop and clean up fake processes
internal/timer/     countdown UI of a fake process
internal/tui/       main terminal UI screens
```

## Tests

```bash
go test ./...
```

## Credits

Original orbshacker by [Strykey](https://github.com/Strykey/orbshacker). Go port by lamgopher.

## License

Same license as the original project. See [LICENSE](./LICENSE).
