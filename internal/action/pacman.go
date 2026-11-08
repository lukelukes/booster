package action

import (
	"booster/internal/cmdexec"
	"context"
	"fmt"
	"strings"
)

var aurHelpers = []string{"paru", "yay"}

type PacmanManager struct {
	Runner cmdexec.Runner

	helperPlanned bool
}

func NewPacmanManager(runner cmdexec.Runner) *PacmanManager {
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}
	return &PacmanManager{Runner: runner}
}

func (m *PacmanManager) Name() string {
	if helper := m.helper(); helper != "" {
		return helper
	}
	return "pacman"
}

func (m *PacmanManager) Available(_ context.Context) bool {
	_, err := m.Runner.LookPath("pacman")
	return err == nil
}

func (m *PacmanManager) ListInstalled(ctx context.Context) ([]string, error) {
	output, err := m.Runner.Run(ctx, "pacman", "-Qq")
	if err != nil {
		return nil, fmt.Errorf("list installed packages: %w", err)
	}
	installed := parseLines(string(output))

	groups, err := m.groups(ctx)
	if err != nil {
		return nil, err
	}
	for group, members := range groups {
		if len(missingFrom(installed, members)) == 0 {
			installed = append(installed, group)
		}
	}

	return installed, nil
}

func (m *PacmanManager) Install(ctx context.Context, pkgs []string) (string, error) {
	if len(pkgs) == 0 {
		return "", nil
	}

	groups, err := m.groups(ctx)
	if err != nil {
		return "", err
	}
	var targets []string
	for _, pkg := range pkgs {
		if members, ok := groups[pkg]; ok {
			targets = append(targets, members...)
		} else {
			targets = append(targets, pkg)
		}
	}

	name, args := "sudo", []string{"pacman", "-S", "--noconfirm", "--needed"}
	switch helper := m.helper(); helper {
	case "paru":
		name, args = helper, []string{"-S", "--noconfirm", "--needed", "--skipreview"}
	case "yay":
		name, args = helper, []string{"-S", "--noconfirm", "--needed"}
	}

	output, err := m.Runner.Run(ctx, name, append(args, targets...)...)
	if err != nil {
		return string(output), fmt.Errorf("%s install: %w", m.Name(), err)
	}

	return string(output), nil
}

func (m *PacmanManager) Installable(ctx context.Context, pkgs []string) error {
	if len(pkgs) == 0 || m.helperPlanned || m.helper() != "" {
		return nil
	}

	repo, err := m.Runner.Run(ctx, "pacman", "-Slq")
	if err != nil {
		return fmt.Errorf("list repo packages: %w", err)
	}
	groups, err := m.Runner.Run(ctx, "pacman", "-Sg")
	if err != nil {
		return fmt.Errorf("list repo groups: %w", err)
	}

	missing := missingFrom(append(parseLines(string(repo)), parseLines(string(groups))...), pkgs)
	if len(missing) > 0 {
		return fmt.Errorf("not in the pacman repos and no AUR helper is installed: %s; add a pkg-manager.install step for paru or yay before this one", strings.Join(missing, ", "))
	}

	return nil
}

func (m *PacmanManager) groups(ctx context.Context) (map[string][]string, error) {
	output, err := m.Runner.Run(ctx, "pacman", "-Sgg")
	if err != nil {
		return nil, fmt.Errorf("list repo groups: %w", err)
	}

	groups := map[string][]string{}
	for _, line := range parseLines(string(output)) {
		if group, member, ok := strings.Cut(line, " "); ok {
			groups[group] = append(groups[group], member)
		}
	}
	return groups, nil
}

func (m *PacmanManager) helper() string {
	for _, helper := range aurHelpers {
		if _, err := m.Runner.LookPath(helper); err == nil {
			return helper
		}
	}
	return ""
}
