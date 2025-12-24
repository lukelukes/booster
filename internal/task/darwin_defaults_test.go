package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDarwinDefaults_SkipOnNonDarwin(t *testing.T) {
	runner := &cmdexec.MockRunner{}
	task := &DarwinDefaults{
		Runner: runner,
		OS:     "linux",
		Entries: []DefaultsEntry{
			{Domain: "com.apple.finder", Key: "AppleShowAllFiles", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "not macOS", result.Message)

	assert.Empty(t, runner.Calls)
}

func TestDarwinDefaults_SkipWhenValueMatches_Bool(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "defaults" && args[0] == "read" {
				return []byte("1"), nil
			}
			return nil, nil
		},
	}

	task := &DarwinDefaults{
		Runner: runner,
		OS:     "darwin",
		Entries: []DefaultsEntry{
			{Domain: "com.apple.finder", Key: "AppleShowAllFiles", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Message, "already configured")

	assert.Len(t, runner.Calls, 1)
	assert.Equal(t, "defaults", runner.Calls[0].Name)
	assert.Equal(t, []string{"read", "com.apple.finder", "AppleShowAllFiles"}, runner.Calls[0].Args)
}

func TestDarwinDefaults_WriteWhenValueDiffers_Bool(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "defaults" && args[0] == "read" {
				return []byte("0"), nil
			}
			return nil, nil
		},
	}

	task := &DarwinDefaults{
		Runner: runner,
		OS:     "darwin",
		Entries: []DefaultsEntry{
			{Domain: "com.apple.finder", Key: "AppleShowAllFiles", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Contains(t, result.Message, "configured 1")

	assert.Len(t, runner.Calls, 2)
	assert.Equal(t, "defaults", runner.Calls[0].Name)
	assert.Equal(t, []string{"read", "com.apple.finder", "AppleShowAllFiles"}, runner.Calls[0].Args)
	assert.Equal(t, "defaults", runner.Calls[1].Name)
	assert.Equal(t, []string{"write", "com.apple.finder", "AppleShowAllFiles", "-bool", "true"}, runner.Calls[1].Args)
}

func TestDarwinDefaults_WriteWhenKeyNotSet(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "defaults" && args[0] == "read" {
				return nil, errors.New("defaults read failed")
			}
			return nil, nil
		},
	}

	task := &DarwinDefaults{
		Runner: runner,
		OS:     "darwin",
		Entries: []DefaultsEntry{
			{Domain: "com.apple.finder", Key: "NewKey", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Contains(t, result.Message, "configured 1")

	assert.Len(t, runner.Calls, 2)
}

func TestDarwinDefaults_AllTypes(t *testing.T) {
	tests := []struct {
		name        string
		entry       DefaultsEntry
		currentVal  string
		shouldWrite bool
		expectedArg string
	}{
		{
			name:        "bool true",
			entry:       DefaultsEntry{Domain: "test", Key: "key1", Type: "bool", Value: true},
			currentVal:  "0",
			shouldWrite: true,
			expectedArg: "true",
		},
		{
			name:        "bool false",
			entry:       DefaultsEntry{Domain: "test", Key: "key2", Type: "bool", Value: false},
			currentVal:  "1",
			shouldWrite: true,
			expectedArg: "false",
		},
		{
			name:        "int",
			entry:       DefaultsEntry{Domain: "test", Key: "key3", Type: "int", Value: 42},
			currentVal:  "0",
			shouldWrite: true,
			expectedArg: "42",
		},
		{
			name:        "float",
			entry:       DefaultsEntry{Domain: "test", Key: "key4", Type: "float", Value: 3.14},
			currentVal:  "0",
			shouldWrite: true,
			expectedArg: "3.14",
		},
		{
			name:        "string",
			entry:       DefaultsEntry{Domain: "test", Key: "key5", Type: "string", Value: "hello"},
			currentVal:  "world",
			shouldWrite: true,
			expectedArg: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &cmdexec.MockRunner{
				RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
					if name == "defaults" && args[0] == "read" {
						return []byte(tt.currentVal), nil
					}
					return nil, nil
				},
			}

			task := &DarwinDefaults{
				Runner:  runner,
				OS:      "darwin",
				Entries: []DefaultsEntry{tt.entry},
			}

			result := task.Run(context.Background())

			assert.Equal(t, StatusDone, result.Status)
			assert.Len(t, runner.Calls, 2)

			writeCall := runner.Calls[1]
			assert.Equal(t, "defaults", writeCall.Name)
			assert.Equal(t, "write", writeCall.Args[0])
			assert.Equal(t, tt.entry.Domain, writeCall.Args[1])
			assert.Equal(t, tt.entry.Key, writeCall.Args[2])
			assert.Equal(t, "-"+tt.entry.Type, writeCall.Args[3])
			assert.Equal(t, tt.expectedArg, writeCall.Args[4])
		})
	}
}

func TestDarwinDefaults_BooleanNormalization(t *testing.T) {
	tests := []struct {
		name       string
		yamlValue  any
		shellValue string
		shouldSkip bool
	}{
		{"true matches 1", true, "1", true},
		{"false matches 0", false, "0", true},
		{"true differs from 0", true, "0", false},
		{"false differs from 1", false, "1", false},
		{"int 1 matches shell 1", 1, "1", true},
		{"int 0 matches shell 0", 0, "0", true},
		{"string true matches 1", "true", "1", true},
		{"string false matches 0", "false", "0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &cmdexec.MockRunner{
				RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
					if name == "defaults" && args[0] == "read" {
						return []byte(tt.shellValue), nil
					}
					return nil, nil
				},
			}

			task := &DarwinDefaults{
				Runner: runner,
				OS:     "darwin",
				Entries: []DefaultsEntry{
					{Domain: "test", Key: "key", Type: "bool", Value: tt.yamlValue},
				},
			}

			result := task.Run(context.Background())

			if tt.shouldSkip {
				assert.Equal(t, StatusSkipped, result.Status, "should skip when values match")
				assert.Len(t, runner.Calls, 1, "should only read")
			} else {
				assert.Equal(t, StatusDone, result.Status, "should write when values differ")
				assert.Len(t, runner.Calls, 2, "should read and write")
			}
		})
	}
}

func TestDarwinDefaults_MultipleEntries(t *testing.T) {
	callCount := 0
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "defaults" && args[0] == "read" {
				callCount++

				if callCount <= 2 {
					return []byte("0"), nil
				}
				return []byte("1"), nil
			}
			return nil, nil
		},
	}

	task := &DarwinDefaults{
		Runner: runner,
		OS:     "darwin",
		Entries: []DefaultsEntry{
			{Domain: "test", Key: "key1", Type: "bool", Value: true},
			{Domain: "test", Key: "key2", Type: "int", Value: 42},
			{Domain: "test", Key: "key3", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Contains(t, result.Message, "configured 2")
	assert.Contains(t, result.Message, "1 already set")

	assert.Len(t, runner.Calls, 5)
}

func TestDarwinDefaults_WriteError(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "defaults" && args[0] == "read" {
				return []byte("0"), nil
			}
			if name == "defaults" && args[0] == "write" {
				return nil, errors.New("permission denied")
			}
			return nil, nil
		},
	}

	task := &DarwinDefaults{
		Runner: runner,
		OS:     "darwin",
		Entries: []DefaultsEntry{
			{Domain: "test", Key: "key", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "permission denied")
}

func TestDarwinDefaults_EmptyEntries(t *testing.T) {
	runner := &cmdexec.MockRunner{}
	task := &DarwinDefaults{
		Runner:  runner,
		OS:      "darwin",
		Entries: []DefaultsEntry{},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "no defaults to set", result.Message)
	assert.Empty(t, runner.Calls)
}

func TestDarwinDefaults_Name(t *testing.T) {
	tests := []struct {
		name     string
		entries  []DefaultsEntry
		expected string
	}{
		{
			name:     "empty",
			entries:  []DefaultsEntry{},
			expected: "set macOS defaults: (none)",
		},
		{
			name: "single",
			entries: []DefaultsEntry{
				{Domain: "com.apple.finder", Key: "AppleShowAllFiles", Type: "bool", Value: true},
			},
			expected: "set macOS defaults: com.apple.finder AppleShowAllFiles",
		},
		{
			name: "multiple",
			entries: []DefaultsEntry{
				{Domain: "com.apple.finder", Key: "key1", Type: "bool", Value: true},
				{Domain: "com.apple.dock", Key: "key2", Type: "int", Value: 42},
			},
			expected: "set macOS defaults: 2 entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &DarwinDefaults{
				Runner:  &cmdexec.MockRunner{},
				OS:      "darwin",
				Entries: tt.entries,
			}
			assert.Equal(t, tt.expected, task.Name())
		})
	}
}

func TestNewDarwinDefaultsFactory_ValidFile(t *testing.T) {
	dir := t.TempDir()
	defaultsFile := filepath.Join(dir, "defaults.yaml")
	content := `defaults:
  - domain: "com.apple.finder"
    key: "AppleShowAllFiles"
    type: "bool"
    value: true
  - domain: "NSGlobalDomain"
    key: "KeyRepeat"
    type: "int"
    value: 2
`
	require.NoError(t, os.WriteFile(defaultsFile, []byte(content), 0o644))

	factory := NewDarwinDefaultsFactory(DarwinDefaultsConfig{
		Runner:    &cmdexec.MockRunner{},
		OS:        "darwin",
		ConfigDir: dir,
	})

	args := map[string]any{
		"file": "defaults.yaml",
	}

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	darwinTask, ok := tasks[0].(*DarwinDefaults)
	require.True(t, ok)
	assert.Len(t, darwinTask.Entries, 2)
	assert.Equal(t, "com.apple.finder", darwinTask.Entries[0].Domain)
	assert.Equal(t, "AppleShowAllFiles", darwinTask.Entries[0].Key)
	assert.Equal(t, "bool", darwinTask.Entries[0].Type)
	assert.Equal(t, true, darwinTask.Entries[0].Value)
}

func TestNewDarwinDefaultsFactory_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	defaultsFile := filepath.Join(dir, "defaults.yaml")
	content := `defaults:
  - domain: "test"
    key: "key"
    type: "bool"
    value: true
`
	require.NoError(t, os.WriteFile(defaultsFile, []byte(content), 0o644))

	factory := NewDarwinDefaultsFactory(DarwinDefaultsConfig{
		Runner:    &cmdexec.MockRunner{},
		OS:        "darwin",
		ConfigDir: "/some/other/dir",
	})

	args := map[string]any{
		"file": defaultsFile,
	}

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)
}

func TestNewDarwinDefaultsFactory_MissingFile(t *testing.T) {
	factory := NewDarwinDefaultsFactory(DarwinDefaultsConfig{
		Runner:    &cmdexec.MockRunner{},
		OS:        "darwin",
		ConfigDir: "/tmp",
	})

	args := map[string]any{
		"file": "nonexistent.yaml",
	}

	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load defaults file")
}

func TestNewDarwinDefaultsFactory_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	defaultsFile := filepath.Join(dir, "invalid.yaml")
	require.NoError(t, os.WriteFile(defaultsFile, []byte("invalid: [yaml"), 0o644))

	factory := NewDarwinDefaultsFactory(DarwinDefaultsConfig{
		Runner:    &cmdexec.MockRunner{},
		OS:        "darwin",
		ConfigDir: dir,
	})

	args := map[string]any{
		"file": "invalid.yaml",
	}

	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse YAML")
}

func TestNewDarwinDefaultsFactory_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name: "missing domain",
			content: `defaults:
  - key: "key"
    type: "bool"
    value: true
`,
			errMsg: "missing 'domain'",
		},
		{
			name: "missing key",
			content: `defaults:
  - domain: "test"
    type: "bool"
    value: true
`,
			errMsg: "missing 'key'",
		},
		{
			name: "missing type",
			content: `defaults:
  - domain: "test"
    key: "key"
    value: true
`,
			errMsg: "missing 'type'",
		},
		{
			name: "missing value",
			content: `defaults:
  - domain: "test"
    key: "key"
    type: "bool"
`,
			errMsg: "missing 'value'",
		},
		{
			name: "invalid type",
			content: `defaults:
  - domain: "test"
    key: "key"
    type: "invalid"
    value: true
`,
			errMsg: "invalid type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			defaultsFile := filepath.Join(dir, "defaults.yaml")
			require.NoError(t, os.WriteFile(defaultsFile, []byte(tt.content), 0o644))

			factory := NewDarwinDefaultsFactory(DarwinDefaultsConfig{
				Runner:    &cmdexec.MockRunner{},
				OS:        "darwin",
				ConfigDir: dir,
			})

			args := map[string]any{
				"file": "defaults.yaml",
			}

			_, err := factory(args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestNewDarwinDefaultsFactory_ValidationErrorIndex(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedIndex string
	}{
		{
			name: "first entry missing domain shows entry 1",
			content: `defaults:
  - key: "key"
    type: "bool"
    value: true
`,
			expectedIndex: "entry 1:",
		},
		{
			name: "second entry missing key shows entry 2",
			content: `defaults:
  - domain: "test1"
    key: "key1"
    type: "bool"
    value: true
  - domain: "test2"
    type: "bool"
    value: true
`,
			expectedIndex: "entry 2:",
		},
		{
			name: "third entry invalid type shows entry 3",
			content: `defaults:
  - domain: "test1"
    key: "key1"
    type: "bool"
    value: true
  - domain: "test2"
    key: "key2"
    type: "int"
    value: 42
  - domain: "test3"
    key: "key3"
    type: "invalid"
    value: true
`,
			expectedIndex: "entry 3:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			defaultsFile := filepath.Join(dir, "defaults.yaml")
			require.NoError(t, os.WriteFile(defaultsFile, []byte(tt.content), 0o644))

			factory := NewDarwinDefaultsFactory(DarwinDefaultsConfig{
				Runner:    &cmdexec.MockRunner{},
				OS:        "darwin",
				ConfigDir: dir,
			})

			args := map[string]any{
				"file": "defaults.yaml",
			}

			_, err := factory(args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedIndex,
				"error message must show correct 1-indexed entry number")
		})
	}
}

func TestNewDarwinDefaultsFactory_InvalidArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   any
		errMsg string
	}{
		{
			name:   "not a map",
			args:   []any{"file.yaml"},
			errMsg: "must be a map",
		},
		{
			name:   "missing file key",
			args:   map[string]any{"other": "value"},
			errMsg: "missing required 'file'",
		},
		{
			name:   "file not string",
			args:   map[string]any{"file": 123},
			errMsg: "'file' must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewDarwinDefaultsFactory(DarwinDefaultsConfig{
				Runner:    &cmdexec.MockRunner{},
				OS:        "darwin",
				ConfigDir: "/tmp",
			})

			_, err := factory(tt.args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestDarwinDefaults_NormalizeValue(t *testing.T) {
	task := &DarwinDefaults{}

	tests := []struct {
		name     string
		typ      string
		value    any
		expected string
	}{
		{"bool true", "bool", true, "1"},
		{"bool false", "bool", false, "0"},
		{"bool int 1", "bool", 1, "1"},
		{"bool int 0", "bool", 0, "0"},
		{"bool string true", "bool", "true", "1"},
		{"bool string false", "bool", "false", "0"},
		{"bool string 1", "bool", "1", "1"},
		{"bool string 0", "bool", "0", "0"},
		{"int", "int", 42, "42"},
		{"float", "float", 3.14, "3.14"},
		{"string", "string", "hello", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := task.normalizeValue(tt.typ, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDarwinDefaults_NormalizeValue_BoolStringEdgeCases(t *testing.T) {
	task := &DarwinDefaults{}

	assert.Equal(t, "0", task.normalizeValue("bool", "false"))
	assert.Equal(t, "0", task.normalizeValue("bool", "0"))
	assert.Equal(t, "0", task.normalizeValue("bool", "FALSE"))
	assert.Equal(t, "0", task.normalizeValue("bool", "False"))

	assert.Equal(t, "1", task.normalizeValue("bool", "true"))
	assert.Equal(t, "1", task.normalizeValue("bool", "1"))

	assert.Equal(t, "other", task.normalizeValue("bool", "other"))
	assert.Equal(t, "yes", task.normalizeValue("bool", "yes"))
}

func TestDarwinDefaults_MessageWhenNoAlreadySet(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "defaults" && args[0] == "read" {
				return nil, errors.New("not found")
			}
			return []byte("set ok"), nil
		},
	}

	task := &DarwinDefaults{
		Runner: runner,
		OS:     "darwin",
		Entries: []DefaultsEntry{
			{Domain: "test", Key: "key1", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Equal(t, "configured 1 settings", result.Message)
	assert.NotContains(t, result.Message, "already set",
		"message should NOT include 'already set' when alreadySet == 0")
}

func TestDarwinDefaults_CapturesWriteOutput(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "defaults" && args[0] == "read" {
				return []byte("0"), nil
			}
			if name == "defaults" && args[0] == "write" {
				return []byte("write output captured"), nil
			}
			return nil, nil
		},
	}

	task := &DarwinDefaults{
		Runner: runner,
		OS:     "darwin",
		Entries: []DefaultsEntry{
			{Domain: "test", Key: "key", Type: "bool", Value: true},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Contains(t, result.Output, "write output captured",
		"non-empty write output should be captured in result")
}
