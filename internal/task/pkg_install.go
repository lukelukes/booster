package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"fmt"
	"strings"
)

// PackageManager abstracts over different system package managers.
type PackageManager interface {
	// Name returns the package manager identifier (e.g., "paru", "homebrew").
	Name() string

	// ListInstalled returns all installed package names.
	// Used for batch idempotency checks (O(1) instead of O(n) shell calls).
	ListInstalled(ctx context.Context) ([]string, error)

	// Install installs the given packages.
	// Returns the command output (stdout/stderr combined) and any error.
	Install(ctx context.Context, pkgs []string) (output string, err error)

	// ListInstalledCasks returns all installed cask names.
	// Returns empty slice on non-Homebrew managers.
	ListInstalledCasks(ctx context.Context) ([]string, error)

	// InstallCasks installs Homebrew casks.
	// Returns the command output (stdout/stderr combined) and any error.
	// No-op on non-Homebrew managers.
	InstallCasks(ctx context.Context, casks []string) (output string, err error)

	// SupportsCasks returns true if this manager supports casks (Homebrew only).
	SupportsCasks() bool
}

// PacmanManager implements PackageManager for Arch Linux using paru/pacman.
type PacmanManager struct {
	Runner cmdexec.Runner
	// Helper is the AUR helper to use (paru or yay). Defaults to paru.
	Helper string
}

// NewPacmanManager creates a new PacmanManager with the given runner.
func NewPacmanManager(runner cmdexec.Runner) *PacmanManager {
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}
	return &PacmanManager{Runner: runner, Helper: "paru"}
}

func (m *PacmanManager) Name() string {
	return m.Helper
}

func (m *PacmanManager) ListInstalled(ctx context.Context) ([]string, error) {
	// pacman -Qq returns just package names, one per line
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

	// paru -S --noconfirm --needed --skipreview pkg1 pkg2 ...
	// --skipreview skips PKGBUILD review prompts for AUR packages
	args := append([]string{"-S", "--noconfirm", "--needed", "--skipreview"}, pkgs...)
	output, err := m.Runner.Run(ctx, helper, args...)
	if err != nil {
		return string(output), fmt.Errorf("%s install: %w", helper, err)
	}
	return string(output), nil
}

func (m *PacmanManager) ListInstalledCasks(ctx context.Context) ([]string, error) {
	// Pacman doesn't support casks
	return nil, nil
}

func (m *PacmanManager) InstallCasks(ctx context.Context, casks []string) (string, error) {
	// Pacman doesn't support casks - no-op
	return "", nil
}

func (m *PacmanManager) SupportsCasks() bool {
	return false
}

// HomebrewManager implements PackageManager for macOS using Homebrew.
type HomebrewManager struct {
	Runner cmdexec.Runner
}

// NewHomebrewManager creates a new HomebrewManager with the given runner.
func NewHomebrewManager(runner cmdexec.Runner) *HomebrewManager {
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}
	return &HomebrewManager{Runner: runner}
}

func (m *HomebrewManager) Name() string {
	return "homebrew"
}

func (m *HomebrewManager) ListInstalled(ctx context.Context) ([]string, error) {
	output, err := m.Runner.Run(ctx, "brew", "list", "--formulae")
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
	output, err := m.Runner.Run(ctx, "brew", args...)
	if err != nil {
		return string(output), fmt.Errorf("brew install: %w", err)
	}
	return string(output), nil
}

func (m *HomebrewManager) ListInstalledCasks(ctx context.Context) ([]string, error) {
	output, err := m.Runner.Run(ctx, "brew", "list", "--casks")
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
	output, err := m.Runner.Run(ctx, "brew", args...)
	if err != nil {
		return string(output), fmt.Errorf("brew install casks: %w", err)
	}
	return string(output), nil
}

func (m *HomebrewManager) SupportsCasks() bool {
	return true
}

// parseLines splits output by newlines and filters empty strings.
func parseLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var result []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

// toSet converts a slice to a map for O(1) lookups.
func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// PkgInstall installs packages using the system package manager.
type PkgInstall struct {
	Manager  PackageManager
	OS       string
	Packages []string
	Casks    []string
}

// Name returns a human-readable description for display.
func (t *PkgInstall) Name() string {
	parts := []string{}
	if len(t.Packages) > 0 {
		if len(t.Packages) <= 3 {
			parts = append(parts, strings.Join(t.Packages, ", "))
		} else {
			parts = append(parts, fmt.Sprintf("%d packages", len(t.Packages)))
		}
	}
	if len(t.Casks) > 0 {
		if len(t.Casks) <= 3 {
			parts = append(parts, "casks: "+strings.Join(t.Casks, ", "))
		} else {
			parts = append(parts, fmt.Sprintf("%d casks", len(t.Casks)))
		}
	}
	if len(parts) == 0 {
		return "install packages: (none)"
	}
	return "install packages: " + strings.Join(parts, " + ")
}

// Run executes the package installation. It is idempotent.
// Uses batch checking for efficiency: O(2) shell calls instead of O(n).
func (t *PkgInstall) Run(ctx context.Context) Result {
	// Warn if casks specified on non-macOS
	if err := t.validateCaskSupport(); err != nil {
		return Result{
			Status:  StatusFailed,
			Message: "casks are only supported on macOS",
			Error:   err,
		}
	}

	// Determine what needs to be installed
	toInstall, err := t.findMissingPackages(ctx)
	if err != nil {
		return Result{Status: StatusFailed, Error: err}
	}

	casksToInstall, err := t.findMissingCasks(ctx)
	if err != nil {
		return Result{Status: StatusFailed, Error: err}
	}

	// If everything is installed, skip
	if len(toInstall) == 0 && len(casksToInstall) == 0 {
		return Result{Status: StatusSkipped, Message: "all packages already installed"}
	}

	// Install packages and casks
	return t.performInstallation(ctx, toInstall, casksToInstall)
}

// validateCaskSupport returns an error if casks are specified on a non-macOS system.
func (t *PkgInstall) validateCaskSupport() error {
	if len(t.Casks) > 0 && t.OS != "darwin" && !t.Manager.SupportsCasks() {
		return fmt.Errorf("casks specified but OS is %s (not darwin)", t.OS)
	}
	return nil
}

// findMissingPackages returns the list of packages that need to be installed.
func (t *PkgInstall) findMissingPackages(ctx context.Context) ([]string, error) {
	if len(t.Packages) == 0 {
		return nil, nil
	}

	installed, err := t.Manager.ListInstalled(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installed: %w", err)
	}

	installedSet := toSet(installed)
	var toInstall []string
	for _, pkg := range t.Packages {
		if !installedSet[pkg] {
			toInstall = append(toInstall, pkg)
		}
	}
	return toInstall, nil
}

// findMissingCasks returns the list of casks that need to be installed.
func (t *PkgInstall) findMissingCasks(ctx context.Context) ([]string, error) {
	if len(t.Casks) == 0 {
		return nil, nil
	}

	installed, err := t.Manager.ListInstalledCasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installed casks: %w", err)
	}

	installedSet := toSet(installed)
	var toInstall []string
	for _, cask := range t.Casks {
		if !installedSet[cask] {
			toInstall = append(toInstall, cask)
		}
	}
	return toInstall, nil
}

// performInstallation installs the specified packages and casks.
func (t *PkgInstall) performInstallation(ctx context.Context, packages, casks []string) Result {
	var allOutput strings.Builder

	// Install packages in batch
	if len(packages) > 0 {
		output, err := t.Manager.Install(ctx, packages)
		if output != "" {
			allOutput.WriteString(output)
		}
		if err != nil {
			return Result{Status: StatusFailed, Error: err, Output: allOutput.String()}
		}
	}

	// Install casks in batch
	if len(casks) > 0 {
		output, err := t.Manager.InstallCasks(ctx, casks)
		if output != "" {
			if allOutput.Len() > 0 {
				allOutput.WriteString("\n")
			}
			allOutput.WriteString(output)
		}
		if err != nil {
			return Result{Status: StatusFailed, Error: err, Output: allOutput.String()}
		}
	}

	// Build result message
	msg := t.buildResultMessage(packages, casks)
	return Result{Status: StatusDone, Message: msg, Output: allOutput.String()}
}

// buildResultMessage constructs a human-readable message about what was installed.
func (t *PkgInstall) buildResultMessage(packages, casks []string) string {
	var msg []string
	if len(packages) > 0 {
		msg = append(msg, fmt.Sprintf("installed %d packages", len(packages)))
	}
	if len(casks) > 0 {
		msg = append(msg, fmt.Sprintf("installed %d casks", len(casks)))
	}
	return strings.Join(msg, ", ")
}

// PkgInstallConfig holds the factory configuration.
type PkgInstallConfig struct {
	Runner  cmdexec.Runner
	Manager PackageManager
	OS      string
}

// NewPkgInstallFactory creates a factory for PkgInstall tasks.
// The factory parses two YAML formats:
//
// Simple format (list of strings):
//
//	args:
//	  - git
//	  - curl
//
// Structured format (with packages and casks):
//
//	args:
//	  - packages:
//	      - git
//	  - casks:
//	      - firefox
func NewPkgInstallFactory(cfg PkgInstallConfig) Factory {
	return func(args any) ([]Task, error) {
		packages, casks, err := parsePkgInstallArgs(args)
		if err != nil {
			return nil, err
		}

		if len(packages) == 0 && len(casks) == 0 {
			return nil, nil
		}

		// Use provided manager or create default based on OS
		manager := cfg.Manager
		if manager == nil {
			if cfg.OS == "darwin" {
				manager = NewHomebrewManager(cfg.Runner)
			} else {
				manager = NewPacmanManager(cfg.Runner)
			}
		}

		return []Task{&PkgInstall{
			Packages: packages,
			Casks:    casks,
			Manager:  manager,
			OS:       cfg.OS,
		}}, nil
	}
}

// parsePkgInstallArgs handles both simple and structured YAML formats.
func parsePkgInstallArgs(args any) (packages, casks []string, err error) {
	list, ok := args.([]any)
	if !ok {
		return nil, nil, errors.New("args must be a list")
	}

	for i, item := range list {
		switch v := item.(type) {
		case string:
			// Simple format: item is a package name
			packages = append(packages, v)

		case map[string]any:
			// Structured format: item has "packages" and/or "casks" keys
			if pkgs, ok := v["packages"]; ok {
				parsed, err := parseStringList(pkgs, fmt.Sprintf("arg %d packages", i+1))
				if err != nil {
					return nil, nil, err
				}
				packages = append(packages, parsed...)
			}
			if caskList, ok := v["casks"]; ok {
				parsed, err := parseStringList(caskList, fmt.Sprintf("arg %d casks", i+1))
				if err != nil {
					return nil, nil, err
				}
				casks = append(casks, parsed...)
			}

		default:
			return nil, nil, fmt.Errorf("arg %d: must be a string or map, got %T", i+1, item)
		}
	}

	return packages, casks, nil
}

// parseStringList converts []interface{} to []string.
func parseStringList(v any, context string) ([]string, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: must be a list", context)
	}

	var result []string
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: must be a string", context, i)
		}
		result = append(result, s)
	}
	return result, nil
}
