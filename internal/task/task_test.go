package task

import (
	"booster/internal/config"
	"booster/internal/expr"
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
	ctx := expr.NewContext().WithProfile("personal")
	ctx.OS = "arch"
	builder := DefaultBuilder(ctx)

	tasks, err := builder.Build([]config.Task{
		{Action: "dir.create", Args: []any{"~/test"}},
	})

	require.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestBuilder_Build_WithCondition(t *testing.T) {
	tests := []struct {
		name     string
		profile  string
		os       string
		whenExpr string
		wantSkip bool
	}{
		{
			name:     "skip when os does not match",
			profile:  "work",
			os:       "nonexistent_os",
			whenExpr: `${ os in ["arch", "darwin"] }`,
			wantSkip: true,
		},
		{
			name:     "skip when profile does not match",
			profile:  "work",
			os:       "arch",
			whenExpr: `${ profile == "personal" }`,
			wantSkip: true,
		},
		{
			name:     "skip when combined condition does not match",
			profile:  "work",
			os:       "arch",
			whenExpr: `${ os == "arch" and profile == "personal" }`,
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprCtx := expr.NewContext().WithProfile(tt.profile)
			exprCtx.OS = tt.os
			builder := NewBuilder().Register("dir.create", NewDirCreate).WithExprContext(exprCtx)

			tasks, err := builder.Build([]config.Task{{
				Action: "dir.create",
				When:   &config.When{Expr: tt.whenExpr},
				Args:   []any{"~/test"},
			}})

			require.NoError(t, err)
			require.Len(t, tasks, 1)

			result := tasks[0].Run(context.Background())
			if tt.wantSkip {
				assert.Equal(t, StatusSkipped, result.Status)
				assert.Contains(t, result.Message, "condition not met")
			} else {
				assert.NotEqual(t, StatusSkipped, result.Status)
			}
		})
	}
}

func TestBuilder_Build_WithoutCondition_NotWrapped(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "nonexistent_os"
	builder := NewBuilder().Register("dir.create", NewDirCreate).WithExprContext(exprCtx)

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

func TestBuilder_Build_InvalidConditionExpression(t *testing.T) {
	builder := NewBuilder().Register("dir.create", NewDirCreate).WithExprContext(expr.NewContext())

	_, err := builder.Build([]config.Task{
		{
			Action: "dir.create",
			When:   &config.When{Expr: `${ 1 + }`},
			Args:   []any{"~/test"},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid when")
}
