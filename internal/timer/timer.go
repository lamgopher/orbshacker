// Package timer is the UI of a fake game process: a countdown in its own console.
package timer

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"orbshacker/internal/config"
)

var (
	accent    = lipgloss.Color("#4ade80")
	muted     = lipgloss.Color("#666666")
	doneColor = lipgloss.Color("#ff6b6b")
)

type tickMsg time.Time

type model struct {
	title    string
	exe      string
	started  time.Time
	deadline time.Time
	now      time.Time
	width    int
	height   int
}

// Run shows the countdown until the user closes it. Without a console it just
// keeps the process alive until it is killed.
func Run(title string) error {
	in, out := os.Stdin, os.Stdout
	if !term.IsTerminal(out.Fd()) {
		cin, cout, err := openConsole()
		if err != nil {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt)
			<-sig
			return nil
		}
		// The handles are deliberately not closed: Bubble Tea's input goroutine
		// may still be blocked reading CONIN$, and closing it would wait for that
		// read forever. The process exits right after Run returns anyway.
		in, out = cin, cout
	}
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(out))

	exe, _ := os.Executable()
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	}
	fmt.Fprintf(out, "\x1b]0;%s — %d min\x07", title, config.TimerMinutes)

	now := time.Now()
	m := model{
		title:    title,
		exe:      filepath.Base(exe),
		started:  now,
		deadline: now.Add(config.TimerMinutes * time.Minute),
		now:      now,
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
	case tickMsg:
		m.now = time.Time(msg)
		return m, tick()
	}
	return m, nil
}

func (m model) View() string {
	total := config.TimerMinutes * time.Minute
	remaining := max(m.deadline.Sub(m.now).Round(time.Second), 0)
	done := remaining == 0
	clock := formatDuration(remaining)

	color, status := accent, "Running — keep this window open"
	if done {
		color, status = doneColor, "Complete — the quest should be done"
	}
	label := lipgloss.NewStyle().Foreground(muted).Width(11)
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#e0e0e0"))
	row := func(k, v string) string { return label.Render(k) + value.Render(v) }

	const barWidth = 34
	filled := min(int(float64(total-remaining)/float64(total)*barWidth), barWidth)
	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(muted).Render(strings.Repeat("░", barWidth-filled))

	info := lipgloss.JoinVertical(lipgloss.Left,
		row("Remaining", lipgloss.NewStyle().Bold(true).Foreground(color).Render(clock)),
		row("Elapsed", formatDuration(min(m.now.Sub(m.started).Round(time.Second), total))),
		row("Ends at", m.deadline.Format("15:04:05")),
		row("Process", m.exe),
	)

	parts := []string{
		lipgloss.NewStyle().Bold(true).Foreground(accent).Render(m.title),
		lipgloss.NewStyle().Foreground(muted).Render("fake game process · orbshacker"),
		"",
	}
	// Big digits need about 12 rows; small windows get the plain text only.
	if m.height == 0 || m.height >= 18 {
		parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(color).Render(bigDigits(clock)), "")
	}
	parts = append(parts,
		bar,
		"",
		info,
		"",
		lipgloss.NewStyle().Foreground(color).Render(status),
		lipgloss.NewStyle().Foreground(muted).Render("q — close (stops the fake game)"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Center, parts...))
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

// glyphs are 3x5 block digits.
var glyphs = map[rune][5]string{
	'0': {"███", "█ █", "█ █", "█ █", "███"},
	'1': {" ██", "  █", "  █", "  █", "  █"},
	'2': {"███", "  █", "███", "█  ", "███"},
	'3': {"███", "  █", "███", "  █", "███"},
	'4': {"█ █", "█ █", "███", "  █", "  █"},
	'5': {"███", "█  ", "███", "  █", "███"},
	'6': {"███", "█  ", "███", "█ █", "███"},
	'7': {"███", "  █", "  █", "  █", "  █"},
	'8': {"███", "█ █", "███", "█ █", "███"},
	'9': {"███", "█ █", "███", "  █", "███"},
	':': {" ", "█", " ", "█", " "},
}

func bigDigits(s string) string {
	var rows [5]strings.Builder
	for i, r := range s {
		g := glyphs[r]
		for row := range rows {
			if i > 0 {
				rows[row].WriteString(" ")
			}
			rows[row].WriteString(g[row])
		}
	}
	lines := make([]string, len(rows))
	for i := range rows {
		lines[i] = rows[i].String()
	}
	return strings.Join(lines, "\n")
}
