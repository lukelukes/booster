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
				When:   func() *config.WhenExpr { w := config.WhenExpr(tt.whenExpr); return &w }(),
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
			When:   func() *config.WhenExpr { w := config.WhenExpr(`${ 1 + }`); return &w }(),
			Args:   []any{"~/test"},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid when")
}

func TestBuilder_Build_ConditionalWithInvalidArgsDoesNotFailBuildWhenConditionFalse(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "darwin"

	factoryCalls := 0
	builder := NewBuilder().
		WithExprContext(exprCtx).
		Register("capture", func(args any) ([]Task, error) {
			factoryCalls++
			return []Task{&mockTask{name: "capture", result: Result{Status: StatusDone}}}, nil
		})

	tasks, err := builder.Build([]config.Task{{
		Action: "capture",
		When:   func() *config.WhenExpr { w := config.WhenExpr(`${ os == "arch" }`); return &w }(),
		Args: map[string]any{
			"bad": "${ 1 + }",
		},
	}})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, 0, factoryCalls)

	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, 0, factoryCalls)
}

func TestBuilder_Build_ConditionalWithInvalidArgsFailsAtRuntimeWhenConditionTrue(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "arch"

	builder := NewBuilder().
		WithExprContext(exprCtx).
		Register("capture", func(args any) ([]Task, error) {
			return []Task{&mockTask{name: "capture", result: Result{Status: StatusDone}}}, nil
		})

	tasks, err := builder.Build([]config.Task{{
		Action: "capture",
		When:   func() *config.WhenExpr { w := config.WhenExpr(`${ os == "arch" }`); return &w }(),
		Args: map[string]any{
			"bad": "${ 1 + }",
		},
	}})
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Message, "args.bad")
}

func TestBuilder_Build_WhenEvaluatesAtRuntime(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "darwin"

	inner := &mockTask{name: "capture", result: Result{Status: StatusDone, Message: "done"}}
	builder := NewBuilder().
		WithExprContext(exprCtx).
		Register("capture", func(args any) ([]Task, error) {
			return []Task{inner}, nil
		})

	tasks, err := builder.Build([]config.Task{{
		Action: "capture",
		When:   func() *config.WhenExpr { w := config.WhenExpr(`${ os == "arch" }`); return &w }(),
		Args:   nil,
	}})
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Flip context after build to prove condition is evaluated at execution time.
	exprCtx.OS = "arch"

	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusDone, result.Status)
	assert.Contains(t, result.Message, "done")
	assert.True(t, inner.called, "wrapped task should run once condition is true at runtime")
}

func TestBuilder_Build_UnknownActionErrorsEvenWhenConditionFalse(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "darwin"
	builder := NewBuilder().WithExprContext(exprCtx)

	_, err := builder.Build([]config.Task{{
		Action: "unknown.action",
		When:   func() *config.WhenExpr { w := config.WhenExpr(`${ os == "arch" }`); return &w }(),
		Args:   nil,
	}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
	assert.Contains(t, err.Error(), "unknown.action")
}

func TestBuilder_Build_ResolvesArgsBeforeFactory_TableDriven(t *testing.T) {
	ctx := expr.NewContext()
	ctx.Home = "/tmp/home"

	tests := []struct {
		name string
		args any
		want any
	}{
		{
			name: "full expression resolves to non string",
			args: map[string]any{"mode": "${ 1 + 1 }"},
			want: map[string]any{"mode": 2},
		},
		{
			name: "interpolation resolves to string",
			args: map[string]any{"path": "${ home }/app-${ 1 + 1 }"},
			want: map[string]any{"path": "/tmp/home/app-2"},
		},
		{
			name: "nested values are resolved",
			args: map[string]any{
				"pairs": []any{
					map[string]any{"source": "${ home }/src", "target": "${ home }/dst"},
				},
			},
			want: map[string]any{
				"pairs": []any{
					map[string]any{"source": "/tmp/home/src", "target": "/tmp/home/dst"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured any
			builder := NewBuilder().
				WithExprContext(ctx).
				Register("capture", func(args any) ([]Task, error) {
					captured = args
					return nil, nil
				})

			_, err := builder.Build([]config.Task{{Action: "capture", Args: tt.args}})
			require.NoError(t, err)
			assert.Equal(t, tt.want, captured)
		})
	}
}

func TestBuilder_Build_ArgResolutionContracts_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		builder *Builder
		task    config.Task
		wantErr string
	}{
		{
			name:    "literal args build without expression context",
			builder: NewBuilder().Register("dir.create", NewDirCreate),
			task: config.Task{
				Action: "dir.create",
				Args:   []any{"~/plain-dir"},
			},
		},
		{
			name:    "expression without context fails fast",
			builder: NewBuilder().Register("dir.create", NewDirCreate),
			task: config.Task{
				Action: "dir.create",
				Args:   []any{"${ home }/expr-dir"},
			},
			wantErr: "task 1 (dir.create): args[0]: expression context is required",
		},
		{
			name: "invalid expression reports deterministic deep path",
			builder: NewBuilder().
				WithExprContext(expr.NewContext()).
				Register("capture", func(args any) ([]Task, error) {
					return nil, nil
				}),
			task: config.Task{
				Action: "capture",
				Args: map[string]any{
					"items": []any{
						map[string]any{"name": "${ 1 + }"},
					},
				},
			},
			wantErr: "task 1 (capture): args.items[0].name: invalid expression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.builder.Build([]config.Task{tt.task})

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestBuilder_AnyNeedsSudo_ConditionalTask(t *testing.T) {
	tests := []struct {
		name             string
		os               string
		args             any
		wantAnyNeedsSudo bool
		wantFactoryCalls int
		wantRunStatus    Status
	}{
		{
			name:             "true when runs task",
			os:               "arch",
			args:             nil,
			wantAnyNeedsSudo: true,
			wantFactoryCalls: 1,
			wantRunStatus:    StatusDone,
		},
		{
			name:             "false when skips task",
			os:               "darwin",
			args:             nil,
			wantAnyNeedsSudo: false,
			wantFactoryCalls: 0,
			wantRunStatus:    StatusSkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprCtx := expr.NewContext().WithProfile("work")
			exprCtx.OS = tt.os

			factoryCalls := 0
			builder := NewBuilder().
				WithExprContext(exprCtx).
				Register("capture", func(args any) ([]Task, error) {
					factoryCalls++
					return []Task{&mockTask{name: "privileged", needsSudo: true, result: Result{Status: StatusDone}}}, nil
				})

			tasks, err := builder.Build([]config.Task{{
				Action: "capture",
				When:   func() *config.WhenExpr { w := config.WhenExpr(`${ os == "arch" }`); return &w }(),
				Args:   tt.args,
			}})
			require.NoError(t, err)
			require.Len(t, tasks, 1)
			assert.Equal(t, 0, factoryCalls)

			assert.Equal(t, tt.wantAnyNeedsSudo, AnyNeedsSudo(tasks))
			assert.Equal(t, tt.wantFactoryCalls, factoryCalls)

			result := tasks[0].Run(context.Background())
			assert.Equal(t, tt.wantRunStatus, result.Status)
		})
	}
}

func TestConditionalTask_NeedsSudo_ConditionErrorIsConservative(t *testing.T) {
	inner := &mockTask{name: "plain", needsSudo: false, result: Result{Status: StatusDone}}
	when, err := expr.NewValue(`${ "not-bool" }`)
	require.NoError(t, err)

	ct, err := NewConditionalTask(inner, when, expr.NewContext(), `${ "not-bool" }`)
	require.NoError(t, err)

	assert.True(t, ct.NeedsSudo(), "condition evaluation errors should be treated as sudo-needed")
}

func TestExpandConditionalDeferredTask_ExpandsGeneratedTasksLazily(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "arch"

	factoryCalls := 0
	deferred := NewDeferredFactoryTask("capture", func() ([]Task, error) {
		factoryCalls++
		return []Task{
			&mockTask{name: "one", result: Result{Status: StatusDone}},
			&mockTask{name: "two", result: Result{Status: StatusDone}},
		}, nil
	})

	conditional, err := NewConditionalTask(deferred, mustValue(t, `${ os == "arch" }`), exprCtx, `${ os == "arch" }`)
	require.NoError(t, err)
	assert.Equal(t, 0, factoryCalls)

	expanded, ok, err := ExpandConditionalDeferredTask(conditional)
	require.NoError(t, err)
	assert.True(t, ok)
	require.Len(t, expanded, 2)
	assert.Equal(t, 1, factoryCalls)
	assert.Equal(t, "one", expanded[0].Name())
	assert.Equal(t, "two", expanded[1].Name())

	expandedAgain, ok, err := ExpandConditionalDeferredTask(conditional)
	require.NoError(t, err)
	assert.True(t, ok)
	require.Len(t, expandedAgain, 2)
	assert.Equal(t, 1, factoryCalls)
}

func TestExpandConditionalDeferredTask_DoesNotExpandWhenConditionFalse(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "darwin"

	factoryCalls := 0
	deferred := NewDeferredFactoryTask("capture", func() ([]Task, error) {
		factoryCalls++
		return []Task{&mockTask{name: "one", result: Result{Status: StatusDone}}}, nil
	})

	conditional, err := NewConditionalTask(deferred, mustValue(t, `${ os == "arch" }`), exprCtx, `${ os == "arch" }`)
	require.NoError(t, err)

	expanded, ok, err := ExpandConditionalDeferredTask(conditional)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, expanded)
	assert.Equal(t, 0, factoryCalls)
}

func TestDeferredFactoryTask_Run_AllSkippedAggregatesSkipped(t *testing.T) {
	deferred := NewDeferredFactoryTask("capture", func() ([]Task, error) {
		return []Task{
			&mockTask{name: "one", result: Result{Status: StatusSkipped, Message: "one skipped"}},
			&mockTask{name: "two", result: Result{Status: StatusSkipped, Message: "two skipped"}},
		}, nil
	})

	result := deferred.Run(context.Background())
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Message, "one skipped")
	assert.Contains(t, result.Message, "two skipped")
}

func TestDeferredFactoryTask_NeedsSudo_LoadErrorReturnsFalse(t *testing.T) {
	deferred := NewDeferredFactoryTask("capture", func() ([]Task, error) {
		return nil, errors.New("prepare failed")
	})

	assert.False(t, deferred.NeedsSudo())
}

func TestBuilder_AnyNeedsSudo_DeferredPreparationErrorDoesNotRequireSudo(t *testing.T) {
	exprCtx := expr.NewContext().WithProfile("work")
	exprCtx.OS = "arch"

	builder := NewBuilder().
		WithExprContext(exprCtx).
		Register("capture", func(args any) ([]Task, error) {
			return []Task{&mockTask{name: "ok", result: Result{Status: StatusDone}}}, nil
		})

	tasks, err := builder.Build([]config.Task{{
		Action: "capture",
		When:   func() *config.WhenExpr { w := config.WhenExpr(`${ os == "arch" }`); return &w }(),
		Args: map[string]any{
			"bad": "${ 1 + }",
		},
	}})
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	assert.False(t, AnyNeedsSudo(tasks))
}
