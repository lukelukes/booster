// Package tui provides the terminal user interface for booster.
package tui

import (
	"booster/internal/executor"
	"booster/internal/task"
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Model is the Bubble Tea model for the TUI.
type Model struct {
	exec       *executor.Executor
	showOutput bool // Toggle to show command output
}

// New creates a new TUI model with the given tasks.
func New(tasks []task.Task) Model {
	return Model{
		exec: executor.New(tasks),
	}
}

// Init starts the first task.
func (m Model) Init() tea.Cmd {
	if m.exec.Done() {
		return nil
	}
	return m.runNext()
}

// taskDoneMsg signals a task completed.
type taskDoneMsg struct {
	result task.Result
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.exec.Done() {
				return m, tea.Quit
			}
		case "o":
			if m.exec.Done() {
				m.showOutput = !m.showOutput
				return m, nil
			}
		}

	case taskDoneMsg:
		if m.exec.Done() {
			return m, nil
		}
		return m, m.runNext()
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("BOOSTER"))
	s.WriteString("\n\n")

	tasks := m.exec.Tasks()
	current := m.exec.Current()
	done := m.exec.Done()

	for i, t := range tasks {
		var line string
		r := m.exec.ResultAt(i)

		if r.Status != task.StatusPending {
			// Completed task
			switch r.Status {
			case task.StatusDone:
				line = doneStyle.Render("✓ " + t.Name())
			case task.StatusSkipped:
				line = skippedStyle.Render("○ " + t.Name() + " (exists)")
			case task.StatusFailed:
				errMsg := "unknown error"
				if r.Error != nil {
					errMsg = r.Error.Error()
				}
				line = failedStyle.Render("✗ " + t.Name() + ": " + errMsg)
			}
		} else if i == current && !done {
			// Currently running
			line = runningStyle.Render("→ " + t.Name() + "...")
		} else {
			// Pending
			line = pendingStyle.Render("  " + t.Name())
		}

		s.WriteString(line + "\n")
	}

	if done {
		s.WriteString("\n")

		summary := m.exec.Summary()

		if summary.HasFailures {
			s.WriteString(summaryStyle.Render(
				fmt.Sprintf("Finished with errors: %d done, %d skipped, %d failed",
					summary.Done, summary.Skipped, summary.Failed)))
		} else {
			s.WriteString(summaryStyle.Render(
				fmt.Sprintf("Done! %d completed, %d skipped", summary.Done, summary.Skipped)))
		}

		// Show output section if toggled
		if m.showOutput {
			s.WriteString("\n")
			s.WriteString(outputHeaderStyle.Render("─── Output ───"))
			s.WriteString("\n")

			for i, t := range tasks {
				r := m.exec.ResultAt(i)
				if r.Output != "" {
					s.WriteString("\n")
					s.WriteString(outputTaskStyle.Render(t.Name()))
					s.WriteString("\n")
					s.WriteString(outputContentStyle.Render(strings.TrimSpace(r.Output)))
					s.WriteString("\n")
				}
			}
		}

		// Build help text
		s.WriteString("\n")
		hasOutput := m.hasTaskOutput()
		if hasOutput {
			if m.showOutput {
				s.WriteString(helpStyle.Render("Press 'o' to hide output, Enter to exit"))
			} else {
				s.WriteString(helpStyle.Render("Press 'o' to view output, Enter to exit"))
			}
		} else {
			s.WriteString(helpStyle.Render("Press Enter to exit"))
		}
	}

	return s.String()
}

// runNext creates a command to run the next task.
func (m Model) runNext() tea.Cmd {
	return func() tea.Msg {
		result, _ := m.exec.RunNext(context.Background())
		return taskDoneMsg{result: result}
	}
}

// hasTaskOutput returns true if any task has output to display.
func (m Model) hasTaskOutput() bool {
	tasks := m.exec.Tasks()
	for i := range tasks {
		if m.exec.ResultAt(i).Output != "" {
			return true
		}
	}
	return false
}
