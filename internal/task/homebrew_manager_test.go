package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- HomebrewManager Tests ---

func TestHomebrewManager_Name(t *testing.T) {
	manager := NewHomebrewManager(nil)
	assert.Equal(t, "homebrew", manager.Name())
}

func TestHomebrewManager_SupportsCasks(t *testing.T) {
	manager := NewHomebrewManager(nil)
	assert.True(t, manager.SupportsCasks())
}

func TestHomebrewManager_ListInstalled(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "brew" && len(args) >= 2 && args[0] == "list" && args[1] == "--formulae" {
				return []byte("git\ncurl\nripgrep\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	manager := NewHomebrewManager(mock)
	installed, err := manager.ListInstalled(context.Background())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"git", "curl", "ripgrep"}, installed)
}

func TestHomebrewManager_ListInstalled_Empty(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	manager := NewHomebrewManager(mock)
	installed, err := manager.ListInstalled(context.Background())

	require.NoError(t, err)
	assert.Empty(t, installed)
}

func TestHomebrewManager_ListInstalled_Error(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("brew not found")
		},
	}

	manager := NewHomebrewManager(mock)
	_, err := manager.ListInstalled(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list installed")
}

func TestHomebrewManager_Install(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("==> Installing git\n==> Installing curl"), nil
		},
	}

	manager := NewHomebrewManager(mock)
	output, err := manager.Install(context.Background(), []string{"git", "curl"})

	require.NoError(t, err)
	assert.Contains(t, output, "Installing git")

	// Verify brew was called with correct args
	require.Len(t, mock.Calls, 1)
	call := mock.Calls[0]
	assert.Equal(t, "brew", call.Name)
	assert.Contains(t, call.Args, "install")
	assert.Contains(t, call.Args, "git")
	assert.Contains(t, call.Args, "curl")
}

func TestHomebrewManager_Install_Empty(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}

	manager := NewHomebrewManager(mock)
	output, err := manager.Install(context.Background(), []string{})

	require.NoError(t, err)
	assert.Empty(t, output)
	assert.Empty(t, mock.Calls, "should not call brew for empty list")
}

func TestHomebrewManager_Install_Error(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Error: No available formula"), errors.New("exit status 1")
		},
	}

	manager := NewHomebrewManager(mock)
	output, err := manager.Install(context.Background(), []string{"nonexistent"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "brew install")
	assert.Contains(t, output, "No available formula")
}

func TestHomebrewManager_ListInstalledCasks(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "brew" && len(args) >= 2 && args[0] == "list" && args[1] == "--casks" {
				return []byte("firefox\nvscode\niterm2\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	manager := NewHomebrewManager(mock)
	casks, err := manager.ListInstalledCasks(context.Background())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"firefox", "vscode", "iterm2"}, casks)
}

func TestHomebrewManager_ListInstalledCasks_Empty(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	manager := NewHomebrewManager(mock)
	casks, err := manager.ListInstalledCasks(context.Background())

	require.NoError(t, err)
	assert.Empty(t, casks)
}

func TestHomebrewManager_ListInstalledCasks_Error(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("brew error")
		},
	}

	manager := NewHomebrewManager(mock)
	_, err := manager.ListInstalledCasks(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list installed casks")
}

func TestHomebrewManager_InstallCasks(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("==> Installing Cask firefox"), nil
		},
	}

	manager := NewHomebrewManager(mock)
	output, err := manager.InstallCasks(context.Background(), []string{"firefox", "vscode"})

	require.NoError(t, err)
	assert.Contains(t, output, "Installing Cask")

	// Verify brew was called with --cask flag
	require.Len(t, mock.Calls, 1)
	call := mock.Calls[0]
	assert.Equal(t, "brew", call.Name)
	assert.Contains(t, call.Args, "install")
	assert.Contains(t, call.Args, "--cask")
	assert.Contains(t, call.Args, "firefox")
	assert.Contains(t, call.Args, "vscode")
}

func TestHomebrewManager_InstallCasks_Empty(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}

	manager := NewHomebrewManager(mock)
	output, err := manager.InstallCasks(context.Background(), []string{})

	require.NoError(t, err)
	assert.Empty(t, output)
	assert.Empty(t, mock.Calls, "should not call brew for empty cask list")
}

func TestHomebrewManager_InstallCasks_Error(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Error: Cask 'badcask' not found"), errors.New("exit status 1")
		},
	}

	manager := NewHomebrewManager(mock)
	output, err := manager.InstallCasks(context.Background(), []string{"badcask"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "brew install casks")
	assert.Contains(t, output, "not found")
}
