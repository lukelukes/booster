package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseToolSpec Tests ---

func TestParseToolSpec_Valid(t *testing.T) {
	tests := []struct {
		input   string
		name    string
		version string
	}{
		{"go@1.22.0", "go", "1.22.0"},
		{"node@20.10.0", "node", "20.10.0"},
		{"rust@1.75.0", "rust", "1.75.0"},
		{"java@21.0.1", "java", "21.0.1"},
		{"bun@1.0.18", "bun", "1.0.18"},
		{"python@3.12", "python", "3.12"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			spec, err := parseToolSpec(tt.input)

			require.NoError(t, err)
			assert.Equal(t, tt.name, spec.Name)
			assert.Equal(t, tt.version, spec.Version)
		})
	}
}

func TestParseToolSpec_Invalid(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"go", "missing version"},
		{"@1.22.0", "missing name"},
		{"go@", "empty version"},
		{"@", "both empty"},
		{"", "empty string"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := parseToolSpec(tt.input)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid tool spec")
			assert.Contains(t, err.Error(), "tool@version")
		})
	}
}

// --- MiseUse.Name() Tests ---

func TestMiseUse_Name(t *testing.T) {
	tests := []struct {
		name     string
		tools    []ToolSpec
		expected string
	}{
		{
			name:     "empty list",
			tools:    []ToolSpec{},
			expected: "mise use: (none)",
		},
		{
			name:     "single tool",
			tools:    []ToolSpec{{Name: "go", Version: "1.22.0"}},
			expected: "mise use: go@1.22.0",
		},
		{
			name: "two tools",
			tools: []ToolSpec{
				{Name: "go", Version: "1.22.0"},
				{Name: "node", Version: "20.10.0"},
			},
			expected: "mise use: go@1.22.0, node@20.10.0",
		},
		{
			name: "three tools - boundary",
			tools: []ToolSpec{
				{Name: "go", Version: "1.22.0"},
				{Name: "node", Version: "20.10.0"},
				{Name: "rust", Version: "1.75.0"},
			},
			expected: "mise use: go@1.22.0, node@20.10.0, rust@1.75.0",
		},
		{
			name: "four tools - shows count",
			tools: []ToolSpec{
				{Name: "go", Version: "1.22.0"},
				{Name: "node", Version: "20.10.0"},
				{Name: "rust", Version: "1.75.0"},
				{Name: "java", Version: "21.0.1"},
			},
			expected: "mise use: 4 tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &MiseUse{Tools: tt.tools}
			assert.Equal(t, tt.expected, task.Name())
		})
	}
}

func TestMiseUse_Name_BoundaryConditions(t *testing.T) {
	// Tests boundary at exactly 3 tools (inline) vs 4+ tools (count)
	tests := []struct {
		name         string
		toolCount    int
		expectInline bool
	}{
		{"exactly 3 tools - inline", 3, true},
		{"exactly 4 tools - count", 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := make([]ToolSpec, tt.toolCount)
			for i := 0; i < tt.toolCount; i++ {
				tools[i] = ToolSpec{Name: "tool", Version: "1.0"}
			}

			task := &MiseUse{Tools: tools}
			name := task.Name()

			if tt.expectInline {
				assert.Contains(t, name, "tool@1.0")
				assert.NotContains(t, name, "tools")
			} else {
				assert.Contains(t, name, "4 tools")
			}
		})
	}
}

// --- MiseUse.Run() Tests ---

func TestMiseUse_SkipsWhenAllAtCorrectVersion(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			// mise current <tool> returns current version
			if name == "mise" && len(args) >= 2 && args[0] == "current" {
				switch args[1] {
				case "go":
					return []byte("1.22.0\n"), nil
				case "node":
					return []byte("20.10.0\n"), nil
				}
			}
			return nil, errors.New("unexpected command")
		},
		LookPathFunc: func(name string) (string, error) {
			if name == "mise" {
				return "/usr/bin/mise", nil
			}
			return "", errors.New("not found")
		},
	}

	task := &MiseUse{
		Runner: mock,
		Tools: []ToolSpec{
			{Name: "go", Version: "1.22.0"},
			{Name: "node", Version: "20.10.0"},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Message, "all tools at correct versions")
}

func TestMiseUse_InstallsMissingTools(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "mise" {
				if len(args) >= 2 && args[0] == "current" {
					switch args[1] {
					case "go":
						return []byte("1.22.0\n"), nil // already correct
					case "node":
						return []byte("18.0.0\n"), nil // wrong version
					case "rust":
						return nil, errors.New("not installed") // not installed
					}
				}
				if len(args) >= 3 && args[0] == "use" && args[1] == "--global" {
					return []byte("installed " + args[2] + "\n"), nil
				}
			}
			return nil, errors.New("unexpected command")
		},
		LookPathFunc: func(name string) (string, error) {
			if name == "mise" {
				return "/usr/bin/mise", nil
			}
			return "", errors.New("not found")
		},
	}

	task := &MiseUse{
		Runner: mock,
		Tools: []ToolSpec{
			{Name: "go", Version: "1.22.0"},    // already correct
			{Name: "node", Version: "20.10.0"}, // needs update
			{Name: "rust", Version: "1.75.0"},  // needs install
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Contains(t, result.Message, "configured 2 tool(s)")
	assert.Contains(t, result.Output, "node@20.10.0")
	assert.Contains(t, result.Output, "rust@1.75.0")
}

func TestMiseUse_FailsWhenMiseNotAvailable(t *testing.T) {
	mock := &cmdexec.MockRunner{
		LookPathFunc: func(name string) (string, error) {
			return "", errors.New("not found")
		},
	}

	task := &MiseUse{
		Runner: mock,
		Tools:  []ToolSpec{{Name: "go", Version: "1.22.0"}},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "mise not found")
	assert.Contains(t, result.Error.Error(), "pkg.install")
}

func TestMiseUse_FailsOnInstallError(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "mise" {
				if len(args) >= 2 && args[0] == "current" {
					return nil, errors.New("not installed")
				}
				if len(args) >= 3 && args[0] == "use" {
					return []byte("error: network timeout"), errors.New("install failed")
				}
			}
			return nil, errors.New("unexpected")
		},
		LookPathFunc: func(name string) (string, error) {
			if name == "mise" {
				return "/usr/bin/mise", nil
			}
			return "", errors.New("not found")
		},
	}

	task := &MiseUse{
		Runner: mock,
		Tools:  []ToolSpec{{Name: "go", Version: "1.22.0"}},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "mise use")
	assert.Contains(t, result.Output, "network timeout")
}

func TestMiseUse_CapturesOutputOnSuccess(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "mise" {
				if len(args) >= 2 && args[0] == "current" {
					return nil, errors.New("not installed")
				}
				if len(args) >= 3 && args[0] == "use" {
					return []byte("mise: installing go@1.22.0\nmise: activated go@1.22.0"), nil
				}
			}
			return nil, nil
		},
		LookPathFunc: func(name string) (string, error) {
			if name == "mise" {
				return "/usr/bin/mise", nil
			}
			return "", errors.New("not found")
		},
	}

	task := &MiseUse{
		Runner: mock,
		Tools:  []ToolSpec{{Name: "go", Version: "1.22.0"}},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Contains(t, result.Output, "installing go@1.22.0")
}

func TestMiseUse_Idempotency(t *testing.T) {
	callCount := 0
	currentVersions := map[string]string{}

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "mise" {
				if len(args) >= 2 && args[0] == "current" {
					if v, ok := currentVersions[args[1]]; ok {
						return []byte(v + "\n"), nil
					}
					return nil, errors.New("not installed")
				}
				if len(args) >= 3 && args[0] == "use" {
					callCount++
					// Simulate installation by updating current version
					spec := args[2]
					// Parse "go@1.22.0" -> currentVersions["go"] = "1.22.0"
					for i := 0; i < len(spec); i++ {
						if spec[i] == '@' {
							currentVersions[spec[:i]] = spec[i+1:]
							break
						}
					}
					return []byte("installed"), nil
				}
			}
			return nil, nil
		},
		LookPathFunc: func(name string) (string, error) {
			if name == "mise" {
				return "/usr/bin/mise", nil
			}
			return "", errors.New("not found")
		},
	}

	task := &MiseUse{
		Runner: mock,
		Tools:  []ToolSpec{{Name: "go", Version: "1.22.0"}},
	}

	// First run: installs
	result1 := task.Run(context.Background())
	assert.Equal(t, StatusDone, result1.Status)

	// Second run: skips (tool now at correct version)
	result2 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result2.Status)

	// Third run: still skips
	result3 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result3.Status)

	// Should have only called mise use once
	assert.Equal(t, 1, callCount, "should only install once")
}

func TestMiseUse_EmptyToolList(t *testing.T) {
	mock := &cmdexec.MockRunner{
		LookPathFunc: func(name string) (string, error) {
			if name == "mise" {
				return "/usr/bin/mise", nil
			}
			return "", errors.New("not found")
		},
	}

	task := &MiseUse{
		Runner: mock,
		Tools:  []ToolSpec{},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Message, "all tools at correct versions")
}

// --- MiseUseFactory Tests ---

func TestMiseUseFactory_ValidArgs(t *testing.T) {
	args := []any{"go@1.22.0", "node@20.10.0", "rust@1.75.0"}

	factory := NewMiseUseFactory(MiseUseConfig{})
	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Verify through Name()
	name := tasks[0].Name()
	assert.Contains(t, name, "go@1.22.0")
	assert.Contains(t, name, "node@20.10.0")
	assert.Contains(t, name, "rust@1.75.0")
}

func TestMiseUseFactory_EmptyList(t *testing.T) {
	args := []any{}

	factory := NewMiseUseFactory(MiseUseConfig{})
	tasks, err := factory(args)

	require.NoError(t, err)
	assert.Nil(t, tasks)
}

func TestMiseUseFactory_InvalidArgs_NotList(t *testing.T) {
	args := "go@1.22.0"

	factory := NewMiseUseFactory(MiseUseConfig{})
	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a list")
}

func TestMiseUseFactory_InvalidArgs_NotString(t *testing.T) {
	args := []any{123}

	factory := NewMiseUseFactory(MiseUseConfig{})
	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a string")
}

func TestMiseUseFactory_InvalidArgs_BadToolSpec(t *testing.T) {
	args := []any{"go"} // missing version

	factory := NewMiseUseFactory(MiseUseConfig{})
	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool spec")
}

func TestMiseUseFactory_ErrorIndices(t *testing.T) {
	tests := []struct {
		name          string
		args          []any
		expectedIndex string
	}{
		{
			name:          "first arg bad type shows arg 1",
			args:          []any{123},
			expectedIndex: "arg 1:",
		},
		{
			name:          "second arg bad type shows arg 2",
			args:          []any{"go@1.22.0", 456},
			expectedIndex: "arg 2:",
		},
		{
			name:          "third arg invalid spec shows arg 3",
			args:          []any{"go@1.22.0", "node@20.10.0", "rust"},
			expectedIndex: "arg 3:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewMiseUseFactory(MiseUseConfig{})
			_, err := factory(tt.args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedIndex)
		})
	}
}

func TestMiseUseFactory_UsesProvidedRunner(t *testing.T) {
	mockRunner := &cmdexec.MockRunner{
		LookPathFunc: func(name string) (string, error) {
			return "", errors.New("mise not found")
		},
	}

	args := []any{"go@1.22.0"}

	factory := NewMiseUseFactory(MiseUseConfig{Runner: mockRunner})
	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Run to verify it uses the mock runner
	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Error.Error(), "mise not found")
}
