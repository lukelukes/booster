package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPkgManagerInstall_Paru(t *testing.T) {
	tests := []struct {
		name               string
		lookPath           func(string) (string, error)
		runFunc            func(context.Context, string, ...string) ([]byte, error)
		wantStatus         Status
		wantMessage        string
		wantErrContains    string
		checkGitClone      bool
		checkMakepkgCalled *bool
	}{
		{
			name: "skips when already installed",
			lookPath: func(name string) (string, error) {
				if name == "paru" {
					return "/usr/bin/paru", nil
				}
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				// pacman -Q paru succeeds
				if name == "pacman" && len(args) >= 2 && args[0] == "-Q" && args[1] == "paru" {
					return []byte("paru 2.0.0-1"), nil
				}
				return nil, errors.New("unexpected command")
			},
			wantStatus:  StatusSkipped,
			wantMessage: "already installed",
		},
		{
			name: "installs when binary missing",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				// All commands succeed for installation
				return []byte("ok"), nil
			},
			wantStatus:    StatusDone,
			wantMessage:   "installed",
			checkGitClone: true,
		},
		{
			name: "installs when package not registered",
			lookPath: func(name string) (string, error) {
				if name == "paru" {
					return "/usr/bin/paru", nil
				}
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				// pacman -Q fails (not registered)
				if name == "pacman" && len(args) >= 2 && args[0] == "-Q" {
					return nil, errors.New("package not found")
				}
				// Other commands succeed
				return []byte("ok"), nil
			},
			wantStatus: StatusDone,
		},
		{
			name: "fails when git clone fails",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" {
					return []byte("fatal: could not connect"), errors.New("git error")
				}
				return nil, nil
			},
			wantStatus:      StatusFailed,
			wantErrContains: "clone",
		},
		{
			name: "fails when makepkg fails",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "git" {
					return []byte("ok"), nil
				}
				// Implementation uses sh -c "cd ... && makepkg ..."
				if name == "sh" && len(args) >= 2 && args[0] == "-c" {
					return []byte("error: missing dependencies"), errors.New("makepkg failed")
				}
				return nil, nil
			},
			wantStatus:      StatusFailed,
			wantErrContains: "makepkg",
		},
		{
			name: "first run installs",
			lookPath: func(name string) (string, error) {
				// Binary does not exist yet
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				// pacman -Q fails (not installed)
				if name == "pacman" && len(args) >= 2 && args[0] == "-Q" && args[1] == "paru" {
					return nil, errors.New("not found")
				}
				return []byte("ok"), nil
			},
			wantStatus:  StatusDone,
			wantMessage: "installed",
		},
		{
			name: "second run skips",
			lookPath: func(name string) (string, error) {
				// Binary already exists
				if name == "paru" {
					return "/usr/bin/paru", nil
				}
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				// pacman -Q succeeds (already installed)
				if name == "pacman" && len(args) >= 2 && args[0] == "-Q" && args[1] == "paru" {
					return []byte("paru 2.0.0-1"), nil
				}
				return []byte("ok"), nil
			},
			wantStatus:  StatusSkipped,
			wantMessage: "already installed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track makepkg calls if requested
			var makepkgCalled bool
			runFunc := tt.runFunc
			if tt.checkMakepkgCalled != nil {
				runFunc = func(ctx context.Context, name string, args ...string) ([]byte, error) {
					// Implementation uses sh -c for makepkg
					if name == "sh" && len(args) >= 2 && args[0] == "-c" {
						makepkgCalled = true
					}
					return tt.runFunc(ctx, name, args...)
				}
			}

			mock := &cmdexec.MockRunner{
				LookPathFunc: tt.lookPath,
				RunFunc:      runFunc,
			}

			task := &PkgManagerInstall{Manager: "paru", Runner: mock}
			result := task.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			if tt.wantMessage != "" {
				assert.Equal(t, tt.wantMessage, result.Message)
			}
			if tt.wantErrContains != "" {
				assert.Error(t, result.Error)
				assert.Contains(t, result.Error.Error(), tt.wantErrContains)
			}
			if tt.checkGitClone {
				hasGitClone := false
				for _, call := range mock.Calls {
					if call.Name == "git" && len(call.Args) > 0 && call.Args[0] == "clone" {
						hasGitClone = true
						break
					}
				}
				assert.True(t, hasGitClone, "should call git clone")
			}
			if tt.checkMakepkgCalled != nil {
				assert.Equal(t, *tt.checkMakepkgCalled, makepkgCalled, "makepkg call mismatch")
			}
		})
	}
}

func TestPkgManagerInstall_Name(t *testing.T) {
	tests := []struct {
		manager  string
		wantName string
	}{
		{manager: "paru", wantName: "install package manager: paru"},
		{manager: "homebrew", wantName: "install package manager: homebrew"},
		{manager: "yay", wantName: "install package manager: yay"},
	}

	for _, tt := range tests {
		t.Run(tt.manager, func(t *testing.T) {
			task := &PkgManagerInstall{Manager: tt.manager}
			assert.Equal(t, tt.wantName, task.Name())
		})
	}
}

func TestPkgManagerInstall_UnsupportedManager(t *testing.T) {
	mock := &cmdexec.MockRunner{
		LookPathFunc: func(name string) (string, error) {
			return "", errors.New("not found")
		},
	}

	task := &PkgManagerInstall{Manager: "unsupported-manager", Runner: mock}
	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "unsupported")
}

func TestNewPkgManagerInstall(t *testing.T) {
	tests := []struct {
		name            string
		args            any
		wantErr         bool
		wantErrContains string
		wantTaskCount   int
		wantFirstName   string
	}{
		{
			name:          "valid single manager",
			args:          []any{"paru"},
			wantErr:       false,
			wantTaskCount: 1,
			wantFirstName: "install package manager: paru",
		},
		{
			name:          "multiple managers",
			args:          []any{"paru", "yay"},
			wantErr:       false,
			wantTaskCount: 2,
		},
		{
			name:          "empty list",
			args:          []any{},
			wantErr:       false,
			wantTaskCount: 0,
		},
		{
			name:            "invalid args - not a list",
			args:            "paru",
			wantErr:         true,
			wantErrContains: "must be a list",
		},
		{
			name:            "invalid args - not a string",
			args:            []any{123},
			wantErr:         true,
			wantErrContains: "must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := NewPkgManagerInstallFactory(nil)(tt.args)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
				assert.Len(t, tasks, tt.wantTaskCount)
				if tt.wantFirstName != "" && len(tasks) > 0 {
					assert.Equal(t, tt.wantFirstName, tasks[0].Name())
				}
			}
		})
	}
}

func TestPkgManagerInstall_Homebrew(t *testing.T) {
	tests := []struct {
		name            string
		lookPath        func(string) (string, error)
		runFunc         func(context.Context, string, ...string) ([]byte, error)
		wantStatus      Status
		wantMessage     string
		wantErrContains string
		checkCurlCalled bool
		checkBashCalled bool
		checkNoCalls    bool
	}{
		{
			name: "skips when already installed",
			lookPath: func(name string) (string, error) {
				if name == "brew" {
					return "/opt/homebrew/bin/brew", nil
				}
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return nil, errors.New("unexpected command")
			},
			wantStatus:   StatusSkipped,
			wantMessage:  "already installed",
			checkNoCalls: true,
		},
		{
			name: "installs when missing",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "curl" {
					return []byte("#!/bin/bash\necho 'install script'"), nil
				}
				// Implementation uses: bash -c "NONINTERACTIVE=1 bash /tmp/script.sh"
				if name == "bash" && len(args) >= 2 && args[0] == "-c" {
					return []byte("==> Installation successful!"), nil
				}
				return nil, errors.New("unexpected command")
			},
			wantStatus:      StatusDone,
			wantMessage:     "installed",
			checkCurlCalled: true,
			checkBashCalled: true,
		},
		{
			name: "fails when curl fails",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "curl" {
					return []byte("curl: (7) Failed to connect"), errors.New("network error")
				}
				return nil, nil
			},
			wantStatus:      StatusFailed,
			wantErrContains: "download",
		},
		{
			name: "fails when install script fails",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			runFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "curl" {
					return []byte("#!/bin/bash\necho 'install script'"), nil
				}
				// Implementation uses: bash -c "NONINTERACTIVE=1 bash /tmp/script.sh"
				if name == "bash" && len(args) >= 2 && args[0] == "-c" {
					return []byte("Error: Xcode command line tools not installed"), errors.New("install failed")
				}
				return nil, nil
			},
			wantStatus:      StatusFailed,
			wantErrContains: "install",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			curlCalled := false
			bashCalled := false

			// Wrap runFunc to track curl and bash calls
			wrappedRunFunc := func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "curl" {
					curlCalled = true
				}
				if name == "bash" && len(args) >= 2 && args[0] == "-c" {
					bashCalled = true
				}
				return tt.runFunc(ctx, name, args...)
			}

			mock := &cmdexec.MockRunner{
				LookPathFunc: tt.lookPath,
				RunFunc:      wrappedRunFunc,
			}

			task := &PkgManagerInstall{Manager: "homebrew", Runner: mock}
			result := task.Run(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
			if tt.wantMessage != "" {
				assert.Equal(t, tt.wantMessage, result.Message)
			}
			if tt.wantErrContains != "" {
				assert.Error(t, result.Error)
				assert.Contains(t, result.Error.Error(), tt.wantErrContains)
			}
			if tt.checkNoCalls {
				assert.Empty(t, mock.Calls, "should not call any commands since binary exists")
			}
			if tt.checkCurlCalled {
				assert.True(t, curlCalled, "should download install script with curl")
			}
			if tt.checkBashCalled {
				assert.True(t, bashCalled, "should execute install script with bash")
			}
		})
	}
}

// Context cancellation tests

func TestPkgManagerInstall_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mock := &cmdexec.MockRunner{
		LookPathFunc: func(name string) (string, error) {
			return "", errors.New("not found")
		},
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return []byte("ok"), nil
			}
		},
	}

	task := &PkgManagerInstall{Manager: "paru", Runner: mock}
	result := task.Run(ctx)

	// Should fail because context was cancelled
	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.ErrorIs(t, result.Error, context.Canceled)
}

func TestPkgManagerInstall_CancelledDuringClone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mock := &cmdexec.MockRunner{
		LookPathFunc: func(name string) (string, error) {
			return "", errors.New("not found")
		},
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			// Cancel context during git clone
			if name == "git" {
				cancel()
				return nil, context.Canceled
			}
			return []byte("ok"), nil
		},
	}

	task := &PkgManagerInstall{Manager: "paru", Runner: mock}
	result := task.Run(ctx)

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
}
