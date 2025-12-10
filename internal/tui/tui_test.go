package tui

import (
	"booster/internal/executor"
	"booster/internal/task"
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTask is a simple task implementation for testing.
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

// newMockTask creates a mock task with the given name and result.
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

// TestNew verifies that New creates a model with correct initial state.
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

// TestInit_WithTasks verifies Init returns a command to run the first task.
func TestInit_WithTasks(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	cmd := model.Init()

	assert.NotNil(t, cmd, "Init should return a command when tasks exist")
}

// TestInit_EmptyTasks verifies Init returns nil when no tasks exist.
func TestInit_EmptyTasks(t *testing.T) {
	model := New([]task.Task{})

	cmd := model.Init()

	assert.Nil(t, cmd, "Init should return nil when no tasks exist")
}

// TestUpdate_KeyHandling verifies key handling behavior in various states.
func TestUpdate_KeyHandling(t *testing.T) {
	tests := []struct {
		name            string
		setupModel      func() Model
		keyMsg          tea.KeyMsg
		wantQuitCmd     bool
		wantShowOutput  *bool // nil means don't check
		wantToggleCount int   // for multiple toggle test
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

	// Test output toggle twice (separate test due to stateful nature)
	t.Run("o toggles output twice when done", func(t *testing.T) {
		exec := executor.New([]task.Task{})
		model := Model{exec: exec, showOutput: false}

		// First toggle to true
		newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		assert.Nil(t, cmd, "Should not return a command")
		updatedModel, ok := newModel.(Model)
		require.True(t, ok, "newModel should be Model type")
		assert.True(t, updatedModel.showOutput, "showOutput should be toggled to true")

		// Second toggle back to false
		newModel2, cmd2 := updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		assert.Nil(t, cmd2, "Should not return a command")
		updatedModel2, ok2 := newModel2.(Model)
		require.True(t, ok2, "newModel2 should be Model type")
		assert.False(t, updatedModel2.showOutput, "showOutput should be toggled back to false")
	})
}

// TestUpdate_TaskDoneMsg verifies taskDoneMsg triggers next task execution.
func TestUpdate_TaskDoneMsg(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	model := New(tasks)

	// Manually advance executor to simulate first task completion
	_, ok := model.exec.RunNext(context.Background())
	require.True(t, ok, "First task should run")

	// Send taskDoneMsg - should trigger next task
	msg := taskDoneMsg{result: task.Result{Status: task.StatusDone}}
	newModel, cmd := model.Update(msg)

	assert.NotNil(t, cmd, "Should return command to run next task")
	assert.IsType(t, Model{}, newModel, "Update should return Model type")

	// Execute the command and verify it returns a taskDoneMsg
	resultMsg := cmd()
	_, isTaskDone := resultMsg.(taskDoneMsg)
	assert.True(t, isTaskDone, "Command should return taskDoneMsg")
}

// TestUpdate_TaskDoneMsgWhenAllComplete verifies taskDoneMsg is ignored when all tasks are done.
func TestUpdate_TaskDoneMsgWhenAllComplete(t *testing.T) {
	// Create an executor that's already done
	exec := executor.New([]task.Task{})
	model := Model{exec: exec}

	msg := taskDoneMsg{result: task.Result{Status: task.StatusDone}}
	newModel, cmd := model.Update(msg)

	assert.Nil(t, cmd, "Should not return command when all tasks are complete")
	assert.IsType(t, Model{}, newModel, "Update should return Model type")
}

// TestUpdate_UnknownMessageIgnored verifies unknown messages don't cause issues.
func TestUpdate_UnknownMessageIgnored(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	// Send unknown message type
	type unknownMsg struct{}
	newModel, cmd := model.Update(unknownMsg{})

	assert.Nil(t, cmd, "Should return nil for unknown messages")
	assert.IsType(t, Model{}, newModel, "Update should return Model type")
}

// TestView_ContainsTitle verifies the view contains the title.
func TestView_ContainsTitle(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	model := New(tasks)

	view := model.View()

	assert.Contains(t, view, "BOOSTER", "View should contain title")
}

// TestView_TaskStatus verifies tasks are displayed correctly based on their status.
func TestView_TaskStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupModel    func() Model
		executeCount  int // number of tasks to execute before checking view
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
			executeCount:  1, // Execute first task so second is running and third is pending
			checkTaskName: "pending task",
			wantContains: []string{
				"pending task",
				"  pending task", // Pending tasks have leading spaces
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
			executeCount:  0, // First task should show as running
			checkTaskName: "running task",
			wantContains: []string{
				"running task...",
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
			name: "skipped task",
			setupModel: func() Model {
				tasks := []task.Task{
					newMockTask("skipped task", task.StatusSkipped, "", nil),
				}
				return New(tasks)
			},
			executeCount:  1,
			checkTaskName: "skipped task",
			wantHelper: func(t *testing.T, view string) {
				AssertTaskStatus(t, view, "skipped task", task.StatusSkipped)
				AssertSkippedReason(t, view, "(exists)")
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
				"unknown error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.setupModel()

			// Execute tasks as needed
			for i := 0; i < tt.executeCount; i++ {
				_, ok := model.exec.RunNext(context.Background())
				require.True(t, ok, "Task %d should run", i+1)
			}

			view := model.View()

			// Check string contains
			for _, want := range tt.wantContains {
				assert.Contains(t, view, want, "View should contain: %s", want)
			}

			// Use helper assertions if provided
			if tt.wantHelper != nil {
				tt.wantHelper(t, view)
			}
		})
	}
}

// viewTestCase defines test cases for view rendering tests.
type viewTestCase struct {
	name            string
	tasks           []task.Task
	showOutput      bool
	wantContains    []string
	wantNotContains []string
}

// runViewTests is a test helper that executes view rendering test cases.
// It creates a model, executes all tasks, sets showOutput, and asserts the view contains/doesn't contain expected strings.
func runViewTests(t *testing.T, tests []viewTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(tt.tasks)

			// Execute all tasks
			for range tt.tasks {
				_, _ = model.exec.RunNext(context.Background())
			}

			model.showOutput = tt.showOutput

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

// TestView_Summary verifies summary is displayed correctly based on task outcomes.
func TestView_Summary(t *testing.T) {
	tests := []viewTestCase{
		{
			name: "success summary without failures",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "", nil),
				newMockTask("task2", task.StatusSkipped, "", nil),
			},
			wantContains: []string{
				"Done!",
				"1 completed",
				"1 skipped",
			},
		},
		{
			name: "error summary with failures",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "", nil),
				newMockTask("task2", task.StatusFailed, "", errors.New("error")),
			},
			wantContains: []string{
				"Finished with errors",
				"1 done",
				"1 failed",
			},
		},
	}

	runViewTests(t, tests)
}

// TestView_HelpText verifies help text is displayed correctly in different scenarios.
func TestView_HelpText(t *testing.T) {
	tests := []viewTestCase{
		{
			name: "without output",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "", nil),
			},
			wantContains: []string{
				"Press Enter to exit",
			},
			wantNotContains: []string{
				"Press 'o'",
			},
		},
		{
			name: "with output hidden",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "some output", nil),
			},
			showOutput: false,
			wantContains: []string{
				"Press 'o' to view output",
			},
		},
		{
			name: "with output visible",
			tasks: []task.Task{
				newMockTask("task1", task.StatusDone, "some output", nil),
			},
			showOutput: true,
			wantContains: []string{
				"Press 'o' to hide output",
			},
		},
	}

	runViewTests(t, tests)
}

// TestView_OutputSection verifies output section behavior in different scenarios.
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

// TestHasTaskOutput verifies hasTaskOutput correctly detects task output.
func TestHasTaskOutput(t *testing.T) {
	t.Run("returns false when no tasks have output", func(t *testing.T) {
		tasks := []task.Task{
			newMockTask("task1", task.StatusDone, "", nil),
			newMockTask("task2", task.StatusDone, "", nil),
		}
		model := New(tasks)

		// Execute all tasks
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

		// Execute all tasks
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

		// Don't execute tasks - all pending

		hasOutput := model.hasTaskOutput()
		assert.False(t, hasOutput, "Should return false when tasks haven't run yet")
	})
}

// TestRunNext verifies runNext creates a command that executes the next task.
func TestRunNext(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "output", nil),
	}
	model := New(tasks)

	cmd := model.runNext()
	require.NotNil(t, cmd, "runNext should return a command")

	// Execute the command
	msg := cmd()

	// Verify it returns a taskDoneMsg
	taskMsg, ok := msg.(taskDoneMsg)
	require.True(t, ok, "Command should return taskDoneMsg")

	// Verify the result has the expected status
	assert.Equal(t, task.StatusDone, taskMsg.result.Status, "Result should have done status")
}

// TestIntegration_FullTaskFlow verifies the complete task execution flow.
func TestIntegration_FullTaskFlow(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "output1", nil),
		newMockTask("task2", task.StatusSkipped, "output2", nil),
		newMockTask("task3", task.StatusFailed, "", errors.New("failure")),
	}
	model := New(tasks)

	// Initialize - should start first task
	cmd := model.Init()
	require.NotNil(t, cmd, "Init should return command")

	// Execute first task
	msg := cmd()
	taskMsg, ok := msg.(taskDoneMsg)
	require.True(t, ok, "Should return taskDoneMsg")
	assert.Equal(t, task.StatusDone, taskMsg.result.Status)

	// Process first completion - should trigger second task
	newModel, cmd := model.Update(taskMsg)
	model, ok = newModel.(Model)
	require.True(t, ok, "newModel should be Model type")
	require.NotNil(t, cmd, "Should return command for second task")

	// Execute second task
	msg = cmd()
	taskMsg, ok = msg.(taskDoneMsg)
	require.True(t, ok, "Should return taskDoneMsg")
	assert.Equal(t, task.StatusSkipped, taskMsg.result.Status)

	// Process second completion - should trigger third task
	newModel, cmd = model.Update(taskMsg)
	model, ok = newModel.(Model)
	require.True(t, ok, "newModel should be Model type")
	require.NotNil(t, cmd, "Should return command for third task")

	// Execute third task
	msg = cmd()
	taskMsg, ok = msg.(taskDoneMsg)
	require.True(t, ok, "Should return taskDoneMsg")
	assert.Equal(t, task.StatusFailed, taskMsg.result.Status)

	// Process third completion - should complete
	newModel, cmd = model.Update(taskMsg)
	model, ok = newModel.(Model)
	require.True(t, ok, "newModel should be Model type")
	assert.Nil(t, cmd, "Should return nil when all tasks complete")

	// Verify final view
	view := model.View()
	assert.Contains(t, view, "Finished with errors", "Should show error summary")
	assert.Contains(t, view, "✓ task1", "Should show completed task")
	assert.Contains(t, view, "○ task2", "Should show skipped task")
	assert.Contains(t, view, "✗ task3", "Should show failed task")
}

// TestView_MultipleTasksWithDifferentStatuses verifies complex scenarios are rendered correctly.
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

	// Execute first three tasks
	_, _ = model.exec.RunNext(context.Background())
	_, _ = model.exec.RunNext(context.Background())
	_, _ = model.exec.RunNext(context.Background())
	// Current is now at index 3, which is "running"

	view := model.View()

	// Verify each task appears with correct indicator
	lines := strings.Split(view, "\n")

	// Find lines containing task names and verify indicators
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
