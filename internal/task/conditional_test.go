package task

import (
	"booster/internal/expr"
	"context"
	"strings"
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

func TestConditionalTask_Run(t *testing.T) {
	tests := []struct {
		name               string
		exprCtxProfile     string
		exprCtxOS          string
		whenExpr           string
		wantStatus         Status
		wantMessageContain string
		wantCalled         bool
		wantError          bool
		wrappedResult      Result
	}{
		{
			name:               "skips when condition false",
			exprCtxProfile:     "work",
			exprCtxOS:          "darwin",
			whenExpr:           `${ os == "arch" }`,
			wantStatus:         StatusSkipped,
			wantMessageContain: "condition not met",
			wantCalled:         false,
			wantError:          false,
			wrappedResult:      Result{Status: StatusDone, Message: "done"},
		},
		{
			name:               "runs wrapped task when condition true",
			exprCtxProfile:     "work",
			exprCtxOS:          "arch",
			whenExpr:           `${ os == "arch" }`,
			wantStatus:         StatusDone,
			wantMessageContain: "created",
			wantCalled:         true,
			wantError:          false,
			wrappedResult:      Result{Status: StatusDone, Message: "created"},
		},
		{
			name:               "fails when condition expression is not bool",
			exprCtxOS:          "arch",
			whenExpr:           `${ "not-a-bool" }`,
			wantStatus:         StatusFailed,
			wantMessageContain: "condition evaluation failed",
			wantCalled:         false,
			wantError:          true,
			wrappedResult:      Result{Status: StatusDone, Message: "created"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &mockTask{name: "test task", result: tt.wrappedResult}
			ctx := expr.NewContext().WithProfile(tt.exprCtxProfile)
			ctx.OS = tt.exprCtxOS
			when, err := expr.NewValue(tt.whenExpr)
			require.NoError(t, err)

			ct, err := NewConditionalTask(inner, when, ctx, tt.whenExpr)
			require.NoError(t, err)

			result := ct.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Contains(t, result.Message, tt.wantMessageContain)
			assert.Equal(t, tt.wantCalled, inner.called)
			if tt.wantError {
				require.Error(t, result.Error)
			} else {
				assert.NoError(t, result.Error)
			}
		})
	}
}

func TestConditionalTask_Name(t *testing.T) {
	inner := &mockTask{name: "create ~/test"}
	ctx := expr.NewContext()
	when, err := expr.NewValue(`${ true }`)
	require.NoError(t, err)

	ct, err := NewConditionalTask(inner, when, ctx, `${ true }`)
	require.NoError(t, err)

	assert.Equal(t, "create ~/test", ct.Name())
}

func TestNewConditionalTask_Validation(t *testing.T) {
	tests := []struct {
		name    string
		when    *expr.Value
		ctx     *expr.Context
		wantErr string
	}{
		{name: "nil expression", when: nil, ctx: expr.NewContext(), wantErr: "when expression cannot be nil"},
		{name: "nil context", when: mustValue(t, `${ true }`), ctx: nil, wantErr: "expression context cannot be nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &mockTask{name: "test", result: Result{Status: StatusDone}}
			ct, err := NewConditionalTask(inner, tt.when, tt.ctx, `${ true }`)
			assert.Nil(t, ct)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestFormatWhenForMessage_TruncatesAndSanitizes(t *testing.T) {
	input := "${\n\t" + strings.Repeat("a", 200) + "\r}"
	formatted := formatWhenForMessage(input)

	assert.NotContains(t, formatted, "\n")
	assert.NotContains(t, formatted, "\t")
	assert.NotContains(t, formatted, "\r")
	assert.LessOrEqual(t, len(formatted), maxWhenMessageLen)
	assert.True(t, strings.HasSuffix(formatted, "..."))
}

func mustValue(t *testing.T, s string) *expr.Value {
	t.Helper()
	v, err := expr.NewValue(s)
	require.NoError(t, err)
	return v
}
