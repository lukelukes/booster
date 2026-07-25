package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var aurHelperRepos = map[string]string{
	"paru": "https://aur.archlinux.org/paru.git",
	"yay":  "https://aur.archlinux.org/yay.git",
}

type PkgManagerInstall struct {
	Runner     cmdexec.Runner
	PathFinder BrewPathFinder
	Managers   []string
}

func NewPkgManagerInstall(args yaml.Node, ctx *expr.Context, runner cmdexec.Runner) (Action, error) {
	if args.IsZero() || args.Kind != yaml.SequenceNode {
		return nil, errors.New("args must be a list of package manager names")
	}

	var raw []expr.Str
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}

	managers, err := expr.ResolveAll(raw, ctx)
	if err != nil {
		return nil, err
	}

	for i, manager := range managers {
		if _, ok := aurHelperRepos[manager]; !ok && manager != "homebrew" {
			return nil, fmt.Errorf("[%d]: unsupported package manager: %s", i, manager)
		}
	}

	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}

	return &PkgManagerInstall{Runner: runner, Managers: managers}, nil
}

func (a *PkgManagerInstall) Name() string {
	return "pkg-manager.install"
}

func (a *PkgManagerInstall) NeedsSudo() bool {
	return true
}

func (a *PkgManagerInstall) Check(ctx context.Context) (model.Change, error) {
	change := model.Change{Verb: "install", Total: len(a.Managers)}

	for _, manager := range a.Managers {
		if a.installed(ctx, manager) {
			continue
		}
		change.Items = append(change.Items, model.Item{Name: manager})
	}

	if len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
		change.NeedsSudo = true
	}

	return change, nil
}

func (a *PkgManagerInstall) Apply(ctx context.Context) error {
	for _, manager := range a.Managers {
		if a.installed(ctx, manager) {
			continue
		}

		if manager == "homebrew" {
			if err := a.installHomebrew(ctx); err != nil {
				return err
			}
			continue
		}

		if err := a.installFromAUR(ctx, manager, aurHelperRepos[manager]); err != nil {
			return err
		}
	}

	return nil
}

func (a *PkgManagerInstall) installed(ctx context.Context, manager string) bool {
	if manager == "homebrew" {
		finder := a.PathFinder
		if finder == nil {
			finder = defaultBrewPathFinder
		}
		_, found := finder()
		return found
	}

	if _, err := a.Runner.LookPath(manager); err != nil {
		return false
	}

	_, err := a.Runner.Run(ctx, "pacman", "-Q", manager)
	return err == nil
}

func (a *PkgManagerInstall) installFromAUR(ctx context.Context, name, url string) error {
	tmpDir, err := os.MkdirTemp("", "booster-aur-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	cloneDir := filepath.Join(tmpDir, name)
	if _, err := a.Runner.Run(ctx, "git", "clone", url, cloneDir); err != nil {
		return fmt.Errorf("clone %s: %w", name, err)
	}

	if _, err := a.Runner.Run(ctx, "sh", "-c", "cd "+cloneDir+" && makepkg -si --noconfirm"); err != nil {
		return fmt.Errorf("makepkg %s: %w", name, err)
	}

	return nil
}

func (a *PkgManagerInstall) installHomebrew(ctx context.Context) error {
	const installURL = "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"

	script, err := a.Runner.Run(ctx, "curl", "-fsSL", installURL)
	if err != nil {
		return fmt.Errorf("download homebrew install script: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "homebrew-install-*.sh")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(script); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write install script: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if _, err := a.Runner.Run(ctx, "bash", "-c", "NONINTERACTIVE=1 bash "+tmpFile.Name()); err != nil {
		return fmt.Errorf("install homebrew: %w", err)
	}

	return nil
}
