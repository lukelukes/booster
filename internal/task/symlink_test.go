package task

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSymlinkCreate_Run(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, dir string) (source, target string)
		wantStatus    Status
		wantMsg       string
		wantErr       string
		verifySymlink bool
	}{
		{
			name: "creates new symlink",
			setup: func(t *testing.T, dir string) (string, string) {
				source := filepath.Join(dir, "source.txt")
				target := filepath.Join(dir, "link.txt")
				require.NoError(t, os.WriteFile(source, []byte("content"), 0o644))
				return source, target
			},
			wantStatus:    StatusDone,
			wantMsg:       "created",
			verifySymlink: true,
		},
		{
			name: "skips existing correct symlink",
			setup: func(t *testing.T, dir string) (string, string) {
				source := filepath.Join(dir, "source.txt")
				target := filepath.Join(dir, "link.txt")
				require.NoError(t, os.WriteFile(source, []byte("content"), 0o644))
				require.NoError(t, os.Symlink(source, target))
				return source, target
			},
			wantStatus: StatusSkipped,
			wantMsg:    "already exists",
		},
		{
			name: "fails when target is regular file",
			setup: func(t *testing.T, dir string) (string, string) {
				source := filepath.Join(dir, "source.txt")
				target := filepath.Join(dir, "existing.txt")
				require.NoError(t, os.WriteFile(source, []byte("source"), 0o644))
				require.NoError(t, os.WriteFile(target, []byte("existing"), 0o644))
				return source, target
			},
			wantStatus: StatusFailed,
			wantErr:    "exists but is not a symlink",
		},
		{
			name: "fails when target is directory",
			setup: func(t *testing.T, dir string) (string, string) {
				source := filepath.Join(dir, "source.txt")
				target := filepath.Join(dir, "existingdir")
				require.NoError(t, os.WriteFile(source, []byte("source"), 0o644))
				require.NoError(t, os.Mkdir(target, 0o755))
				return source, target
			},
			wantStatus: StatusFailed,
			wantErr:    "exists but is not a symlink",
		},
		{
			name: "fails when symlink points elsewhere",
			setup: func(t *testing.T, dir string) (string, string) {
				source := filepath.Join(dir, "source.txt")
				otherSource := filepath.Join(dir, "other.txt")
				target := filepath.Join(dir, "link.txt")
				require.NoError(t, os.WriteFile(source, []byte("source"), 0o644))
				require.NoError(t, os.WriteFile(otherSource, []byte("other"), 0o644))
				require.NoError(t, os.Symlink(otherSource, target))
				return source, target
			},
			wantStatus: StatusFailed,
			wantErr:    "points to different source",
		},
		{
			name: "fails when source does not exist",
			setup: func(t *testing.T, dir string) (string, string) {
				source := filepath.Join(dir, "nonexistent.txt")
				target := filepath.Join(dir, "link.txt")
				return source, target
			},
			wantStatus: StatusFailed,
			wantErr:    "source does not exist",
		},
		{
			name: "creates parent directories",
			setup: func(t *testing.T, dir string) (string, string) {
				source := filepath.Join(dir, "source.txt")
				target := filepath.Join(dir, "nested", "deep", "link.txt")
				require.NoError(t, os.WriteFile(source, []byte("content"), 0o644))
				return source, target
			},
			wantStatus:    StatusDone,
			verifySymlink: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source, target := tt.setup(t, dir)

			task := &SymlinkCreate{Source: source, Target: target}
			result := task.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, result.Message)
			}
			if tt.wantErr != "" {
				require.Error(t, result.Error)
				assert.Contains(t, result.Error.Error(), tt.wantErr)
			}
			if tt.verifySymlink {
				linkTarget, err := os.Readlink(target)
				require.NoError(t, err)
				assert.Equal(t, source, linkTarget)
			}
		})
	}
}

func TestSymlinkCreate_Idempotency(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	target := filepath.Join(dir, "link.txt")

	require.NoError(t, os.WriteFile(source, []byte("content"), 0o644))

	task := &SymlinkCreate{Source: source, Target: target}

	// First run: creates
	result1 := task.Run(context.Background())
	assert.Equal(t, StatusDone, result1.Status)

	// Second run: skips
	result2 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result2.Status)

	// Third run: still skips
	result3 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result3.Status)
}

func TestSymlinkCreate_Name(t *testing.T) {
	task := &SymlinkCreate{Source: "dotfiles/zshrc", Target: "~/.zshrc"}
	assert.Equal(t, "link dotfiles/zshrc → ~/.zshrc", task.Name())
}

// Factory tests

func TestNewSymlinkCreate(t *testing.T) {
	tests := []struct {
		name        string
		args        any
		wantLen     int
		wantNames   []string
		wantErr     string
		wantErrArgs []string
	}{
		{
			name: "valid args",
			args: []any{
				map[string]any{"source": "zsh/zshrc", "target": "~/.zshrc"},
				map[string]any{"source": "vim/vimrc", "target": "~/.vimrc"},
			},
			wantLen:   2,
			wantNames: []string{"link zsh/zshrc → ~/.zshrc", "link vim/vimrc → ~/.vimrc"},
		},
		{
			name:    "empty list",
			args:    []any{},
			wantLen: 0,
		},
		{
			name:    "not a list",
			args:    "not a list",
			wantErr: "must be a list",
		},
		{
			name:        "not a map",
			args:        []any{"not a map"},
			wantErr:     "must be a map",
			wantErrArgs: []string{"arg 1"},
		},
		{
			name: "missing source",
			args: []any{
				map[string]any{"target": "~/.zshrc"},
			},
			wantErr:     "missing 'source'",
			wantErrArgs: []string{"arg 1"},
		},
		{
			name: "missing target",
			args: []any{
				map[string]any{"source": "zsh/zshrc"},
			},
			wantErr:     "missing 'target'",
			wantErrArgs: []string{"arg 1"},
		},
		{
			name: "source not string",
			args: []any{
				map[string]any{"source": 123, "target": "~/.zshrc"},
			},
			wantErr:     "'source' must be a string",
			wantErrArgs: []string{"arg 1"},
		},
		{
			name: "target not string",
			args: []any{
				map[string]any{"source": "zsh/zshrc", "target": 123},
			},
			wantErr:     "'target' must be a string",
			wantErrArgs: []string{"arg 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := NewSymlinkCreate(tt.args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				for _, arg := range tt.wantErrArgs {
					assert.Contains(t, err.Error(), arg)
				}
				return
			}

			require.NoError(t, err)
			assert.Len(t, tasks, tt.wantLen)
			for i, name := range tt.wantNames {
				assert.Equal(t, name, tasks[i].Name())
			}
		})
	}
}
