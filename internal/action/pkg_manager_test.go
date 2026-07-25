package action

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func outputRunner(output string, err error) *cmdexec.MockRunner {
	return &cmdexec.MockRunner{
		RunFunc: func(context.Context, string, ...string) ([]byte, error) {
			return []byte(output), err
		},
	}
}

func TestPacmanManager(t *testing.T) {
	t.Run("lists installed packages one per line", func(t *testing.T) {
		manager := NewPacmanManager(outputRunner("fd\nripgrep\n\n  yq  \n", nil))

		installed, err := manager.ListInstalled(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []string{"fd", "ripgrep", "yq"}, installed)
	})

	t.Run("installs through the AUR helper", func(t *testing.T) {
		runner := outputRunner("", nil)
		manager := NewPacmanManager(runner)

		_, err := manager.Install(context.Background(), []string{"fd", "yq"})

		require.NoError(t, err)
		require.Len(t, runner.Calls, 1)
		assert.Equal(t, "paru", runner.Calls[0].Name)
		assert.Equal(t, []string{"-S", "--noconfirm", "--needed", "--skipreview", "fd", "yq"}, runner.Calls[0].Args)
	})

	t.Run("runs nothing for an empty package list", func(t *testing.T) {
		runner := outputRunner("", nil)
		manager := NewPacmanManager(runner)

		_, err := manager.Install(context.Background(), nil)

		require.NoError(t, err)
		assert.Empty(t, runner.Calls)
	})

	t.Run("is unavailable without pacman", func(t *testing.T) {
		manager := NewPacmanManager(&cmdexec.MockRunner{
			LookPathFunc: func(string) (string, error) { return "", errors.New("not found") },
		})

		assert.False(t, manager.Available(context.Background()))
	})
}

func TestHomebrewManager(t *testing.T) {
	t.Run("uses the discovered brew path", func(t *testing.T) {
		runner := outputRunner("fd\n", nil)
		manager := NewHomebrewManager(runner, func() (string, bool) { return "/opt/homebrew/bin/brew", true })

		installed, err := manager.ListInstalled(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []string{"fd"}, installed)
		assert.Equal(t, "/opt/homebrew/bin/brew", runner.Calls[0].Name)
		assert.Equal(t, []string{"list", "--formulae"}, runner.Calls[0].Args)
	})

	t.Run("falls back to brew on PATH", func(t *testing.T) {
		runner := outputRunner("", nil)
		manager := NewHomebrewManager(runner, func() (string, bool) { return "", false })

		_, err := manager.Install(context.Background(), []string{"fd"})

		require.NoError(t, err)
		assert.Equal(t, "brew", runner.Calls[0].Name)
		assert.Equal(t, []string{"install", "fd"}, runner.Calls[0].Args)
	})

	t.Run("installs casks with the cask flag", func(t *testing.T) {
		runner := outputRunner("", nil)
		manager := NewHomebrewManager(runner, func() (string, bool) { return "brew", true })

		_, err := manager.InstallCasks(context.Background(), []string{"ghostty"})

		require.NoError(t, err)
		assert.Equal(t, []string{"install", "--cask", "ghostty"}, runner.Calls[0].Args)
	})

	t.Run("is available when brew is on PATH but in no known location", func(t *testing.T) {
		manager := NewHomebrewManager(&cmdexec.MockRunner{}, func() (string, bool) { return "", false })

		assert.True(t, manager.Available(context.Background()))
	})

	t.Run("is unavailable when brew is nowhere", func(t *testing.T) {
		manager := NewHomebrewManager(&cmdexec.MockRunner{
			LookPathFunc: func(string) (string, error) { return "", errors.New("not found") },
		}, func() (string, bool) { return "", false })

		assert.False(t, manager.Available(context.Background()))
	})
}
