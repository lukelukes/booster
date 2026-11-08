package integration

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"booster/internal/model"
	"booster/internal/tui"
	"booster/internal/variable"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatch_SelectsActionArgsFromConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, fmt.Appendf(nil, `version: "1"
tasks:
  - action: dir.create
    args:
      match:
        on: ${ os_family }
        cases:
          darwin:
            - "%[1]s/mac/${ profile }"
          linux:
            - "%[1]s/linux/${ profile }"
`, dir), 0o644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	exprCtx := &expr.Context{OS: "arch", OSFamily: "linux", Profile: "work"}
	args, err := expr.ResolveMatches(cfg.Tasks[0].Args, exprCtx)
	require.NoError(t, err)
	created, err := action.NewDirCreate(args, exprCtx)
	require.NoError(t, err)

	selected := filepath.Join(dir, "linux", "work")
	change, err := created.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, model.ChangeCreate, change.Kind)
	assert.Equal(t, []model.Item{{Name: selected}}, change.Items)

	require.NoError(t, created.Apply(context.Background()))

	assert.DirExists(t, selected)
	assert.NoDirExists(t, filepath.Join(dir, "mac"))
	change, err = created.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, model.ChangeNone, change.Kind)
}

func TestMatch_SelectsVariableDefaultFromConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`version: "1"
variables:
  Email:
    prompt: Your email
    default:
      match:
        on: ${ profile }
        cases:
          work: me@work.example.com
        default: me@example.com
tasks: []
`), 0o644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	def, err := cfg.Variables["Email"].ResolveDefault(&expr.Context{Profile: "work"})
	require.NoError(t, err)

	store := variable.NewFileStore(filepath.Join(dir, "values.yaml"))
	collector := tui.NewPromptCollector().WithInput(strings.NewReader("\n"))
	resolver := variable.NewResolver(store, variable.WithCollector(collector))

	result, err := resolver.Resolve([]variable.Definition{
		{Name: "Email", Prompt: cfg.Variables["Email"].Prompt, Default: def},
	})

	require.NoError(t, err)
	assert.Equal(t, "me@work.example.com", result["Email"])
	stored, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "me@work.example.com", stored["Email"])
}

type fakeBrew struct {
	t        *testing.T
	formulae []string
	casks    []string
	calls    []string
}

func (b *fakeBrew) LookPath(name string) (string, error) {
	if name != "brew" {
		b.t.Errorf("unexpected lookup of %q", name)
		return "", fmt.Errorf("unexpected lookup of %q", name)
	}
	return "/usr/local/bin/brew", nil
}

func (b *fakeBrew) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := strings.Join(append([]string{filepath.Base(name)}, args...), " ")
	b.calls = append(b.calls, call)

	switch {
	case call == "brew list --formulae":
		return []byte(strings.Join(b.formulae, "\n")), nil
	case call == "brew list --casks":
		return []byte(strings.Join(b.casks, "\n")), nil
	case strings.HasPrefix(call, "brew install --cask "):
		b.casks = append(b.casks, args[2:]...)
		return nil, nil
	case strings.HasPrefix(call, "brew install "):
		b.formulae = append(b.formulae, args[1:]...)
		return nil, nil
	}
	b.t.Errorf("unexpected command %q", call)
	return nil, fmt.Errorf("unexpected command %q", call)
}

func TestMatch_SelectsMixedPackagesAndCasksFromConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`version: "1"
tasks:
  - action: pkg.install
    args:
      match:
        on: ${ os }
        cases:
          arch: [git-delta, github-cli, ghostty-linux]
          darwin:
            - packages: [delta, "${ vars.editor }"]
            - casks: [ghostty]
`), 0o644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	exprCtx := &expr.Context{OS: "darwin", OSFamily: "darwin", Vars: map[string]any{"editor": "neovim"}}
	args, err := expr.ResolveMatches(cfg.Tasks[0].Args, exprCtx)
	require.NoError(t, err)
	brew := &fakeBrew{t: t, formulae: []string{"git"}}
	install, err := action.NewPkgInstall(args, exprCtx, brew)
	require.NoError(t, err)

	change, err := install.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, model.ChangeCreate, change.Kind)
	assert.Equal(t, []model.Item{{Name: "delta"}, {Name: "neovim"}, {Name: "ghostty"}}, change.Items)

	require.NoError(t, install.Apply(context.Background()))

	assert.Contains(t, brew.calls, "brew install delta neovim")
	assert.Contains(t, brew.calls, "brew install --cask ghostty")
	for _, call := range brew.calls {
		for _, unselected := range []string{"git-delta", "github-cli", "ghostty-linux", "${"} {
			assert.NotContains(t, call, unselected)
		}
	}
	change, err = install.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, model.ChangeNone, change.Kind)
}
