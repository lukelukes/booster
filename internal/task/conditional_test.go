package task

import (
	"booster/internal/condition"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTask struct {
	result    Result
	name      string
	called    bool
	needsSudo bool
}

func (m *mockTask) Name() string { return m.name }
func (m *mockTask) Run(ctx context.Context) Result {
	m.called = true
	return m.result
}
func (m *mockTask) NeedsSudo() bool { return m.needsSudo }

func TestConditionalTask_SkipsWhenConditionNotMet(t *testing.T) {
	inner := &mockTask{
		name:   "test task",
		result: Result{Status: StatusDone, Message: "done"},
	}

	ctx := condition.Context{OS: "darwin"}
	eval := condition.NewEvaluator(ctx)
	cond := &condition.Condition{OS: []string{"arch"}}

	ct, err := NewConditionalTask(inner, cond, eval)
	require.NoError(t, err)

	result := ct.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Message, "condition not met")
	assert.Contains(t, result.Message, "os=darwin")
	assert.False(t, inner.called, "wrapped task should not be called")
}

func TestConditionalTask_ExecutesWhenConditionMet(t *testing.T) {
	inner := &mockTask{
		name:   "test task",
		result: Result{Status: StatusDone, Message: "created"},
	}

	ctx := condition.Context{OS: "arch"}
	eval := condition.NewEvaluator(ctx)
	cond := &condition.Condition{OS: []string{"arch"}}

	ct, err := NewConditionalTask(inner, cond, eval)
	require.NoError(t, err)

	result := ct.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Equal(t, "created", result.Message)
	assert.True(t, inner.called, "wrapped task should be called")
}

func TestConditionalTask_MultipleOSMatches(t *testing.T) {
	inner := &mockTask{
		name:   "test task",
		result: Result{Status: StatusDone},
	}

	ctx := condition.Context{OS: "darwin"}
	eval := condition.NewEvaluator(ctx)
	cond := &condition.Condition{OS: []string{"arch", "darwin"}}

	ct, err := NewConditionalTask(inner, cond, eval)
	require.NoError(t, err)

	result := ct.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.True(t, inner.called)
}

func TestConditionalTask_Name(t *testing.T) {
	inner := &mockTask{name: "create ~/test"}
	ctx := condition.Context{OS: "arch"}
	eval := condition.NewEvaluator(ctx)
	cond := &condition.Condition{OS: []string{"arch"}}

	ct, err := NewConditionalTask(inner, cond, eval)
	require.NoError(t, err)

	assert.Equal(t, "create ~/test", ct.Name())
}

func TestConditionalTask_PropagatesWrappedResult(t *testing.T) {
	tests := []struct {
		wrappedRes  Result
		expectedRes Result
		name        string
	}{
		{
			name:        "propagates StatusDone",
			wrappedRes:  Result{Status: StatusDone, Message: "created"},
			expectedRes: Result{Status: StatusDone, Message: "created"},
		},
		{
			name:        "propagates StatusSkipped from idempotency",
			wrappedRes:  Result{Status: StatusSkipped, Message: "already exists"},
			expectedRes: Result{Status: StatusSkipped, Message: "already exists"},
		},
		{
			name:        "propagates StatusFailed",
			wrappedRes:  Result{Status: StatusFailed, Error: assert.AnError},
			expectedRes: Result{Status: StatusFailed, Error: assert.AnError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &mockTask{result: tt.wrappedRes}
			ctx := condition.Context{OS: "arch"}
			eval := condition.NewEvaluator(ctx)
			cond := &condition.Condition{OS: []string{"arch"}}

			ct, err := NewConditionalTask(inner, cond, eval)
			require.NoError(t, err)
			result := ct.Run(context.Background())

			assert.Equal(t, tt.expectedRes.Status, result.Status)
			assert.Equal(t, tt.expectedRes.Message, result.Message)
			assert.Equal(t, tt.expectedRes.Error, result.Error)
		})
	}
}

func TestNewConditionalTask_RejectsNilEvaluator(t *testing.T) {
	inner := &mockTask{name: "test", result: Result{Status: StatusDone}}
	cond := &condition.Condition{OS: []string{"arch"}}

	ct, err := NewConditionalTask(inner, cond, nil)

	assert.Nil(t, ct)
	assert.EqualError(t, err, "evaluator cannot be nil")
}
