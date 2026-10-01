package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"orbshacker/internal/faker"
)

type procsScreen struct {
	cursor int
}

func (s *procsScreen) update(a *App, msg tea.Msg) (screen, tea.Cmd) {
	fakes := a.mgr.Fakes()
	s.cursor = min(s.cursor, max(len(fakes)-1, 0))

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q", "backspace":
		return &menuScreen{cursor: 3}, nil
	case "up", "k":
		s.cursor = max(s.cursor-1, 0)
	case "down", "j", "tab":
		s.cursor = min(s.cursor+1, max(len(fakes)-1, 0))
	case "x":
		if len(fakes) > 0 {
			f := fakes[s.cursor]
			if err := f.Stop(); err != nil {
				a.setStatus(statusErr, fmt.Sprintf("Failed to stop %s: %v", filepath.Base(f.ExePath), err))
			}
		}
	case "d", "delete":
		if len(fakes) > 0 {
			return s, a.stopAndClean(fakes[s.cursor])
		}
	case "D":
		var cmds []tea.Cmd
		for _, f := range fakes {
			cmds = append(cmds, a.stopAndClean(f))
		}
		return s, tea.Batch(cmds...)
	}
	return s, nil
}

func (s *procsScreen) view(a *App) string {
	var b strings.Builder
	b.WriteString(sTitle.Render("RUNNING FAKES") + "\n\n")

	fakes := a.mgr.Fakes()
	if len(fakes) == 0 {
		b.WriteString(sMuted.Render("Nothing launched yet. Pick a game from the menu — each one runs in its own window.") + "\n\n")
		b.WriteString(help("esc", "back"))
		return b.String()
	}

	width := a.contentWidth()
	nameW := max((width-40)/2, 12)
	exeW := max(width-40-nameW, 12)
	row := func(cells ...string) string {
		widths := []int{3, nameW, exeW, 8, 12, 9}
		var parts []string
		for i, c := range cells {
			parts = append(parts, lipgloss.NewStyle().Width(widths[i]).MaxWidth(widths[i]).Render(c))
		}
		return strings.Join(parts, " ")
	}

	b.WriteString(sMuted.Bold(true).Render(row("#", "GAME", "EXECUTABLE", "PID", "STATUS", "UPTIME")) + "\n")
	for i, f := range fakes {
		status, style := fakeStatus(f)
		cells := []string{
			fmt.Sprint(f.ID),
			truncate(f.Game, nameW),
			truncate(filepath.Base(f.ExePath), exeW),
			fmt.Sprint(f.PID),
			status,
			uptime(f),
		}
		line := row(cells...)
		if i == s.cursor {
			b.WriteString(sCursor.Render(line) + "\n")
		} else {
			b.WriteString(style.Render(line) + "\n")
		}
	}

	f := fakes[s.cursor]
	b.WriteString("\n" + field("Exe", sMuted.Render(truncate(f.ExePath, width-14))) + "\n")
	if f.ManifestPath != "" {
		b.WriteString(field("AppManifest", sMuted.Render(truncate(f.ManifestPath, width-14))) + "\n")
	}

	b.WriteString("\n" + help("↑/↓", "move", "x", "stop", "d", "stop & delete files", "D", "stop & delete all", "esc", "back"))
	return b.String()
}

func fakeStatus(f *faker.Fake) (string, lipgloss.Style) {
	switch {
	case f.CleanupPending:
		return "cleaning…", sYellow
	case f.Exited:
		return "exited", sMuted
	default:
		return "● running", sAccent
	}
}

func uptime(f *faker.Fake) string {
	end := time.Now()
	if f.Exited {
		end = f.ExitedAt
	}
	d := end.Sub(f.Started).Round(time.Second)
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}
