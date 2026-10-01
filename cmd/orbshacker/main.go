// Command orbshacker is a TUI tool that creates fake game processes for Discord Orb quests.
//
// When started with --timer-mode (by a renamed copy of itself) it runs as the
// fake game process and shows a countdown instead of the main UI.
//
// EDUCATIONAL PURPOSES ONLY.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lamgopher/orbshacker/internal/config"
	"github.com/lamgopher/orbshacker/internal/faker"
	"github.com/lamgopher/orbshacker/internal/timer"
	"github.com/lamgopher/orbshacker/internal/tui"
)

func main() {
	timerMode := flag.Bool(faker.TimerModeFlag[2:], false, "run as a fake game process")
	title := flag.String(faker.TitleFlag[2:], "", "game name shown in timer mode")
	flag.Parse()

	if err := run(*timerMode, *title); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(timerMode bool, title string) error {
	if timerMode {
		return timer.Run(title)
	}
	settings := config.Load()
	mgr, err := faker.NewManager(settings.FakeExeDir)
	if err != nil {
		return err
	}
	return tui.Run(settings, mgr)
}
