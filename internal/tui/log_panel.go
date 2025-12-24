package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LogPanelModel struct {
	viewport viewport.Model
	logs     []string
	width    int
	height   int
}

func NewLogPanel() LogPanelModel {
	return LogPanelModel{}
}

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

func (l *LogPanelModel) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.viewport = viewport.New(width, height)

	if len(l.logs) > 0 {
		l.viewport.SetContent(strings.Join(l.logs, "\n"))
	}
}

func (l *LogPanelModel) SetLogs(logs []string) {
	wasAtBottom := l.viewport.AtBottom()
	l.logs = logs
	l.viewport.SetContent(strings.Join(logs, "\n"))
	if wasAtBottom {
		l.viewport.GotoBottom()
	}
}

func (l *LogPanelModel) AppendLog(line string) {
	wasAtBottom := l.viewport.AtBottom()
	l.logs = append(l.logs, line)
	l.viewport.SetContent(strings.Join(l.logs, "\n"))
	if wasAtBottom {
		l.viewport.GotoBottom()
	}
}

func (l *LogPanelModel) Clear() {
	l.logs = nil
	l.viewport.SetContent("")
}

func (l LogPanelModel) Logs() []string {
	return l.logs
}

func (l LogPanelModel) View() string {
	if len(l.logs) == 0 {
		return l.renderEmpty()
	}
	return l.viewport.View()
}

func (l LogPanelModel) renderEmpty() string {
	return lipgloss.NewStyle().Faint(true).Render("Waiting for output...")
}

func (l *LogPanelModel) ScrollUp(n int) {
	l.viewport.ScrollUp(n)
}

func (l *LogPanelModel) ScrollDown(n int) {
	l.viewport.ScrollDown(n)
}

func (l LogPanelModel) AtBottom() bool {
	return l.viewport.AtBottom()
}

func (l LogPanelModel) TotalLineCount() int {
	return l.viewport.TotalLineCount()
}

func (l LogPanelModel) ScrollPercent() float64 {
	return l.viewport.ScrollPercent()
}

func (l LogPanelModel) Height() int {
	return l.viewport.Height
}
