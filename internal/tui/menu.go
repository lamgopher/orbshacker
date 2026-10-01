package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lamgopher/orbshacker/internal/config"
)

type menuItem struct {
	title string
	desc  string
	open  func(a *App) (screen, tea.Cmd)
}

var menuItems = []menuItem{
	{"Search Discord database", "find a game by name or abbreviation (official API)", func(a *App) (screen, tea.Cmd) {
		return newDiscordSearch(a)
	}},
	{"Manual mode", "enter a custom executable name", func(a *App) (screen, tea.Cmd) {
		return newManual(a, "", "")
	}},
	{"Steam Quest Mode", "fake appmanifest + exe for games that check Steam", func(a *App) (screen, tea.Cmd) {
		return newSteam(a)
	}},
	{"Running fakes", "stop processes and clean up files", func(a *App) (screen, tea.Cmd) {
		return &procsScreen{}, nil
	}},
	{"Credits & info", "how it works", func(a *App) (screen, tea.Cmd) {
		return &creditsScreen{}, nil
	}},
	{"Exit", "", func(a *App) (screen, tea.Cmd) {
		return a.requestQuit(a.screen)
	}},
}

type menuScreen struct {
	cursor int
}

func (s *menuScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "up", "k":
		s.cursor = (s.cursor + len(menuItems) - 1) % len(menuItems)
	case "down", "j", "tab":
		s.cursor = (s.cursor + 1) % len(menuItems)
	case "enter", " ":
		return menuItems[s.cursor].open(a)
	case "q", "esc":
		return a.requestQuit(s)
	default:
		if k := key.String(); len(k) == 1 && k[0] >= '1' && int(k[0]-'0') <= len(menuItems) {
			s.cursor = int(k[0] - '1')
			return menuItems[s.cursor].open(a)
		}
	}
	return s, nil
}

func (s *menuScreen) view(a *App) string {
	var b strings.Builder
	if a.height >= 24 && a.width >= 70 {
		b.WriteString(sAccent.Bold(true).Render(banner))
		b.WriteString("\n")
		b.WriteString(sMuted.Render("Discord Orb Quest Faker · by " + config.Developer + " · Go port by " + config.PortAuthor))
		b.WriteString("\n\n")
	}

	for i, item := range menuItems {
		num := fmt.Sprintf(" %d ", i+1)
		title := item.title
		if i == 3 {
			title = fmt.Sprintf("%s (%d)", title, len(a.mgr.Fakes()))
		}
		line := num + " " + lipgloss.NewStyle().Width(30).Render(title)
		if i == s.cursor {
			b.WriteString(sCursor.Render("▸" + line))
		} else {
			b.WriteString(" " + sAccent.Render(num) + " " + lipgloss.NewStyle().Width(30).Render(title))
		}
		if item.desc != "" {
			b.WriteString("  " + sMuted.Render(item.desc))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + help("↑/↓", "move", "enter", "select", "1-6", "jump", "q", "quit"))
	return b.String()
}

type quitScreen struct {
	back screen
}

func (s *quitScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	if a.quitting {
		return s, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "s", "y", "enter":
		return s, a.quitWithCleanup()
	case "l":
		return s, tea.Quit
	case "esc", "n":
		return s.back, nil
	}
	return s, nil
}

func (s *quitScreen) view(a *App) string {
	if a.quitting {
		return sTitle.Render("EXIT") + "\n\n" + a.spinner.View() + " stopping fake processes and removing their files…"
	}
	running := a.mgr.RunningCount()
	total := len(a.mgr.Fakes())
	return sTitle.Render("EXIT") + "\n\n" +
		fmt.Sprintf("You have %d fake(s), %d of them running.\n", total, running) +
		sMuted.Render("Fakes keep working after orbshacker exits — they live in their own windows.") + "\n\n" +
		sKey.Render("s") + "  stop all and remove their files (incl. Steam appmanifests), then exit\n" +
		sKey.Render("l") + "  leave them as they are and exit\n" +
		sKey.Render("esc") + " cancel\n"
}

type creditsScreen struct {
	offset int
}

func creditsText() string {
	h := sSection.Render
	return strings.Join([]string{
		field("Developer", sCyan.Render(config.Developer)+sMuted.Render(" (original)")),
		field("Go port", sCyan.Render(config.PortAuthor)),
		field("Version", config.Version),
		field("Repository", sMuted.Render(config.RepoURL)),
		field("Based on", sMuted.Render(config.OriginalRepoURL)),
		"",
		"This tool works as a game process spoofer. It tricks Discord into thinking",
		"you're running a game by creating fake processes with the exact names",
		"Discord expects.",
		"",
		sBold.Render("IMPORTANT: ") + sRed.Render("Discord MUST be running for this to work!"),
		"",
		h("How it works (game spoofing)"),
		" 1. Connects to Discord's official API to get the latest game list",
		" 2. Finds the exact process name Discord expects for each game",
		" 3. Copies orbshacker itself to Desktop/Win64/ under that name",
		" 4. Launches the copy in its own window, showing a 15-minute timer",
		" 5. Discord scans running processes and detects the process name",
		" 6. The fake process must stay running for Discord to keep detecting it",
		"",
		h("Steam Quest Mode"),
		"Some games (Marathon, Toxic Commando…) require Discord to verify that Steam",
		"has at least partially downloaded them. Steam Quest Mode:",
		" 1. Fetches app info automatically from the SteamCMD public API",
		" 2. Generates a fake appmanifest_<appid>.acf in your steamapps/ folder",
		" 3. Places the fake exe in steamapps/common/<installdir>/",
		"It never overwrites an existing manifest or executable.",
		"",
		h("Database sources"),
		" • Primary: Discord Official API",
		" • Backup:  GitHub archive by Cynosphere",
		"",
		h("Multi-game emulation"),
		" • Launch a game, return to the menu, launch another — all run in parallel",
		" • Complete all orb quests simultaneously in about 15 minutes",
		" • Manage everything in \"Running fakes\": stop, remove files",
		"",
		sRed.Bold(true).Render("WARNING — EDUCATIONAL PURPOSES ONLY"),
		" • Users are solely responsible for compliance with Discord ToS",
		" • The developers are not responsible for any consequences",
		" • Use at your own risk",
	}, "\n")
}

func (s *creditsScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q", "enter", "backspace":
		return &menuScreen{cursor: 4}, nil
	case "up", "k":
		s.offset = max(s.offset-1, 0)
	case "down", "j":
		s.offset++
	}
	return s, nil
}

func (s *creditsScreen) view(a *App) string {
	lines := strings.Split(creditsText(), "\n")
	visible := max(a.height-10, 5)
	s.offset = min(s.offset, max(len(lines)-visible, 0))
	end := min(s.offset+visible, len(lines))
	return sTitle.Render("CREDITS") + "\n\n" +
		strings.Join(lines[s.offset:end], "\n") + "\n\n" +
		help("↑/↓", "scroll", "esc", "back")
}
