package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lamgopher/orbshacker/internal/discord"
	"github.com/lamgopher/orbshacker/internal/steam"
)

// --- search ------------------------------------------------------------------

type discordSearchScreen struct {
	input   textinput.Model
	results []discord.Game
	cursor  int
}

func newDiscordSearch(a *App) (screen, tea.Cmd) {
	in := textinput.New()
	in.Placeholder = "PUBG, Fortnite, League, Valorant, Minecraft…"
	in.Prompt = "Search › "
	in.PromptStyle = sAccent
	in.CharLimit = 100
	cmd := in.Focus()
	return &discordSearchScreen{input: in}, cmd
}

func (s *discordSearchScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	if _, loaded := msg.(dbLoadedMsg); loaded {
		s.refresh(a)
		return s, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return &menuScreen{cursor: 0}, nil
		case "up", "ctrl+p":
			s.cursor = max(s.cursor-1, 0)
			return s, nil
		case "down", "ctrl+n", "tab":
			s.cursor = min(s.cursor+1, max(len(s.results)-1, 0))
			return s, nil
		case "enter":
			if len(s.results) == 0 {
				return s, nil
			}
			return newDiscordGame(s.results[s.cursor], s)
		}
	}

	prev := s.input.Value()
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	if s.input.Value() != prev {
		s.refresh(a)
	}
	return s, cmd
}

func (s *discordSearchScreen) refresh(a *App) {
	s.cursor = 0
	s.results = nil
	if a.db != nil {
		s.results = a.db.Search(s.input.Value(), a.settings.MaxSearchResults)
	}
}

func (s *discordSearchScreen) view(a *App) string {
	var b strings.Builder
	b.WriteString(sTitle.Render("DATABASE SEARCH") + "\n\n")
	b.WriteString(s.input.View() + "\n\n")

	query := strings.TrimSpace(s.input.Value())
	switch {
	case a.dbLoading:
		b.WriteString(a.spinner.View() + " loading games database…\n")
	case a.db == nil:
		b.WriteString(sRed.Render("Games database could not be loaded. Use manual mode instead.") + "\n")
	case query == "":
		b.WriteString(sMuted.Render(fmt.Sprintf("Type to search %d games by name or abbreviation.", len(a.db.Games))) + "\n")
	case len(s.results) == 0:
		b.WriteString(sRed.Render(fmt.Sprintf("No games found for %q", query)) + "\n" +
			sYellow.Render("Try a different search term or abbreviation") + "\n")
	default:
		b.WriteString(s.resultsView(a))
	}

	b.WriteString("\n" + help("type", "search", "↑/↓", "move", "enter", "select", "esc", "back"))
	return b.String()
}

func (s *discordSearchScreen) resultsView(a *App) string {
	// Each result takes one line; keep the cursor visible in short terminals.
	visible := max(a.height-14, 3)
	start := 0
	if s.cursor >= visible {
		start = s.cursor - visible + 1
	}
	end := min(start+visible, len(s.results))
	width := a.contentWidth()

	var b strings.Builder
	b.WriteString(sMuted.Render(fmt.Sprintf("%d result(s)", len(s.results))) + "\n")
	for i := start; i < end; i++ {
		g := s.results[i]
		extra := ""
		if len(g.Aliases) > 0 {
			extra = "  " + aliasesText(g.Aliases)
		}
		if i == s.cursor {
			b.WriteString(sCursor.Render(truncate(fmt.Sprintf("▸ %2d. %s", i+1, g.Name), width/2)))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s", sAccent.Render(fmt.Sprintf("%2d.", i+1)), truncate(g.Name, width/2)))
		}
		b.WriteString(sMuted.Render(truncate(extra, width/2)) + "\n")
	}
	return b.String()
}

func aliasesText(aliases []string) string {
	if len(aliases) <= 3 {
		return strings.Join(aliases, ", ")
	}
	return strings.Join(aliases[:3], ", ") + fmt.Sprintf(" (+%d more)", len(aliases)-3)
}

// --- game details --------------------------------------------------------------

// steamExesMsg carries executable names looked up via SteamCMD for a game
// that has none in the Discord database.
type steamExesMsg struct {
	gameID string
	exes   []string
	err    error
}

type discordGameScreen struct {
	game   discord.Game
	exes   []string
	cursor int
	back   screen

	// fromSteam is set when exes came from SteamCMD instead of Discord.
	fromSteam bool
	lookingUp bool
	lookupErr error
}

// newDiscordGame opens the game card. When Discord lists no Windows executables
// but knows the Steam AppID, executable names are looked up in public SteamCMD
// metadata (no license needed); the fake is still launched as a normal process.
func newDiscordGame(g discord.Game, back screen) (screen, tea.Cmd) {
	s := &discordGameScreen{game: g, exes: discord.AllExecutables(g), back: back}
	appID, hasSteam := discord.SteamAppID(g)
	if len(s.exes) > 0 || !hasSteam {
		return s, nil
	}
	s.lookingUp = true
	return s, func() tea.Msg {
		exes, err := steam.FetchWindowsExecutables(context.Background(), appID)
		return steamExesMsg{gameID: g.ID, exes: exes, err: err}
	}
}

func (s *discordGameScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	if m, ok := msg.(steamExesMsg); ok {
		if m.gameID == s.game.ID {
			s.lookingUp = false
			s.lookupErr = m.err
			s.exes, s.fromSteam = m.exes, len(m.exes) > 0
		}
		return s, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "backspace":
		return s.back, nil
	case "up", "k":
		s.cursor = max(s.cursor-1, 0)
	case "down", "j", "tab":
		s.cursor = min(s.cursor+1, max(len(s.exes)-1, 0))
	case "m":
		exe := ""
		if len(s.exes) > 0 {
			exe = s.exes[s.cursor]
		}
		return newManual(a, s.game.Name, exe)
	case "enter":
		if s.lookingUp {
			return s, nil
		}
		if len(s.exes) == 0 {
			return newManual(a, s.game.Name, "")
		}
		f, err := a.mgr.LaunchDesktop(s.game.Name, s.exes[s.cursor])
		if err != nil {
			a.setStatus(statusErr, err.Error())
			return s, nil
		}
		return &menuScreen{}, a.launched(f)
	}
	return s, nil
}

func (s *discordGameScreen) view(a *App) string {
	var b strings.Builder
	b.WriteString(sTitle.Render("GAME") + "\n\n")
	b.WriteString(field("Name", sCyan.Bold(true).Render(s.game.Name)) + "\n")
	b.WriteString(field("ID", sMuted.Render(s.game.ID)) + "\n")
	if len(s.game.Aliases) > 0 {
		b.WriteString(field("Aliases", sMuted.Render(truncate(strings.Join(s.game.Aliases, ", "), a.contentWidth()-14))) + "\n")
	}
	if id, ok := discord.SteamAppID(s.game); ok {
		b.WriteString(field("Steam AppID", sMuted.Render(fmt.Sprint(id))) + "\n")
	}
	b.WriteString("\n")

	switch {
	case s.lookingUp:
		b.WriteString(sYellow.Render("Discord lists no executables for this game.") + "\n")
		b.WriteString(a.spinner.View() + " looking up the executable name in Steam metadata…\n\n")
		b.WriteString(help("esc", "back"))
		return b.String()
	case len(s.exes) == 0:
		b.WriteString(sYellow.Render("No executable name found for this game.") + "\n")
		switch _, hasSteam := discord.SteamAppID(s.game); {
		case s.lookupErr != nil:
			b.WriteString(sRed.Render("Steam lookup failed: "+s.lookupErr.Error()) + "\n")
		case hasSteam:
			b.WriteString(sMuted.Render("Discord has none, and the Steam launch options have no Windows .exe.") + "\n")
		default:
			b.WriteString(sMuted.Render("Discord has none, and the game is not linked to Steam.") + "\n")
		}
		b.WriteString(sMuted.Render("Try another entry of the same game (search by alias) or enter the name manually.") + "\n\n")
		b.WriteString(help("enter", "enter name manually", "esc", "back"))
		return b.String()
	}

	if s.fromSteam {
		b.WriteString(sYellow.Render("Discord lists no executables for this game — names below come from Steam launch options.") + "\n")
		b.WriteString(sMuted.Render("The fake still runs as a normal process; no Steam files are created.") + "\n\n")
	}
	b.WriteString(sSection.Render("Executable to fake") + sMuted.Render("  (first one is the most likely)") + "\n")
	visible := max(a.height-20, 3)
	start := max(0, s.cursor-visible+1)
	for i := start; i < min(start+visible, len(s.exes)); i++ {
		tag := ""
		if i == 0 {
			tag = sMuted.Render("  primary")
		}
		if i == s.cursor {
			b.WriteString(sCursor.Render("▸ "+s.exes[i]) + tag + "\n")
		} else {
			b.WriteString("  " + s.exes[i] + tag + "\n")
		}
	}
	if more := len(s.exes) - min(start+visible, len(s.exes)); more > 0 {
		b.WriteString(sMuted.Render(fmt.Sprintf("  … %d more", more)) + "\n")
	}

	b.WriteString("\n")
	if target, err := a.mgr.DesktopTarget(s.exes[s.cursor]); err == nil {
		b.WriteString(field("Will create", sMuted.Render(target)) + "\n")
	} else {
		b.WriteString(field("Will create", sRed.Render(err.Error())) + "\n")
	}
	b.WriteString("\n" + help("↑/↓", "choose exe", "enter", "create & launch", "m", "edit name", "esc", "back"))
	return b.String()
}

// --- manual mode -------------------------------------------------------------------

type manualScreen struct {
	game  string
	input textinput.Model
}

func newManual(a *App, game, exe string) (screen, tea.Cmd) {
	in := textinput.New()
	in.Placeholder = "TslGame.exe"
	in.Prompt = "Executable › "
	in.PromptStyle = sAccent
	in.CharLimit = 260
	in.SetValue(exe)
	in.CursorEnd()
	cmd := in.Focus()
	return &manualScreen{game: game, input: in}, cmd
}

func (s *manualScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return &menuScreen{cursor: 1}, nil
		case "enter":
			game := s.game
			if game == "" {
				game = strings.TrimSpace(s.input.Value())
			}
			f, err := a.mgr.LaunchDesktop(game, s.input.Value())
			if err != nil {
				a.setStatus(statusErr, err.Error())
				return s, nil
			}
			return &menuScreen{}, a.launched(f)
		}
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

func (s *manualScreen) view(a *App) string {
	var b strings.Builder
	b.WriteString(sTitle.Render("MANUAL MODE") + "\n\n")
	if s.game != "" {
		b.WriteString(field("Game", sCyan.Render(s.game)) + "\n\n")
	}
	b.WriteString(sCyan.Render("Enter the exact process name Discord expects.") + "\n")
	b.WriteString(sMuted.Render("Examples:  TslGame.exe (PUBG)  ·  League of Legends.exe (LoL)  ·  Overwatch.exe") + "\n")
	b.WriteString(sMuted.Render("Subfolders are allowed: bin/win64/Game.exe") + "\n\n")
	b.WriteString(s.input.View() + "\n\n")

	if strings.TrimSpace(s.input.Value()) != "" {
		if target, err := a.mgr.DesktopTarget(s.input.Value()); err == nil {
			b.WriteString(field("Will create", sMuted.Render(target)) + "\n")
		} else {
			b.WriteString(field("Will create", sRed.Render(err.Error())) + "\n")
		}
	}
	b.WriteString("\n" + help("enter", "create & launch", "esc", "back"))
	return b.String()
}
