package action

import (
	"booster/internal/cmdexec"
	"context"
	"fmt"
)

type HomebrewManager struct {
	Runner     cmdexec.Runner
	PathFinder BrewPathFinder
}

func NewHomebrewManager(runner cmdexec.Runner, pathFinder BrewPathFinder) *HomebrewManager {
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}
	return &HomebrewManager{Runner: runner, PathFinder: pathFinder}
}

func (m *HomebrewManager) Name() string {
	return "homebrew"
}

func (m *HomebrewManager) Available(_ context.Context) bool {
	if _, found := m.finder()(); found {
		return true
	}

	_, err := m.Runner.LookPath("brew")
	return err == nil
}

func (m *HomebrewManager) ListInstalled(ctx context.Context) ([]string, error) {
	output, err := m.Runner.Run(ctx, m.brewPath(), "list", "--formulae")
	if err != nil {
		return nil, fmt.Errorf("list installed packages: %w", err)
	}

	return parseLines(string(output)), nil
}

func (m *HomebrewManager) Install(ctx context.Context, pkgs []string) (string, error) {
	if len(pkgs) == 0 {
		return "", nil
	}

	args := append([]string{"install"}, pkgs...)
	output, err := m.Runner.Run(ctx, m.brewPath(), args...)
	if err != nil {
		return string(output), fmt.Errorf("brew install: %w", err)
	}

	return string(output), nil
}

func (m *HomebrewManager) ListInstalledCasks(ctx context.Context) ([]string, error) {
	output, err := m.Runner.Run(ctx, m.brewPath(), "list", "--casks")
	if err != nil {
		return nil, fmt.Errorf("list installed casks: %w", err)
	}

	return parseLines(string(output)), nil
}

func (m *HomebrewManager) InstallCasks(ctx context.Context, casks []string) (string, error) {
	if len(casks) == 0 {
		return "", nil
	}

	args := append([]string{"install", "--cask"}, casks...)
	output, err := m.Runner.Run(ctx, m.brewPath(), args...)
	if err != nil {
		return string(output), fmt.Errorf("brew install casks: %w", err)
	}

	return string(output), nil
}

func (m *HomebrewManager) finder() BrewPathFinder {
	if m.PathFinder != nil {
		return m.PathFinder
	}
	return defaultBrewPathFinder
}

func (m *HomebrewManager) brewPath() string {
	if path, found := m.finder()(); found {
		return path
	}
	return "brew"
}
