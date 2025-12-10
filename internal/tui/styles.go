package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	green  = lipgloss.Color("#00FF00")
	yellow = lipgloss.Color("#FFFF00")
	red    = lipgloss.Color("#FF0000")
	cyan   = lipgloss.Color("#00FFFF")
	gray   = lipgloss.Color("#808080")

	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan).
			MarginBottom(1)

	// Task status styles
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

	// Summary styles
	summaryStyle = lipgloss.NewStyle().
			MarginTop(1).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(gray).
			MarginTop(1)

	// Output section styles
	outputHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(cyan).
				MarginTop(1)

	outputTaskStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(yellow)

	outputContentStyle = lipgloss.NewStyle().
				Foreground(gray).
				PaddingLeft(2)
)
