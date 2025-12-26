package tui

import (
	"booster/internal/executor"
	"booster/internal/task"
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskListModel_New(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)

	model := NewTaskList(exec)

	assert.Equal(t, 0, model.Selected(), "initial selection should be 0")
}

func TestTaskListModel_SetSelectionMsg_ValidRange(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	cmd := model.Update(SetSelectionMsg{Index: 1})
	require.NotNil(t, cmd, "should return command for TaskSelectedMsg")
	msg := cmd()
	selectedMsg, ok := msg.(TaskSelectedMsg)
	require.True(t, ok, "command should produce TaskSelectedMsg")
	assert.Equal(t, 1, selectedMsg.Index)
	assert.Equal(t, 1, model.Selected())

	cmd = model.Update(SetSelectionMsg{Index: 2})
	require.NotNil(t, cmd)
	msg = cmd()
	selectedMsg, ok = msg.(TaskSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 2, selectedMsg.Index)
	assert.Equal(t, 2, model.Selected())

	cmd = model.Update(SetSelectionMsg{Index: 0})
	require.NotNil(t, cmd)
	msg = cmd()
	selectedMsg, ok = msg.(TaskSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 0, selectedMsg.Index)
	assert.Equal(t, 0, model.Selected())
}

func TestTaskListModel_SetSelectionMsg_OutOfBounds(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	model.Update(SetSelectionMsg{Index: 0})

	cmd := model.Update(SetSelectionMsg{Index: -1})
	assert.Nil(t, cmd, "negative index should not emit command")
	assert.Equal(t, 0, model.Selected(), "negative index should be ignored")

	cmd = model.Update(SetSelectionMsg{Index: 2})
	assert.Nil(t, cmd, "index >= total should not emit command")
	assert.Equal(t, 0, model.Selected(), "index >= total should be ignored")

	cmd = model.Update(SetSelectionMsg{Index: 100})
	assert.Nil(t, cmd, "large index should not emit command")
	assert.Equal(t, 0, model.Selected(), "large index should be ignored")
}

func TestTaskListModel_Update_NavigateDown(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	require.NotNil(t, cmd, "should return command for TaskSelectedMsg")

	msg := cmd()
	selectedMsg, ok := msg.(TaskSelectedMsg)
	require.True(t, ok, "command should produce TaskSelectedMsg")
	assert.Equal(t, 1, selectedMsg.Index)

	cmd = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.NotNil(t, cmd)

	msg = cmd()
	selectedMsg, ok = msg.(TaskSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 2, selectedMsg.Index)
}

func TestTaskListModel_Update_NavigateUp(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)
	model.Update(SetSelectionMsg{Index: 2})

	cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	require.NotNil(t, cmd)

	msg := cmd()
	selectedMsg, ok := msg.(TaskSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, selectedMsg.Index)

	cmd = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	require.NotNil(t, cmd)

	msg = cmd()
	selectedMsg, ok = msg.(TaskSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 0, selectedMsg.Index)
}

func TestTaskListModel_Update_BoundsEnforced(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Nil(t, cmd, "should not emit command when already at top")
	assert.Equal(t, 0, model.Selected())

	model.Update(SetSelectionMsg{Index: 1})

	cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Nil(t, cmd, "should not emit command when already at bottom")
	assert.Equal(t, 1, model.Selected())
}

func TestTaskListModel_Update_UnknownKeyIgnored(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	assert.Nil(t, cmd, "unknown key should not produce command")
}

func TestTaskListModel_View_ShowsSelectionIndicator(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)

	exec.RunNext(context.Background())
	exec.RunNext(context.Background())

	model := NewTaskList(exec)
	model.SetSize(40, 10)

	view := model.View()

	assert.Contains(t, view, SelectionIndicator, "view should contain selection indicator")

	lines := strings.Split(view, "\n")
	var selectedLine string
	for _, line := range lines {
		if strings.Contains(line, SelectionIndicator) {
			selectedLine = line
			break
		}
	}
	require.NotEmpty(t, selectedLine, "should find line with selection indicator")
	assert.Contains(t, selectedLine, "task1", "selection should be on task1")
}

func TestTaskListModel_View_SelectionMoves(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	exec.RunNext(context.Background())
	exec.RunNext(context.Background())

	model := NewTaskList(exec)
	model.SetSize(40, 10)

	view := model.View()
	assert.True(t, selectionIsOnTask(view, "task1"), "selection should start on task1")

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	view = model.View()
	assert.True(t, selectionIsOnTask(view, "task2"), "selection should move to task2")
}

func TestTaskListModel_View_TaskStatuses(t *testing.T) {
	tasks := []task.Task{
		newMockTask("done_task", task.StatusDone, "", nil),
		newMockTask("skipped_task", task.StatusSkipped, "", nil),
		newMockTask("pending_task", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)

	exec.RunNext(context.Background())
	exec.RunNext(context.Background())

	model := NewTaskList(exec)
	model.SetSize(60, 15)

	view := model.View()

	assert.Contains(t, view, "✓", "done task should have checkmark")

	for line := range strings.SplitSeq(view, "\n") {
		if strings.Contains(line, "done_task") {
			assert.Contains(t, line, "✓", "done_task line should have checkmark")
		}
	}
}

func TestTaskListModel_SpinnerTick(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)

	cmd := model.SpinnerTick()
	assert.NotNil(t, cmd, "SpinnerTick should return a command")
}

func TestTaskListModel_SpinnerTick_DoesNotEmitTaskSelectedMsg(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	cmd := model.Update(spinnerTickMsg{})
	assert.Nil(t, cmd, "spinnerTickMsg should not emit any command")
}

func TestTaskListModel_AdvanceSelectionMsg_StopsAtLastTask(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)
	model.Update(SetSelectionMsg{Index: 1})

	cmd := model.Update(AdvanceSelectionMsg{})
	assert.Nil(t, cmd, "should not emit command when already at last task")
	assert.Equal(t, 1, model.Selected(), "should not advance beyond last task")
}

func TestTaskListModel_AdvanceSelectionMsg_Advances(t *testing.T) {
	tasks := []task.Task{
		newMockTask("task1", task.StatusDone, "", nil),
		newMockTask("task2", task.StatusDone, "", nil),
		newMockTask("task3", task.StatusDone, "", nil),
	}
	exec := executor.New(tasks)
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	cmd := model.Update(AdvanceSelectionMsg{})
	require.NotNil(t, cmd, "should return command for TaskSelectedMsg")
	msg := cmd()
	selectedMsg, ok := msg.(TaskSelectedMsg)
	require.True(t, ok, "command should produce TaskSelectedMsg")
	assert.Equal(t, 1, selectedMsg.Index)
	assert.Equal(t, 1, model.Selected())

	cmd = model.Update(AdvanceSelectionMsg{})
	require.NotNil(t, cmd)
	msg = cmd()
	selectedMsg, ok = msg.(TaskSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 2, selectedMsg.Index)
	assert.Equal(t, 2, model.Selected())

	cmd = model.Update(AdvanceSelectionMsg{})
	assert.Nil(t, cmd, "should not emit command at last task")
	assert.Equal(t, 2, model.Selected())
}

func TestTaskListModel_EmptyTaskList(t *testing.T) {
	exec := executor.New([]task.Task{})
	model := NewTaskList(exec)
	model.SetSize(40, 10)

	assert.Equal(t, 0, model.Selected(), "selected should be 0 for empty list")

	cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Nil(t, cmd, "navigating down on empty list should not panic or emit command")
	assert.Equal(t, 0, model.Selected())

	cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Nil(t, cmd, "navigating up on empty list should not panic or emit command")
	assert.Equal(t, 0, model.Selected())

	cmd = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Nil(t, cmd, "down arrow on empty list should not panic or emit command")
	assert.Equal(t, 0, model.Selected())

	cmd = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Nil(t, cmd, "up arrow on empty list should not panic or emit command")
	assert.Equal(t, 0, model.Selected())

	assert.NotPanics(t, func() {
		model.View()
	}, "View should not panic on empty list")

	assert.NotPanics(t, func() {
		model.Update(AdvanceSelectionMsg{})
	}, "AdvanceSelectionMsg should not panic on empty list")
	assert.Equal(t, 0, model.Selected())

	assert.NotPanics(t, func() {
		model.Update(SetSelectionMsg{Index: 0})
	}, "SetSelectionMsg{0} should not panic on empty list")
	assert.Equal(t, 0, model.Selected())

	assert.NotPanics(t, func() {
		model.Update(SetSelectionMsg{Index: -1})
	}, "SetSelectionMsg{-1} should not panic on empty list")
	assert.Equal(t, 0, model.Selected())
}

func selectionIsOnTask(view, taskName string) bool {
	lines := strings.SplitSeq(view, "\n")
	for line := range lines {
		if strings.Contains(line, SelectionIndicator) && strings.Contains(line, taskName) {
			return true
		}
	}
	return false
}
