package tui

import "github.com/charmbracelet/lipgloss"

var (
	green  = lipgloss.Color("#00FF00")
	gray   = lipgloss.Color("#808080")
	red    = lipgloss.Color("#FF0000")
	yellow = lipgloss.Color("#FFFF00")
	cyan   = lipgloss.Color("#00FFFF")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan)

	doneStyle = lipgloss.NewStyle().
			Foreground(green)

	skippedStyle = lipgloss.NewStyle().
			Foreground(gray)

	runningStyle = lipgloss.NewStyle().
			Foreground(yellow)

	pendingStyle = lipgloss.NewStyle().
			Foreground(gray).
			Faint(true)

	failedStyle = lipgloss.NewStyle().
			Foreground(red)

	dimStyle = lipgloss.NewStyle().
			Foreground(gray).
			Faint(true)

	leaderStyle = lipgloss.NewStyle().
			Foreground(gray).
			Faint(true)

	logHeaderStyle = lipgloss.NewStyle().
			Foreground(gray)
)
