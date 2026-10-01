package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lamgopher/orbshacker/internal/faker"
	"github.com/lamgopher/orbshacker/internal/steam"
)

type steamStep int

const (
	stepPath steamStep = iota
	stepQuery
	stepResults
	stepForm
)

const (
	fieldName = iota
	fieldInstallDir
	fieldExe
)

type (
	steamSearchMsg struct {
		seq   int
		items []steam.StoreItem
		err   error
	}
	steamInfoMsg struct {
		seq  int
		item steam.StoreItem
		info steam.AppInfo
		err  error
	}
)

type steamScreen struct {
	step      steamStep
	steamPath string

	pathInput  textinput.Model
	queryInput textinput.Model

	results []steam.StoreItem
	cursor  int

	appID  int
	fields [3]textinput.Model
	focus  int
	depot  string

	// busy is the text shown next to the spinner while a request runs.
	busy string
	// seq invalidates responses of requests the user has abandoned.
	seq int
}

func newSteam(a *App) (screen, tea.Cmd) {
	s := &steamScreen{steamPath: steam.FindPath()}
	s.pathInput = newInput("Steam path › ", `C:\Program Files (x86)\Steam`)
	s.queryInput = newInput("Search › ", "Marathon, Toxic Commando Demo…")
	labels := [3]string{"Name        ", "Install dir ", "Executable  "}
	for i := range s.fields {
		s.fields[i] = newInput(labels[i]+"› ", "")
	}
	s.fields[fieldExe].Placeholder = "Bin/Game.exe"

	if s.steamPath == "" {
		s.step = stepPath
		return s, s.pathInput.Focus()
	}
	s.step = stepQuery
	return s, s.queryInput.Focus()
}

func newInput(prompt, placeholder string) textinput.Model {
	in := textinput.New()
	in.Prompt = prompt
	in.PromptStyle = sAccent
	in.Placeholder = placeholder
	in.CharLimit = 260
	return in
}

func (s *steamScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case steamSearchMsg:
		if msg.seq != s.seq {
			return s, nil
		}
		s.busy = ""
		if msg.err != nil {
			a.setStatus(statusErr, "Steam search error: "+msg.err.Error())
			return s, nil
		}
		if len(msg.items) == 0 {
			a.setStatus(statusWarn, fmt.Sprintf("No results for %q — try a different search term", s.queryInput.Value()))
			return s, nil
		}
		s.results, s.cursor = msg.items, 0
		s.step = stepResults
		s.queryInput.Blur()
		return s, nil

	case steamInfoMsg:
		if msg.seq != s.seq {
			return s, nil
		}
		s.busy = ""
		info := msg.info
		if msg.err != nil {
			a.setStatus(statusWarn, "Could not fetch app info automatically — fill in the details: "+msg.err.Error())
			info = steam.AppInfo{AppID: msg.item.ID, Name: msg.item.Name, InstallDir: msg.item.Name}
		}
		return s, s.openForm(info)

	case tea.KeyMsg:
		if msg.String() == "esc" {
			return s.back()
		}
		if s.busy != "" {
			return s, nil
		}
		switch s.step {
		case stepPath:
			if msg.String() == "enter" {
				return s, s.submitPath(a)
			}
		case stepQuery:
			if msg.String() == "enter" {
				return s, s.search()
			}
		case stepResults:
			return s, s.updateResults(msg)
		case stepForm:
			if next, cmd, handled := s.updateForm(a, msg); handled {
				return next, cmd
			}
		}
	}

	return s, s.updateFocusedInput(msg)
}

// back steps one page back; leaving the first page returns to the menu.
func (s *steamScreen) back() (screen, tea.Cmd) {
	if s.busy != "" {
		s.busy = ""
		s.seq++
		return s, nil
	}
	switch s.step {
	case stepForm:
		s.step = stepResults
		s.fields[s.focus].Blur()
		return s, nil
	case stepResults:
		s.step = stepQuery
		return s, s.queryInput.Focus()
	}
	return &menuScreen{cursor: 2}, nil
}

func (s *steamScreen) submitPath(a *App) tea.Cmd {
	p := strings.TrimSpace(s.pathInput.Value())
	if st, err := os.Stat(p); p == "" || err != nil || !st.IsDir() {
		a.setStatus(statusErr, "Folder not found: "+p)
		return nil
	}
	s.steamPath = p
	s.pathInput.Blur()
	s.step = stepQuery
	return s.queryInput.Focus()
}

func (s *steamScreen) search() tea.Cmd {
	query := strings.TrimSpace(s.queryInput.Value())
	if query == "" {
		return nil
	}
	s.seq++
	seq := s.seq
	s.busy = fmt.Sprintf("Searching Steam for %q…", query)
	return func() tea.Msg {
		items, err := steam.SearchStore(context.Background(), query)
		return steamSearchMsg{seq: seq, items: items, err: err}
	}
}

func (s *steamScreen) updateResults(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "up", "k":
		s.cursor = max(s.cursor-1, 0)
	case "down", "j", "tab":
		s.cursor = min(s.cursor+1, len(s.results)-1)
	case "enter":
		return s.fetchInfo(s.results[s.cursor])
	}
	return nil
}

func (s *steamScreen) fetchInfo(item steam.StoreItem) tea.Cmd {
	s.seq++
	seq := s.seq
	s.busy = fmt.Sprintf("Fetching Steam app info for %d…", item.ID)
	return func() tea.Msg {
		info, err := steam.FetchAppInfo(context.Background(), item.ID)
		return steamInfoMsg{seq: seq, item: item, info: info, err: err}
	}
}

func (s *steamScreen) openForm(info steam.AppInfo) tea.Cmd {
	s.step = stepForm
	s.appID, s.depot = info.AppID, info.DepotID
	s.fields[fieldName].SetValue(info.Name)
	s.fields[fieldInstallDir].SetValue(info.InstallDir)
	s.fields[fieldExe].SetValue(info.Executable)
	for i := range s.fields {
		s.fields[i].CursorEnd()
		s.fields[i].Blur()
	}
	s.focus = fieldExe
	return s.fields[s.focus].Focus()
}

// updateForm handles navigation keys of the form. handled is false for keys
// that must go to the focused input.
func (s *steamScreen) updateForm(a *App, key tea.KeyMsg) (next screen, cmd tea.Cmd, handled bool) {
	move := 0
	switch key.String() {
	case "tab", "down":
		move = 1
	case "shift+tab", "up":
		move = -1
	case "enter":
		if cmd, ok := s.launch(a); ok {
			return &menuScreen{}, cmd, true
		}
		return s, nil, true
	default:
		return s, nil, false
	}
	s.fields[s.focus].Blur()
	s.focus = (s.focus + move + len(s.fields)) % len(s.fields)
	return s, s.fields[s.focus].Focus(), true
}

func (s *steamScreen) formInfo() (steam.AppInfo, error) {
	info := steam.AppInfo{
		AppID:      s.appID,
		Name:       strings.TrimSpace(s.fields[fieldName].Value()),
		InstallDir: strings.Trim(strings.ReplaceAll(strings.TrimSpace(s.fields[fieldInstallDir].Value()), `\`, "/"), "/"),
		DepotID:    s.depot,
	}
	if info.Name == "" {
		return info, errors.New("game name is empty")
	}
	if err := faker.ValidateRelPath(info.InstallDir); err != nil {
		return info, fmt.Errorf("install dir: %w", err)
	}
	exe, err := faker.NormalizeExeName(s.fields[fieldExe].Value())
	if err != nil {
		return info, err
	}
	info.Executable = exe
	return info, nil
}

// launch writes the manifest and starts the fake exe. ok is false on failure.
func (s *steamScreen) launch(a *App) (cmd tea.Cmd, ok bool) {
	info, err := s.formInfo()
	if err != nil {
		a.setStatus(statusErr, err.Error())
		return nil, false
	}
	exePath := steam.FakeExePath(s.steamPath, info)
	if _, err := os.Stat(exePath); err == nil {
		a.setStatus(statusErr, "Refusing to overwrite existing file (is the game installed?): "+exePath)
		return nil, false
	}
	manifest, err := steam.WriteManifest(info, s.steamPath)
	if err != nil {
		a.setStatus(statusErr, "Failed to write appmanifest: "+err.Error())
		return nil, false
	}
	f, err := a.mgr.LaunchAt(info.Name, exePath, manifest)
	if err != nil {
		os.Remove(manifest)
		a.setStatus(statusErr, err.Error())
		return nil, false
	}
	return a.launched(f), true
}

func (s *steamScreen) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch s.step {
	case stepPath:
		s.pathInput, cmd = s.pathInput.Update(msg)
	case stepQuery:
		s.queryInput, cmd = s.queryInput.Update(msg)
	case stepForm:
		s.fields[s.focus], cmd = s.fields[s.focus].Update(msg)
	}
	return cmd
}

func (s *steamScreen) view(a *App) string {
	var b strings.Builder
	b.WriteString(sTitle.Render("STEAM QUEST MODE") + "\n\n")
	b.WriteString(sMuted.Render("Fake Steam appmanifest + exe for games where Discord checks that Steam is downloading them.") + "\n")
	b.WriteString(sYellow.Render("Demos and full games have different AppIDs — search \"Toxic Commando Demo\" for a demo quest.") + "\n\n")

	if s.step != stepPath {
		b.WriteString(field("Steam", sAccent.Render(s.steamPath)) + "\n\n")
	}

	switch s.step {
	case stepPath:
		b.WriteString(sYellow.Render("Could not locate Steam automatically. Enter its folder:") + "\n\n")
		b.WriteString(s.pathInput.View() + "\n")
	case stepQuery:
		b.WriteString(s.queryInput.View() + "\n")
	case stepResults:
		b.WriteString(s.resultsView(a))
	case stepForm:
		b.WriteString(s.formView(a))
	}

	if s.busy != "" {
		b.WriteString("\n" + a.spinner.View() + " " + s.busy + "\n")
	}

	b.WriteString("\n")
	switch s.step {
	case stepPath, stepQuery:
		b.WriteString(help("enter", "continue", "esc", "back"))
	case stepResults:
		b.WriteString(help("↑/↓", "move", "enter", "select", "esc", "back"))
	case stepForm:
		b.WriteString(help("tab/↑/↓", "field", "enter", "create & launch", "esc", "back"))
	}
	return b.String()
}

func (s *steamScreen) resultsView(a *App) string {
	var b strings.Builder
	b.WriteString(sMuted.Render(fmt.Sprintf("%d result(s) for %q", len(s.results), s.queryInput.Value())) + "\n")
	visible := max(a.height-18, 3)
	start := max(0, s.cursor-visible+1)
	for i := start; i < min(start+visible, len(s.results)); i++ {
		r := s.results[i]
		id := sMuted.Render(fmt.Sprintf("  AppID %d", r.ID))
		if i == s.cursor {
			b.WriteString(sCursor.Render(fmt.Sprintf("▸ %2d. %s", i+1, r.Name)) + id + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s %s%s\n", sAccent.Render(fmt.Sprintf("%2d.", i+1)), r.Name, id))
		}
	}
	return b.String()
}

func (s *steamScreen) formView(a *App) string {
	var b strings.Builder
	b.WriteString(field("AppID", sCyan.Render(fmt.Sprint(s.appID))))
	if s.depot != "" {
		b.WriteString(sMuted.Render("   depot " + s.depot))
	}
	b.WriteString("\n\n")
	for i := range s.fields {
		b.WriteString(s.fields[i].View() + "\n")
	}
	b.WriteString("\n")

	info, err := s.formInfo()
	if err != nil {
		b.WriteString(sRed.Render(err.Error()) + "\n")
		return b.String()
	}
	width := a.contentWidth() - 14
	b.WriteString(field("AppManifest", sMuted.Render(truncate(steam.ManifestPath(s.steamPath, info.AppID), width))) + "\n")
	b.WriteString(field("Fake exe", sMuted.Render(truncate(steam.FakeExePath(s.steamPath, info), width))) + "\n")
	return b.String()
}
