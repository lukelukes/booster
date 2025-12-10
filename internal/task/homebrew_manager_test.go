package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

// mockBrewPathFinder creates a simple mock BrewPathFinder for testing.
func mockBrewPathFinder(path string, found bool) BrewPathFinder {
	return func() (string, bool) {
		return path, found
	}
}

// --- Homebrew Path Detection Tests ---

func TestHomebrewManager_brewPath_UsesDiscoveredPath(t *testing.T) {
	// When PathFinder finds brew, brewPath() should return the full path
	finder := mockBrewPathFinder("/opt/homebrew/bin/brew", true)
	manager := NewHomebrewManager(nil, finder)
	assert.Equal(t, "/opt/homebrew/bin/brew", manager.brewPath())
}

func TestHomebrewManager_brewPath_FallsBackToBrewWhenNotFound(t *testing.T) {
	// When PathFinder doesn't find brew, brewPath() should return "brew"
	finder := mockBrewPathFinder("", false)
	manager := NewHomebrewManager(nil, finder)
	assert.Equal(t, "brew", manager.brewPath())
}

func TestHomebrewManager_UsesFullPathWhenDiscovered(t *testing.T) {
	// Simulate brew being installed at a known path
	finder := mockBrewPathFinder("/opt/homebrew/bin/brew", true)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			// Should be called with full path, not just "brew"
			assert.Equal(t, "/opt/homebrew/bin/brew", name, "should use discovered full path")
			return []byte("git\ncurl\n"), nil
		},
	}

	manager := NewHomebrewManager(mock, finder)
	_, err := manager.ListInstalled(context.Background())
	require.NoError(t, err)

	// Verify the command was called with the full path
	require.Len(t, mock.Calls, 1)
	assert.Equal(t, "/opt/homebrew/bin/brew", mock.Calls[0].Name)
}

func TestHomebrewManager_DetectsFreshlyInstalledBrew(t *testing.T) {
	// Simulate the scenario where brew is installed mid-session
	// First call: brew not found
	// Second call: brew found (simulating installation happened)

	callCount := 0
	finder := func() (string, bool) {
		callCount++
		if callCount == 1 {
			return "", false // Not found on first call
		}
		return "/opt/homebrew/bin/brew", true // Found on second call
	}

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	manager := NewHomebrewManager(mock, finder)

	// First call - brew not found, should use "brew" fallback
	_, _ = manager.ListInstalled(context.Background())
	require.Len(t, mock.Calls, 1)
	assert.Equal(t, "brew", mock.Calls[0].Name, "first call should use fallback")

	// Second call - brew now found at known path
	_, _ = manager.ListInstalled(context.Background())
	require.Len(t, mock.Calls, 2)
	assert.Equal(t, "/opt/homebrew/bin/brew", mock.Calls[1].Name, "second call should use discovered path")
}

// --- HomebrewManager Tests ---

func TestHomebrewManager_Name(t *testing.T) {
	manager := NewHomebrewManager(nil, nil)
	assert.Equal(t, "homebrew", manager.Name())
}

func TestHomebrewManager_SupportsCasks(t *testing.T) {
	manager := NewHomebrewManager(nil, nil)
	assert.True(t, manager.SupportsCasks())
}

func TestHomebrewManager_ListInstalled(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "brew" && len(args) >= 2 && args[0] == "list" && args[1] == "--formulae" {
				return []byte("git\ncurl\nripgrep\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	manager := NewHomebrewManager(mock, finder)
	installed, err := manager.ListInstalled(context.Background())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"git", "curl", "ripgrep"}, installed)
}

func TestHomebrewManager_ListInstalled_Empty(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	manager := NewHomebrewManager(mock, finder)
	installed, err := manager.ListInstalled(context.Background())

	require.NoError(t, err)
	assert.Empty(t, installed)
}

func TestHomebrewManager_ListInstalled_Error(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("brew not found")
		},
	}

	manager := NewHomebrewManager(mock, finder)
	_, err := manager.ListInstalled(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list installed")
}

func TestHomebrewManager_Install(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("==> Installing git\n==> Installing curl"), nil
		},
	}

	manager := NewHomebrewManager(mock, finder)
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

	manager := NewHomebrewManager(mock, nil)
	output, err := manager.Install(context.Background(), []string{})

	require.NoError(t, err)
	assert.Empty(t, output)
	assert.Empty(t, mock.Calls, "should not call brew for empty list")
}

func TestHomebrewManager_Install_Error(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Error: No available formula"), errors.New("exit status 1")
		},
	}

	manager := NewHomebrewManager(mock, finder)
	output, err := manager.Install(context.Background(), []string{"nonexistent"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "brew install")
	assert.Contains(t, output, "No available formula")
}

func TestHomebrewManager_ListInstalledCasks(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "brew" && len(args) >= 2 && args[0] == "list" && args[1] == "--casks" {
				return []byte("firefox\nvscode\niterm2\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	manager := NewHomebrewManager(mock, finder)
	casks, err := manager.ListInstalledCasks(context.Background())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"firefox", "vscode", "iterm2"}, casks)
}

func TestHomebrewManager_ListInstalledCasks_Empty(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	manager := NewHomebrewManager(mock, finder)
	casks, err := manager.ListInstalledCasks(context.Background())

	require.NoError(t, err)
	assert.Empty(t, casks)
}

func TestHomebrewManager_ListInstalledCasks_Error(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("brew error")
		},
	}

	manager := NewHomebrewManager(mock, finder)
	_, err := manager.ListInstalledCasks(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list installed casks")
}

func TestHomebrewManager_InstallCasks(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("==> Installing Cask firefox"), nil
		},
	}

	manager := NewHomebrewManager(mock, finder)
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

	manager := NewHomebrewManager(mock, nil)
	output, err := manager.InstallCasks(context.Background(), []string{})

	require.NoError(t, err)
	assert.Empty(t, output)
	assert.Empty(t, mock.Calls, "should not call brew for empty cask list")
}

func TestHomebrewManager_InstallCasks_Error(t *testing.T) {
	finder := mockBrewPathFinder("", false)

	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Error: Cask 'badcask' not found"), errors.New("exit status 1")
		},
	}

	manager := NewHomebrewManager(mock, finder)
	output, err := manager.InstallCasks(context.Background(), []string{"badcask"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "brew install casks")
	assert.Contains(t, output, "not found")
}
