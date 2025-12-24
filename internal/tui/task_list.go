package tui

import (
	"booster/internal/executor"
	"booster/internal/task"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TaskListModel struct {
	exec     *executor.Executor
	viewport viewport.Model
	spinner  SpinnerModel
	selected int
	width    int
	height   int
}

func NewTaskList(exec *executor.Executor) TaskListModel {
	return TaskListModel{
		exec:     exec,
		spinner:  NewSpinner(),
		selected: 0,
	}
}

func (t TaskListModel) Update(msg tea.Msg) (TaskListModel, tea.Cmd) {
	t.spinner = t.spinner.Update(msg)

	needsRefresh := false

	switch msg := msg.(type) {
	case spinnerTickMsg:
		needsRefresh = !t.exec.Stopped()
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if t.selected < t.exec.Total()-1 {
				t.selected++
				t.ensureVisible()
				needsRefresh = true
			}
		case "k", "up":
			if t.selected > 0 {
				t.selected--
				t.ensureVisible()
				needsRefresh = true
			}
		}
	}

	if needsRefresh {
		t.RefreshContent()
	}
	return t, nil
}

func (t *TaskListModel) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.viewport = viewport.New(width, height)
}

func (t TaskListModel) Selected() int {
	return t.selected
}

func (t *TaskListModel) SetSelected(idx int) {
	if idx >= 0 && idx < t.exec.Total() {
		t.selected = idx
		t.ensureVisible()
	}
}

func (t *TaskListModel) ensureVisible() {
	if t.viewport.Height == 0 {
		return
	}

	visibleStart := t.viewport.YOffset
	visibleEnd := visibleStart + t.viewport.Height

	if t.selected < visibleStart {
		t.viewport.SetYOffset(t.selected)
	}
	if t.selected >= visibleEnd {
		t.viewport.SetYOffset(t.selected - t.viewport.Height + 1)
	}
}

func (t TaskListModel) View() string {
	return t.viewport.View()
}

func (t *TaskListModel) RefreshContent() {
	content := t.renderTaskLines()
	t.viewport.SetContent(content)
}

func (t TaskListModel) renderTaskLines() string {
	var s strings.Builder

	tasks := t.exec.Tasks()
	current := t.exec.Current()
	stopped := t.exec.Stopped()

	for i, tsk := range tasks {
		var line string
		result := t.exec.ResultAt(i)
		isSelected := i == t.selected

		prefix := "○ "
		if isSelected {
			prefix = "▶ "
		}

		if result.Status != task.StatusPending {
			switch result.Status {
			case task.StatusDone:
				suffix := formatElapsedCompact(result.Duration)
				taskLine := renderTaskWithLeader(prefix+"✓ ", tsk.Name(), suffix, t.width)
				line = doneStyle.Render(taskLine)
			case task.StatusSkipped:
				label := "exists"
				if strings.HasPrefix(result.Message, "condition not met:") {
					label = "skipped"
				}
				taskLine := renderTaskWithLeader(prefix+"○ ", tsk.Name(), label, t.width)
				line = skippedStyle.Render(taskLine)
			case task.StatusFailed:
				line = failedStyle.Render(prefix + "✗ " + tsk.Name())
			}
		} else if i == current && !stopped {
			line = runningStyle.Render(prefix + "→ " + tsk.Name() + " " + t.spinner.View())
		} else {
			line = pendingStyle.Render(prefix + "  " + tsk.Name())
		}

		if isSelected {
			lineWidth := lipgloss.Width(line)
			if lineWidth < t.width {
				line += strings.Repeat(" ", t.width-lineWidth-4)
			}
			line = selectedRowStyle.Render(line)
		}

		s.WriteString(line)
		if i < len(tasks)-1 {
			s.WriteString("\n")
		}
	}

	return s.String()
}

func (t TaskListModel) SpinnerTick() tea.Cmd {
	return t.spinner.Tick()
}
