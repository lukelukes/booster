package tui

import "github.com/charmbracelet/lipgloss"

var (
	green  = lipgloss.Color("#00FF00")
	yellow = lipgloss.Color("#FFFF00")
	red    = lipgloss.Color("#FF0000")
	cyan   = lipgloss.Color("#00FFFF")
	gray   = lipgloss.Color("#808080")

	FocusedBorderColor   = lipgloss.Color("14")
	UnfocusedBorderColor = lipgloss.Color("8")

	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("237")).
				Bold(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan).
			MarginBottom(1)

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

	leaderStyle = lipgloss.NewStyle().
			Foreground(gray).
			Faint(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(gray).
			MarginTop(1)

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

	logHeaderStyle = lipgloss.NewStyle().
			Foreground(gray).
			MarginTop(1)

	logLineStyle = lipgloss.NewStyle().
			Foreground(gray).
			PaddingLeft(2)

	progressFilledStyle = lipgloss.NewStyle().
				Foreground(cyan)

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(gray).
				Faint(true)

	progressTextStyle = lipgloss.NewStyle().
				Foreground(cyan)

	failureBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(red).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	failureHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(red).
				Background(lipgloss.Color("#330000"))

	failureTaskStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(red)

	failureErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6666")).
				PaddingLeft(2)

	failureOutputHeaderStyle = lipgloss.NewStyle().
					Foreground(gray).
					Italic(true)

	failureOutputStyle = lipgloss.NewStyle().
				Foreground(gray).
				PaddingLeft(2).
				Faint(true)

	summaryBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			MarginBottom(1)

	summarySuccessStyle = summaryBoxStyle.
				BorderForeground(green).
				Foreground(green).
				Bold(true)

	summaryFailureStyle = summaryBoxStyle.
				BorderForeground(red).
				Foreground(red).
				Bold(true)

	summaryStatStyle = lipgloss.NewStyle().
				Foreground(gray)

	summaryBarStyle = lipgloss.NewStyle().
			Foreground(cyan)

	summaryBarEmptyStyle = lipgloss.NewStyle().
				Foreground(gray).
				Faint(true)

	appContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)
)
