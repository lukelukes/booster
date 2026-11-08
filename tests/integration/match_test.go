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
