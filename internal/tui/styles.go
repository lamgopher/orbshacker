package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colAccent = lipgloss.Color("#4ade80")
	colCyan   = lipgloss.Color("#22d3ee")
	colYellow = lipgloss.Color("#facc15")
	colRed    = lipgloss.Color("#f87171")
	colPurple = lipgloss.Color("#c084fc")
	colMuted  = lipgloss.Color("#7a7a7a")
	colSelBg  = lipgloss.Color("#1f3a24")

	sAccent  = lipgloss.NewStyle().Foreground(colAccent)
	sBold    = lipgloss.NewStyle().Bold(true)
	sCyan    = lipgloss.NewStyle().Foreground(colCyan)
	sYellow  = lipgloss.NewStyle().Foreground(colYellow)
	sRed     = lipgloss.NewStyle().Foreground(colRed)
	sPurple  = lipgloss.NewStyle().Foreground(colPurple)
	sMuted   = lipgloss.NewStyle().Foreground(colMuted)
	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent).Border(lipgloss.RoundedBorder()).BorderForeground(colAccent).Padding(0, 2)
	sLabel   = lipgloss.NewStyle().Foreground(colMuted).Width(14)
	sKey     = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	sCursor  = lipgloss.NewStyle().Bold(true).Foreground(colAccent).Background(colSelBg)
	sSection = lipgloss.NewStyle().Bold(true).Foreground(colCyan)
)

const banner = ` _____ _____ _____ _____    _____ _____ _____ _____ _____ _____
|     | __  | __  |   __|  |  |  |  _  |     |  |  |   __| __  |
|  |  |    -| __ -|__   |  |     |     |   --|    -|   __|    -|
|_____|__|__|_____|_____|  |__|__|__|__|_____|__|__|_____|__|__|`

// help renders a footer like "enter launch · esc back".
func help(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, sKey.Render(pairs[i])+" "+sMuted.Render(pairs[i+1]))
	}
	return strings.Join(parts, sMuted.Render("  ·  "))
}

// field renders "Label   value".
func field(label, value string) string {
	return sLabel.Render(label) + value
}

// truncate shortens s to max display cells.
func truncate(s string, max int) string {
	if max <= 1 || lipgloss.Width(s) <= max {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > max {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
