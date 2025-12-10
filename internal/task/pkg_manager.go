package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PkgManagerInstall installs a package manager (e.g., paru for Arch Linux).
type PkgManagerInstall struct {
	Runner     cmdexec.Runner
	Manager    string
	PathFinder BrewPathFinder
}

// Name returns a human-readable description for display.
func (t *PkgManagerInstall) Name() string {
	return "install package manager: " + t.Manager
}

// NeedsSudo returns true - installing package managers requires elevated privileges.
// On Linux: makepkg -si needs sudo for the install phase.
// On macOS: Homebrew install script needs sudo to create /opt/homebrew or /usr/local.
func (t *PkgManagerInstall) NeedsSudo() bool {
	return true
}

// Run executes the installation. It is idempotent - skips if already installed.
// Uses belt-and-suspenders check: binary must exist AND package must be registered.
func (t *PkgManagerInstall) Run(ctx context.Context) Result {
	runner := t.Runner
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}

	// Belt-and-suspenders: check both binary AND package registration
	binaryExists := t.checkBinaryExists(runner)
	packageRegistered := t.checkPackageRegistered(ctx, runner)

	if binaryExists && packageRegistered {
		return Result{Status: StatusSkipped, Message: "already installed"}
	}

	// Install based on package manager type
	switch t.Manager {
	case "paru":
		return t.installParu(ctx, runner)
	case "yay":
		return t.installYay(ctx, runner)
	case "homebrew":
		return t.installHomebrew(ctx, runner)
	default:
		return Result{
			Status: StatusFailed,
			Error:  fmt.Errorf("unsupported package manager: %s", t.Manager),
		}
	}
}

// checkBinaryExists returns true if the binary is found.
// For Homebrew, checks well-known paths directly to avoid stale PATH issues.
func (t *PkgManagerInstall) checkBinaryExists(runner cmdexec.Runner) bool {
	// Homebrew: check known paths directly (PATH may be stale if installed this session)
	if t.Manager == "homebrew" {
		finder := t.PathFinder
		if finder == nil {
			finder = defaultBrewPathFinder
		}
		_, found := finder()
		return found
	}

	// Other package managers: use PATH lookup
	_, err := runner.LookPath(t.Manager)
	return err == nil
}

// checkPackageRegistered returns true if the package is registered with the system.
// For Arch (paru/yay), uses pacman -Q. For Homebrew, binary check is sufficient.
func (t *PkgManagerInstall) checkPackageRegistered(ctx context.Context, runner cmdexec.Runner) bool {
	// Homebrew has no package registry - binary check is sufficient
	if t.Manager == "homebrew" {
		return true
	}
	_, err := runner.Run(ctx, "pacman", "-Q", t.Manager)
	return err == nil
}

// installParu installs paru from AUR.
func (t *PkgManagerInstall) installParu(ctx context.Context, runner cmdexec.Runner) Result {
	return t.installFromAUR(ctx, runner, "paru", "https://aur.archlinux.org/paru.git")
}

// installYay installs yay from AUR.
func (t *PkgManagerInstall) installYay(ctx context.Context, runner cmdexec.Runner) Result {
	return t.installFromAUR(ctx, runner, "yay", "https://aur.archlinux.org/yay.git")
}

// installFromAUR clones and builds a package from AUR.
func (t *PkgManagerInstall) installFromAUR(ctx context.Context, runner cmdexec.Runner, name, url string) Result {
	// Create temp directory for build
	tmpDir, err := os.MkdirTemp("", "booster-aur-")
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("create temp dir: %w", err)}
	}
	defer func() {
		// Cleanup temp directory - error ignored as we're already returning
		_ = os.RemoveAll(tmpDir)
	}()

	cloneDir := filepath.Join(tmpDir, name)
	var allOutput string

	// Clone the AUR repo
	cloneOutput, err := runner.Run(ctx, "git", "clone", url, cloneDir)
	allOutput = string(cloneOutput)
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("clone %s: %w", name, err), Output: allOutput}
	}

	// Build and install with makepkg
	// Note: makepkg needs to run in the cloned directory
	// We use sh -c to handle the cd
	cmd := fmt.Sprintf("cd %s && makepkg -si --noconfirm", cloneDir)
	makepkgOutput, err := runner.Run(ctx, "sh", "-c", cmd)
	if allOutput != "" && len(makepkgOutput) > 0 {
		allOutput += "\n"
	}
	allOutput += string(makepkgOutput)
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("makepkg %s: %w", name, err), Output: allOutput}
	}

	return Result{Status: StatusDone, Message: "installed", Output: allOutput}
}

// installHomebrew installs Homebrew on macOS using the official install script.
func (t *PkgManagerInstall) installHomebrew(ctx context.Context, runner cmdexec.Runner) Result {
	const installURL = "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"

	var allOutput string

	// Download the install script
	scriptOutput, err := runner.Run(ctx, "curl", "-fsSL", installURL)
	allOutput = string(scriptOutput)
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("download homebrew install script: %w", err), Output: allOutput}
	}

	// Write script to temp file for execution
	tmpFile, err := os.CreateTemp("", "homebrew-install-*.sh")
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("create temp file: %w", err), Output: allOutput}
	}
	defer func() {
		// Cleanup temp file - error ignored as we're already returning
		_ = os.Remove(tmpFile.Name())
	}()

	if _, writeErr := tmpFile.Write(scriptOutput); writeErr != nil {
		_ = tmpFile.Close()
		return Result{Status: StatusFailed, Error: fmt.Errorf("write install script: %w", writeErr), Output: allOutput}
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("close temp file: %w", closeErr), Output: allOutput}
	}

	// Execute the install script with NONINTERACTIVE=1
	cmd := "NONINTERACTIVE=1 bash " + tmpFile.Name()
	installOutput, err := runner.Run(ctx, "bash", "-c", cmd)
	if len(installOutput) > 0 {
		if allOutput != "" {
			allOutput += "\n"
		}
		allOutput += string(installOutput)
	}
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("install homebrew: %w", err), Output: allOutput}
	}

	return Result{Status: StatusDone, Message: "installed", Output: allOutput}
}

// NewPkgManagerInstallFactory creates a factory for PkgManagerInstall tasks.
// If runner is nil, tasks will use the default system runner.
func NewPkgManagerInstallFactory(runner cmdexec.Runner) Factory {
	return func(args any) ([]Task, error) {
		list, ok := args.([]any)
		if !ok {
			return nil, errors.New("args must be a list of package manager names")
		}

		var tasks []Task
		for i, item := range list {
			name, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("arg %d: must be a string", i+1)
			}
			tasks = append(tasks, &PkgManagerInstall{
				Manager: name,
				Runner:  runner,
			})
		}

		return tasks, nil
	}
}
