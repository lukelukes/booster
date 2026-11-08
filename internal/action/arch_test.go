package action_test

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"booster/internal/model"
	"booster/internal/run"
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeArch struct {
	t         *testing.T
	repo      []string
	groups    map[string][]string
	aur       []string
	installed []string
	binaries  []string
	tools     map[string]string
	commands  []string
}

func (f *fakeArch) LookPath(name string) (string, error) {
	if name == "pacman" || slices.Contains(f.binaries, name) {
		return "/usr/bin/" + name, nil
	}
	return "", errors.New("not found")
}

func (f *fakeArch) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	switch {
	case command == "pacman -Qq":
		return []byte(strings.Join(f.installed, "\n")), nil
	case command == "pacman -Slq":
		return []byte(strings.Join(f.repo, "\n")), nil
	case command == "pacman -Sg":
		return []byte(strings.Join(slices.Collect(maps.Keys(f.groups)), "\n")), nil
	case command == "pacman -Sgg":
		var lines []string
		for group, members := range f.groups {
			for _, member := range members {
				lines = append(lines, group+" "+member)
			}
		}
		return []byte(strings.Join(lines, "\n")), nil
	case name == "pacman" && len(args) == 2 && args[0] == "-Q":
		if slices.Contains(f.installed, args[1]) {
			return nil, nil
		}
		return nil, errors.New("not installed")
	case name == "mise" && len(args) == 2 && args[0] == "current":
		if version, ok := f.tools[args[1]]; ok {
			return []byte(version + "\n"), nil
		}
		return nil, errors.New("no version set")
	}

	f.commands = append(f.commands, command)

	switch {
	case strings.HasPrefix(command, "sudo -n pacman "):
		return nil, f.install(args[2:], f.repo)
	case name == "paru" || name == "yay":
		return nil, f.install(args, append(slices.Clone(f.repo), f.aur...))
	case name == "git" && len(args) > 0 && args[0] == "clone":
		return nil, nil
	case name == "sh" && strings.Contains(command, "makepkg"):
		for _, helper := range []string{"paru", "yay"} {
			if strings.Contains(command, "/"+helper+" ") {
				f.binaries = append(f.binaries, helper)
				f.installed = append(f.installed, helper)
			}
		}
		return nil, nil
	case name == "mise" && len(args) == 3 && args[0] == "use" && args[1] == "--global":
		tool, version, _ := strings.Cut(args[2], "@")
		f.tools[tool] = version
		return nil, nil
	}

	f.t.Errorf("unexpected command: %s", command)
	return nil, errors.New("unexpected command: " + command)
}

func (f *fakeArch) install(args, available []string) error {
	for _, pkg := range args {
		if strings.HasPrefix(pkg, "-") {
			continue
		}
		if !slices.Contains(available, pkg) {
			return errors.New("target not found: " + pkg)
		}
		f.installed = append(f.installed, pkg)
	}
	return nil
}

func planArch(t *testing.T, machine *fakeArch, src string) (run.PlanResult, error) {
	t.Helper()

	var cfg config.Config
	require.NoError(t, yaml.Unmarshal([]byte(src), &cfg))

	ctx := &expr.Context{OS: "linux", Arch: "amd64", Home: "/home/test"}
	compiled, err := run.Compile(cfg.Tasks, ctx, action.NewRegistry(machine))
	require.NoError(t, err)

	return run.Plan(context.Background(), compiled)
}

func apply(t *testing.T, result run.PlanResult) {
	t.Helper()

	summary := run.Apply(context.Background(), result, func(e model.Event) {
		if done, ok := e.(model.TaskDone); ok {
			require.NoError(t, done.Result.Err)
		}
	})
	assert.Zero(t, summary.Failed)
}

func TestArchPackages(t *testing.T) {
	t.Run("a fresh box with only pacman installs repo packages through sudo pacman", func(t *testing.T) {
		machine := &fakeArch{t: t, repo: []string{"fd", "ripgrep", "git"}, installed: []string{"git"}}
		config := `
tasks:
  - action: pkg.install
    args: [git, fd, ripgrep]
`

		result, err := planArch(t, machine, config)
		require.NoError(t, err)
		change := result.Steps[0].Change
		assert.Equal(t, model.ChangeCreate, change.Kind)
		assert.Equal(t, []model.Item{{Name: "fd"}, {Name: "ripgrep"}}, change.Items)
		assert.True(t, change.NeedsSudo)

		apply(t, result)

		assert.Equal(t, []string{"sudo -n pacman -S --noconfirm --needed fd ripgrep"}, machine.commands)
		result, err = planArch(t, machine, config)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, result.Steps[0].Change.Kind)
	})

	t.Run("a package group installs its missing members and then converges", func(t *testing.T) {
		machine := &fakeArch{
			t:         t,
			repo:      []string{"fd", "autoconf", "make"},
			groups:    map[string][]string{"base-devel": {"autoconf", "make"}},
			installed: []string{"make"},
		}
		config := `
tasks:
  - action: pkg.install
    args: [base-devel, fd]
`

		result, err := planArch(t, machine, config)
		require.NoError(t, err)
		assert.Equal(t, []model.Item{{Name: "base-devel"}, {Name: "fd"}}, result.Steps[0].Change.Items)

		apply(t, result)

		assert.Equal(t, []string{"sudo -n pacman -S --noconfirm --needed autoconf make fd"}, machine.commands)
		result, err = planArch(t, machine, config)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, result.Steps[0].Change.Kind)
	})

	helpers := []struct {
		helper  string
		command string
	}{
		{helper: "paru", command: "paru -S --noconfirm --needed --skipreview fd zen-browser-bin"},
		{helper: "yay", command: "yay -S --noconfirm --needed fd zen-browser-bin"},
	}
	for _, tt := range helpers {
		t.Run("installs through "+tt.helper+" when it is present", func(t *testing.T) {
			machine := &fakeArch{
				t:         t,
				repo:      []string{"fd"},
				aur:       []string{"zen-browser-bin"},
				binaries:  []string{tt.helper},
				installed: []string{tt.helper},
			}

			result, err := planArch(t, machine, `
tasks:
  - action: pkg.install
    args: [fd, zen-browser-bin]
`)
			require.NoError(t, err)
			assert.Len(t, result.Steps[0].Change.Items, 2)

			apply(t, result)

			assert.Equal(t, []string{tt.command}, machine.commands)
		})
	}

	t.Run("prefers paru when both helpers are present", func(t *testing.T) {
		machine := &fakeArch{t: t, repo: []string{"fd"}, binaries: []string{"yay", "paru"}}

		result, err := planArch(t, machine, `
tasks:
  - action: pkg.install
    args: [fd]
`)
		require.NoError(t, err)

		apply(t, result)

		require.Len(t, machine.commands, 1)
		assert.True(t, strings.HasPrefix(machine.commands[0], "paru "))
	})

	t.Run("an AUR package without a helper fails the plan and names the package", func(t *testing.T) {
		machine := &fakeArch{t: t, repo: []string{"fd"}, aur: []string{"zen-browser-bin", "obsidian-appimage"}}

		_, err := planArch(t, machine, `
tasks:
  - action: pkg.install
    args: [fd, zen-browser-bin, obsidian-appimage]
`)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "zen-browser-bin, obsidian-appimage")
		assert.Contains(t, err.Error(), "paru or yay")
		assert.NotContains(t, err.Error(), "fd")
		assert.Empty(t, machine.commands)
	})

	t.Run("an AUR helper installed earlier in the same run covers AUR packages", func(t *testing.T) {
		machine := &fakeArch{t: t, repo: []string{"fd"}, aur: []string{"zen-browser-bin"}}

		result, err := planArch(t, machine, `
tasks:
  - action: pkg-manager.install
    args: [yay]
  - action: pkg.install
    args: [fd, zen-browser-bin]
`)
		require.NoError(t, err)
		assert.Equal(t, []model.Item{{Name: "yay"}}, result.Steps[0].Change.Items)
		assert.Len(t, result.Steps[1].Change.Items, 2)

		apply(t, result)

		require.Len(t, machine.commands, 3)
		assert.True(t, strings.HasPrefix(machine.commands[0], "git clone https://aur.archlinux.org/yay.git"))
		assert.Equal(t, "yay -S --noconfirm --needed fd zen-browser-bin", machine.commands[2])
	})

	t.Run("an AUR helper installed after the package step does not cover it", func(t *testing.T) {
		machine := &fakeArch{t: t, repo: []string{"fd"}, aur: []string{"zen-browser-bin"}}

		_, err := planArch(t, machine, `
tasks:
  - action: pkg.install
    args: [zen-browser-bin]
  - action: pkg-manager.install
    args: [yay]
`)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "zen-browser-bin")
	})

	t.Run("a helper planned in one registry does not cover another", func(t *testing.T) {
		machine := &fakeArch{t: t, repo: []string{"fd"}, aur: []string{"zen-browser-bin"}}

		_, err := planArch(t, machine, `
tasks:
  - action: pkg-manager.install
    args: [yay]
`)
		require.NoError(t, err)

		_, err = planArch(t, machine, `
tasks:
  - action: pkg.install
    args: [zen-browser-bin]
`)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "zen-browser-bin")
	})

	t.Run("homebrew in the same run does not cover AUR packages", func(t *testing.T) {
		machine := &fakeArch{t: t, repo: []string{"fd"}, aur: []string{"zen-browser-bin"}}

		_, err := planArch(t, machine, `
tasks:
  - action: pkg-manager.install
    args: [homebrew]
  - action: pkg.install
    args: [zen-browser-bin]
`)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "zen-browser-bin")
	})
}

func TestArchMiseUse(t *testing.T) {
	machine := &fakeArch{t: t, binaries: []string{"mise"}, tools: map[string]string{"go": "1.25"}}
	config := `
tasks:
  - action: mise.use
    args: [go@1.26, node@22]
`

	result, err := planArch(t, machine, config)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeUpdate, result.Steps[0].Change.Kind)
	assert.Equal(t, []model.Item{{Name: "go", From: "1.25", To: "1.26"}, {Name: "node", To: "22"}}, result.Steps[0].Change.Items)

	apply(t, result)

	assert.Equal(t, []string{"mise use --global go@1.26", "mise use --global node@22"}, machine.commands)
	result, err = planArch(t, machine, config)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeNone, result.Steps[0].Change.Kind)
}
