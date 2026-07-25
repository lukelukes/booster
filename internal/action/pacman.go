package action

import (
	"booster/internal/cmdexec"
	"context"
	"fmt"
)

type PacmanManager struct {
	Runner cmdexec.Runner

	Helper string
}

func NewPacmanManager(runner cmdexec.Runner) *PacmanManager {
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}
	return &PacmanManager{Runner: runner, Helper: "paru"}
}

func (m *PacmanManager) Name() string {
	return m.Helper
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

	helper := m.Helper
	if helper == "" {
		helper = "paru"
	}

	args := append([]string{"-S", "--noconfirm", "--needed", "--skipreview"}, pkgs...)
	output, err := m.Runner.Run(ctx, helper, args...)
	if err != nil {
		return string(output), fmt.Errorf("%s install: %w", helper, err)
	}

	return string(output), nil
}
