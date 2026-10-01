package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lamgopher/orbshacker/internal/config"
	"github.com/lamgopher/orbshacker/internal/discord"
	"github.com/lamgopher/orbshacker/internal/faker"
	"github.com/lamgopher/orbshacker/internal/steam"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	mgr, err := faker.NewManager("Win64")
	if err != nil {
		t.Fatal(err)
	}
	a := New(config.Default(), mgr)
	send(a, tea.WindowSizeMsg{Width: 120, Height: 40})
	send(a, dbLoadedMsg{db: &discord.DB{Source: "test", Games: []discord.Game{{
		ID:          "1",
		Name:        "PUBG: BATTLEGROUNDS",
		Aliases:     []string{"PUBG"},
		Executables: []discord.Executable{{OS: "win32", Name: "TslGame.exe"}, {OS: "win32", Name: "TslGame_BE.exe"}},
	}}}})
	return a
}

func send(a *App, msg tea.Msg) tea.Cmd {
	_, cmd := a.Update(msg)
	return cmd
}

func typeText(a *App, text string) {
	for _, r := range text {
		send(a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func key(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func TestDiscordSearchFlow(t *testing.T) {
	a := newTestApp(t)

	typeText(a, "1")
	if _, ok := a.screen.(*discordSearchScreen); !ok {
		t.Fatalf("expected search screen, got %T", a.screen)
	}
	typeText(a, "pubg")
	if v := a.View(); !strings.Contains(v, "PUBG: BATTLEGROUNDS") {
		t.Fatalf("search results missing:\n%s", v)
	}

	send(a, key(tea.KeyEnter))
	game, ok := a.screen.(*discordGameScreen)
	if !ok {
		t.Fatalf("expected game screen, got %T", a.screen)
	}
	v := a.View()
	for _, want := range []string{"TslGame.exe", "primary", "TslGame_BE.exe", "Will create"} {
		if !strings.Contains(v, want) {
			t.Errorf("game view missing %q:\n%s", want, v)
		}
	}

	send(a, key(tea.KeyEsc))
	if a.screen != game.back {
		t.Fatalf("esc should return to the same search screen")
	}
	send(a, key(tea.KeyEsc))
	if _, ok := a.screen.(*menuScreen); !ok {
		t.Fatalf("expected menu, got %T", a.screen)
	}
}

func TestManualModeShowsTargetAndRejectsTraversal(t *testing.T) {
	a := newTestApp(t)
	typeText(a, "2")
	typeText(a, "../evil")
	if v := a.View(); !strings.Contains(v, "invalid path") {
		t.Fatalf("expected validation error:\n%s", v)
	}
	send(a, key(tea.KeyEnter))
	if _, ok := a.screen.(*manualScreen); !ok || a.status == "" {
		t.Fatalf("invalid name must not launch; screen %T, status %q", a.screen, a.status)
	}
}

func TestQuitWithoutFakes(t *testing.T) {
	a := newTestApp(t)
	cmd := send(a, key(tea.KeyCtrlC))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg")
	}
}

func TestMenuExitItem(t *testing.T) {
	a := newTestApp(t)
	cmd := send(a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg")
	}
}

func TestGameWithoutExesUsesSteamNames(t *testing.T) {
	a := newTestApp(t)
	game := discord.Game{
		ID:             "2",
		Name:           "Seasons after Fall",
		ThirdPartySKUs: []discord.SKU{{Distributor: "steam", ID: "366320"}},
	}
	next, cmd := newDiscordGame(game, &menuScreen{})
	if cmd == nil {
		t.Fatal("expected a Steam lookup command")
	}
	a.screen = next
	if v := a.View(); !strings.Contains(v, "looking up") {
		t.Fatalf("expected lookup spinner:\n%s", v)
	}

	send(a, steamExesMsg{gameID: "2", exes: []string{"SeasonsAfterFall.exe"}})
	v := a.View()
	for _, want := range []string{"SeasonsAfterFall.exe", "Steam launch options", "Will create"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
	if _, ok := a.screen.(*discordGameScreen); !ok {
		t.Fatalf("must stay on the game card, got %T", a.screen)
	}
}

func TestGameWithoutExesOrSteam(t *testing.T) {
	a := newTestApp(t)
	next, cmd := newDiscordGame(discord.Game{ID: "3", Name: "Nothing"}, &menuScreen{})
	if cmd != nil {
		t.Fatal("no lookup expected without a Steam AppID")
	}
	a.screen = next
	send(a, key(tea.KeyEnter))
	if _, ok := a.screen.(*manualScreen); !ok {
		t.Fatalf("expected manual mode, got %T", a.screen)
	}
}

func TestSteamExistingExeAsksWhatToDo(t *testing.T) {
	a := newTestApp(t)
	steamPath := t.TempDir()
	info := steam.AppInfo{AppID: 42, Name: "Demo", InstallDir: "Demo", Executable: "Demo.exe"}
	exe := steam.FakeExePath(steamPath, info)
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(exe, []byte("real game"), 0o644)

	s, _ := newSteam(a)
	st := s.(*steamScreen)
	st.steamPath = steamPath
	st.openForm(info)
	a.screen = st

	send(a, key(tea.KeyEnter))
	if st.step != stepExists {
		t.Fatalf("expected confirm step, got %v (status %q)", st.step, a.status)
	}
	if st.existCursor != existingLaunch {
		t.Fatalf("default choice = %d, want launch existing", st.existCursor)
	}
	if v := a.View(); !strings.Contains(v, "Launch the existing file") || !strings.Contains(v, "Overwrite") {
		t.Fatalf("choices missing:\n%s", v)
	}

	send(a, key(tea.KeyEsc))
	if st.step != stepForm {
		t.Fatalf("esc should return to the form, got %v", st.step)
	}
	if data, _ := os.ReadFile(exe); string(data) != "real game" {
		t.Fatalf("existing file was modified: %q", data)
	}
}
