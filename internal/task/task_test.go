package task

import (
	"booster/internal/condition"
	"booster/internal/config"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilder_Register(t *testing.T) {
	called := false
	builder := NewBuilder().Register("test.action", func(args any) ([]Task, error) {
		called = true
		return nil, nil
	})

	tasks, err := builder.Build([]config.Task{{Action: "test.action", Args: nil}})
	require.NoError(t, err)
	assert.Nil(t, tasks)

	assert.True(t, called, "registered factory should be called")
}

func TestBuilder_Build_KnownAction(t *testing.T) {
	builder := NewBuilder().Register("dir.create", NewDirCreate)

	tasks, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			Args:   []any{"~/test-dir"},
		},
	})

	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Contains(t, tasks[0].Name(), "test-dir")
}

func TestBuilder_Build_UnknownAction(t *testing.T) {
	builder := NewBuilder()

	_, err := builder.Build([]config.Task{
		{Action: "unknown.action", Args: nil},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
	assert.Contains(t, err.Error(), "unknown.action")
}

func TestBuilder_Build_FactoryError(t *testing.T) {
	builder := NewBuilder().Register("test.failing", func(args any) ([]Task, error) {
		return nil, errors.New("factory failed")
	})

	_, err := builder.Build([]config.Task{
		{Action: "test.failing", Args: nil},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "factory failed")
}

func TestBuilder_Build_MultipleTasks(t *testing.T) {
	builder := NewBuilder().Register("dir.create", NewDirCreate)

	tasks, err := builder.Build([]config.Task{
		{Action: "dir.create", Args: []any{"~/dir1"}},
		{Action: "dir.create", Args: []any{"~/dir2"}},
	})

	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}

func TestBuilder_Build_EmptyTasks(t *testing.T) {
	builder := NewBuilder()

	tasks, err := builder.Build([]config.Task{})

	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestBuilder_Build_FactoryReturnsMultipleTasks(t *testing.T) {
	builder := NewBuilder().Register("dir.create", NewDirCreate)

	tasks, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			Args:   []any{"~/a", "~/b", "~/c"},
		},
	})

	require.NoError(t, err)
	assert.Len(t, tasks, 3)
}

func TestBuilder_Build_ErrorIncludesTaskIndex(t *testing.T) {
	builder := NewBuilder().Register("dir.create", NewDirCreate)

	_, err := builder.Build([]config.Task{
		{Action: "dir.create", Args: []any{"~/valid"}},
		{Action: "nonexistent", Args: nil},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "task 2")
}

func TestBuilder_Build_ErrorIndex_FirstTask(t *testing.T) {
	builder := NewBuilder().Register("failing", func(args any) ([]Task, error) {
		return nil, errors.New("factory error")
	})

	_, err := builder.Build([]config.Task{
		{Action: "failing", Args: nil},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "task 1", "error must show 1-indexed task number")
	assert.NotContains(t, err.Error(), "task 0", "error must NOT show 0-indexed task number")
}

func TestBuilder_Build_ErrorIndex_ThirdTask(t *testing.T) {
	builder := NewBuilder().
		Register("ok", func(args any) ([]Task, error) {
			return nil, nil
		}).
		Register("failing", func(args any) ([]Task, error) {
			return nil, errors.New("factory error")
		})

	_, err := builder.Build([]config.Task{
		{Action: "ok", Args: nil},
		{Action: "ok", Args: nil},
		{Action: "failing", Args: nil},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "task 3", "error must show correct 1-indexed task number")
}

func TestDefaultBuilder_RegistersAllTasks(t *testing.T) {
	ctx := condition.Context{OS: "arch", Profile: "personal"}
	builder := DefaultBuilder(ctx)

	tasks, err := builder.Build([]config.Task{
		{Action: "dir.create", Args: []any{"~/test"}},
	})

	require.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestBuilder_Build_WithCondition(t *testing.T) {
	eval := condition.NewEvaluator(condition.Context{OS: "nonexistent_os"})
	builder := NewBuilder().Register("dir.create", NewDirCreate).WithEvaluator(eval)

	tasks, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			When:   &config.When{OS: config.StringOrSlice{"arch", "darwin"}},
			Args:   []any{"~/test"},
		},
	})

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusSkipped, result.Status, "task should be skipped when condition doesn't match")
	assert.Contains(t, result.Message, "condition not met", "skip message should indicate condition not met")
}

func TestBuilder_Build_WithoutCondition_NotWrapped(t *testing.T) {
	eval := condition.NewEvaluator(condition.Context{OS: "nonexistent_os"})
	builder := NewBuilder().Register("dir.create", NewDirCreate).WithEvaluator(eval)

	tasks, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			Args:   []any{"~/test-no-condition"},
		},
	})

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	result := tasks[0].Run(context.Background())

	if result.Status == StatusSkipped {
		assert.NotContains(t, result.Message, "condition not met",
			"task without condition should not skip due to unmet condition")
	}
}

func TestBuilder_Build_WithProfileCondition(t *testing.T) {
	eval := condition.NewEvaluator(condition.Context{OS: "arch", Profile: "work"})
	builder := NewBuilder().Register("dir.create", NewDirCreate).WithEvaluator(eval)

	tasks, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			When:   &config.When{Profile: config.StringOrSlice{"personal"}},
			Args:   []any{"~/test"},
		},
	})

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusSkipped, result.Status, "task should be skipped when profile doesn't match")
	assert.Contains(t, result.Message, "condition not met")
	assert.Contains(t, result.Message, "profile=work")
}

func TestBuilder_Build_WithBothOSAndProfileCondition(t *testing.T) {
	eval := condition.NewEvaluator(condition.Context{OS: "arch", Profile: "work"})
	builder := NewBuilder().Register("dir.create", NewDirCreate).WithEvaluator(eval)

	tasks, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			When: &config.When{
				OS:      config.StringOrSlice{"arch"},
				Profile: config.StringOrSlice{"personal"},
			},
			Args: []any{"~/test"},
		},
	})

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusSkipped, result.Status, "task should be skipped when profile doesn't match")
	assert.Contains(t, result.Message, "profile=work")
}

func TestBuilder_Build_NoEvaluator_NoWrapping(t *testing.T) {
	builder := NewBuilder().Register("dir.create", NewDirCreate)

	tasks, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			When:   &config.When{OS: config.StringOrSlice{"arch"}},
			Args:   []any{"~/test-no-eval"},
		},
	})

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	result := tasks[0].Run(context.Background())

	if result.Status == StatusSkipped {
		assert.NotContains(t, result.Message, "condition not met",
			"task should not skip due to unmet condition when no evaluator present")
	}
}
