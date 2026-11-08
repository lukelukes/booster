package action

import (
	"booster/internal/cmdexec"
	"context"
	"fmt"
)

var aurHelpers = []string{"paru", "yay"}

type helperPlan struct {
	pending bool
}

type PacmanManager struct {
	Runner cmdexec.Runner

	plan *helperPlan
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

	return parseLines(string(output)), nil
}

func (m *PacmanManager) Install(ctx context.Context, pkgs []string) (string, error) {
	if len(pkgs) == 0 {
		return "", nil
	}

	name, args := "sudo", []string{"pacman", "-S", "--noconfirm", "--needed"}
	switch helper := m.helper(); helper {
	case "paru":
		name, args = helper, []string{"-S", "--noconfirm", "--needed", "--skipreview"}
	case "yay":
		name, args = helper, []string{"-S", "--noconfirm", "--needed"}
	}

	output, err := m.Runner.Run(ctx, name, append(args, pkgs...)...)
	if err != nil {
		return string(output), fmt.Errorf("%s install: %w", m.Name(), err)
	}

	return string(output), nil
}

func (m *PacmanManager) Uninstallable(ctx context.Context, pkgs []string) ([]string, error) {
	if len(pkgs) == 0 || m.helper() != "" || (m.plan != nil && m.plan.pending) {
		return nil, nil
	}

	repo, err := m.Runner.Run(ctx, "pacman", "-Slq")
	if err != nil {
		return nil, fmt.Errorf("list repo packages: %w", err)
	}
	groups, err := m.Runner.Run(ctx, "pacman", "-Sg")
	if err != nil {
		return nil, fmt.Errorf("list repo groups: %w", err)
	}

	return missingFrom(append(parseLines(string(repo)), parseLines(string(groups))...), pkgs), nil
}

func (m *PacmanManager) helper() string {
	for _, helper := range aurHelpers {
		if _, err := m.Runner.LookPath(helper); err == nil {
			return helper
		}
	}
	return ""
}
