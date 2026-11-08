package action

import (
	"booster/internal/cmdexec"
	"booster/internal/model"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGit struct {
	global  map[string]string
	missing bool
	failSet bool
	sets    []string
}

func (g *fakeGit) runner() *cmdexec.MockRunner {
	return &cmdexec.MockRunner{
		LookPathFunc: func(string) (string, error) {
			if g.missing {
				return "", errors.New("not found")
			}
			return "/usr/bin/git", nil
		},
		RunFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			switch {
			case len(args) == 4 && args[2] == "--get":
				value, ok := g.global[args[3]]
				if !ok {
					return nil, errors.New("exit status 1")
				}
				return []byte(value + "\n"), nil
			case len(args) == 4:
				if g.failSet {
					return []byte("error: could not lock config file"), errors.New("exit status 255")
				}
				g.global[args[2]] = args[3]
				g.sets = append(g.sets, args[2])
				return nil, nil
			}
			return nil, errors.New("unexpected git call")
		},
	}
}

func newGitConfig(git *fakeGit, entries ...GitConfigEntry) *GitConfig {
	return &GitConfig{Runner: git.runner(), Entries: entries}
}

func TestGitConfig_Check(t *testing.T) {
	tests := []struct {
		name      string
		global    map[string]string
		missing   bool
		wantKind  model.ChangeKind
		wantItems []model.Item
	}{
		{
			name:     "all keys already set",
			global:   map[string]string{"user.email": "me@example.com", "init.defaultBranch": "main"},
			wantKind: model.ChangeNone,
		},
		{
			name:      "a differing key is an update",
			global:    map[string]string{"user.email": "old@example.com", "init.defaultBranch": "main"},
			wantKind:  model.ChangeUpdate,
			wantItems: []model.Item{{Name: "user.email", From: "old@example.com", To: "me@example.com"}},
		},
		{
			name:     "unset keys are a create",
			global:   map[string]string{},
			wantKind: model.ChangeCreate,
			wantItems: []model.Item{
				{Name: "user.email", To: "me@example.com"},
				{Name: "init.defaultBranch", To: "main"},
			},
		},
		{
			name:     "unknown while git is not installed yet",
			missing:  true,
			wantKind: model.ChangeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := &fakeGit{global: tt.global, missing: tt.missing}
			action := newGitConfig(git,
				GitConfigEntry{Key: "user.email", Value: "me@example.com"},
				GitConfigEntry{Key: "init.defaultBranch", Value: "main"},
			)

			change, err := action.Check(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.wantKind, change.Kind)
			assert.Equal(t, 2, change.Total)
			assert.Equal(t, tt.wantItems, change.Items)
		})
	}
}

func TestGitConfig_Apply(t *testing.T) {
	t.Run("writes only differing keys and converges", func(t *testing.T) {
		git := &fakeGit{global: map[string]string{"user.email": "old@example.com", "init.defaultBranch": "main"}}
		action := newGitConfig(git,
			GitConfigEntry{Key: "user.email", Value: "me@example.com"},
			GitConfigEntry{Key: "init.defaultBranch", Value: "main"},
			GitConfigEntry{Key: "pull.rebase", Value: "true"},
		)

		require.NoError(t, action.Apply(context.Background()))

		assert.Equal(t, []string{"user.email", "pull.rebase"}, git.sets)
		change, err := action.Check(context.Background())
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
	})

	t.Run("reports the key and git output when a write fails", func(t *testing.T) {
		git := &fakeGit{global: map[string]string{}, failSet: true}
		action := newGitConfig(git, GitConfigEntry{Key: "user.email", Value: "me@example.com"})

		err := action.Apply(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "git config user.email")
		assert.Contains(t, err.Error(), "could not lock config file")
	})

	t.Run("fails when git is still missing", func(t *testing.T) {
		git := &fakeGit{missing: true}
		action := newGitConfig(git, GitConfigEntry{Key: "user.email", Value: "me@example.com"})

		err := action.Apply(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "git not found in PATH")
	})
}

func TestNewGitConfig(t *testing.T) {
	t.Run("resolves values from variables", func(t *testing.T) {
		args := argsNode(t, `
- key: user.email
  value: ${ vars.config_dir }
- key: init.defaultBranch
  value: main
`)

		created, err := NewGitConfig(args, testContext())

		require.NoError(t, err)
		assert.Equal(t, []GitConfigEntry{
			{Key: "user.email", Value: "/home/tester/.config"},
			{Key: "init.defaultBranch", Value: "main"},
		}, created.(*GitConfig).Entries)
	})

	tests := []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "not a list", args: `key: user.email`, wantErr: "args must be a list"},
		{name: "entry not a mapping", args: `- user.email`, wantErr: "[0]: must be a mapping"},
		{name: "missing key", args: `- value: main`, wantErr: "[0]: missing 'key'"},
		{name: "missing value", args: `- key: user.email`, wantErr: "[0]: missing 'value'"},
		{
			name:    "prompt points to variables",
			args:    "- key: init.defaultBranch\n  value: main\n- key: user.email\n  prompt: Email?",
			wantErr: "[1]: 'prompt' is no longer supported",
		},
		{name: "bad expression", args: "- key: user.email\n  value: ${ (( }", wantErr: "[0].value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitConfig(argsNode(t, tt.args), testContext())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
