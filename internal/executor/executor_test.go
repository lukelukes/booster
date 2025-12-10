package executor

import (
	"booster/internal/task"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTask is a test double for task.Task
type mockTask struct {
	result task.Result
	name   string
}

func (m *mockTask) Name() string                        { return m.name }
func (m *mockTask) Run(ctx context.Context) task.Result { return m.result }

func TestExecutor_New(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1"},
		&mockTask{name: "task2"},
	}

	exec := New(tasks)

	assert.Equal(t, 2, exec.Total())
	assert.Equal(t, 0, exec.Current())
	assert.False(t, exec.Done())
}

func TestExecutor_Empty(t *testing.T) {
	exec := New(nil)

	assert.Equal(t, 0, exec.Total())
	assert.True(t, exec.Done())
}

func TestExecutor_RunNext_ExecutesTasks(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "task2", result: task.Result{Status: task.StatusSkipped}},
	}
	exec := New(tasks)

	// Run first task
	result1, ok := exec.RunNext(context.Background())
	require.True(t, ok)
	assert.Equal(t, task.StatusDone, result1.Status)
	assert.Equal(t, 1, exec.Current())
	assert.False(t, exec.Done())

	// Run second task
	result2, ok := exec.RunNext(context.Background())
	require.True(t, ok)
	assert.Equal(t, task.StatusSkipped, result2.Status)
	assert.Equal(t, 2, exec.Current())
	assert.True(t, exec.Done())

	// No more tasks
	_, ok = exec.RunNext(context.Background())
	assert.False(t, ok)
}

func TestExecutor_Results(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone, Message: "created"}},
		&mockTask{name: "task2", result: task.Result{Status: task.StatusFailed, Error: errors.New("oops")}},
	}
	exec := New(tasks)

	exec.RunNext(context.Background())
	exec.RunNext(context.Background())

	results := exec.Results()
	require.Len(t, results, 2)
	assert.Equal(t, task.StatusDone, results[0].Status)
	assert.Equal(t, task.StatusFailed, results[1].Status)
}

func TestExecutor_Tasks(t *testing.T) {
	task1 := &mockTask{name: "task1"}
	task2 := &mockTask{name: "task2"}
	exec := New([]task.Task{task1, task2})

	tasks := exec.Tasks()
	require.Len(t, tasks, 2)
	assert.Equal(t, "task1", tasks[0].Name())
	assert.Equal(t, "task2", tasks[1].Name())
}

func TestExecutor_Summary(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "t1", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "t2", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "t3", result: task.Result{Status: task.StatusSkipped}},
		&mockTask{name: "t4", result: task.Result{Status: task.StatusFailed}},
	}
	exec := New(tasks)

	// Run all tasks
	for range tasks {
		exec.RunNext(context.Background())
	}

	summary := exec.Summary()
	assert.Equal(t, 2, summary.Done)
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, 1, summary.Failed)
	assert.True(t, summary.HasFailures)
}

func TestExecutor_Summary_NoFailures(t *testing.T) {
	// Tests boundary condition: Failed == 0 means HasFailures must be false
	// This kills mutation: s.Failed > 0 → s.Failed >= 0
	tasks := []task.Task{
		&mockTask{name: "t1", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "t2", result: task.Result{Status: task.StatusSkipped}},
	}
	exec := New(tasks)

	for range tasks {
		exec.RunNext(context.Background())
	}

	summary := exec.Summary()
	assert.Equal(t, 1, summary.Done)
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, 0, summary.Failed)
	assert.False(t, summary.HasFailures, "HasFailures must be false when Failed == 0")
}

func TestExecutor_ResultAt(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "task2", result: task.Result{Status: task.StatusSkipped}},
	}
	exec := New(tasks)

	// Before running, results are pending
	assert.Equal(t, task.StatusPending, exec.ResultAt(0).Status)
	assert.Equal(t, task.StatusPending, exec.ResultAt(1).Status)

	exec.RunNext(context.Background())
	assert.Equal(t, task.StatusDone, exec.ResultAt(0).Status)
	assert.Equal(t, task.StatusPending, exec.ResultAt(1).Status)
}

func TestExecutor_ResultAt_OutOfBounds(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}},
	}
	exec := New(tasks)

	// Negative index returns pending
	assert.Equal(t, task.StatusPending, exec.ResultAt(-1).Status)

	// Index beyond length returns pending
	assert.Equal(t, task.StatusPending, exec.ResultAt(100).Status)

	// Index at exactly length returns pending
	assert.Equal(t, task.StatusPending, exec.ResultAt(1).Status)
}

func TestExecutor_RunNext_OnEmptyExecutor(t *testing.T) {
	exec := New(nil)

	// Calling RunNext on empty executor returns false
	result, ok := exec.RunNext(context.Background())
	assert.False(t, ok)
	assert.Equal(t, task.Result{}, result)

	// Calling again still returns false
	_, ok = exec.RunNext(context.Background())
	assert.False(t, ok)
}
