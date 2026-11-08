package action

import (
	"booster/internal/model"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubManager struct {
	listErr     error
	installErr  error
	installed   []string
	installCall [][]string
	unavailable bool
}

func (m *stubManager) Name() string { return "stub" }

func (m *stubManager) Available(context.Context) bool { return !m.unavailable }

func (m *stubManager) ListInstalled(context.Context) ([]string, error) {
	return m.installed, m.listErr
}

func (m *stubManager) Install(_ context.Context, pkgs []string) (string, error) {
	m.installCall = append(m.installCall, pkgs)
	return "", m.installErr
}

type stubCaskManager struct {
	*stubManager
	installedCasks  []string
	installCaskCall [][]string
}

func (m *stubCaskManager) ListInstalledCasks(context.Context) ([]string, error) {
	return m.installedCasks, m.listErr
}

func (m *stubCaskManager) InstallCasks(_ context.Context, casks []string) (string, error) {
	m.installCaskCall = append(m.installCaskCall, casks)
	return "", m.installErr
}

func TestPkgInstall_Check(t *testing.T) {
	t.Run("reports the packages that are not installed", func(t *testing.T) {
		manager := &stubManager{installed: []string{"fd", "ripgrep"}}
		action := &PkgInstall{
			Manager:  manager,
			OS:       "arch",
			Packages: []string{"fd", "ripgrep", "yq", "uv"},
		}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, change.Kind)
		assert.Equal(t, 4, change.Total)
		assert.Equal(t, []model.Item{{Name: "yq"}, {Name: "uv"}}, change.Items)
		assert.True(t, change.NeedsSudo)
	})

	t.Run("reports no change when everything is installed", func(t *testing.T) {
		manager := &stubManager{installed: []string{"fd", "ripgrep"}}
		action := &PkgInstall{Manager: manager, OS: "arch", Packages: []string{"fd"}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
		assert.Empty(t, change.Items)
		assert.False(t, change.NeedsSudo)
	})

	t.Run("reports unknown when the package manager is not installed yet", func(t *testing.T) {
		manager := &stubManager{unavailable: true}
		action := &PkgInstall{Manager: manager, OS: "arch", Packages: []string{"fd"}}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeUnknown, change.Kind)
		assert.Equal(t, 1, change.Total)
		assert.Empty(t, change.Items)
	})

	t.Run("fails when casks are used off darwin", func(t *testing.T) {
		action := &PkgInstall{Manager: &stubManager{}, OS: "arch", Casks: []string{"firefox"}}

		_, err := action.Check(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not darwin")
	})

	t.Run("propagates a listing failure", func(t *testing.T) {
		manager := &stubManager{listErr: errors.New("pacman exploded")}
		action := &PkgInstall{Manager: manager, OS: "arch", Packages: []string{"fd"}}

		_, err := action.Check(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "pacman exploded")
	})
}

func TestPkgInstall_Apply(t *testing.T) {
	t.Run("installs only the missing packages", func(t *testing.T) {
		manager := &stubManager{installed: []string{"fd"}}
		action := &PkgInstall{Manager: manager, OS: "arch", Packages: []string{"fd", "yq"}}

		require.NoError(t, action.Apply(context.Background()))

		assert.Equal(t, [][]string{{"yq"}}, manager.installCall)
	})

	t.Run("installs nothing when everything is present", func(t *testing.T) {
		manager := &stubManager{installed: []string{"fd"}}
		action := &PkgInstall{Manager: manager, OS: "arch", Packages: []string{"fd"}}

		require.NoError(t, action.Apply(context.Background()))

		assert.Empty(t, manager.installCall)
	})

	t.Run("installs packages and casks separately", func(t *testing.T) {
		manager := &stubCaskManager{stubManager: &stubManager{}}
		action := &PkgInstall{
			Manager:  manager,
			OS:       "darwin",
			Packages: []string{"fd"},
			Casks:    []string{"ghostty"},
		}

		require.NoError(t, action.Apply(context.Background()))

		assert.Equal(t, [][]string{{"fd"}}, manager.installCall)
		assert.Equal(t, [][]string{{"ghostty"}}, manager.installCaskCall)
	})

	t.Run("propagates an install failure", func(t *testing.T) {
		manager := &stubManager{installErr: errors.New("target not found")}
		action := &PkgInstall{Manager: manager, OS: "arch", Packages: []string{"ghostty"}}

		err := action.Apply(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "target not found")
	})
}

func TestNewPkgInstall(t *testing.T) {
	t.Run("accepts plain names and packages/casks mappings", func(t *testing.T) {
		args := argsNode(t, `
- ripgrep
- ${ vars.editor }
- packages: [fd, yq]
  casks: [ghostty]
`)
		ctx := testContext()
		ctx.OS = "darwin"
		ctx.Vars["editor"] = "neovim"

		created, err := NewPkgInstall(args, ctx, nil)

		require.NoError(t, err)
		pkg := created
		assert.Equal(t, []string{"ripgrep", "neovim", "fd", "yq"}, pkg.Packages)
		assert.Equal(t, []string{"ghostty"}, pkg.Casks)
		assert.IsType(t, &HomebrewManager{}, pkg.Manager)
	})

	t.Run("uses pacman off darwin", func(t *testing.T) {
		created, err := NewPkgInstall(argsNode(t, "- ripgrep"), testContext(), nil)

		require.NoError(t, err)
		pkg := created
		assert.IsType(t, &PacmanManager{}, pkg.Manager)
	})

	t.Run("rejects a scalar", func(t *testing.T) {
		_, err := NewPkgInstall(argsNode(t, "ripgrep"), testContext(), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "args must be a list")
	})

	t.Run("rejects an entry that is neither a name nor a mapping", func(t *testing.T) {
		_, err := NewPkgInstall(argsNode(t, "- [ripgrep]"), testContext(), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a package name or a mapping")
	})

	t.Run("reports which package failed to resolve", func(t *testing.T) {
		_, err := NewPkgInstall(argsNode(t, "- ripgrep\n- ${ nope() }"), testContext(), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "packages[1]")
	})
}
