package tui

import (
	"booster/internal/task"
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestTeatest_ShowsCompletion(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks, ""),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("task1")) &&
			bytes.Contains(bts, []byte("task2"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(2*time.Second))
}

func TestTeatest_ShowsAsciiArt(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks, "/tmp/booster-123.log"),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("___")) &&
			bytes.Contains(bts, []byte("tail -f /tmp/booster-123.log"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(2*time.Second))
}

func TestTeatest_QuitOnEnter(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}

	tm := teatest.NewTestModel(t, New(tasks, ""),
		teatest.WithInitialTermSize(100, 40),
	)
	t.Cleanup(func() { tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("task1"))
	}, teatest.WithCheckInterval(10*time.Millisecond),
		teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.WaitFinished(t)
}
