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

type gitSetCall struct {
	Key   string
	Value string
}

type FakeGitConfigStore struct {
	GetFunc func(context.Context, string) (string, error)
	SetFunc func(context.Context, string, string) (string, error)

	GetCalls []string
	SetCalls []gitSetCall
}

func (s *FakeGitConfigStore) GetGlobal(ctx context.Context, key string) (string, error) {
	s.GetCalls = append(s.GetCalls, key)
	if s.GetFunc != nil {
		return s.GetFunc(ctx, key)
	}
	return "", errors.New("exit status 1")
}

func (s *FakeGitConfigStore) SetGlobal(ctx context.Context, key, value string) (string, error) {
	s.SetCalls = append(s.SetCalls, gitSetCall{Key: key, Value: value})
	if s.SetFunc != nil {
		return s.SetFunc(ctx, key, value)
	}
	return "", nil
}

func TestGitConfig_Run(t *testing.T) {
	tests := []struct {
		name        string
		setupStore  func() *FakeGitConfigStore
		items       []GitConfigItem
		wantStatus  Status
		wantMessage string
		checkStore  func(t *testing.T, store *FakeGitConfigStore)
	}{
		{
			name: "skips when value already set",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{
					GetFunc: func(context.Context, string) (string, error) {
						return "main", nil
					},
				}
			},
			items:       []GitConfigItem{{Key: "init.defaultBranch", Value: "main"}},
			wantStatus:  StatusSkipped,
			wantMessage: "all keys already configured",
			checkStore: func(t *testing.T, store *FakeGitConfigStore) {
				assert.Equal(t, []string{"init.defaultBranch"}, store.GetCalls)
				assert.Empty(t, store.SetCalls)
			},
		},
		{
			name: "sets new value when key doesn't exist",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{}
			},
			items:       []GitConfigItem{{Key: "init.defaultBranch", Value: "main"}},
			wantStatus:  StatusDone,
			wantMessage: "configured 1 keys",
			checkStore: func(t *testing.T, store *FakeGitConfigStore) {
				assert.Equal(t, []string{"init.defaultBranch"}, store.GetCalls)
				require.Len(t, store.SetCalls, 1)
				assert.Equal(t, "init.defaultBranch", store.SetCalls[0].Key)
				assert.Equal(t, "main", store.SetCalls[0].Value)
			},
		},
		{
			name: "updates value when different",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{
					GetFunc: func(context.Context, string) (string, error) {
						return "master", nil
					},
				}
			},
			items:       []GitConfigItem{{Key: "init.defaultBranch", Value: "main"}},
			wantStatus:  StatusDone,
			wantMessage: "configured 1 keys",
			checkStore: func(t *testing.T, store *FakeGitConfigStore) {
				assert.Equal(t, []string{"init.defaultBranch"}, store.GetCalls)
				require.Len(t, store.SetCalls, 1)
				assert.Equal(t, "init.defaultBranch", store.SetCalls[0].Key)
				assert.Equal(t, "main", store.SetCalls[0].Value)
			},
		},
		{
			name: "skips when key exists and no explicit value",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{
					GetFunc: func(context.Context, string) (string, error) {
						return "Existing User", nil
					},
				}
			},
			items:       []GitConfigItem{{Key: "user.name", Prompt: "What is your name?"}},
			wantStatus:  StatusSkipped,
			wantMessage: "all keys already configured",
			checkStore: func(t *testing.T, store *FakeGitConfigStore) {
				assert.Equal(t, []string{"user.name"}, store.GetCalls)
				assert.Empty(t, store.SetCalls)
			},
		},
		{
			name: "skips when no prompt and no value",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{}
			},
			items:       []GitConfigItem{{Key: "user.name"}},
			wantStatus:  StatusSkipped,
			wantMessage: "all keys already configured",
			checkStore: func(t *testing.T, store *FakeGitConfigStore) {
				assert.Equal(t, []string{"user.name"}, store.GetCalls)
				assert.Empty(t, store.SetCalls)
			},
		},
		{
			name: "empty items",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{}
			},
			items:       []GitConfigItem{},
			wantStatus:  StatusSkipped,
			wantMessage: "no items to configure",
			checkStore: func(t *testing.T, store *FakeGitConfigStore) {
				assert.Empty(t, store.GetCalls)
				assert.Empty(t, store.SetCalls)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStore()
			prompter := &MockPrompter{
				PromptFunc: func(ctx context.Context, promptText string) (string, error) {
					t.Fatal("Prompter should not be called in this test")
					return "", nil
				},
			}
			task := &GitConfig{Store: store, Prompter: prompter, Items: tt.items}

			result := task.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantMessage, result.Message)
			if tt.checkStore != nil {
				tt.checkStore(t, store)
			}
		})
	}
}

func TestGitConfig_Prompting(t *testing.T) {
	tests := []struct {
		name           string
		setupStore     func() *FakeGitConfigStore
		promptFunc     func(context.Context, string) (string, error)
		withPrompter   bool
		items          []GitConfigItem
		wantStatus     Status
		wantMessage    string
		wantErrContain string
		checkCalls     func(t *testing.T, store *FakeGitConfigStore, prompter *MockPrompter)
	}{
		{
			name: "prompts only when no existing value and no explicit value",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{}
			},
			promptFunc: func(ctx context.Context, promptText string) (string, error) {
				assert.Equal(t, "What is your name for git commits?", promptText)
				return "John Doe", nil
			},
			withPrompter: true,
			items:        []GitConfigItem{{Key: "user.name", Prompt: "What is your name for git commits?"}},
			wantStatus:   StatusDone,
			wantMessage:  "configured 1 keys",
			checkCalls: func(t *testing.T, store *FakeGitConfigStore, prompter *MockPrompter) {
				assert.Len(t, prompter.Calls, 1)
				require.Len(t, store.SetCalls, 1)
				assert.Equal(t, "user.name", store.SetCalls[0].Key)
				assert.Equal(t, "John Doe", store.SetCalls[0].Value)
			},
		},
		{
			name: "does not prompt when explicit value provided",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{}
			},
			promptFunc: func(ctx context.Context, promptText string) (string, error) {
				t.Fatal("Prompter should not be called when explicit value is provided")
				return "", nil
			},
			withPrompter: true,
			items:        []GitConfigItem{{Key: "user.name", Value: "Jane Doe", Prompt: "What is your name?"}},
			wantStatus:   StatusDone,
			wantMessage:  "configured 1 keys",
			checkCalls: func(t *testing.T, store *FakeGitConfigStore, prompter *MockPrompter) {
				assert.Empty(t, prompter.Calls)
				require.Len(t, store.SetCalls, 1)
				assert.Equal(t, "user.name", store.SetCalls[0].Key)
				assert.Equal(t, "Jane Doe", store.SetCalls[0].Value)
			},
		},
		{
			name: "fails when prompt cancelled",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{}
			},
			promptFunc: func(ctx context.Context, promptText string) (string, error) {
				return "", errors.New("user cancelled")
			},
			withPrompter:   true,
			items:          []GitConfigItem{{Key: "user.name", Prompt: "What is your name?"}},
			wantStatus:     StatusFailed,
			wantErrContain: "prompt for user.name",
			checkCalls: func(t *testing.T, store *FakeGitConfigStore, prompter *MockPrompter) {
				assert.Len(t, prompter.Calls, 1)
				assert.Empty(t, store.SetCalls)
			},
		},
		{
			name: "fails when prompter not configured",
			setupStore: func() *FakeGitConfigStore {
				return &FakeGitConfigStore{}
			},
			withPrompter:   false,
			items:          []GitConfigItem{{Key: "user.name", Prompt: "What is your name?"}},
			wantStatus:     StatusFailed,
			wantErrContain: "no prompter configured",
			checkCalls: func(t *testing.T, store *FakeGitConfigStore, prompter *MockPrompter) {
				assert.Empty(t, store.SetCalls)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStore()

			var prompter Prompter
			var mockPrompter *MockPrompter
			if tt.withPrompter {
				mockPrompter = &MockPrompter{PromptFunc: tt.promptFunc}
				prompter = mockPrompter
			}

			task := &GitConfig{Store: store, Prompter: prompter, Items: tt.items}
			result := task.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			if tt.wantMessage != "" {
				assert.Equal(t, tt.wantMessage, result.Message)
			}
			if tt.wantErrContain != "" {
				assert.Error(t, result.Error)
				assert.Contains(t, result.Error.Error(), tt.wantErrContain)
			}
			if tt.checkCalls != nil {
				tt.checkCalls(t, store, mockPrompter)
			}
		})
	}
}

func TestGitConfig_HandlesMultipleItems(t *testing.T) {
	store := &FakeGitConfigStore{
		GetFunc: func(ctx context.Context, key string) (string, error) {
			switch key {
			case "user.name":
				return "John Doe", nil
			case "user.email", "init.defaultBranch":
				return "", errors.New("exit status 1")
			default:
				return "", errors.New("unexpected key")
			}
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
		Store:    store,
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
	assert.Len(t, store.GetCalls, 3)
	assert.Len(t, store.SetCalls, 2)
}

func TestGitConfig_FailsWhenSetFails(t *testing.T) {
	store := &FakeGitConfigStore{
		SetFunc: func(ctx context.Context, key, value string) (string, error) {
			return "permission denied", errors.New("exit status 1")
		},
	}

	task := &GitConfig{
		Store:    store,
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

func TestGitCLIStore_UsesGitGlobalCommands(t *testing.T) {
	runner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if args[2] == "--get" {
				return []byte("main\n"), nil
			}

			return []byte("ok"), nil
		},
	}

	store := &GitCLIStore{Runner: runner}

	value, err := store.GetGlobal(context.Background(), "init.defaultBranch")
	require.NoError(t, err)
	assert.Equal(t, "main", value)

	_, err = store.SetGlobal(context.Background(), "init.defaultBranch", "main")
	require.NoError(t, err)

	require.Len(t, runner.Calls, 2)
	assert.Equal(t, []string{"config", "--global", "--get", "init.defaultBranch"}, runner.Calls[0].Args)
	assert.Equal(t, []string{"config", "--global", "init.defaultBranch", "main"}, runner.Calls[1].Args)
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
	store := &FakeGitConfigStore{
		SetFunc: func(ctx context.Context, key, value string) (string, error) {
			return "error output", errors.New("permission denied")
		},
	}

	task := &GitConfig{
		Store:    store,
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
