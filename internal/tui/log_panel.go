package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogPanelModel manages the log display panel.
// Follows the model tree pattern as a child of the main Model.
type LogPanelModel struct {
	viewport viewport.Model
	logs     []string
	width    int
	height   int
}

// NewLogPanel creates a new log panel model.
func NewLogPanel() LogPanelModel {
	return LogPanelModel{}
}

// Update handles messages for the log panel.
func (l LogPanelModel) Update(msg tea.Msg) (LogPanelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			l.viewport.ScrollDown(1)
		case "k", "up":
			l.viewport.ScrollUp(1)
		case "G":
			l.viewport.GotoBottom()
		}
	}
	return l, nil
}

// SetSize updates the panel dimensions.
func (l *LogPanelModel) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.viewport = viewport.New(width, height)
	// Re-apply content if we have any
	if len(l.logs) > 0 {
		l.viewport.SetContent(strings.Join(l.logs, "\n"))
	}
}

// SetLogs updates the log content, preserving scroll position if at bottom.
func (l *LogPanelModel) SetLogs(logs []string) {
	wasAtBottom := l.viewport.AtBottom()
	l.logs = logs
	l.viewport.SetContent(strings.Join(logs, "\n"))
	if wasAtBottom {
		l.viewport.GotoBottom()
	}
}

// AppendLog adds a new log line, auto-scrolling if at bottom.
func (l *LogPanelModel) AppendLog(line string) {
	wasAtBottom := l.viewport.AtBottom()
	l.logs = append(l.logs, line)
	l.viewport.SetContent(strings.Join(l.logs, "\n"))
	if wasAtBottom {
		l.viewport.GotoBottom()
	}
}

// Clear removes all logs.
func (l *LogPanelModel) Clear() {
	l.logs = nil
	l.viewport.SetContent("")
}

// Logs returns the current log lines.
func (l LogPanelModel) Logs() []string {
	return l.logs
}

// View renders the log panel content.
func (l LogPanelModel) View() string {
	if len(l.logs) == 0 {
		return l.renderEmpty()
	}
	return l.viewport.View()
}

// renderEmpty renders a placeholder when no logs are available.
func (l LogPanelModel) renderEmpty() string {
	return lipgloss.NewStyle().Faint(true).Render("Waiting for output...")
}

// ScrollUp scrolls the viewport up by n lines.
func (l *LogPanelModel) ScrollUp(n int) {
	l.viewport.ScrollUp(n)
}

// ScrollDown scrolls the viewport down by n lines.
func (l *LogPanelModel) ScrollDown(n int) {
	l.viewport.ScrollDown(n)
}

// AtBottom returns true if the viewport is scrolled to the bottom.
func (l LogPanelModel) AtBottom() bool {
	return l.viewport.AtBottom()
}

// TotalLineCount returns the total number of lines in the viewport.
func (l LogPanelModel) TotalLineCount() int {
	return l.viewport.TotalLineCount()
}

// ScrollPercent returns the current scroll position as a percentage.
func (l LogPanelModel) ScrollPercent() float64 {
	return l.viewport.ScrollPercent()
}

// Height returns the viewport height.
func (l LogPanelModel) Height() int {
	return l.viewport.Height
}
