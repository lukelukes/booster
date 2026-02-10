package tui

import (
	"booster/internal/task"
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_WithTasks(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}},
	}
	model := New(tasks, "/tmp/booster-123.log")
	cmd := model.Init()

	assert.NotNil(t, cmd, "Init should return a command when tasks exist")
}

func TestInit_EmptyTasks(t *testing.T) {
	model := New([]task.Task{}, "")
	cmd := model.Init()

	assert.Nil(t, cmd, "Init should return nil when no tasks exist")
}

func TestUpdate_KeyQuit(t *testing.T) {
	tasks := []task.Task{&mockTask{name: "t", result: task.Result{Status: task.StatusDone}}}
	model := New(tasks, "")

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.NotNil(t, cmd)
}

func TestUpdate_StartTaskRunsTask(t *testing.T) {
	runCalled := false
	tasks := []task.Task{
		&mockTask{
			name:   "task1",
			result: task.Result{Status: task.StatusDone},
			runFunc: func() {
				runCalled = true
			},
		},
	}
	model := New(tasks, "")
	model.width = 80

	_, cmd := model.Update(startTaskMsg{})
	require.NotNil(t, cmd)
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				m := c()
				if done, ok := m.(taskDoneMsg); ok {
					assert.Equal(t, task.StatusDone, done.result.Status)
					break
				}
			}
		}
	} else if done, ok := msg.(taskDoneMsg); ok {
		assert.Equal(t, task.StatusDone, done.result.Status)
	}
	assert.True(t, runCalled)
}

func TestView_ShowsAsciiAndLogPath(t *testing.T) {
	tasks := []task.Task{&mockTask{name: "task1", result: task.Result{Status: task.StatusPending}}}
	model := New(tasks, "/tmp/booster-123.log")
	model.width = 80

	view := model.View()

	assert.Contains(t, view, "___")
	assert.Contains(t, view, "tail -f /tmp/booster-123.log")
	assert.Contains(t, view, "task1")
}

func TestView_ShowsTaskStatuses(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "done", result: task.Result{Status: task.StatusDone, Duration: 1000000000}},
		&mockTask{name: "skipped", result: task.Result{Status: task.StatusSkipped, Message: "when: false"}},
		&mockTask{name: "pending", result: task.Result{Status: task.StatusPending}},
	}
	model := New(tasks, "")
	model.results = []task.Result{
		{Status: task.StatusDone, Duration: 1000000000},
		{Status: task.StatusSkipped, Message: "when: false"},
		{Status: task.StatusPending},
	}
	model.current = 2
	model.width = 80

	view := model.View()

	assert.Contains(t, view, "✓")
	assert.Contains(t, view, "done")
	assert.Contains(t, view, "○")
	assert.Contains(t, view, "skipped")
	assert.Contains(t, view, "└")
	assert.Contains(t, view, "when: false")
	assert.Contains(t, view, "pending")
}

func TestView_FailedShowsError(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "failed", result: task.Result{Status: task.StatusFailed, Error: assert.AnError}},
	}
	model := New(tasks, "")
	model.results = []task.Result{{Status: task.StatusFailed, Error: assert.AnError}}
	model.current = 1
	model.aborted = true
	model.width = 80

	view := model.View()

	assert.Contains(t, view, "✗")
	assert.Contains(t, view, "failed")
	assert.Contains(t, view, "└")
}

func TestUpdate_StopsOnFailure(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "fail", result: task.Result{Status: task.StatusFailed}},
	}
	model := New(tasks, "")
	model.width = 80

	_, cmd := model.Update(startTaskMsg{})
	require.NotNil(t, cmd)
	msg := cmd()
	var doneMsg taskDoneMsg
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				m := c()
				if d, ok := m.(taskDoneMsg); ok {
					doneMsg = d
					break
				}
			}
		}
	} else if d, ok := msg.(taskDoneMsg); ok {
		doneMsg = d
	}

	updated, _ := model.Update(doneMsg)
	m := updated.(Model)
	assert.True(t, m.aborted)
	assert.Equal(t, 1, m.current)
}

type mockTask struct {
	name     string
	result   task.Result
	runFunc  func()
	needsSudo bool
}

func (t *mockTask) Name() string {
	return t.name
}

func (t *mockTask) Run(ctx context.Context) task.Result {
	if t.runFunc != nil {
		t.runFunc()
	}
	return t.result
}

func (t *mockTask) NeedsSudo() bool {
	return t.needsSudo
}

func TestDots(t *testing.T) {
	result := dots("short", 60)
	assert.True(t, strings.Contains(result, "·"))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 6))
}

func TestFormatDuration(t *testing.T) {
	assert.Contains(t, formatDuration(0), "s")
}

func newMockTask(name string, status task.Status, message string, err error) *mockTask {
	return &mockTask{
		name:   name,
		result: task.Result{Status: status, Message: message, Error: err},
	}
}
