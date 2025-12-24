package executor

import (
	"booster/internal/task"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTask struct {
	result    task.Result
	name      string
	sleepTime time.Duration
}

func (m *mockTask) Name() string { return m.name }
func (m *mockTask) Run(ctx context.Context) task.Result {
	if m.sleepTime > 0 {
		time.Sleep(m.sleepTime)
	}
	return m.result
}
func (m *mockTask) NeedsSudo() bool { return false }

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

	result1, ok := exec.RunNext(context.Background())
	require.True(t, ok)
	assert.Equal(t, task.StatusDone, result1.Status)
	assert.Equal(t, 1, exec.Current())
	assert.False(t, exec.Done())

	result2, ok := exec.RunNext(context.Background())
	require.True(t, ok)
	assert.Equal(t, task.StatusSkipped, result2.Status)
	assert.Equal(t, 2, exec.Current())
	assert.True(t, exec.Done())

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

	assert.Equal(t, task.StatusPending, exec.ResultAt(-1).Status)

	assert.Equal(t, task.StatusPending, exec.ResultAt(100).Status)

	assert.Equal(t, task.StatusPending, exec.ResultAt(1).Status)
}

func TestExecutor_RunNext_OnEmptyExecutor(t *testing.T) {
	exec := New(nil)

	result, ok := exec.RunNext(context.Background())
	assert.False(t, ok)
	assert.Equal(t, task.Result{}, result)

	_, ok = exec.RunNext(context.Background())
	assert.False(t, ok)
}

func TestExecutor_Abort_StopsExecution(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "task2", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "task3", result: task.Result{Status: task.StatusDone}},
	}
	exec := New(tasks)

	result1, ok := exec.RunNext(context.Background())
	require.True(t, ok)
	assert.Equal(t, task.StatusDone, result1.Status)

	exec.Abort()

	_, ok = exec.RunNext(context.Background())
	assert.False(t, ok, "RunNext should return false after Abort")

	assert.True(t, exec.Stopped())

	// Done should still be false (not all tasks ran)
	assert.False(t, exec.Done())

	// Current should still be 1 (only one task ran)
	assert.Equal(t, 1, exec.Current())
}

func TestExecutor_Stopped_WhenNotAborted(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}},
	}
	exec := New(tasks)

	// Initially not stopped
	assert.False(t, exec.Stopped())

	// Run task to completion
	exec.RunNext(context.Background())

	// Stopped should be true (because Done is true)
	assert.True(t, exec.Stopped())
	assert.True(t, exec.Done())
}

func TestExecutor_Summary_WithPendingTasks(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "t1", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "t2", result: task.Result{Status: task.StatusFailed}},
		&mockTask{name: "t3", result: task.Result{Status: task.StatusDone}},
		&mockTask{name: "t4", result: task.Result{Status: task.StatusDone}},
	}
	exec := New(tasks)

	// Run first two tasks, then abort
	exec.RunNext(context.Background())
	exec.RunNext(context.Background())
	exec.Abort()

	summary := exec.Summary()
	assert.Equal(t, 1, summary.Done)
	assert.Equal(t, 0, summary.Skipped)
	assert.Equal(t, 1, summary.Failed)
	assert.Equal(t, 2, summary.Pending, "Tasks 3 and 4 should be pending")
	assert.True(t, summary.HasFailures)
}

func TestExecutor_ElapsedTime_BeforeExecution(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}},
	}
	exec := New(tasks)

	// ElapsedTime should be 0 before any tasks run
	elapsed := exec.ElapsedTime()
	assert.Equal(t, time.Duration(0), elapsed, "ElapsedTime should be 0 before execution starts")
}

func TestExecutor_ElapsedTime_AfterExecution(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}, sleepTime: 10 * time.Millisecond},
	}
	exec := New(tasks)

	exec.RunNext(context.Background())

	// ElapsedTime should be greater than 0 and at least the sleep time
	elapsed := exec.ElapsedTime()
	assert.Greater(t, elapsed, time.Duration(0), "ElapsedTime should be greater than 0 after execution")
	assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond, "ElapsedTime should be at least as long as task execution")
}

func TestExecutor_ElapsedTime_MultipleTasksIncreases(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}, sleepTime: 10 * time.Millisecond},
		&mockTask{name: "task2", result: task.Result{Status: task.StatusDone}, sleepTime: 10 * time.Millisecond},
	}
	exec := New(tasks)

	exec.RunNext(context.Background())
	elapsedAfterFirst := exec.ElapsedTime()

	exec.RunNext(context.Background())
	elapsedAfterSecond := exec.ElapsedTime()

	// Elapsed time should increase after second task
	assert.Greater(t, elapsedAfterSecond, elapsedAfterFirst, "ElapsedTime should increase after running more tasks")
	assert.GreaterOrEqual(t, elapsedAfterSecond, 20*time.Millisecond, "ElapsedTime should account for both tasks")
}

func TestExecutor_TaskDuration_RecordedInResult(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}, sleepTime: 10 * time.Millisecond},
	}
	exec := New(tasks)

	result, ok := exec.RunNext(context.Background())
	require.True(t, ok)

	// Duration should be recorded in the result
	assert.Greater(t, result.Duration, time.Duration(0), "Task duration should be greater than 0")
	assert.GreaterOrEqual(t, result.Duration, 10*time.Millisecond, "Task duration should be at least the sleep time")

	// Duration should also be accessible via Results()
	results := exec.Results()
	assert.Greater(t, results[0].Duration, time.Duration(0))
	assert.GreaterOrEqual(t, results[0].Duration, 10*time.Millisecond)
}

func TestExecutor_TaskDuration_MultipleTasks(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}, sleepTime: 10 * time.Millisecond},
		&mockTask{name: "task2", result: task.Result{Status: task.StatusDone}, sleepTime: 20 * time.Millisecond},
		&mockTask{name: "task3", result: task.Result{Status: task.StatusSkipped}}, // No sleep
	}
	exec := New(tasks)

	for range tasks {
		exec.RunNext(context.Background())
	}

	results := exec.Results()
	require.Len(t, results, 3)

	// Each task should have its own duration
	assert.GreaterOrEqual(t, results[0].Duration, 10*time.Millisecond, "Task 1 duration")
	assert.GreaterOrEqual(t, results[1].Duration, 20*time.Millisecond, "Task 2 duration")
	assert.Greater(t, results[2].Duration, time.Duration(0), "Task 3 duration should be recorded even if instant")

	// Task 2 should have taken longer than task 1
	assert.Greater(t, results[1].Duration, results[0].Duration)
}

func TestExecutor_TaskDuration_OnAbort(t *testing.T) {
	tasks := []task.Task{
		&mockTask{name: "task1", result: task.Result{Status: task.StatusDone}, sleepTime: 10 * time.Millisecond},
		&mockTask{name: "task2", result: task.Result{Status: task.StatusDone}},
	}
	exec := New(tasks)

	result, _ := exec.RunNext(context.Background())
	exec.Abort()

	// First task should have duration recorded
	assert.GreaterOrEqual(t, result.Duration, 10*time.Millisecond)

	// Second task should not have duration (never executed)
	results := exec.Results()
	assert.Equal(t, task.StatusPending, results[1].Status)
	assert.Equal(t, time.Duration(0), results[1].Duration, "Pending tasks should have 0 duration")
}

func TestExecutor_ElapsedTime_OnEmptyExecutor(t *testing.T) {
	exec := New(nil)

	// ElapsedTime should be 0 for empty executor
	elapsed := exec.ElapsedTime()
	assert.Equal(t, time.Duration(0), elapsed)

	// Even after calling RunNext (which returns false)
	exec.RunNext(context.Background())
	elapsed = exec.ElapsedTime()
	assert.Equal(t, time.Duration(0), elapsed, "ElapsedTime should remain 0 for empty executor")
}
