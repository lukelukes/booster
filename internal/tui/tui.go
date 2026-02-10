package tui

import (
	"booster/internal/task"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const asciiArt = ` ___  ___  ___  ___ _____ ___ ___
| _ )/ _ \/ _ \/ __|_   _| __| _ \
| _ \ (_) | (_) \__ \ | | | _||   /
|___/\___/ \___/|___/ |_| |___|_|_\` + "\n"

type startTaskMsg struct{}

type taskDoneMsg struct {
	result task.Result
	index  int
}

type spinnerTickMsg struct{}

type tickMsg struct{}

type Model struct {
	tasks    []task.Task
	results  []task.Result
	current  int
	aborted  bool
	startTime time.Time
	width    int
	spinner  SpinnerModel
	logPath  string
	elapsed  time.Duration
}

func New(tasks []task.Task, logPath string) Model {
	results := make([]task.Result, len(tasks))
	for i := range results {
		results[i] = task.Result{Status: task.StatusPending}
	}
	return Model{
		tasks:   tasks,
		results: results,
		spinner: NewSpinner(),
		logPath: logPath,
	}
}

func (m Model) Init() tea.Cmd {
	if m.done() {
		return nil
	}
	return func() tea.Msg { return startTaskMsg{} }
}

func (m Model) done() bool {
	return m.aborted || m.current >= len(m.tasks)
}

func (m Model) stopped() bool {
	return m.done()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if batch, ok := msg.(tea.BatchMsg); ok {
		var cmd tea.Cmd
		for _, c := range batch {
			if c != nil {
				bmsg := c()
				var updated tea.Model
				updated, cmd = m.Update(bmsg)
				m = updated.(Model)
				if cmd != nil {
					return m, cmd
				}
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.aborted = true
			return m, tea.Quit
		}
		if m.stopped() && msg.String() == "enter" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		if m.width < 40 {
			m.width = 80
		}

	case startTaskMsg:
		if m.done() {
			return m, nil
		}
		if m.current == 0 {
			m.startTime = time.Now()
		}
		idx := m.current
		return m, tea.Batch(
			runTaskAsync(m.tasks[idx], idx),
			m.spinner.Tick(),
			tickEvery(time.Second),
		)

	case taskDoneMsg:
		m.results[msg.index] = msg.result
		m.current++
		if msg.result.Status == task.StatusFailed {
			m.aborted = true
			return m, nil
		}
		if m.done() {
			return m, nil
		}
		return m, func() tea.Msg { return startTaskMsg{} }

	case spinnerTickMsg:
		m.spinner = m.spinner.Update(msg)
		if !m.done() && m.current < len(m.tasks) {
			return m, m.spinner.Tick()
		}
		return m, nil

	case tickMsg:
		if !m.startTime.IsZero() && !m.stopped() {
			m.elapsed = time.Since(m.startTime)
		}
		if !m.done() {
			return m, tickEvery(time.Second)
		}
		return m, nil
	}

	return m, nil
}

func runTaskAsync(t task.Task, index int) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		result := t.Run(context.Background())
		result.Duration = time.Since(start)
		return taskDoneMsg{result: result, index: index}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(asciiArt))
	if m.logPath != "" {
		b.WriteString(logHeaderStyle.Render("  logs → tail -f " + m.logPath))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	done := 0
	skipped := 0
	failed := 0
	for _, r := range m.results {
		switch r.Status {
		case task.StatusDone:
			done++
		case task.StatusSkipped:
			skipped++
		case task.StatusFailed:
			failed++
		}
	}

	for i, t := range m.tasks {
		r := m.results[i]
		name := t.Name()
		if m.width > 0 && len(name) > m.width-20 {
			name = name[:m.width-23] + "..."
		}

		switch r.Status {
		case task.StatusDone:
			b.WriteString(doneStyle.Render(fmt.Sprintf("  ✓  %s", name)))
			b.WriteString(" ")
			b.WriteString(leaderStyle.Render(dots(name, m.width)))
			b.WriteString(" ")
			b.WriteString(dimStyle.Render(formatDuration(r.Duration)))
		case task.StatusSkipped:
			b.WriteString(skippedStyle.Render(fmt.Sprintf("  ○  %s", name)))
			if r.Message != "" {
				b.WriteString("\n")
				b.WriteString(skippedStyle.Render("     └ " + truncate(r.Message, m.width-6)))
			}
		case task.StatusFailed:
			b.WriteString(failedStyle.Render(fmt.Sprintf("  ✗  %s", name)))
			if r.Error != nil {
				b.WriteString("\n")
				b.WriteString(failedStyle.Render("     └ " + truncate(r.Error.Error(), m.width-6)))
			} else if r.Message != "" {
				b.WriteString("\n")
				b.WriteString(failedStyle.Render("     └ " + truncate(r.Message, m.width-6)))
			}
		case task.StatusPending:
			if i == m.current {
				b.WriteString(runningStyle.Render(fmt.Sprintf("  %s  %s", m.spinner.View(), name)))
				b.WriteString(" ")
				b.WriteString(leaderStyle.Render(dots(name, m.width)))
				b.WriteString(" ")
				elapsed := m.elapsed
				if !m.startTime.IsZero() {
					elapsed = time.Since(m.startTime)
				}
				b.WriteString(dimStyle.Render(formatDuration(elapsed)))
			} else {
				b.WriteString(pendingStyle.Render(fmt.Sprintf("  ·  %s", name)))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(footer(m)))
	return b.String()
}

func dots(name string, width int) string {
	if width <= 0 {
		width = 60
	}
	need := width - lipglossWidth(name) - 15
	if need < 3 {
		need = 3
	}
	return strings.Repeat("·", need)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max || max <= 0 {
		return s
	}
	return s[:max-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func footer(m Model) string {
	done := 0
	skipped := 0
	failed := 0
	for _, r := range m.results {
		switch r.Status {
		case task.StatusDone:
			done++
		case task.StatusSkipped:
			skipped++
		case task.StatusFailed:
			failed++
		}
	}
	total := len(m.tasks)
	elapsed := m.elapsed
	if !m.startTime.IsZero() && !m.stopped() {
		elapsed = time.Since(m.startTime)
	}

	if m.stopped() {
		if failed > 0 {
			return fmt.Sprintf("  %d/%d · %d failed · %s", done+skipped, total, failed, formatDuration(elapsed))
		}
		return fmt.Sprintf("  %d/%d · %d skipped · %s", done, total, skipped, formatDuration(elapsed))
	}
	return fmt.Sprintf("  %d/%d · %d skipped · %s", done, total, skipped, formatDuration(elapsed))
}

func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}
