package action

import (
	"booster/internal/model"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeArch struct {
	repo      []string
	groups    []string
	aur       []string
	installed []string
	binaries  []string
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
		return []byte(strings.Join(f.groups, "\n")), nil
	case name == "pacman" && len(args) == 2 && args[0] == "-Q":
		if slices.Contains(f.installed, args[1]) {
			return nil, nil
		}
		return nil, errors.New("not installed")
	}

	f.commands = append(f.commands, command)

	switch {
	case name == "sudo" && len(args) > 0 && args[0] == "pacman":
		return nil, f.install(args[4:], f.repo)
	case name == "paru" || name == "yay":
		return nil, f.install(args[3:], append(slices.Clone(f.repo), f.aur...))
	case name == "sh" && strings.Contains(command, "makepkg"):
		for _, helper := range aurHelpers {
			if strings.Contains(command, "/"+helper+" ") {
				f.binaries = append(f.binaries, helper)
				f.installed = append(f.installed, helper)
			}
		}
	}

	return nil, nil
}

func (f *fakeArch) install(args, available []string) error {
	for _, pkg := range args {
		if strings.HasPrefix(pkg, "--") {
			continue
		}
		if !slices.Contains(available, pkg) {
			return errors.New("target not found: " + pkg)
		}
		f.installed = append(f.installed, pkg)
	}
	return nil
}

type archStep struct {
	Action string    `yaml:"action"`
	Args   yaml.Node `yaml:"args"`
}

func planArch(t *testing.T, machine *fakeArch, config string) ([]Action, []model.Change, error) {
	t.Helper()

	var steps []archStep
	require.NoError(t, yaml.Unmarshal([]byte(config), &steps))

	factories := map[string]Factory{}
	h := newHost(machine)
	factories["pkg-manager.install"] = h.pkgManagerInstall
	factories["pkg.install"] = h.pkgInstall

	var actions []Action
	var changes []model.Change
	for _, step := range steps {
		act, err := factories[step.Action](step.Args, testContext())
		require.NoError(t, err)
		actions = append(actions, act)
	}
	for _, act := range actions {
		change, err := act.Check(context.Background())
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, change)
	}

	return actions, changes, nil
}

func applyAll(t *testing.T, actions []Action) {
	t.Helper()

	for _, act := range actions {
		require.NoError(t, act.Apply(context.Background()))
	}
}

func TestArchPackages(t *testing.T) {
	t.Run("a fresh box with only pacman installs repo packages through sudo pacman", func(t *testing.T) {
		machine := &fakeArch{repo: []string{"fd", "ripgrep", "git"}, installed: []string{"git"}}

		actions, changes, err := planArch(t, machine, `
- action: pkg.install
  args: [git, fd, ripgrep]
`)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, changes[0].Kind)
		assert.Equal(t, []model.Item{{Name: "fd"}, {Name: "ripgrep"}}, changes[0].Items)
		assert.True(t, changes[0].NeedsSudo)

		applyAll(t, actions)

		assert.Equal(t, []string{"sudo pacman -S --noconfirm --needed fd ripgrep"}, machine.commands)
		_, changes, err = planArch(t, machine, `
- action: pkg.install
  args: [git, fd, ripgrep]
`)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, changes[0].Kind)
	})

	t.Run("package groups count as repo packages", func(t *testing.T) {
		machine := &fakeArch{repo: []string{"fd"}, groups: []string{"base-devel"}}

		_, changes, err := planArch(t, machine, `
- action: pkg.install
  args: [base-devel, fd]
`)

		require.NoError(t, err)
		assert.Len(t, changes[0].Items, 2)
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
				repo:      []string{"fd"},
				aur:       []string{"zen-browser-bin"},
				binaries:  []string{tt.helper},
				installed: []string{tt.helper},
			}

			actions, changes, err := planArch(t, machine, `
- action: pkg.install
  args: [fd, zen-browser-bin]
`)
			require.NoError(t, err)
			assert.Len(t, changes[0].Items, 2)

			applyAll(t, actions)

			assert.Equal(t, []string{tt.command}, machine.commands)
		})
	}

	t.Run("prefers paru when both helpers are present", func(t *testing.T) {
		machine := &fakeArch{repo: []string{"fd"}, binaries: []string{"yay", "paru"}}

		actions, _, err := planArch(t, machine, `
- action: pkg.install
  args: [fd]
`)
		require.NoError(t, err)

		applyAll(t, actions)

		require.Len(t, machine.commands, 1)
		assert.True(t, strings.HasPrefix(machine.commands[0], "paru "))
	})

	t.Run("an AUR package without a helper fails the plan and names the package", func(t *testing.T) {
		machine := &fakeArch{repo: []string{"fd"}, aur: []string{"zen-browser-bin", "obsidian-appimage"}}

		_, _, err := planArch(t, machine, `
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
		machine := &fakeArch{repo: []string{"fd"}, aur: []string{"zen-browser-bin"}}

		actions, changes, err := planArch(t, machine, `
- action: pkg-manager.install
  args: [yay]
- action: pkg.install
  args: [fd, zen-browser-bin]
`)
		require.NoError(t, err)
		assert.Equal(t, []model.Item{{Name: "yay"}}, changes[0].Items)
		assert.Len(t, changes[1].Items, 2)

		applyAll(t, actions)

		require.Len(t, machine.commands, 3)
		assert.True(t, strings.HasPrefix(machine.commands[0], "git clone https://aur.archlinux.org/yay.git"))
		assert.Equal(t, "yay -S --noconfirm --needed fd zen-browser-bin", machine.commands[2])
	})

	t.Run("homebrew in the same run does not cover AUR packages", func(t *testing.T) {
		machine := &fakeArch{repo: []string{"fd"}, aur: []string{"zen-browser-bin"}}

		_, _, err := planArch(t, machine, `
- action: pkg-manager.install
  args: [homebrew]
- action: pkg.install
  args: [zen-browser-bin]
`)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "zen-browser-bin")
	})
}

func TestRegistry_PortedActions(t *testing.T) {
	registry := Registry()

	for _, name := range []string{"template.render", "pkg-manager.install", "pkg.install", "mise.use"} {
		assert.Contains(t, registry, name)
	}
}
