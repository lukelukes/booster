package tui

import (
	"booster/internal/coordinator"
	"booster/internal/executor"
	"booster/internal/logstream"
	"booster/internal/task"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTask struct {
	name   string
	result task.Result
}

func (m *mockTask) Name() string {
	return m.name
}

func (m *mockTask) Run(ctx context.Context) task.Result {
	return m.result
}

func (m *mockTask) NeedsSudo() bool {
	return false
}

func newMockTask(name string, status task.Status, output string, err error) *mockTask {
	return &mockTask{
		name: name,
		result: task.Result{
			Status: status,
			Output: output,
			Error:  err,
		},
	}
}

func newMockTaskWithMessage(name string, status task.Status, message string) *mockTask {
	return &mockTask{
		name: name,
		result: task.Result{
			Status:  status,
			Message: message,
		},
	}
}

func TestNew(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}

	model := New(tasks)

	assert.NotNil(t, model.exec, "executor should be initialized")
	assert.False(t, model.showOutput, "showOutput should default to false")
	assert.Equal(t, 2, model.exec.Total(), "should have correct number of tasks")
}

func TestInit_WithTasks(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	cmd := model.Init()

	assert.NotNil(t, cmd, "Init should return a command when tasks exist")
}

func TestInit_EmptyTasks(t *testing.T) {
	model := New([]task.Task{})

	cmd := model.Init()

	assert.Nil(t, cmd, "Init should return nil when no tasks exist")
}

func TestUpdate_KeyHandling(t *testing.T) {
	tests := []struct {
		name            string
		setupModel      func() Model
		keyMsg          tea.KeyMsg
		wantQuitCmd     bool
		wantShowOutput  *bool
		wantToggleCount int
	}{
		{
			name: "q quits",
			setupModel: func() Model {
				tasks := []task.Task{newMockTask("task1", task.StatusDone, "", nil)}
				return New(tasks)
			},
			keyMsg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			wantQuitCmd: true,
		},
		{
			name: "Ctrl+C quits",
			setupModel: func() Model {
				tasks := []task.Task{newMockTask("task1", task.StatusDone, "", nil)}
				return New(tasks)
			},
			keyMsg:      tea.KeyMsg{Type: tea.KeyCtrlC},
			wantQuitCmd: true,
		},
		{
			name: "Enter quits when done",
			setupModel: func() Model {
				exec := executor.New([]task.Task{})
				return Model{exec: exec}
			},
			keyMsg:      tea.KeyMsg{Type: tea.KeyEnter},
			wantQuitCmd: true,
		},
		{
			name: "Enter ignored when not done",
			setupModel: func() Model {
				tasks := []task.Task{newMockTask("task1", task.StatusDone, "", nil)}
				return New(tasks)
			},
			keyMsg:      tea.KeyMsg{Type: tea.KeyEnter},
			wantQuitCmd: false,
		},
		{
			name: "o toggles output when done",
			setupModel: func() Model {
				exec := executor.New([]task.Task{})
				return Model{exec: exec, showOutput: false}
			},
			keyMsg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")},
			wantShowOutput: func() *bool {
				b := true
				return &b
			}(),
		},
		{
			name: "o ignored when not done",
			setupModel: func() Model {
				tasks := []task.Task{newMockTask("task1", task.StatusDone, "", nil)}
				return New(tasks)
			},
			keyMsg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")},
			wantShowOutput: func() *bool {
				b := false
				return &b
			}(),
		},
		{
			name: "unknown key ignored",
			setupModel: func() Model {
				tasks := []task.Task{newMockTask("task1", task.StatusDone, "", nil)}
				return New(tasks)
			},
			keyMsg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")},
			wantQuitCmd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.setupModel()
			newModel, cmd := model.Update(tt.keyMsg)

			assert.IsType(t, Model{}, newModel, "Update should return Model type")

			if tt.wantQuitCmd {
				require.NotNil(t, cmd, "Should return quit command")
				msg := cmd()
				_, isQuit := msg.(tea.QuitMsg)
				assert.True(t, isQuit, "Command should be a quit message")
			} else if tt.wantShowOutput != nil {
				assert.Nil(t, cmd, "Should not return a command")
				updatedModel, ok := newModel.(Model)
				require.True(t, ok, "newModel should be Model type")
				assert.Equal(t, *tt.wantShowOutput, updatedModel.showOutput, "showOutput state mismatch")
			} else {
				assert.Nil(t, cmd, "Should not return a command")
			}
		})
	}

	t.Run("o toggles output twice when done", func(t *testing.T) {
		exec := executor.New([]task.Task{})
		model := Model{exec: exec, showOutput: false}

		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		assert.Nil(t, cmd, "Should not return a command")
		updatedModel, ok := newModel.(Model)
		require.True(t, ok, "newModel should be Model type")
		assert.True(t, updatedModel.showOutput, "showOutput should be toggled to true")

		newModel2, cmd2 := updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		assert.Nil(t, cmd2, "Should not return a command")
		updatedModel2, ok2 := newModel2.(Model)
		require.True(t, ok2, "newModel2 should be Model type")
		assert.False(t, updatedModel2.showOutput, "showOutput should be toggled back to false")
	})
}

func TestUpdate_TaskDoneMsg(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	_, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "First task should run")

	model.coord.StartTask(0)

	model.coord.LogsDone()

	msg := taskDoneMsg{result: task.Result{Status: task.StatusDone}}
	newModel, cmd := model.Update(msg)

	assert.NotNil(t, cmd, "Should return command to run next task")
	assert.IsType(t, Model{}, newModel, "Update should return Model type")

	assert.NotNil(t, cmd, "Batch command should not be nil")
}

func TestUpdate_TaskDoneMsgWhenAllComplete(t *testing.T) {
	exec := executor.New([]task.Task{})
	model := Model{exec: exec, coord: coordinator.New()}

	msg := taskDoneMsg{result: task.Result{Status: task.StatusDone}}
	newModel, cmd := model.Update(msg)

	assert.Nil(t, cmd, "Should not return command when all tasks are complete")
	assert.IsType(t, Model{}, newModel, "Update should return Model type")
}

func TestUpdate_TaskFailure_StopsExecution(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusFailed, "", errors.New("fail")),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	model := New(tasks)

	result1, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "Task1 should run")
	require.Equal(t, task.StatusDone, result1.Status)

	result2, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "Task2 should run")
	require.Equal(t, task.StatusFailed, result2.Status)

	model.coord.StartTask(1)
	model.coord.LogsDone()

	msg := taskDoneMsg{result: result2}
	newModel, cmd := model.Update(msg)
	model = newModel.(Model)

	assert.Nil(t, cmd, "Should NOT return command after failure")
	assert.True(t, model.exec.Stopped(), "Executor should be stopped")
	assert.False(t, model.exec.Done(), "Should NOT be done (task3 didn't run)")

	assert.Equal(t, task.StatusPending, model.exec.ResultAt(2).Status,
		"Task3 should remain pending after abort")

	view := model.View()
	assert.Contains(t, view, "task3", "Should show pending task in list")
	assert.Contains(t, view, "BOOSTER FAILED", "Should show failure summary")
}

func TestUpdate_UnknownMessageIgnored(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	type unknownMsg struct{}
	newModel, cmd := model.Update(unknownMsg{})

	assert.Nil(t, cmd, "Should return nil for unknown messages")
	assert.IsType(t, Model{}, newModel, "Update should return Model type")
}

func TestView_ContainsTitle(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	view := model.View()

	assert.Contains(t, view, "BOOSTER", "View should contain title")
}

func TestView_TaskStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupModel    func() Model
		executeCount  int
		checkTaskName string
		wantContains  []string
		wantHelper    func(t *testing.T, view string)
	}{
		{
			name: "pending task",
			setupModel: func() Model {
				tasks := []task.Task{
					newMockTask("completed task", task.StatusDone, "", nil),
					newMockTask("running task", task.StatusDone, "", nil),
					newMockTask("pending task", task.StatusDone, "", nil),
				}
				return New(tasks)
			},
			executeCount:  1,
			checkTaskName: "pending task",
			wantContains: []string{
				"pending task",
				"  pending task",
			},
		},
		{
			name: "running task",
			setupModel: func() Model {
				tasks := []task.Task{
					newMockTask("running task", task.StatusDone, "", nil),
					newMockTask("pending task", task.StatusDone, "", nil),
				}
				return New(tasks)
			},
			executeCount:  0,
			checkTaskName: "running task",
			wantContains: []string{
				"running task",
				"→",
			},
		},
		{
			name: "done task",
			setupModel: func() Model {
				tasks := []task.Task{
					newMockTask("completed task", task.StatusDone, "", nil),
				}
				return New(tasks)
			},
			executeCount:  1,
			checkTaskName: "completed task",
			wantHelper: func(t *testing.T, view string) {
				AssertTaskStatus(t, view, "completed task", task.StatusDone)
			},
		},
		{
			name: "skipped task (idempotent - exists)",
			setupModel: func() Model {
				tasks := []task.Task{
					newMockTaskWithMessage("skipped task", task.StatusSkipped, "already exists"),
				}
				return New(tasks)
			},
			executeCount:  1,
			checkTaskName: "skipped task",
			wantHelper: func(t *testing.T, view string) {
				AssertTaskStatus(t, view, "skipped task", task.StatusSkipped)
				AssertSkippedReason(t, view, "exists")
			},
		},
		{
			name: "skipped task (condition not met)",
			setupModel: func() Model {
				tasks := []task.Task{
					newMockTaskWithMessage("conditional task", task.StatusSkipped, "condition not met: os=darwin, want arch"),
				}
				return New(tasks)
			},
			executeCount:  1,
			checkTaskName: "conditional task",
			wantHelper: func(t *testing.T, view string) {
				AssertTaskStatus(t, view, "conditional task", task.StatusSkipped)
				AssertSkippedReason(t, view, "skipped")
			},
		},
		{
			name: "failed task with error",
			setupModel: func() Model {
				testErr := errors.New("test error message")
				tasks := []task.Task{
					newMockTask("failed task", task.StatusFailed, "", testErr),
				}
				return New(tasks)
			},
			executeCount:  1,
			checkTaskName: "failed task",
			wantHelper: func(t *testing.T, view string) {
				AssertTaskStatus(t, view, "failed task", task.StatusFailed)
				AssertHasError(t, view, "test error message")
			},
		},
		{
			name: "failed task with nil error",
			setupModel: func() Model {
				tasks := []task.Task{
					newMockTask("failed task", task.StatusFailed, "", nil),
				}
				return New(tasks)
			},
			executeCount:  1,
			checkTaskName: "failed task",
			wantContains: []string{
				"failed task",
				"FAILED",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.setupModel()

			for i := 0; i < tt.executeCount; i++ {
				_, ok := model.exec.RunNext(context.Background())
				require.True(t, ok, "Task %d should run", i+1)
			}

			view := model.View()

			for _, want := range tt.wantContains {
				assert.Contains(t, view, want, "View should contain: %s", want)
			}

			if tt.wantHelper != nil {
				tt.wantHelper(t, view)
			}
		})
	}
}

type viewTestCase struct {
	name            string
	tasks           []task.Task
	showOutput      bool
	wantContains    []string
	wantNotContains []string
}

func runViewTests(t *testing.T, tests []viewTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(tt.tasks)

			model.width = 50
			model.height = 40

			for range tt.tasks {
				_, _ = model.exec.RunNext(context.Background())
			}

			model.showOutput = tt.showOutput

			if tt.showOutput {
				model.outputViewport = model.createOutputViewport()
			}

			view := model.View()

			for _, want := range tt.wantContains {
				assert.Contains(t, view, want, "View should contain: %s", want)
			}

			for _, wantNot := range tt.wantNotContains {
				assert.NotContains(t, view, wantNot, "View should not contain: %s", wantNot)
			}
		})
	}
}

func TestView_Summary(t *testing.T) {
	tests := []viewTestCase{
		{
			name: "success summary without failures",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "", nil),
				newMockTask("task2", task.StatusSkipped, "", nil),
			},
			wantContains: []string{
				"BOOSTER COMPLETE",
				"completed",
				"skipped",
			},
		},
		{
			name: "error summary with failures",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "", nil),
				newMockTask("task2", task.StatusFailed, "", errors.New("error")),
			},
			wantContains: []string{
				"BOOSTER FAILED",
				"completed",
				"failed",
			},
		},
	}

	runViewTests(t, tests)
}

func TestView_HelpText(t *testing.T) {
	tests := []viewTestCase{
		{
			name: "without output",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "", nil),
			},
			wantContains: []string{
				"Enter exit",
			},
			wantNotContains: []string{
				"'o'",
			},
		},
		{
			name: "with output hidden",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "some output", nil),
			},
			showOutput: false,
			wantContains: []string{
				"'o' view output",
			},
		},
		{
			name: "with output visible",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "some output", nil),
			},
			showOutput: true,
			wantContains: []string{
				"'o' hide",
				"scroll",
			},
		},
	}

	runViewTests(t, tests)
}

func TestView_OutputSection(t *testing.T) {
	tests := []viewTestCase{
		{
			name: "hidden by default",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "some output", nil),
			},
			showOutput: false,
			wantNotContains: []string{
				"─── Output ───",
				"some output",
			},
		},
		{
			name: "visible when toggled",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "some output", nil),
			},
			showOutput: true,
			wantContains: []string{
				"─── Output ───",
				"task1",
				"some output",
			},
		},
		{
			name: "multiple tasks with output",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "output 1", nil),
				newMockTask("task2", task.StatusDone, "output 2", nil),
				newMockTask("task3", task.StatusDone, "", nil),
			},
			showOutput: true,
			wantContains: []string{
				"task1",
				"output 1",
				"task2",
				"output 2",
			},
		},
		{
			name: "trims whitespace",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "  \n  output with spaces  \n  ", nil),
			},
			showOutput: true,
			wantContains: []string{
				"output with spaces",
			},
			wantNotContains: []string{
				"  \n  output with spaces  \n  ",
			},
		},
	}

	runViewTests(t, tests)
}

func TestHasTaskOutput(t *testing.T) {
	t.Run("returns false when no tasks have output", func(t *testing.T) {
		tasks := []task.Task{
			newMockTask("task1", task.StatusDone, "", nil),
			newMockTask("task2", task.StatusDone, "", nil),
		}
		model := New(tasks)

		_, _ = model.exec.RunNext(context.Background())
		_, _ = model.exec.RunNext(context.Background())

		hasOutput := model.hasTaskOutput()
		assert.False(t, hasOutput, "Should return false when no tasks have output")
	})

	t.Run("returns true when at least one task has output", func(t *testing.T) {
		tasks := []task.Task{
			newMockTask("task1", task.StatusDone, "", nil),
			newMockTask("task2", task.StatusDone, "some output", nil),
		}
		model := New(tasks)

		_, _ = model.exec.RunNext(context.Background())
		_, _ = model.exec.RunNext(context.Background())

		hasOutput := model.hasTaskOutput()
		assert.True(t, hasOutput, "Should return true when at least one task has output")
	})

	t.Run("returns false for pending tasks", func(t *testing.T) {
		tasks := []task.Task{
			newMockTask("task1", task.StatusDone, "some output", nil),
		}
		model := New(tasks)

		hasOutput := model.hasTaskOutput()
		assert.False(t, hasOutput, "Should return false when tasks haven't run yet")
	})
}

func TestStartTask(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "output", nil),
	}
	model := New(tasks)

	logWriter, logCh, cmd := model.startTask()
	require.NotNil(t, logWriter, "startTask should return a logWriter")
	require.NotNil(t, logCh, "startTask should return a logCh")
	require.NotNil(t, cmd, "startTask should return a command")

	taskCmd := runTask(model.exec, logWriter)
	msg := taskCmd()

	taskMsg, ok := msg.(taskDoneMsg)
	require.True(t, ok, "Should return taskDoneMsg")
	assert.Equal(t, task.StatusDone, taskMsg.result.Status, "Result should have done status")

	_, ok = <-logCh
	assert.False(t, ok, "Log channel should be closed after task")
}

func TestIntegration_FullTaskFlow(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "output1", nil),
		newMockTask("task2", task.StatusSkipped, "output2", nil),
		newMockTask("task3", task.StatusFailed, "", errors.New("failure")),
	}
	model := New(tasks)

	cmd := model.Init()
	require.NotNil(t, cmd, "Init should return command")

	result1, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "Task1 should run")
	assert.Equal(t, task.StatusDone, result1.Status)

	model.coord.StartTask(0)
	model.coord.LogsDone()
	newModel, cmd := model.Update(taskDoneMsg{result: result1})
	model, ok = newModel.(Model)
	require.True(t, ok, "newModel should be Model type")
	require.NotNil(t, cmd, "Should return command for task2")

	result2, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "Task2 should run")
	assert.Equal(t, task.StatusSkipped, result2.Status)

	model.coord.StartTask(1)
	model.coord.LogsDone()
	newModel, cmd = model.Update(taskDoneMsg{result: result2})
	model, ok = newModel.(Model)
	require.True(t, ok, "newModel should be Model type")
	require.NotNil(t, cmd, "Should return command for task3")

	result3, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "Task3 should run")
	assert.Equal(t, task.StatusFailed, result3.Status)

	model.coord.StartTask(2)
	model.coord.LogsDone()
	newModel, cmd = model.Update(taskDoneMsg{result: result3})
	model, ok = newModel.(Model)
	require.True(t, ok, "newModel should be Model type")
	assert.Nil(t, cmd, "Should return nil after failure (aborted)")

	assert.True(t, model.exec.Stopped(), "Executor should be stopped")

	view := model.View()
	assert.Contains(t, view, "BOOSTER FAILED", "Should show error summary")
	assert.Contains(t, view, "✓ task1", "Should show completed task")
	assert.Contains(t, view, "○ task2", "Should show skipped task")
	assert.Contains(t, view, "✗ task3", "Should show failed task")
}

func TestView_MultipleTasksWithDifferentStatuses(t *testing.T) {
	tasks := []task.Task{
		newMockTask("done task", task.StatusDone, "", nil),
		newMockTask("skipped task", task.StatusSkipped, "", nil),
		newMockTask("failed task", task.StatusFailed, "", errors.New("error")),
		newMockTask("running task", task.StatusDone, "", nil),
		newMockTask("pending task 1", task.StatusDone, "", nil),
		newMockTask("pending task 2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	_, _ = model.exec.RunNext(context.Background())
	_, _ = model.exec.RunNext(context.Background())
	_, _ = model.exec.RunNext(context.Background())

	view := model.View()

	lines := strings.Split(view, "\n")

	foundDone := false
	foundSkipped := false
	foundFailed := false
	foundRunning := false
	foundPending := false

	for _, line := range lines {
		if strings.Contains(line, "done task") && strings.Contains(line, "✓") {
			foundDone = true
		}
		if strings.Contains(line, "skipped task") && strings.Contains(line, "○") {
			foundSkipped = true
		}
		if strings.Contains(line, "failed task") && strings.Contains(line, "✗") {
			foundFailed = true
		}
		if strings.Contains(line, "running task") && strings.Contains(line, "→") {
			foundRunning = true
		}
		if strings.Contains(line, "pending task") && strings.Contains(line, "  pending") {
			foundPending = true
		}
	}

	assert.True(t, foundDone, "Should find done task with checkmark")
	assert.True(t, foundSkipped, "Should find skipped task with circle")
	assert.True(t, foundFailed, "Should find failed task with X")
	assert.True(t, foundRunning, "Should find running task with arrow")
	assert.True(t, foundPending, "Should find pending task with spaces")
}

type streamingMockTask struct {
	name   string
	lines  []string
	result task.Result
}

func (m *streamingMockTask) Name() string    { return m.name }
func (m *streamingMockTask) NeedsSudo() bool { return false }
func (m *streamingMockTask) Run(ctx context.Context) task.Result {
	if w := logstream.Writer(ctx); w != nil {
		for _, line := range m.lines {
			w.Write([]byte(line + "\n"))
		}
	}
	return m.result
}

func TestLogStreaming_Integration(t *testing.T) {
	streamTask := &streamingMockTask{
		name:   "streaming task",
		lines:  []string{"line1", "line2", "line3"},
		result: task.Result{Status: task.StatusDone},
	}

	model := New([]task.Task{streamTask})

	logWriter, logCh, cmd := model.startTask()
	require.NotNil(t, logWriter, "logWriter should be returned")
	require.NotNil(t, logCh, "logCh should be returned")
	require.NotNil(t, cmd, "startTask should return a command")

	taskCmd := runTask(model.exec, logWriter)
	taskMsg := taskCmd()

	var receivedLines []string
	for line := range logCh {
		receivedLines = append(receivedLines, line)
	}

	assert.Equal(t, []string{"line1", "line2", "line3"}, receivedLines,
		"Should receive all streamed log lines in order")

	doneMsg, ok := taskMsg.(taskDoneMsg)
	require.True(t, ok, "Should return taskDoneMsg")
	assert.Equal(t, task.StatusDone, doneMsg.result.Status, "Task should complete successfully")
}

func TestLogStreaming_IntegrationWithModelUpdate(t *testing.T) {
	streamTask := &streamingMockTask{
		name:   "streaming task",
		lines:  []string{"log line 1", "log line 2"},
		result: task.Result{Status: task.StatusDone},
	}

	model := New([]task.Task{streamTask})

	result, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "Task should run")
	require.Equal(t, task.StatusDone, result.Status)

	logWriter, logCh, cmd := model.startTask()
	model.logWriter = logWriter
	model.logCh = logCh
	require.NotNil(t, cmd, "startTask should return a command")

	newModel, cmd := model.Update(logLineMsg{line: "log line 1"})
	model, ok2 := newModel.(Model)
	require.True(t, ok2, "newModel should be Model type")
	assert.Len(t, model.coord.CurrentLogs(), 1, "Should have 1 log line")
	assert.Equal(t, "log line 1", model.coord.CurrentLogs()[0], "Should contain first log line")
	assert.NotNil(t, cmd, "Should return listenForLogs command")

	newModel, cmd = model.Update(logLineMsg{line: "log line 2"})
	model, ok2 = newModel.(Model)
	require.True(t, ok2, "newModel should be Model type")
	assert.Len(t, model.coord.CurrentLogs(), 2, "Should have 2 log lines")
	assert.Equal(t, "log line 1", model.coord.CurrentLogs()[0], "Should contain first log line")
	assert.Equal(t, "log line 2", model.coord.CurrentLogs()[1], "Should contain second log line")
	assert.NotNil(t, cmd, "Should return listenForLogs command")

	newModel, cmd = model.Update(logDoneMsg{})
	model, ok2 = newModel.(Model)
	require.True(t, ok2, "newModel should be Model type")
	assert.Nil(t, cmd, "Should return nil command for logDoneMsg")

	newModel, _ = model.Update(taskDoneMsg{result: result})
	model, ok2 = newModel.(Model)
	require.True(t, ok2, "newModel should be Model type")
	assert.Empty(t, model.coord.CurrentLogs(), "Current log lines should be cleared after task completion")
	assert.Len(t, model.coord.LogsFor(0), 2, "Log lines should be moved to history")
}

func TestLogStreaming_MaxLinesLimit(t *testing.T) {
	model := New([]task.Task{newMockTask("task", task.StatusDone, "", nil)})

	for i := range maxLogLines + 5 {
		msg := logLineMsg{line: fmt.Sprintf("line %d", i)}
		newModel, _ := model.Update(msg)
		model = newModel.(Model)
	}

	assert.Len(t, model.coord.CurrentLogs(), maxLogLines+5, "currentLogs should contain all lines")

	view := model.View()

	assert.Contains(t, view, "line 12", "View should show most recent line")
	assert.Contains(t, view, "line 5", "View should show oldest of recent lines")
	assert.NotContains(t, view, "line 0", "View should not show old lines")
}

func TestLogHistory_Persistence(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	model := New(tasks)

	_, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "Task1 should run")
	model.coord.StartTask(0)

	newModel, _ := model.Update(logLineMsg{line: "task1 log line 1"})
	model = newModel.(Model)
	newModel, _ = model.Update(logLineMsg{line: "task1 log line 2"})
	model = newModel.(Model)

	assert.Len(t, model.coord.CurrentLogs(), 2, "currentLogs should have 2 lines")
	assert.Equal(t, "task1 log line 1", model.coord.CurrentLogs()[0])
	assert.Equal(t, "task1 log line 2", model.coord.CurrentLogs()[1])

	model.coord.LogsDone()
	newModel, _ = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.Empty(t, model.coord.CurrentLogs(), "currentLogs should be cleared after task completion")
	assert.Len(t, model.coord.LogsFor(0), 2, "logHistory[0] should have 2 lines")
	assert.Equal(t, "task1 log line 1", model.coord.LogsFor(0)[0])
	assert.Equal(t, "task1 log line 2", model.coord.LogsFor(0)[1])

	_, ok = model.exec.RunNext(context.Background())
	require.True(t, ok, "Task2 should run")
	model.coord.StartTask(1)

	newModel, _ = model.Update(logLineMsg{line: "task2 log line 1"})
	model = newModel.(Model)
	newModel, _ = model.Update(logLineMsg{line: "task2 log line 2"})
	model = newModel.(Model)
	newModel, _ = model.Update(logLineMsg{line: "task2 log line 3"})
	model = newModel.(Model)

	model.coord.LogsDone()
	newModel, _ = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.Len(t, model.coord.LogsFor(0), 2, "logHistory[0] should still have 2 lines")
	assert.Len(t, model.coord.LogsFor(1), 3, "logHistory[1] should have 3 lines")
	assert.Equal(t, "task2 log line 1", model.coord.LogsFor(1)[0])
	assert.Equal(t, "task2 log line 2", model.coord.LogsFor(1)[1])
	assert.Equal(t, "task2 log line 3", model.coord.LogsFor(1)[2])

	_, ok = model.exec.RunNext(context.Background())
	require.True(t, ok, "Task3 should run")
	model.coord.StartTask(2)

	model.coord.LogsDone()
	newModel, _ = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.Nil(t, model.coord.LogsFor(2), "logHistory[2] should be nil for task with no logs")

	assert.Len(t, model.coord.LogsFor(0), 2, "logHistory[0] should still have 2 lines")
	assert.Len(t, model.coord.LogsFor(1), 3, "logHistory[1] should still have 3 lines")
}

func TestSelectedTask_AutoAdvancement(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	model := New(tasks)

	assert.Equal(t, 0, model.selectedTask, "Initial selectedTask should be 0")

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(0)
	model.coord.LogsDone()
	newModel, _ := model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.Equal(t, 1, model.selectedTask, "selectedTask should advance to 1")

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(1)
	model.coord.LogsDone()
	newModel, _ = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.Equal(t, 2, model.selectedTask, "selectedTask should advance to 2")

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(2)
	model.coord.LogsDone()
	newModel, _ = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.Equal(t, 2, model.selectedTask, "selectedTask should not advance beyond last task")
}

func TestLogHistory_FailedTask(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusFailed, "", errors.New("failure")),
	}
	model := New(tasks)

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(0)
	newModel, _ := model.Update(logLineMsg{line: "task1 log"})
	model = newModel.(Model)
	model.coord.LogsDone()
	newModel, _ = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(1)
	newModel, _ = model.Update(logLineMsg{line: "task2 log 1"})
	model = newModel.(Model)
	newModel, _ = model.Update(logLineMsg{line: "task2 log 2"})
	model = newModel.(Model)
	model.coord.LogsDone()
	newModel, _ = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusFailed}})
	model = newModel.(Model)

	assert.Len(t, model.coord.LogsFor(0), 1, "logHistory[0] should have 1 line")
	assert.Equal(t, "task1 log", model.coord.LogsFor(0)[0])

	assert.Len(t, model.coord.LogsFor(1), 2, "logHistory[1] should have 2 lines")
	assert.Equal(t, "task2 log 1", model.coord.LogsFor(1)[0])
	assert.Equal(t, "task2 log 2", model.coord.LogsFor(1)[1])

	assert.True(t, model.exec.Stopped(), "Executor should be stopped after failure")
}

func TestLogTaskCoordination_TaskDoneBeforeLogDone(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(0)

	newModel, _ := model.Update(logLineMsg{line: "log line 1"})
	model = newModel.(Model)
	newModel, _ = model.Update(logLineMsg{line: "log line 2"})
	model = newModel.(Model)

	newModel, cmd := model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.Nil(t, cmd, "Should not return command until logs are done")
	assert.Len(t, model.coord.CurrentLogs(), 2, "currentLogs should still have lines")

	newModel, _ = model.Update(logLineMsg{line: "log line 3"})
	model = newModel.(Model)
	assert.Len(t, model.coord.CurrentLogs(), 3, "currentLogs should have 3 lines now")

	newModel, cmd = model.Update(logDoneMsg{})
	model = newModel.(Model)

	assert.NotNil(t, cmd, "Should return command to start next task")
	assert.Empty(t, model.coord.CurrentLogs(), "currentLogs should be cleared")
	assert.Len(t, model.coord.LogsFor(0), 3, "All 3 log lines should be in history")
	assert.Equal(t, "log line 1", model.coord.LogsFor(0)[0])
	assert.Equal(t, "log line 2", model.coord.LogsFor(0)[1])
	assert.Equal(t, "log line 3", model.coord.LogsFor(0)[2])
}

func TestLogTaskCoordination_LogDoneBeforeTaskDone(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(0)

	newModel, _ := model.Update(logLineMsg{line: "log line 1"})
	model = newModel.(Model)

	newModel, cmd := model.Update(logDoneMsg{})
	model = newModel.(Model)

	assert.Nil(t, cmd, "No command from logDoneMsg alone")

	newModel, cmd = model.Update(taskDoneMsg{result: task.Result{Status: task.StatusDone}})
	model = newModel.(Model)

	assert.NotNil(t, cmd, "Should return command to start next task")
	assert.Empty(t, model.coord.CurrentLogs(), "currentLogs should be cleared")
	assert.Len(t, model.coord.LogsFor(0), 1, "Log should be in history")
}

func TestFocusMode_TabKeyTogglesFocus(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)

	assert.Equal(t, FocusTaskList, model.focusedPanel, "Initial focus should be on TaskList")

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	model = newModel.(Model)
	assert.Nil(t, cmd, "Tab should not return a command")
	assert.Equal(t, FocusLogs, model.focusedPanel, "Focus should switch to Logs")

	newModel, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	model = newModel.(Model)
	assert.Nil(t, cmd, "Tab should not return a command")
	assert.Equal(t, FocusTaskList, model.focusedPanel, "Focus should switch back to TaskList")
}

func TestFocusMode_JKNavigationTaskList(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)
	model.focusedPanel = FocusTaskList

	assert.Equal(t, 0, model.selectedTask, "Initial selectedTask should be 0")

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newModel.(Model)
	assert.Equal(t, 1, model.selectedTask, "selectedTask should be 1")

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newModel.(Model)
	assert.Equal(t, 2, model.selectedTask, "selectedTask should be 2")

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newModel.(Model)
	assert.Equal(t, 2, model.selectedTask, "selectedTask should stay at 2 (max)")

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = newModel.(Model)
	assert.Equal(t, 1, model.selectedTask, "selectedTask should be 1")

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = newModel.(Model)
	assert.Equal(t, 0, model.selectedTask, "selectedTask should be 0")

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = newModel.(Model)
	assert.Equal(t, 0, model.selectedTask, "selectedTask should stay at 0 (min)")
}

func TestFocusMode_ArrowKeyNavigationTaskList(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)
	model.focusedPanel = FocusTaskList

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = newModel.(Model)
	assert.Equal(t, 1, model.selectedTask, "selectedTask should be 1")

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = newModel.(Model)
	assert.Equal(t, 0, model.selectedTask, "selectedTask should be 0")
}

func TestFocusMode_JKScrollsLogsWhenFocused(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)
	model.focusedPanel = FocusLogs

	model.logViewport = viewport.New(model.layout.RightWidth-panelBorderWidth, model.layout.Height-logPanelOverhead)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("log line %d", i)
	}
	model.logViewport.SetContent(strings.Join(lines, "\n"))

	initialY := model.logViewport.YOffset

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newModel.(Model)
	assert.Greater(t, model.logViewport.YOffset, initialY, "YOffset should increase after pressing j")

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = newModel.(Model)
	assert.Equal(t, initialY, model.logViewport.YOffset, "YOffset should return to initial after pressing k")
}

func TestFocusMode_GKeyJumpsToBottom(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)
	model.focusedPanel = FocusLogs

	model.logViewport = viewport.New(model.layout.RightWidth-panelBorderWidth, model.layout.Height-logPanelOverhead)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("log line %d", i)
	}
	model.logViewport.SetContent(strings.Join(lines, "\n"))

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model = newModel.(Model)
	assert.True(t, model.logViewport.AtBottom(), "Viewport should be at bottom after pressing G")
}

func TestFocusMode_GKeyIgnoredWhenTaskListFocused(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)
	model.focusedPanel = FocusTaskList

	model.logViewport = viewport.New(model.layout.RightWidth-panelBorderWidth, model.layout.Height-logPanelOverhead)
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("log line %d", i)
	}
	model.logViewport.SetContent(strings.Join(lines, "\n"))

	initialY := model.logViewport.YOffset

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model = newModel.(Model)
	assert.Equal(t, initialY, model.logViewport.YOffset, "YOffset should not change when G pressed with task list focused")
}

func TestFocusMode_NavigationBounds(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)
	model.focusedPanel = FocusTaskList

	model.selectedTask = 0
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = newModel.(Model)
	assert.Equal(t, 0, model.selectedTask, "selectedTask should not go below 0")

	model.selectedTask = 2
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = newModel.(Model)
	assert.Equal(t, 2, model.selectedTask, "selectedTask should not exceed task count - 1")
}

func TestShowLogs_DefaultTrue(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	assert.True(t, model.showLogs, "showLogs should default to true")
}

func TestShowLogs_ToggleWithOKey(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = newModel.(Model)

	assert.True(t, model.showLogs, "showLogs should default to true")

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = newModel.(Model)
	assert.Nil(t, cmd, "Should not return a command")
	assert.False(t, model.showLogs, "showLogs should be toggled to false")

	newModel, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = newModel.(Model)
	assert.Nil(t, cmd, "Should not return a command")
	assert.True(t, model.showLogs, "showLogs should be toggled back to true")
}

func TestShowLogs_PanelShowsPlaceholderWhenEmpty(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = newModel.(Model)

	model.logViewport = viewport.New(model.layout.RightWidth-panelBorderWidth, model.layout.Height-logPanelOverhead)

	assert.Empty(t, model.coord.CurrentLogs(), "currentLogs should be empty")
	assert.Nil(t, model.coord.LogsFor(0), "logHistory should be empty")

	view := model.View()

	assert.Contains(t, view, "task1", "View should contain task name")
	assert.Contains(t, view, "Waiting for output...", "View should contain waiting message")
}

func TestShowLogs_DisplaysHistoryWhenStopped(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	m, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = m.(Model)

	model.logViewport = viewport.New(model.layout.RightWidth-panelBorderWidth, model.layout.Height-logPanelOverhead)

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(0)
	model.coord.AddLogLine("task1 log line 1")
	model.coord.AddLogLine("task1 log line 2")
	model.coord.LogsDone()
	model.coord.TaskDone(task.Result{Status: task.StatusDone})

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(1)
	model.coord.AddLogLine("task2 log line 1")
	model.coord.AddLogLine("task2 log line 2")
	model.coord.AddLogLine("task2 log line 3")
	model.coord.LogsDone()
	model.coord.TaskDone(task.Result{Status: task.StatusDone})

	model.selectedTask = 0

	logs := model.getDisplayLogs()
	assert.Len(t, logs, 2, "Should return task1 logs")
	assert.Equal(t, "task1 log line 1", logs[0])
	assert.Equal(t, "task1 log line 2", logs[1])

	model.selectedTask = 1

	logs = model.getDisplayLogs()
	assert.Len(t, logs, 3, "Should return task2 logs")
	assert.Equal(t, "task2 log line 1", logs[0])
	assert.Equal(t, "task2 log line 2", logs[1])
	assert.Equal(t, "task2 log line 3", logs[2])
}

func TestShowLogs_AutoscrollStickToBottom(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = newModel.(Model)

	model.logViewport = viewport.New(model.layout.RightWidth-panelBorderWidth, model.layout.Height-logPanelOverhead)

	for i := range 50 {
		msg := logLineMsg{line: fmt.Sprintf("log line %d", i)}
		newModel, _ := model.Update(msg)
		model = newModel.(Model)
	}

	assert.True(t, model.logViewport.AtBottom(), "Viewport should be at bottom after autoscroll")

	model.logViewport.ScrollUp(5)
	assert.False(t, model.logViewport.AtBottom(), "Viewport should not be at bottom after scrolling up")

	beforeY := model.logViewport.YOffset
	msg := logLineMsg{line: "new log line"}
	newModel, _ = model.Update(msg)
	model = newModel.(Model)

	assert.InDelta(t, beforeY, model.logViewport.YOffset, 1.0, "YOffset should not change significantly")
	assert.False(t, model.logViewport.AtBottom(), "Viewport should not auto-scroll when user scrolled up")

	model.logViewport.GotoBottom()
	assert.True(t, model.logViewport.AtBottom(), "Viewport should be at bottom after GotoBottom")

	msg = logLineMsg{line: "another log line"}
	newModel, _ = model.Update(msg)
	model = newModel.(Model)

	assert.True(t, model.logViewport.AtBottom(), "Viewport should autoscroll when at bottom")
}

func TestShowLogs_GetDisplayLogs(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	model := New(tasks)

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(0)
	model.coord.AddLogLine("task1 log")
	model.coord.LogsDone()
	model.coord.TaskDone(task.Result{Status: task.StatusDone})

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(1)
	model.coord.AddLogLine("task2 log")
	model.coord.LogsDone()
	model.coord.TaskDone(task.Result{Status: task.StatusDone})

	_, _ = model.exec.RunNext(context.Background())
	model.coord.StartTask(2)

	model.coord.LogsDone()
	model.coord.TaskDone(task.Result{Status: task.StatusDone})

	model.selectedTask = 0
	logs := model.getDisplayLogs()
	assert.Len(t, logs, 1, "Should return task1 logs")
	assert.Equal(t, "task1 log", logs[0])

	model.selectedTask = 1
	logs = model.getDisplayLogs()
	assert.Len(t, logs, 1, "Should return task2 logs")
	assert.Equal(t, "task2 log", logs[0])

	model.selectedTask = 2
	logs = model.getDisplayLogs()
	assert.Nil(t, logs, "Should return nil for task with no logs")
}

func TestAppContainer_RendersWithBorder(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	model.width = 80
	model.height = 40

	view := model.View()

	assert.Contains(t, view, "╭", "View should contain top-left border corner")

	assert.Contains(t, view, "╰", "View should contain bottom-left border corner")

	assert.Contains(t, view, "╮", "View should contain top-right border corner")

	assert.Contains(t, view, "╯", "View should contain bottom-right border corner")
}

func TestAppContainer_HandlesSmallTerminal(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	model.width = 20
	model.height = 10

	view := model.View()

	assert.NotEmpty(t, view, "View should not be empty even with small dimensions")

	assert.Contains(t, view, "╭", "View should contain border even with small dimensions")
}

func TestAppContainer_HandlesZeroDimensions(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	model.width = 0
	model.height = 0

	view := model.View()

	assert.NotEmpty(t, view, "View should not be empty even with zero dimensions")
}

func TestAppContainer_TwoColumnMode(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	model.width = 120
	model.height = 40

	view := model.View()

	assert.Contains(t, view, "╭", "View should contain border in two-column mode")
	assert.Contains(t, view, "╯", "View should contain border in two-column mode")

	assert.Contains(t, view, "BOOSTER", "View should contain title in two-column mode")
}

func TestAppContainer_HelpBarInsideContainer(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model = newModel.(Model)

	_, _ = model.exec.RunNext(context.Background())

	view := model.View()

	assert.Contains(t, view, "╭", "View should contain border")
	assert.Contains(t, view, "╯", "View should contain border")

	assert.Contains(t, view, "enter exit", "View should contain help text")

	lines := strings.Split(view, "\n")
	assert.Greater(t, len(lines), 2, "View should have multiple lines")

	assert.Contains(t, lines[0], "╭", "First line should contain top border")

	lastLine := lines[len(lines)-1]
	assert.Contains(t, lastLine, "╯", "Last line should contain bottom border")
}
