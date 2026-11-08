package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanCmd(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "dotfiles", "zshrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
	require.NoError(t, os.WriteFile(source, []byte("export X=1"), 0o644))
	newDir := filepath.Join(dir, "home", ".config")
	link := filepath.Join(dir, "home", ".zshrc")
	excluded := filepath.Join(dir, "never")

	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, fmt.Appendf(nil, `version: "1"
tasks:
  - name: dirs
    action: dir.create
    args: [%q]
  - name: elsewhere
    action: dir.create
    when: ${ os == "plan9" }
    args: [%q]
  - name: links
    action: symlink.create
    args:
      - {source: %q, target: %q}
`, newDir, excluded, source, link), 0o644))

	cli := &CLI{Config: configPath}

	t.Run("reports pending changes without touching the system", func(t *testing.T) {
		var out strings.Builder

		err := (&PlanCmd{}).run(cli, &out)

		require.ErrorIs(t, err, errChangesPending)
		assert.Contains(t, out.String(), "+ dirs")
		assert.Contains(t, out.String(), "+ links")
		assert.Contains(t, out.String(), link+"  → "+source)
		assert.NotContains(t, out.String(), "elsewhere")
		assert.Contains(t, out.String(), "2 to change · 0 up to date · 1 excluded")
		assert.NoDirExists(t, newDir)
		assert.NoFileExists(t, link)
	})

	t.Run("reports up to date once the system matches", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(newDir, 0o755))
		require.NoError(t, os.Symlink(source, link))
		var out strings.Builder

		err := (&PlanCmd{}).run(cli, &out)

		require.NoError(t, err)
		assert.Contains(t, out.String(), "✓ dirs")
		assert.Contains(t, out.String(), "✓ links")
		assert.Contains(t, out.String(), "0 to change · 2 up to date · 1 excluded")
	})
}

func TestPlanCmd_CompileErrorsStopBeforeChecking(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`version: "1"
tasks:
  - name: links
    action: symlink.create
    args:
      - {source: /does/not/exist, target: /nowhere}
  - name: mystery
    action: nope.nothing
`), 0o644))
	var out strings.Builder

	err := (&PlanCmd{}).run(&CLI{Config: configPath}, &out)

	require.Error(t, err)
	assert.NotErrorIs(t, err, errChangesPending)
	assert.Contains(t, err.Error(), `unknown action "nope.nothing"`)
	assert.NotContains(t, err.Error(), "source does not exist")
	assert.Empty(t, out.String())
}
