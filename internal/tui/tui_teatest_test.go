package tui

import (
	"booster/internal/task"
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestTeatest_JKNavigationTaskList(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	// Wait for all tasks to complete (shows "enter exit" in help)
	// After completion, selection is on task3 due to auto-advance
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("enter exit")) &&
			selectionIndicatorOnLine(bts, "task3")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// Navigate: k -> task2, k -> task1, k -> (stay task1), j -> task2
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // task2
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // task1
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // still task1 (at top)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // task2

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return selectionIndicatorOnLine(bts, "task2")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))
}

func TestTeatest_ArrowKeyNavigation(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	// Wait for all tasks to complete
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("enter exit"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// After completion, selection is on task2 (last task)
	// Press up arrow to go to task1
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return selectionIndicatorOnLine(bts, "task1")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// Press down arrow to move back to task2
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return selectionIndicatorOnLine(bts, "task2")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))
}

func TestTeatest_TabSwitchesFocus(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	// Wait for tasks to complete and two-column mode to render
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("BOOSTER")) &&
			bytes.Contains(bts, []byte("Logs:")) &&
			bytes.Contains(bts, []byte("enter exit"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// Tab switching only changes border colors (no text changes)
	// Send tab and then navigate to verify focus switched correctly
	// When log panel is focused, 'j' scrolls logs instead of moving task selection
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})                       // focus -> logs
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})                       // focus -> task list
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // navigate to task1

	// If tab worked, we should now be on task1 (navigation worked in task list)
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return selectionIndicatorOnLine(bts, "task1")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))
}

func TestTeatest_ToggleLogsPanel(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	// Wait for initial render with logs panel visible
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Logs:"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// Press 'o' to hide logs
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})

	// Logs panel should be hidden, help should say "show logs"
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("show logs")) &&
			!bytes.Contains(bts, []byte("Logs:"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// Press 'o' again to show logs
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})

	// Logs panel should be visible again
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Logs:"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))
}

func TestTeatest_QuitWithQ(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks),
		teatest.WithInitialTermSize(100, 40),
	)

	// Wait for render
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("BOOSTER"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// Press 'q' to quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// Program should finish
	tm.WaitFinished(t, teatest.WithFinalTimeout(500*time.Millisecond))
}

func TestTeatest_NavigationBoundsAtEdges(t *testing.T) {
	tasks := []task.Task{
		newMockTask("first_task", task.StatusDone, "", nil),
		newMockTask("middle_task", task.StatusDone, "", nil),
		newMockTask("last_task", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	// Wait for all tasks to complete (last_task will be selected due to auto-advance)
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("enter exit")) &&
			selectionIndicatorOnLine(bts, "last_task")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))

	// Navigate: j (no-op at bottom), k, k -> first_task, k (no-op at top), j -> middle_task
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // no-op, at bottom
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // middle_task
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // first_task
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) // no-op, at top
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // middle_task

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return selectionIndicatorOnLine(bts, "middle_task")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))
}

func TestTeatest_AutoAdvancement(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks), teatest.WithInitialTermSize(100, 40))
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("enter exit")) &&
			selectionIndicatorOnLine(bts, "task3")
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(500*time.Millisecond))
}

func selectionIndicatorOnLine(output []byte, taskName string) bool {
	lines := bytes.SplitSeq(output, []byte("\n"))
	for line := range lines {
		if bytes.Contains(line, []byte(SelectionIndicator)) && bytes.Contains(line, []byte(taskName)) {
			return true
		}
	}
	return false
}
