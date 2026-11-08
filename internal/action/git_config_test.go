package action_test

import (
	"booster/internal/action"
	"booster/internal/cmdexec"
	"booster/internal/config"
	"booster/internal/expr"
	"booster/internal/model"
	"booster/internal/run"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func isolatedGit(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	path := filepath.Join(home, ".gitconfig")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return path
}

func git(t *testing.T, args ...string) string {
	t.Helper()

	output, err := exec.Command("git", append([]string{"config", "--global"}, args...)...).CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}

func compileGit(src string) (run.Compiled, error) {
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		return run.Compiled{}, err
	}

	ctx := &expr.Context{OS: "linux", Vars: map[string]any{"git_email": "me@example.com"}}
	return run.Compile(cfg.Tasks, ctx, action.NewRegistry(cmdexec.DefaultRunner()))
}

func planGit(t *testing.T, src string) (run.PlanResult, error) {
	t.Helper()

	compiled, err := compileGit(src)
	require.NoError(t, err)
	return run.Plan(context.Background(), compiled)
}

func applyErr(result run.PlanResult) error {
	var failure error
	run.Apply(context.Background(), result, func(e model.Event) {
		if done, ok := e.(model.TaskDone); ok && done.Result.Err != nil && failure == nil {
			failure = done.Result.Err
		}
	})
	return failure
}

const gitConfigTasks = `
tasks:
  - action: git.config
    args:
      - {key: user.email, value: "${ vars.git_email }"}
      - {key: init.defaultBranch, value: main}
      - {key: pull.rebase, value: "true"}
`

func TestGitConfig(t *testing.T) {
	t.Run("plans each differing key as old → new, applies it, and converges", func(t *testing.T) {
		isolatedGit(t)
		git(t, "user.email", "old@example.com")
		git(t, "init.defaultBranch", "main")

		result, err := planGit(t, gitConfigTasks)
		require.NoError(t, err)
		change := result.Steps[0].Change
		assert.Equal(t, model.ChangeUpdate, change.Kind)
		assert.Equal(t, 3, change.Total)
		assert.Equal(t, []model.Item{
			{Name: "user.email", From: "old@example.com", To: "me@example.com"},
			{Name: "pull.rebase", To: "true"},
		}, change.Items)
		assert.False(t, change.NeedsSudo)

		apply(t, result)

		assert.Equal(t, "me@example.com\n", git(t, "--get", "user.email"))
		assert.Equal(t, "true\n", git(t, "--get", "pull.rebase"))
		result, err = planGit(t, gitConfigTasks)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, result.Steps[0].Change.Kind)
	})

	t.Run("values keep surrounding whitespace and may be empty", func(t *testing.T) {
		isolatedGit(t)
		tasks := `
tasks:
  - action: git.config
    args:
      - {key: user.name, value: "  padded  "}
      - {key: commit.template, value: ""}
`

		result, err := planGit(t, tasks)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, result.Steps[0].Change.Kind)
		assert.Equal(t, []model.Item{
			{Name: "user.name", To: "  padded  "},
			{Name: "commit.template", To: ""},
		}, result.Steps[0].Change.Items)

		apply(t, result)

		assert.Equal(t, "  padded  \n", git(t, "--get", "user.name"))
		assert.Equal(t, "\n", git(t, "--get", "commit.template"))
		result, err = planGit(t, tasks)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, result.Steps[0].Change.Kind)
	})

	t.Run("a broken global config fails the plan instead of reading as unset", func(t *testing.T) {
		path := isolatedGit(t)
		require.NoError(t, os.WriteFile(path, []byte("[broken\n"), 0o644))

		_, err := planGit(t, gitConfigTasks)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read git config user.email")
	})

	t.Run("apply writes nothing when the config breaks after planning", func(t *testing.T) {
		path := isolatedGit(t)
		result, err := planGit(t, gitConfigTasks)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte("[broken\n"), 0o644))

		err = applyErr(result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read git config user.email")
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "[broken\n", string(content))
	})

	t.Run("a failed write reports the key and git output", func(t *testing.T) {
		path := isolatedGit(t)
		t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(filepath.Dir(path), "missing", ".gitconfig"))
		result, err := planGit(t, gitConfigTasks)
		require.NoError(t, err)

		err = applyErr(result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "git config user.email")
		assert.Contains(t, err.Error(), "could not lock config file")
	})

	t.Run("unknown until git is installed, and apply says so", func(t *testing.T) {
		isolatedGit(t)
		t.Setenv("PATH", t.TempDir())

		result, err := planGit(t, gitConfigTasks)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeUnknown, result.Steps[0].Change.Kind)

		err = applyErr(result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "git not found in PATH")
	})
}

func TestGitConfig_InvalidArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "not a list", args: `{key: user.email}`, wantErr: "args must be a list"},
		{name: "entry not a mapping", args: `[user.email]`, wantErr: "[0]: must be a mapping"},
		{name: "missing key", args: `[{value: main}]`, wantErr: "[0]: missing 'key'"},
		{name: "missing value", args: `[{key: user.email}]`, wantErr: "[0]: missing 'value'"},
		{name: "null value", args: `[{key: user.email, value: }]`, wantErr: "[0]: missing 'value'"},
		{
			name:    "prompt points to variables",
			args:    `[{key: init.defaultBranch, value: main}, {key: user.email, prompt: "Email?"}]`,
			wantErr: "[1]: 'prompt' is no longer supported",
		},
		{name: "bad expression", args: `[{key: user.email, value: "${ (( }"}]`, wantErr: "[0].value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileGit("tasks:\n  - action: git.config\n    args: " + tt.args + "\n")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
