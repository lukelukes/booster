package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type PkgManagerInstall struct {
	Runner     cmdexec.Runner
	Manager    string
	PathFinder BrewPathFinder
}

func (t *PkgManagerInstall) Name() string {
	return "install package manager: " + t.Manager
}

func (t *PkgManagerInstall) NeedsSudo() bool {
	return true
}

func (t *PkgManagerInstall) Run(ctx context.Context) Result {
	runner := t.Runner
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}

	binaryExists := t.checkBinaryExists(runner)
	packageRegistered := t.checkPackageRegistered(ctx, runner)

	if binaryExists && packageRegistered {
		return Result{Status: StatusSkipped, Message: "already installed"}
	}

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

func (t *PkgManagerInstall) checkBinaryExists(runner cmdexec.Runner) bool {
	if t.Manager == "homebrew" {
		finder := t.PathFinder
		if finder == nil {
			finder = defaultBrewPathFinder
		}
		_, found := finder()
		return found
	}

	_, err := runner.LookPath(t.Manager)
	return err == nil
}

func (t *PkgManagerInstall) checkPackageRegistered(ctx context.Context, runner cmdexec.Runner) bool {
	if t.Manager == "homebrew" {
		return true
	}
	_, err := runner.Run(ctx, "pacman", "-Q", t.Manager)
	return err == nil
}

func (t *PkgManagerInstall) installParu(ctx context.Context, runner cmdexec.Runner) Result {
	return t.installFromAUR(ctx, runner, "paru", "https://aur.archlinux.org/paru.git")
}

func (t *PkgManagerInstall) installYay(ctx context.Context, runner cmdexec.Runner) Result {
	return t.installFromAUR(ctx, runner, "yay", "https://aur.archlinux.org/yay.git")
}

func (t *PkgManagerInstall) installFromAUR(ctx context.Context, runner cmdexec.Runner, name, url string) Result {
	tmpDir, err := os.MkdirTemp("", "booster-aur-")
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("create temp dir: %w", err)}
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	cloneDir := filepath.Join(tmpDir, name)
	var allOutput string

	cloneOutput, err := runner.Run(ctx, "git", "clone", url, cloneDir)
	allOutput = string(cloneOutput)
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("clone %s: %w", name, err), Output: allOutput}
	}

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

func (t *PkgManagerInstall) installHomebrew(ctx context.Context, runner cmdexec.Runner) Result {
	const installURL = "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"

	var allOutput string

	scriptOutput, err := runner.Run(ctx, "curl", "-fsSL", installURL)
	allOutput = string(scriptOutput)
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("download homebrew install script: %w", err), Output: allOutput}
	}

	tmpFile, err := os.CreateTemp("", "homebrew-install-*.sh")
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("create temp file: %w", err), Output: allOutput}
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, writeErr := tmpFile.Write(scriptOutput); writeErr != nil {
		_ = tmpFile.Close()
		return Result{Status: StatusFailed, Error: fmt.Errorf("write install script: %w", writeErr), Output: allOutput}
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("close temp file: %w", closeErr), Output: allOutput}
	}

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
