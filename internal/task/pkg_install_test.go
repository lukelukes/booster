package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- PkgInstall Task Tests ---

func TestPkgInstall_SkipsWhenAllInstalled(t *testing.T) {
	manager := newMockManager("paru", false)
	manager.installed["git"] = true
	manager.installed["curl"] = true

	task := &PkgInstall{
		Packages: []string{"git", "curl"},
		Manager:  manager,
		OS:       "arch",
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "all packages already installed", result.Message)
	assert.Empty(t, manager.installCalls, "should not call install")
}

func TestPkgInstall_InstallsMissingPackages(t *testing.T) {
	manager := newMockManager("paru", false)
	manager.installed["git"] = true // git already installed

	task := &PkgInstall{
		Packages: []string{"git", "curl", "ripgrep"},
		Manager:  manager,
		OS:       "arch",
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	// New format: "3 pkgs (1 existed, 2 installed)"
	assert.Contains(t, result.Message, "3 pkgs")
	assert.Contains(t, result.Message, "1 existed")
	assert.Contains(t, result.Message, "2 installed")
	require.Len(t, manager.installCalls, 1)
	assert.ElementsMatch(t, []string{"curl", "ripgrep"}, manager.installCalls[0])
}

func TestPkgInstall_InstallsAllWhenNoneInstalled(t *testing.T) {
	manager := newMockManager("paru", false)

	task := &PkgInstall{
		Packages: []string{"git", "curl"},
		Manager:  manager,
		OS:       "arch",
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	require.Len(t, manager.installCalls, 1)
	assert.ElementsMatch(t, []string{"git", "curl"}, manager.installCalls[0])
}

func TestPkgInstall_FailsOnInstallError(t *testing.T) {
	manager := newMockManager("paru", false)
	manager.installErr = errors.New("network error")

	task := &PkgInstall{
		Packages: []string{"git"},
		Manager:  manager,
		OS:       "arch",
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "network error")
	assert.NotEmpty(t, result.Output, "should capture output on failure")
}

func TestPkgInstall_CapturesOutputOnSuccess(t *testing.T) {
	manager := newMockManager("paru", false)

	task := &PkgInstall{
		Packages: []string{"git"},
		Manager:  manager,
		OS:       "arch",
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.NotEmpty(t, result.Output, "should capture output on success")
}

func TestPkgInstall_WarnsOnCasksWithNonDarwin(t *testing.T) {
	manager := newMockManager("paru", false)

	task := &PkgInstall{
		Packages: []string{"git"},
		Casks:    []string{"firefox"},
		Manager:  manager,
		OS:       "arch", // Not darwin
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Message, "casks are only supported on macOS")
	assert.Contains(t, result.Error.Error(), "not darwin")
}

func TestPkgInstall_InstallsCasksOnDarwin(t *testing.T) {
	manager := newMockManager("homebrew", true)

	task := &PkgInstall{
		Packages: []string{"git"},
		Casks:    []string{"firefox", "vscode"},
		Manager:  manager,
		OS:       "darwin",
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	// New format: "1 pkg installed | 2 casks installed"
	assert.Contains(t, result.Message, "1 pkg")
	assert.Contains(t, result.Message, "2 casks")
	require.Len(t, manager.caskCalls, 1)
	assert.ElementsMatch(t, []string{"firefox", "vscode"}, manager.caskCalls[0])
}

func TestPkgInstall_SkipsCasksAlreadyInstalled(t *testing.T) {
	manager := newMockManager("homebrew", true)
	manager.installed["git"] = true
	manager.casksInstalled["firefox"] = true

	task := &PkgInstall{
		Packages: []string{"git"},
		Casks:    []string{"firefox"},
		Manager:  manager,
		OS:       "darwin",
	}

	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Empty(t, manager.installCalls)
	assert.Empty(t, manager.caskCalls)
}

func TestPkgInstall_Idempotency(t *testing.T) {
	manager := newMockManager("paru", false)

	task := &PkgInstall{
		Packages: []string{"git", "curl"},
		Manager:  manager,
		OS:       "arch",
	}

	// First run: installs
	result1 := task.Run(context.Background())
	assert.Equal(t, StatusDone, result1.Status)

	// Second run: skips (packages now "installed" in mock)
	result2 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result2.Status)

	// Third run: still skips
	result3 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result3.Status)

	// Should have only called install once
	assert.Len(t, manager.installCalls, 1)
}

func TestPkgInstall_Name(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		packages []string
		casks    []string
	}{
		{
			name:     "few packages",
			packages: []string{"git", "curl"},
			expected: "install packages: git, curl",
		},
		{
			name:     "many packages",
			packages: []string{"a", "b", "c", "d", "e"},
			expected: "install packages: 5 packages",
		},
		{
			name:     "packages and casks",
			packages: []string{"git"},
			casks:    []string{"firefox"},
			expected: "install packages: git + casks: firefox",
		},
		{
			name:     "many casks",
			packages: []string{"git"},
			casks:    []string{"a", "b", "c", "d"},
			expected: "install packages: git + 4 casks",
		},
		{
			name:     "empty",
			packages: []string{},
			casks:    []string{},
			expected: "install packages: (none)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &PkgInstall{Packages: tt.packages, Casks: tt.casks}
			assert.Equal(t, tt.expected, task.Name())
		})
	}
}

func TestPkgInstall_Name_BoundaryConditions(t *testing.T) {
	// Tests CONDITIONALS_BOUNDARY mutations: <= 3 vs < 3 or <= 2
	tests := []struct {
		name         string
		packages     []string
		casks        []string
		expectInline bool // true = names inline, false = count
	}{
		{
			name:         "exactly 3 packages - should be inline",
			packages:     []string{"git", "curl", "vim"},
			expectInline: true,
		},
		{
			name:         "4 packages - should be count",
			packages:     []string{"git", "curl", "vim", "tmux"},
			expectInline: false,
		},
		{
			name:         "exactly 3 casks - should be inline",
			packages:     []string{"git"},
			casks:        []string{"firefox", "chrome", "slack"},
			expectInline: true,
		},
		{
			name:         "4 casks - should be count",
			packages:     []string{"git"},
			casks:        []string{"firefox", "chrome", "slack", "vscode"},
			expectInline: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &PkgInstall{Packages: tt.packages, Casks: tt.casks}
			name := task.Name()

			if tt.expectInline {
				// Should contain actual package/cask names
				if len(tt.packages) == 3 {
					assert.Contains(t, name, "git")
					assert.Contains(t, name, "curl")
					assert.Contains(t, name, "vim")
					assert.NotContains(t, name, "3 packages")
				}
				if len(tt.casks) == 3 {
					assert.Contains(t, name, "firefox")
					assert.NotContains(t, name, "3 casks")
				}
			} else {
				// Should contain count
				if len(tt.packages) > 3 {
					assert.Contains(t, name, "4 packages")
				}
				if len(tt.casks) > 3 {
					assert.Contains(t, name, "4 casks")
				}
			}
		})
	}
}

func TestFormatInstallStats(t *testing.T) {
	tests := []struct {
		name      string
		category  string
		skipped   int
		installed int
		expected  string
	}{
		{
			name:      "all installed",
			category:  "pkg",
			skipped:   0,
			installed: 3,
			expected:  "3 pkgs installed",
		},
		{
			name:      "all skipped",
			category:  "pkg",
			skipped:   5,
			installed: 0,
			expected:  "5 pkgs (all existed)",
		},
		{
			name:      "mixed packages",
			category:  "pkg",
			skipped:   31,
			installed: 2,
			expected:  "33 pkgs (31 existed, 2 installed)",
		},
		{
			name:      "mixed casks",
			category:  "cask",
			skipped:   10,
			installed: 5,
			expected:  "15 casks (10 existed, 5 installed)",
		},
		{
			name:      "single package installed",
			category:  "pkg",
			skipped:   0,
			installed: 1,
			expected:  "1 pkgs installed",
		},
		{
			name:      "single package skipped",
			category:  "pkg",
			skipped:   1,
			installed: 0,
			expected:  "1 pkg (all existed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatInstallStats(tt.category, tt.skipped, tt.installed)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- PacmanManager Tests ---

func TestPacmanManager_ListInstalled(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "pacman" && len(args) >= 1 && args[0] == "-Qq" {
				return []byte("git\ncurl\nripgrep\n"), nil
			}
			return nil, errors.New("unexpected")
		},
	}

	manager := NewPacmanManager(mock)
	installed, err := manager.ListInstalled(context.Background())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"git", "curl", "ripgrep"}, installed)
}

func TestPacmanManager_ListInstalled_Error(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("pacman error")
		},
	}

	manager := NewPacmanManager(mock)
	_, err := manager.ListInstalled(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list installed")
}

func TestPacmanManager_ListInstalled_Empty(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	manager := NewPacmanManager(mock)
	installed, err := manager.ListInstalled(context.Background())

	require.NoError(t, err)
	assert.Empty(t, installed)
}

func TestPacmanManager_Install(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}

	manager := NewPacmanManager(mock)
	output, err := manager.Install(context.Background(), []string{"git", "curl"})

	require.NoError(t, err)
	assert.Equal(t, "ok", output)

	// Verify paru was called with correct args
	require.Len(t, mock.Calls, 1)
	call := mock.Calls[0]
	assert.Equal(t, "paru", call.Name)
	assert.Contains(t, call.Args, "-S")
	assert.Contains(t, call.Args, "--noconfirm")
	assert.Contains(t, call.Args, "--needed")
	assert.Contains(t, call.Args, "git")
	assert.Contains(t, call.Args, "curl")
}

func TestPacmanManager_Install_Error(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("error output from paru"), errors.New("dependency conflict")
		},
	}

	manager := NewPacmanManager(mock)
	output, err := manager.Install(context.Background(), []string{"git"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "paru install")
	assert.Equal(t, "error output from paru", output)
}

func TestPacmanManager_DoesNotSupportCasks(t *testing.T) {
	manager := NewPacmanManager(nil)

	assert.False(t, manager.SupportsCasks())

	casks, err := manager.ListInstalledCasks(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, casks)

	output, err := manager.InstallCasks(context.Background(), []string{"anything"})
	assert.NoError(t, err)
	assert.Empty(t, output)
}

func TestPacmanManager_Name(t *testing.T) {
	manager := NewPacmanManager(nil)
	assert.Equal(t, "paru", manager.Name())

	manager.Helper = "yay"
	assert.Equal(t, "yay", manager.Name())
}

func TestPacmanManager_Install_DefaultsToParuWhenHelperEmpty(t *testing.T) {
	// Tests: if helper == "" - verify empty helper defaults to paru
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}

	manager := NewPacmanManager(mock)
	manager.Helper = "" // Explicitly set to empty

	_, err := manager.Install(context.Background(), []string{"git"})

	require.NoError(t, err)
	require.Len(t, mock.Calls, 1)
	// Should use "paru" as default when Helper is empty
	assert.Equal(t, "paru", mock.Calls[0].Name,
		"should default to paru when Helper is empty")
}

func TestPacmanManager_Install_EmptyList(t *testing.T) {
	mock := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}

	manager := NewPacmanManager(mock)
	output, err := manager.Install(context.Background(), []string{})

	require.NoError(t, err)
	assert.Empty(t, output)
	assert.Empty(t, mock.Calls, "should not call paru for empty list")
}

// --- Factory Tests ---

func TestNewPkgInstallFactory_SimpleFormat(t *testing.T) {
	// Simple format: list of package names
	args := []any{"git", "curl", "ripgrep"}
	manager := newMockManager("paru", false)

	factory := NewPkgInstallFactory(PkgInstallConfig{
		Manager: manager,
		OS:      "arch",
	})

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Test through observable behavior (Name()) rather than type casting
	name := tasks[0].Name()
	assert.Contains(t, name, "git")
	assert.Contains(t, name, "curl")
	assert.Contains(t, name, "ripgrep")
	assert.NotContains(t, name, "casks")
}

func TestNewPkgInstallFactory_StructuredFormat(t *testing.T) {
	// Structured format: packages and casks in maps
	args := []any{
		map[string]any{
			"packages": []any{"git", "curl"},
		},
		map[string]any{
			"casks": []any{"firefox", "vscode"},
		},
	}
	manager := newMockManager("homebrew", true)

	factory := NewPkgInstallFactory(PkgInstallConfig{
		Manager: manager,
		OS:      "darwin",
	})

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Test through observable behavior (Name()) rather than type casting
	name := tasks[0].Name()
	assert.Contains(t, name, "git")
	assert.Contains(t, name, "curl")
	assert.Contains(t, name, "casks")
	assert.Contains(t, name, "firefox")
	assert.Contains(t, name, "vscode")
}

func TestNewPkgInstallFactory_MixedFormat(t *testing.T) {
	// Mixed: both packages and casks in same map
	args := []any{
		map[string]any{
			"packages": []any{"git"},
			"casks":    []any{"firefox"},
		},
	}
	manager := newMockManager("homebrew", true)

	factory := NewPkgInstallFactory(PkgInstallConfig{
		Manager: manager,
		OS:      "darwin",
	})

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Test through observable behavior (Name()) rather than type casting
	name := tasks[0].Name()
	assert.Contains(t, name, "git")
	assert.Contains(t, name, "casks")
	assert.Contains(t, name, "firefox")
}

func TestNewPkgInstallFactory_EmptyList(t *testing.T) {
	args := []any{}

	factory := NewPkgInstallFactory(PkgInstallConfig{OS: "arch"})
	tasks, err := factory(args)

	require.NoError(t, err)
	assert.Nil(t, tasks)
}

func TestNewPkgInstallFactory_InvalidArgs_NotList(t *testing.T) {
	args := "git"

	factory := NewPkgInstallFactory(PkgInstallConfig{OS: "arch"})
	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a list")
}

func TestNewPkgInstallFactory_InvalidArgs_BadType(t *testing.T) {
	args := []any{123} // number instead of string/map

	factory := NewPkgInstallFactory(PkgInstallConfig{OS: "arch"})
	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a string or map")
}

func TestNewPkgInstallFactory_InvalidArgs_PackagesNotList(t *testing.T) {
	args := []any{
		map[string]any{
			"packages": "git", // should be list
		},
	}

	factory := NewPkgInstallFactory(PkgInstallConfig{OS: "arch"})
	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a list")
}

func TestNewPkgInstallFactory_InvalidArgs_CaskNotString(t *testing.T) {
	args := []any{
		map[string]any{
			"casks": []any{123}, // should be string
		},
	}

	factory := NewPkgInstallFactory(PkgInstallConfig{OS: "darwin"})
	_, err := factory(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a string")
}

func TestNewPkgInstallFactory_ErrorIndices(t *testing.T) {
	// Tests ARITHMETIC_BASE mutations: i+1 vs i-1 in error messages
	tests := []struct {
		name          string
		args          []any
		expectedIndex string
	}{
		{
			name: "first arg bad type shows arg 1",
			args: []any{
				123, // invalid type
			},
			expectedIndex: "arg 1:",
		},
		{
			name: "second arg bad type shows arg 2",
			args: []any{
				"valid-package",
				456, // invalid type
			},
			expectedIndex: "arg 2:",
		},
		{
			name: "packages list first item not string shows index 0",
			args: []any{
				map[string]any{
					"packages": []any{123}, // not string
				},
			},
			expectedIndex: "[0]:",
		},
		{
			name: "packages list second item not string shows index 1",
			args: []any{
				map[string]any{
					"packages": []any{"valid", 456},
				},
			},
			expectedIndex: "[1]:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewPkgInstallFactory(PkgInstallConfig{OS: "arch"})
			_, err := factory(tt.args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedIndex,
				"error message must show correct index")
		})
	}
}

func TestNewPkgInstallFactory_SetsOSFromConfig(t *testing.T) {
	args := []any{"git"}
	manager := newMockManager("paru", false)

	factory := NewPkgInstallFactory(PkgInstallConfig{
		Manager: manager,
		OS:      "manjaro",
	})

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Verify OS propagation by testing cask behavior:
	// On non-darwin OS, casks should fail - but this task has no casks
	// so we verify the task was created and can run successfully
	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusDone, result.Status, "task should execute on manjaro")
}

func TestNewPkgInstallFactory_CreatesDefaultManager(t *testing.T) {
	args := []any{"git"}

	// No manager provided - should create PacmanManager
	// Inject a mock runner to verify paru is called (default helper for PacmanManager)
	mockRunner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "pacman" {
				return []byte("git\n"), nil // git already installed
			}
			return []byte("ok"), nil
		},
	}

	factory := NewPkgInstallFactory(PkgInstallConfig{
		OS:     "arch",
		Runner: mockRunner,
	})

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Test through behavior: run the task and verify it uses pacman/paru
	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusSkipped, result.Status, "git is already installed in mock")

	// Verify pacman was called (ListInstalled uses pacman -Qq)
	var pacmanCalled bool
	for _, call := range mockRunner.Calls {
		if call.Name == "pacman" {
			pacmanCalled = true
			break
		}
	}
	assert.True(t, pacmanCalled, "should use pacman-based manager")
}

func TestNewPkgInstallFactory_CreatesHomebrewManagerOnDarwin(t *testing.T) {
	args := []any{"git"}

	// No manager provided + darwin OS - should create HomebrewManager
	mockRunner := &cmdexec.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "brew" {
				return []byte("git\n"), nil // git already installed
			}
			return []byte("ok"), nil
		},
	}

	// Mock PathFinder to return consistent "brew" path for testing
	mockPathFinder := func() (string, bool) {
		return "brew", true
	}

	factory := NewPkgInstallFactory(PkgInstallConfig{
		OS:         "darwin",
		Runner:     mockRunner,
		PathFinder: mockPathFinder,
	})

	tasks, err := factory(args)

	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Test through behavior: run the task and verify it uses brew
	result := tasks[0].Run(context.Background())
	assert.Equal(t, StatusSkipped, result.Status, "git is already installed in mock")

	// Verify brew was called (ListInstalled uses brew list --formulae)
	var brewCalled bool
	for _, call := range mockRunner.Calls {
		if call.Name == "brew" {
			brewCalled = true
			break
		}
	}
	assert.True(t, brewCalled, "should use homebrew manager on darwin")
}
