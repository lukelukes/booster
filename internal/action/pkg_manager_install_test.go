package action

import (
	"booster/internal/cmdexec"
	"booster/internal/model"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aurRunner(present bool) *cmdexec.MockRunner {
	return &cmdexec.MockRunner{
		LookPathFunc: func(name string) (string, error) {
			if !present {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + name, nil
		},
		RunFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name == "pacman" && !present {
				return nil, errors.New("package not found")
			}
			_ = args
			return nil, nil
		},
	}
}

func commands(runner *cmdexec.MockRunner) []string {
	var out []string
	for _, call := range runner.Calls {
		out = append(out, strings.TrimSpace(call.Name+" "+strings.Join(call.Args, " ")))
	}
	return out
}

func TestPkgManagerInstall_Check(t *testing.T) {
	t.Run("reports a missing helper as a create", func(t *testing.T) {
		action := &PkgManagerInstall{Runner: aurRunner(false), Managers: []string{"paru"}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, change.Kind)
		assert.Equal(t, 1, change.Total)
		assert.Equal(t, []model.Item{{Name: "paru"}}, change.Items)
		assert.True(t, change.NeedsSudo)
	})

	t.Run("reports no change when the helper is installed and registered", func(t *testing.T) {
		action := &PkgManagerInstall{Runner: aurRunner(true), Managers: []string{"paru"}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
		assert.Empty(t, change.Items)
	})

	t.Run("reports a binary that pacman does not know about as a create", func(t *testing.T) {
		runner := &cmdexec.MockRunner{
			LookPathFunc: func(name string) (string, error) { return "/usr/bin/" + name, nil },
			RunFunc: func(context.Context, string, ...string) ([]byte, error) {
				return nil, errors.New("error: package 'paru' was not found")
			},
		}
		action := &PkgManagerInstall{Runner: runner, Managers: []string{"paru"}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []model.Item{{Name: "paru"}}, change.Items)
	})

	t.Run("uses the brew path finder for homebrew", func(t *testing.T) {
		action := &PkgManagerInstall{
			Runner:     aurRunner(false),
			PathFinder: func() (string, bool) { return "/opt/homebrew/bin/brew", true },
			Managers:   []string{"homebrew"},
		}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
	})
}

func TestPkgManagerInstall_Apply(t *testing.T) {
	t.Run("clones and builds a missing AUR helper", func(t *testing.T) {
		runner := aurRunner(false)
		action := &PkgManagerInstall{Runner: runner, Managers: []string{"paru"}}

		require.NoError(t, action.Apply(context.Background()))

		ran := commands(runner)
		assert.Contains(t, strings.Join(ran, "\n"), "git clone https://aur.archlinux.org/paru.git")
		assert.Contains(t, strings.Join(ran, "\n"), "makepkg -si --noconfirm")
	})

	t.Run("does nothing when the helper is already installed", func(t *testing.T) {
		runner := aurRunner(true)
		action := &PkgManagerInstall{Runner: runner, Managers: []string{"paru"}}

		require.NoError(t, action.Apply(context.Background()))

		assert.Equal(t, []string{"pacman -Q paru"}, commands(runner))
	})

	t.Run("propagates a clone failure", func(t *testing.T) {
		runner := &cmdexec.MockRunner{
			LookPathFunc: func(string) (string, error) { return "", errors.New("not found") },
			RunFunc: func(_ context.Context, name string, _ ...string) ([]byte, error) {
				if name == "git" {
					return []byte("fatal: unable to access"), errors.New("exit status 128")
				}
				return nil, errors.New("package not found")
			},
		}
		action := &PkgManagerInstall{Runner: runner, Managers: []string{"paru"}}

		err := action.Apply(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "clone paru")
	})
}

func TestNewPkgManagerInstall(t *testing.T) {
	t.Run("accepts the supported managers", func(t *testing.T) {
		created, err := NewPkgManagerInstall(argsNode(t, "- paru\n- homebrew"), testContext(), aurRunner(true))

		require.NoError(t, err)
		assert.Equal(t, []string{"paru", "homebrew"}, created.(*PkgManagerInstall).Managers)
	})

	t.Run("rejects an unsupported manager", func(t *testing.T) {
		_, err := NewPkgManagerInstall(argsNode(t, "- paru\n- apt"), testContext(), aurRunner(true))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "[1]: unsupported package manager: apt")
	})

	t.Run("rejects a scalar", func(t *testing.T) {
		_, err := NewPkgManagerInstall(argsNode(t, "paru"), testContext(), aurRunner(true))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "args must be a list")
	})
}
