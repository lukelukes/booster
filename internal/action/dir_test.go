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

func TestDirCreate_Check(t *testing.T) {
	t.Run("reports missing directories as creates", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "a", "b")
		existing := filepath.Join(dir, "existing")
		require.NoError(t, os.Mkdir(existing, 0o755))

		action := &DirCreate{Paths: []string{existing, missing}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, change.Kind)
		assert.Equal(t, 2, change.Total)
		assert.Equal(t, []model.Item{{Name: missing}}, change.Items)
	})

	t.Run("reports no change when every directory exists", func(t *testing.T) {
		dir := t.TempDir()

		action := &DirCreate{Paths: []string{dir}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
		assert.Empty(t, change.Items)
	})

	t.Run("fails when the path is a file", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "file")
		require.NoError(t, os.WriteFile(file, []byte("content"), 0o644))

		action := &DirCreate{Paths: []string{file}}

		_, err := action.Check(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})
}

func TestDirCreate_Apply(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")

	action := &DirCreate{Paths: []string{nested}}

	require.NoError(t, action.Apply(context.Background()))

	info, err := os.Stat(nested)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	require.NoError(t, action.Apply(context.Background()))

	change, err := action.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, model.ChangeNone, change.Kind)
}

func TestNewDirCreate(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		wantPaths []string
		wantErr   string
	}{
		{
			name:      "resolves expressions",
			args:      "- ${ vars.config_dir }\n- ~/.local/bin\n",
			wantPaths: []string{"/home/tester/.config", "~/.local/bin"},
		},
		{
			name:      "accepts an empty list",
			args:      "[]",
			wantPaths: nil,
		},
		{
			name:    "rejects a scalar",
			args:    "not a list",
			wantErr: "args must be a list of paths",
		},
		{
			name:    "rejects a mapping",
			args:    "path: ~/.config",
			wantErr: "args must be a list of paths",
		},
		{
			name:    "reports which path failed to resolve",
			args:    "- ~/.config\n- ${ nope() }\n",
			wantErr: "[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := NewDirCreate(argsNode(t, tt.args), testContext())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPaths, created.(*DirCreate).Paths)
		})
	}
}
