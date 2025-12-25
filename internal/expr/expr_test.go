package expr

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValue_Literal(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		want any
	}{
		{"string", "hello", "hello"},
		{"int", 42, 42},
		{"bool", true, true},
		{"list", []any{"a", "b"}, []any{"a", "b"}},
	}

	ctx := NewContext()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValue(tt.raw)
			require.NoError(t, err)
			assert.True(t, v.IsLiteral())

			got, err := v.Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValue_FullExpression(t *testing.T) {
	ctx := NewContext()
	ctx.Vars["name"] = "Luke"

	tests := []struct {
		name string
		raw  string
		want any
	}{
		{"os reference", "${ os }", runtime.GOOS},
		{"variable reference", "${ vars.name }", "Luke"},
		{"comparison", "${ os == \"linux\" }", runtime.GOOS == "linux"},
		{"arithmetic", "${ 1 + 2 }", 3},
		{"boolean logic", "${ true and false }", false},
		{"with spaces", "${  vars.name  }", "Luke"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValue(tt.raw)
			require.NoError(t, err)
			assert.False(t, v.IsLiteral())
			assert.True(t, v.isFullExpr)

			got, err := v.Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValue_Interpolation(t *testing.T) {
	ctx := NewContext()
	ctx.Vars["name"] = "Luke"
	ctx.Vars["version"] = "1.0"

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"prefix only", "Hello ${ vars.name }", "Hello Luke"},
		{"suffix only", "${ vars.name } is here", "Luke is here"},
		{"both sides", "Hello ${ vars.name }!", "Hello Luke!"},
		{"multiple", "${ vars.name } v${ vars.version }", "Luke v1.0"},
		{"with os", "Running on ${ os }", "Running on " + runtime.GOOS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValue(tt.raw)
			require.NoError(t, err)
			assert.False(t, v.IsLiteral())
			assert.False(t, v.isFullExpr)

			got, err := v.Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValue_BuiltinFunctions(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name   string
		raw    string
		wantFn func(any) bool
	}{
		{
			"installed - git should exist",
			"${ installed(\"git\") }",
			func(v any) bool { return v == true },
		},
		{
			"installed - nonexistent",
			"${ installed(\"definitely-not-a-real-command-12345\") }",
			func(v any) bool { return v == false },
		},
		{
			"default - with value",
			"${ default(\"hello\", \"fallback\") }",
			func(v any) bool { return v == "hello" },
		},
		{
			"default - nil fallback",
			"${ default(nil, \"fallback\") }",
			func(v any) bool { return v == "fallback" },
		},
		{
			"hasSubstr - true",
			"${ hasSubstr(\"hello world\", \"world\") }",
			func(v any) bool { return v == true },
		},
		{
			"hasSubstr - false",
			"${ hasSubstr(\"hello world\", \"xyz\") }",
			func(v any) bool { return v == false },
		},
		{
			"expand - tilde",
			"${ expand(\"~/test\") }",
			func(v any) bool {
				s, ok := v.(string)
				return ok && len(s) > 6 && s[len(s)-5:] == "/test"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValue(tt.raw)
			require.NoError(t, err)

			got, err := v.Resolve(ctx)
			require.NoError(t, err)
			assert.True(t, tt.wantFn(got), "got %v", got)
		})
	}
}

func TestContext_WithProfile(t *testing.T) {
	ctx := NewContext().WithProfile("work")

	v, err := NewValue("${ profile }")
	require.NoError(t, err)

	got, err := v.Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "work", got)
}

func TestContext_EnvAccess(t *testing.T) {
	ctx := NewContext()

	// HOME should always be set
	v, err := NewValue("${ env.HOME }")
	require.NoError(t, err)

	got, err := v.Resolve(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, got)
}

func TestValue_BuiltinContainsOperator(t *testing.T) {
	ctx := NewContext()

	// expr-lang's contains works on strings
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"string contains", `${ "hello" contains "ell" }`, true},
		{"string not contains", `${ "hello" contains "xyz" }`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValue(tt.raw)
			require.NoError(t, err)

			got, err := v.Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValue_InOperator(t *testing.T) {
	ctx := NewContext()

	// For list membership, use the 'in' operator
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"in list - true", `${ "b" in ["a", "b", "c"] }`, true},
		{"in list - false", `${ "z" in ["a", "b", "c"] }`, false},
		{"in map keys", `${ "foo" in {"foo": 1, "bar": 2} }`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValue(tt.raw)
			require.NoError(t, err)

			got, err := v.Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveCondition(t *testing.T) {
	ctx := NewContext().WithProfile("work")

	tests := []struct {
		name      string
		when      string
		wantRun   bool
		wantError bool
	}{
		// Nil/empty should always run
		{"empty string", "", true, false},

		// Boolean expressions
		{"true literal", "${ true }", true, false},
		{"false literal", "${ false }", false, false},

		// OS conditions
		{"os match", "${ os == \"" + ctx.OS + "\" }", true, false},
		{"os mismatch", "${ os == \"windows\" }", false, false},

		// Profile conditions
		{"profile match", "${ profile == \"work\" }", true, false},
		{"profile mismatch", "${ profile == \"personal\" }", false, false},

		// Combined conditions
		{"and - both true", "${ true and true }", true, false},
		{"and - one false", "${ true and false }", false, false},
		{"or - one true", "${ false or true }", true, false},

		// Function-based conditions
		{"installed check", "${ installed(\"git\") }", true, false},

		// Error cases: non-bool result
		{"string result", "${ \"hello\" }", false, true},
		{"int result", "${ 42 }", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v *Value
			var err error

			if tt.when == "" {
				v = nil
			} else {
				v, err = NewValue(tt.when)
				require.NoError(t, err)
			}

			shouldRun, err := ResolveCondition(v, ctx)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantRun, shouldRun)
			}
		})
	}
}

func TestValue_InvalidExpression(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"undefined variable", "${ undefined_var }"},
		{"syntax error", "${ 1 + }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewValue(tt.raw)
			assert.Error(t, err)
		})
	}
}
