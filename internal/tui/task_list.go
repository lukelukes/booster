package tui

import (
	"booster/internal/executor"
	"booster/internal/task"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskListModel manages the task list display and navigation.
// Follows the model tree pattern as a child of the main Model.
type TaskListModel struct {
	exec     *executor.Executor
	viewport viewport.Model
	spinner  SpinnerModel
	selected int
	width    int
	height   int
}

// NewTaskList creates a new task list model.
func NewTaskList(exec *executor.Executor) TaskListModel {
	return TaskListModel{
		exec:     exec,
		spinner:  NewSpinner(),
		selected: 0,
	}
}

// Update handles messages for the task list.
// Returns the updated model and any commands to execute.
func (t TaskListModel) Update(msg tea.Msg) (TaskListModel, tea.Cmd) {
	// Update spinner on tick
	t.spinner = t.spinner.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if t.selected < t.exec.Total()-1 {
				t.selected++
				t.ensureVisible()
			}
		case "k", "up":
			if t.selected > 0 {
				t.selected--
				t.ensureVisible()
			}
		}
	}

	return t, nil
}

// SetSize updates the viewport dimensions.
func (t *TaskListModel) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.viewport = viewport.New(width, height)
}

// Selected returns the currently selected task index.
func (t TaskListModel) Selected() int {
	return t.selected
}

// SetSelected sets the selected task index.
func (t *TaskListModel) SetSelected(idx int) {
	if idx >= 0 && idx < t.exec.Total() {
		t.selected = idx
		t.ensureVisible()
	}
}

// ensureVisible scrolls the viewport to keep the selected task visible.
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

// View renders the task list.
func (t TaskListModel) View() string {
	content := t.renderTaskLines()
	t.viewport.SetContent(content)
	return t.viewport.View()
}

// renderTaskLines renders all task lines for the viewport content.
func (t TaskListModel) renderTaskLines() string {
	var s strings.Builder

	tasks := t.exec.Tasks()
	current := t.exec.Current()
	stopped := t.exec.Stopped()

	for i, tsk := range tasks {
		var line string
		result := t.exec.ResultAt(i)
		isSelected := i == t.selected

		// Selection indicator prefix
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
			// Currently running with animated spinner
			line = runningStyle.Render(prefix + "→ " + tsk.Name() + " " + t.spinner.View())
		} else {
			// Pending
			line = pendingStyle.Render(prefix + "  " + tsk.Name())
		}

		// Apply selection highlight
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

// SpinnerTick returns the spinner's tick command.
func (t TaskListModel) SpinnerTick() tea.Cmd {
	return t.spinner.Tick()
}
