// Package tui implements the interactive terminal UI.
package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"orbshacker/internal/config"
	"orbshacker/internal/discord"
	"orbshacker/internal/faker"
)

// screen is one page of the application.
type screen interface {
	update(a *App, msg tea.Msg) (screen, tea.Cmd)
	view(a *App) string
}

type statusKind int

const (
	statusInfo statusKind = iota
	statusOK
	statusWarn
	statusErr
)

type (
	dbLoadedMsg struct {
		db  *discord.DB
		err error
	}
	fakeExitedMsg  struct{ id int }
	cleanupDoneMsg struct {
		id  int
		err error
	}
	clockMsg time.Time
)

// App is the root Bubble Tea model.
type App struct {
	settings config.Settings
	mgr      *faker.Manager

	db        *discord.DB
	dbErr     error
	dbLoading bool

	screen  screen
	spinner spinner.Model
	width   int
	height  int

	status     string
	statusKind statusKind

	// quitting is set when the user chose to stop and clean everything before exit.
	quitting bool
}

// New creates the application model.
func New(settings config.Settings, mgr *faker.Manager) *App {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(sAccent))
	return &App{
		settings:  settings,
		mgr:       mgr,
		dbLoading: true,
		screen:    &menuScreen{},
		spinner:   sp,
	}
}

// Run starts the UI and blocks until it exits.
func Run(settings config.Settings, mgr *faker.Manager) error {
	_, err := tea.NewProgram(New(settings, mgr), tea.WithAltScreen()).Run()
	return err
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(loadDB(), a.spinner.Tick, clock(), tea.SetWindowTitle("orbshacker"))
}

func loadDB() tea.Cmd {
	return func() tea.Msg {
		db, err := discord.Load(context.Background())
		return dbLoadedMsg{db: db, err: err}
	}
}

func clock() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockMsg(t) })
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			var cmd tea.Cmd
			a.screen, cmd = a.requestQuit(a.screen)
			return a, cmd
		}
		// Any key press clears a stale status line.
		a.status = ""
	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd
	case clockMsg:
		return a, clock()
	case dbLoadedMsg:
		a.dbLoading = false
		a.db, a.dbErr = msg.db, msg.err
		if msg.err == nil {
			a.setStatus(statusOK, fmt.Sprintf("Loaded %d games from %s", len(msg.db.Games), msg.db.Source))
		} else {
			a.setStatus(statusErr, msg.err.Error())
		}
	case fakeExitedMsg:
		return a, a.onFakeExited(msg.id)
	case cleanupDoneMsg:
		return a, a.onCleanupDone(msg)
	}

	next, cmd := a.screen.update(a, msg)
	a.screen = next
	return a, cmd
}

func (a *App) View() string {
	if a.width == 0 {
		return ""
	}
	header := a.header()
	footer := a.statusLine()
	bodyHeight := a.height - lipgloss.Height(header) - lipgloss.Height(footer)
	body := lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).Padding(0, 2).Render(a.screen.view(a))
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (a *App) header() string {
	left := sAccent.Bold(true).Render("orbshacker") + sMuted.Render(" v"+config.Version)

	var db string
	switch {
	case a.dbLoading:
		db = a.spinner.View() + sMuted.Render(" loading games database…")
	case a.dbErr != nil:
		db = sRed.Render("● database unavailable")
	default:
		db = sAccent.Render("● ") + sMuted.Render(fmt.Sprintf("%s · %d games", a.db.Source, len(a.db.Games)))
	}

	fakes := sMuted.Render("no fakes running")
	if n := a.mgr.RunningCount(); n > 0 {
		fakes = sAccent.Render(fmt.Sprintf("▶ %d fake(s) running", n))
	}

	right := db + sMuted.Render("   ") + fakes
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}
	line := lipgloss.NewStyle().Padding(0, 2).Render(left + strings.Repeat(" ", gap) + right)
	rule := sMuted.Render(strings.Repeat("─", max(a.width, 0)))
	return line + "\n" + rule + "\n"
}

func (a *App) statusLine() string {
	if a.status == "" {
		return ""
	}
	style := sCyan
	prefix := "• "
	switch a.statusKind {
	case statusOK:
		style, prefix = sAccent, "✔ "
	case statusWarn:
		style, prefix = sYellow, "! "
	case statusErr:
		style, prefix = sRed, "✖ "
	}
	return lipgloss.NewStyle().Padding(0, 2).Render(style.Render(truncate(prefix+a.status, a.width-4)))
}

func (a *App) setStatus(kind statusKind, text string) {
	a.statusKind, a.status = kind, text
}

// contentWidth is the usable width inside the body padding.
func (a *App) contentWidth() int {
	return max(a.width-4, 20)
}

// --- fake lifecycle --------------------------------------------------------

func waitFake(f *faker.Fake) tea.Cmd {
	return func() tea.Msg {
		_ = f.Wait()
		return fakeExitedMsg{id: f.ID}
	}
}

func cleanupFake(f *faker.Fake) tea.Cmd {
	return func() tea.Msg {
		return cleanupDoneMsg{id: f.ID, err: f.RemoveFiles()}
	}
}

// launched reports a successful launch and starts watching the process.
func (a *App) launched(f *faker.Fake) tea.Cmd {
	a.setStatus(statusOK, fmt.Sprintf("Launched %s (PID %d) — keep Discord running; it should detect the game in a few seconds",
		filepath.Base(f.ExePath), f.PID))
	return waitFake(f)
}

func (a *App) onFakeExited(id int) tea.Cmd {
	f := a.mgr.Get(id)
	if f == nil {
		return nil
	}
	f.MarkExited()
	if f.CleanupPending {
		return cleanupFake(f)
	}
	return nil
}

func (a *App) onCleanupDone(msg cleanupDoneMsg) tea.Cmd {
	f := a.mgr.Get(msg.id)
	if f == nil {
		return nil
	}
	if msg.err != nil {
		f.CleanupPending = false
		a.setStatus(statusErr, fmt.Sprintf("Cleanup of %s failed: %v", filepath.Base(f.ExePath), msg.err))
	} else {
		a.mgr.Forget(msg.id)
		a.setStatus(statusOK, fmt.Sprintf("Stopped and removed %s", filepath.Base(f.ExePath)))
	}
	if a.quitting && !a.cleanupInProgress() {
		return tea.Quit
	}
	return nil
}

// stopAndClean kills the fake (if running) and deletes its files once it has exited.
func (a *App) stopAndClean(f *faker.Fake) tea.Cmd {
	if f.CleanupPending {
		return nil
	}
	f.CleanupPending = true
	if f.Exited {
		return cleanupFake(f)
	}
	if err := f.Stop(); err != nil {
		f.CleanupPending = false
		a.setStatus(statusErr, fmt.Sprintf("Failed to stop %s: %v", filepath.Base(f.ExePath), err))
	}
	return nil
}

func (a *App) cleanupInProgress() bool {
	for _, f := range a.mgr.Fakes() {
		if f.CleanupPending {
			return true
		}
	}
	return false
}

// requestQuit exits immediately when nothing was launched, otherwise asks what to do.
func (a *App) requestQuit(current screen) (screen, tea.Cmd) {
	if len(a.mgr.Fakes()) == 0 {
		return current, tea.Quit
	}
	if _, already := current.(*quitScreen); already {
		return current, nil
	}
	return &quitScreen{back: current}, nil
}

func (a *App) quitWithCleanup() tea.Cmd {
	a.quitting = true
	var cmds []tea.Cmd
	for _, f := range a.mgr.Fakes() {
		cmds = append(cmds, a.stopAndClean(f))
	}
	if !a.cleanupInProgress() {
		return tea.Quit
	}
	return tea.Batch(cmds...)
}
