package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockPrompter struct {
	PromptFunc func(ctx context.Context, promptText string) (string, error)

	Calls []PromptCall
}

type PromptCall struct {
	PromptText string
}

func (m *MockPrompter) Prompt(ctx context.Context, promptText string) (string, error) {
	m.Calls = append(m.Calls, PromptCall{PromptText: promptText})
	if m.PromptFunc != nil {
		return m.PromptFunc(ctx, promptText)
	}
	return "", nil
}

func TestGitConfig_Run(t *testing.T) {
	tests := []struct {
		name        string
		runFunc     func(context.Context, string, ...string) ([]byte, error)
		items       []GitConfigItem
		wantStatus  Status
		wantMessage string
		wantCalls   int
		checkCalls  func(t *testing.T, runner *cmdexec.MockRunner)
	}{
		{
			name: "skips when value already set",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[3] == "init.defaultBranch" {
					return []byte("main\n"), nil
				}
				return nil, errors.New("unexpected command")
			},
			items:       []GitConfigItem{{Key: "init.defaultBranch", Value: "main"}},
			wantStatus:  StatusSkipped,
			wantMessage: "all keys already configured",
			wantCalls:   1,
			checkCalls: func(t *testing.T, runner *cmdexec.MockRunner) {
				assert.Equal(t, "git", runner.Calls[0].Name)
				assert.Equal(t, []string{"config", "--global", "--get", "init.defaultBranch"}, runner.Calls[0].Args)
			},
		},
		{
			name: "sets new value when key doesn't exist",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" {
					if args[2] == "--get" {
						return nil, errors.New("exit status 1")
					}

					return []byte(""), nil
				}
				return nil, errors.New("unexpected command")
			},
			items:       []GitConfigItem{{Key: "init.defaultBranch", Value: "main"}},
			wantStatus:  StatusDone,
			wantMessage: "configured 1 keys",
			wantCalls:   2,
			checkCalls: func(t *testing.T, runner *cmdexec.MockRunner) {
				assert.Equal(t, []string{"config", "--global", "--get", "init.defaultBranch"}, runner.Calls[0].Args)
				assert.Equal(t, []string{"config", "--global", "init.defaultBranch", "main"}, runner.Calls[1].Args)
			},
		},
		{
			name: "updates value when different",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" {
					if args[2] == "--get" && args[3] == "init.defaultBranch" {
						return []byte("master\n"), nil
					}

					return []byte(""), nil
				}
				return nil, errors.New("unexpected command")
			},
			items:       []GitConfigItem{{Key: "init.defaultBranch", Value: "main"}},
			wantStatus:  StatusDone,
			wantMessage: "configured 1 keys",
			wantCalls:   2,
			checkCalls: func(t *testing.T, runner *cmdexec.MockRunner) {
				assert.Equal(t, []string{"config", "--global", "--get", "init.defaultBranch"}, runner.Calls[0].Args)
				assert.Equal(t, []string{"config", "--global", "init.defaultBranch", "main"}, runner.Calls[1].Args)
			},
		},
		{
			name: "skips when key exists and no explicit value",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[3] == "user.name" {
					return []byte("Existing User\n"), nil
				}
				return nil, errors.New("unexpected command")
			},
			items:       []GitConfigItem{{Key: "user.name", Prompt: "What is your name?"}},
			wantStatus:  StatusSkipped,
			wantMessage: "all keys already configured",
			wantCalls:   1,
			checkCalls: func(t *testing.T, runner *cmdexec.MockRunner) {
				assert.Equal(t, []string{"config", "--global", "--get", "user.name"}, runner.Calls[0].Args)
			},
		},
		{
			name: "skips when no prompt and no value",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" && args[2] == "--get" {
					return nil, errors.New("exit status 1")
				}
				return nil, errors.New("unexpected command")
			},
			items:       []GitConfigItem{{Key: "user.name"}},
			wantStatus:  StatusSkipped,
			wantMessage: "all keys already configured",
			wantCalls:   1,
		},
		{
			name:        "empty items",
			runFunc:     nil,
			items:       []GitConfigItem{},
			wantStatus:  StatusSkipped,
			wantMessage: "no items to configure",
			wantCalls:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &cmdexec.MockRunner{RunFunc: tt.runFunc}
			prompter := &MockPrompter{
				PromptFunc: func(ctx context.Context, promptText string) (string, error) {
					t.Fatal("Prompter should not be called in this test")
					return "", nil
				},
			}
			task := &GitConfig{
				Runner:   runner,
				Prompter: prompter,
				Items:    tt.items,
			}

			result := task.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantMessage, result.Message)
			assert.Len(t, runner.Calls, tt.wantCalls)
			if tt.checkCalls != nil {
				tt.checkCalls(t, runner)
			}
		})
	}
}

func TestGitConfig_Prompting(t *testing.T) {
	tests := []struct {
		name           string
		runFunc        func(context.Context, string, ...string) ([]byte, error)
		promptFunc     func(context.Context, string) (string, error)
		items          []GitConfigItem
		wantStatus     Status
		wantMessage    string
		wantPromptCall bool
		checkCalls     func(t *testing.T, runner *cmdexec.MockRunner, prompter *MockPrompter)
	}{
		{
			name: "prompts only when no existing value and no explicit value",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" {
					if args[2] == "--get" {
						return nil, errors.New("exit status 1")
					}

					return []byte(""), nil
				}
				return nil, errors.New("unexpected command")
			},
			promptFunc: func(ctx context.Context, promptText string) (string, error) {
				assert.Equal(t, "What is your name for git commits?", promptText)
				return "John Doe", nil
			},
			items:          []GitConfigItem{{Key: "user.name", Prompt: "What is your name for git commits?"}},
			wantStatus:     StatusDone,
			wantMessage:    "configured 1 keys",
			wantPromptCall: true,
			checkCalls: func(t *testing.T, runner *cmdexec.MockRunner, prompter *MockPrompter) {
				assert.Len(t, prompter.Calls, 1)
				assert.Len(t, runner.Calls, 2)
				assert.Equal(t, []string{"config", "--global", "user.name", "John Doe"}, runner.Calls[1].Args)
			},
		},
		{
			name: "does not prompt when explicit value provided",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" {
					if args[2] == "--get" {
						return nil, errors.New("exit status 1")
					}

					return []byte(""), nil
				}
				return nil, errors.New("unexpected command")
			},
			promptFunc: func(ctx context.Context, promptText string) (string, error) {
				t.Fatal("Prompter should not be called when explicit value is provided")
				return "", nil
			},
			items:          []GitConfigItem{{Key: "user.name", Value: "Jane Doe", Prompt: "What is your name?"}},
			wantStatus:     StatusDone,
			wantMessage:    "configured 1 keys",
			wantPromptCall: false,
			checkCalls: func(t *testing.T, runner *cmdexec.MockRunner, prompter *MockPrompter) {
				assert.Empty(t, prompter.Calls, "Prompter should not be called")
				assert.Len(t, runner.Calls, 2)
				assert.Equal(t, []string{"config", "--global", "user.name", "Jane Doe"}, runner.Calls[1].Args)
			},
		},
		{
			name: "fails when prompt cancelled",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" && args[2] == "--get" {
					return nil, errors.New("exit status 1")
				}
				return nil, errors.New("unexpected command")
			},
			promptFunc: func(ctx context.Context, promptText string) (string, error) {
				return "", errors.New("user cancelled")
			},
			items:          []GitConfigItem{{Key: "user.name", Prompt: "What is your name?"}},
			wantStatus:     StatusFailed,
			wantMessage:    "",
			wantPromptCall: true,
			checkCalls: func(t *testing.T, runner *cmdexec.MockRunner, prompter *MockPrompter) {
			},
		},
		{
			name: "fails when prompter not configured",
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" && args[2] == "--get" {
					return nil, errors.New("exit status 1")
				}
				return nil, errors.New("unexpected command")
			},
			promptFunc:     nil,
			items:          []GitConfigItem{{Key: "user.name", Prompt: "What is your name?"}},
			wantStatus:     StatusFailed,
			wantMessage:    "",
			wantPromptCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &cmdexec.MockRunner{RunFunc: tt.runFunc}

			var prompter Prompter
			if tt.name != "fails when prompter not configured" {
				prompter = &MockPrompter{PromptFunc: tt.promptFunc}
			}

			task := &GitConfig{
				Runner:   runner,
				Prompter: prompter,
				Items:    tt.items,
			}

			result := task.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			if tt.wantMessage != "" {
				assert.Equal(t, tt.wantMessage, result.Message)
			}

			if tt.name == "fails when prompt cancelled" {
				assert.Error(t, result.Error)
				assert.Contains(t, result.Error.Error(), "prompt for user.name")
			}
			if tt.name == "fails when prompter not configured" {
				assert.Error(t, result.Error)
				assert.Contains(t, result.Error.Error(), "no prompter configured")
			}

			if tt.checkCalls != nil && prompter != nil {
				mockPrompter, ok := prompter.(*MockPrompter)
				require.True(t, ok, "prompter should be *MockPrompter")
				tt.checkCalls(t, runner, mockPrompter)
			}
		})
	}
}

func TestGitConfig_HandlesMultipleItems(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" {
				if args[2] != "--get" {
					return []byte(""), nil
				}
				key := args[3]
				if key == "user.name" {
					return []byte("John Doe\n"), nil
				}
				if key == "user.email" || key == "init.defaultBranch" {
					return nil, errors.New("exit status 1")
				}
			}
			return nil, errors.New("unexpected command")
		},
	}

	prompter := &MockPrompter{
		PromptFunc: func(ctx context.Context, promptText string) (string, error) {
			if promptText == "What is your email?" {
				return "john@example.com", nil
			}
			return "", errors.New("unexpected prompt")
		},
	}

	task := &GitConfig{
		Runner:   runner,
		Prompter: prompter,
		Items: []GitConfigItem{
			{Key: "user.name", Prompt: "What is your name?"},
			{Key: "user.email", Prompt: "What is your email?"},
			{Key: "init.defaultBranch", Value: "main"},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Equal(t, "configured 2 keys (skipped 1)", result.Message)
	assert.Len(t, prompter.Calls, 1, "Should prompt once for user.email")

	assert.Len(t, runner.Calls, 5)
}

func TestGitConfig_FailsWhenGitCommandFails(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "git" && len(args) == 4 && args[0] == "config" && args[1] == "--global" {
				if args[2] == "--get" {
					return nil, errors.New("exit status 1")
				}

				return []byte("permission denied"), errors.New("exit status 1")
			}
			return nil, errors.New("unexpected command")
		},
	}

	task := &GitConfig{
		Runner:   runner,
		Prompter: &MockPrompter{},
		Items: []GitConfigItem{
			{Key: "user.name", Value: "John Doe"},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "set user.name")
}

func TestGitConfig_Name(t *testing.T) {
	tests := []struct {
		name     string
		items    []GitConfigItem
		wantName string
	}{
		{
			name:     "single item",
			items:    []GitConfigItem{{Key: "user.name"}},
			wantName: "configure git: user.name",
		},
		{
			name: "multiple items",
			items: []GitConfigItem{
				{Key: "user.name"},
				{Key: "user.email"},
				{Key: "init.defaultBranch"},
			},
			wantName: "configure git: user.name, user.email, init.defaultBranch",
		},
		{
			name:     "no items",
			items:    []GitConfigItem{},
			wantName: "configure git: (none)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &GitConfig{Items: tt.items}
			assert.Equal(t, tt.wantName, task.Name())
		})
	}
}

func TestNewGitConfig(t *testing.T) {
	tests := []struct {
		name           string
		args           any
		wantErr        bool
		wantErrStrings []string
		wantTaskCount  int
		checkTask      func(t *testing.T, task Task)
	}{
		{
			name: "valid args",
			args: []any{
				map[string]any{
					"key":    "user.name",
					"prompt": "What is your name?",
				},
				map[string]any{
					"key":   "user.email",
					"value": "test@example.com",
				},
			},
			wantErr:       false,
			wantTaskCount: 1,
			checkTask: func(t *testing.T, task Task) {
				name := task.Name()
				assert.Contains(t, name, "user.name")
				assert.Contains(t, name, "user.email")
			},
		},
		{
			name:           "invalid args - not list",
			args:           "not a list",
			wantErr:        true,
			wantErrStrings: []string{"must be a list"},
		},
		{
			name:           "invalid args - not map",
			args:           []any{"string instead of map"},
			wantErr:        true,
			wantErrStrings: []string{"arg 1", "must be a map"},
		},
		{
			name: "invalid args - missing key",
			args: []any{
				map[string]any{
					"value": "test",
				},
			},
			wantErr:        true,
			wantErrStrings: []string{"arg 1", "key", "required"},
		},
		{
			name: "invalid args - empty key",
			args: []any{
				map[string]any{
					"key": "",
				},
			},
			wantErr:        true,
			wantErrStrings: []string{"arg 1", "key"},
		},
		{
			name:          "empty list",
			args:          []any{},
			wantErr:       false,
			wantTaskCount: 0,
		},
		{
			name: "only key provided",
			args: []any{
				map[string]any{
					"key": "user.name",
				},
			},
			wantErr:       false,
			wantTaskCount: 1,
			checkTask: func(t *testing.T, task Task) {
				name := task.Name()
				assert.Contains(t, name, "user.name")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewGitConfig(&cmdexec.MockRunner{}, &MockPrompter{})
			tasks, err := factory(tt.args)

			if tt.wantErr {
				require.Error(t, err)
				for _, errStr := range tt.wantErrStrings {
					assert.Contains(t, err.Error(), errStr)
				}
			} else {
				require.NoError(t, err)
				if tt.wantTaskCount == 0 {
					assert.Nil(t, tasks)
				} else {
					assert.Len(t, tasks, tt.wantTaskCount)
					if tt.checkTask != nil {
						tt.checkTask(t, tasks[0])
					}
				}
			}
		})
	}
}

func TestGitConfig_ErrorOutputNoLeadingNewline(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "git" && args[2] == "--get" {
				return nil, errors.New("not found")
			}

			return []byte("error output"), errors.New("permission denied")
		},
	}

	task := &GitConfig{
		Runner:   runner,
		Prompter: &MockPrompter{},
		Items: []GitConfigItem{
			{Key: "user.name", Value: "test"},
		},
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)

	assert.False(t, len(result.Output) > 0 && result.Output[0] == '\n',
		"output should not start with newline when there's no previous output")
}

func TestNewGitConfig_ErrorIndexFirstArg(t *testing.T) {
	factory := NewGitConfig(&cmdexec.MockRunner{}, &MockPrompter{})

	args := []any{
		map[string]any{
			"value": "no key provided",
		},
	}

	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "arg 1:", "error must show 1-indexed position")
	assert.NotContains(t, err.Error(), "arg 0:", "error must NOT show 0-indexed position")
}

func TestNewGitConfig_ErrorIndexSecondArg(t *testing.T) {
	factory := NewGitConfig(&cmdexec.MockRunner{}, &MockPrompter{})

	args := []any{
		map[string]any{
			"key": "valid.key",
		},
		map[string]any{
			"value": "missing key",
		},
	}

	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "arg 2:", "error must show correct 1-indexed position")
}
