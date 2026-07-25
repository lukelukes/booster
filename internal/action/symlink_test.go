package action

import (
	"booster/internal/model"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) string {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

func TestSymlinkCreate_Check(t *testing.T) {
	t.Run("reports a missing link as a create", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "zshrc"), "content")
		target := filepath.Join(dir, "home", ".zshrc")

		action := &SymlinkCreate{Links: []SourceTarget{{Source: source, Target: target}}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, change.Kind)
		assert.Equal(t, 1, change.Total)
		assert.Equal(t, []model.Item{{Name: target, To: source}}, change.Items)
	})

	t.Run("reports no change when the link already points at the source", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "zshrc"), "content")
		target := filepath.Join(dir, ".zshrc")
		require.NoError(t, os.Symlink(source, target))

		action := &SymlinkCreate{Links: []SourceTarget{{Source: source, Target: target}}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
		assert.Empty(t, change.Items)
	})

	t.Run("reports a link pointing elsewhere as an update", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "zshrc"), "content")
		other := writeFile(t, filepath.Join(dir, "other"), "other")
		target := filepath.Join(dir, ".zshrc")
		require.NoError(t, os.Symlink(other, target))

		action := &SymlinkCreate{Links: []SourceTarget{{Source: source, Target: target}}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeUpdate, change.Kind)
		assert.Equal(t, []model.Item{{Name: target, From: other, To: source}}, change.Items)
	})

	t.Run("fails when the target is not a symlink", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "zshrc"), "content")
		target := writeFile(t, filepath.Join(dir, ".zshrc"), "existing")

		action := &SymlinkCreate{Links: []SourceTarget{{Source: source, Target: target}}}

		_, err := action.Check(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a symlink")
	})

	t.Run("fails when the source does not exist", func(t *testing.T) {
		dir := t.TempDir()

		action := &SymlinkCreate{Links: []SourceTarget{{
			Source: filepath.Join(dir, "missing"),
			Target: filepath.Join(dir, ".zshrc"),
		}}}

		_, err := action.Check(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "source does not exist")
	})
}

func TestSymlinkCreate_Apply(t *testing.T) {
	t.Run("creates the link and its parent directories", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "zshrc"), "content")
		target := filepath.Join(dir, "home", "nested", ".zshrc")

		action := &SymlinkCreate{Links: []SourceTarget{{Source: source, Target: target}}}

		require.NoError(t, action.Apply(context.Background()))

		dest, err := os.Readlink(target)
		require.NoError(t, err)
		assert.Equal(t, source, dest)

		require.NoError(t, action.Apply(context.Background()))

		change, err := action.Check(context.Background())
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
	})

	t.Run("repoints a link that points elsewhere", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "zshrc"), "content")
		other := writeFile(t, filepath.Join(dir, "other"), "other")
		target := filepath.Join(dir, ".zshrc")
		require.NoError(t, os.Symlink(other, target))

		action := &SymlinkCreate{Links: []SourceTarget{{Source: source, Target: target}}}

		require.NoError(t, action.Apply(context.Background()))

		dest, err := os.Readlink(target)
		require.NoError(t, err)
		assert.Equal(t, source, dest)
	})

	t.Run("links a relative source as an absolute path", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "dotfiles", "zshrc"), "export PATH=...")
		target := filepath.Join(dir, ".zshrc")
		t.Chdir(dir)

		action := &SymlinkCreate{Links: []SourceTarget{{Source: "dotfiles/zshrc", Target: target}}}

		require.NoError(t, action.Apply(context.Background()))

		dest, err := os.Readlink(target)
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(dest), "symlink should contain an absolute path, got: %s", dest)

		content, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, "export PATH=...", string(content))
	})
}

func TestNewSymlinkCreate(t *testing.T) {
	args := argsNode(t, `
- source: dotfiles/zshrc
  target: ${ vars.config_dir }/zshrc
`)

	created, err := NewSymlinkCreate(args, testContext())

	require.NoError(t, err)
	assert.Equal(t, []SourceTarget{{
		Source: "dotfiles/zshrc",
		Target: "/home/tester/.config/zshrc",
	}}, created.(*SymlinkCreate).Links)
}
