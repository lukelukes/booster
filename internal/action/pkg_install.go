package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

type PkgInstall struct {
	Manager  PackageManager
	OS       string
	Packages []string
	Casks    []string
}

type pkgInstallArgs struct {
	Packages []expr.Str `yaml:"packages"`
	Casks    []expr.Str `yaml:"casks"`
}

func (p *pkgInstallArgs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var pkg expr.Str
		if err := pkg.UnmarshalYAML(node); err != nil {
			return err
		}
		p.Packages = []expr.Str{pkg}
		return nil
	}

	if node.Kind != yaml.MappingNode {
		return errors.New("must be a package name or a mapping with 'packages' and/or 'casks'")
	}

	type plain pkgInstallArgs
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*p = pkgInstallArgs(decoded)

	return nil
}

func NewPkgInstall(args yaml.Node, ctx *expr.Context, runner cmdexec.Runner) (*PkgInstall, error) {
	if args.IsZero() || args.Kind != yaml.SequenceNode {
		return nil, errors.New("args must be a list of package names or {packages, casks} entries")
	}

	var raw []pkgInstallArgs
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}

	var packages, casks []expr.Str
	for _, entry := range raw {
		packages = append(packages, entry.Packages...)
		casks = append(casks, entry.Casks...)
	}

	resolvedPackages, err := expr.ResolveAll(packages, ctx)
	if err != nil {
		return nil, fmt.Errorf("packages%w", err)
	}
	resolvedCasks, err := expr.ResolveAll(casks, ctx)
	if err != nil {
		return nil, fmt.Errorf("casks%w", err)
	}

	os := ""
	if ctx != nil {
		os = ctx.OS
	}

	var manager PackageManager = NewPacmanManager(runner)
	if os == "darwin" {
		manager = NewHomebrewManager(runner, nil)
	}

	return &PkgInstall{
		Manager:  manager,
		OS:       os,
		Packages: resolvedPackages,
		Casks:    resolvedCasks,
	}, nil
}

func (a *PkgInstall) Name() string {
	return "pkg.install"
}

func (a *PkgInstall) Check(ctx context.Context) (model.Change, error) {
	change := model.Change{Verb: "install", Total: len(a.Packages) + len(a.Casks)}

	if err := a.validateCasks(); err != nil {
		return model.Change{}, err
	}

	if !a.Manager.Available(ctx) {
		change.Kind = model.ChangeUnknown
		return change, nil
	}

	packages, err := a.missingPackages(ctx)
	if err != nil {
		return model.Change{}, err
	}

	if err := a.checkInstallable(ctx, packages); err != nil {
		return model.Change{}, err
	}

	casks, err := a.missingCasks(ctx)
	if err != nil {
		return model.Change{}, err
	}

	for _, pkg := range append(packages, casks...) {
		change.Items = append(change.Items, model.Item{Name: pkg})
	}
	if len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
		change.NeedsSudo = a.OS != "darwin" || len(casks) > 0
	}

	return change, nil
}

func (a *PkgInstall) Apply(ctx context.Context) error {
	if err := a.validateCasks(); err != nil {
		return err
	}

	packages, err := a.missingPackages(ctx)
	if err != nil {
		return err
	}
	if len(packages) > 0 {
		if _, err := a.Manager.Install(ctx, packages); err != nil {
			return err
		}
	}

	casks, err := a.missingCasks(ctx)
	if err != nil {
		return err
	}
	if len(casks) > 0 {
		if _, err := a.Manager.(CaskInstaller).InstallCasks(ctx, casks); err != nil {
			return err
		}
	}

	return nil
}

func (a *PkgInstall) checkInstallable(ctx context.Context, missing []string) error {
	checker, ok := a.Manager.(interface {
		Installable(ctx context.Context, pkgs []string) error
	})
	if !ok {
		return nil
	}

	return checker.Installable(ctx, missing)
}

func (a *PkgInstall) validateCasks() error {
	if len(a.Casks) == 0 {
		return nil
	}
	if a.OS != "darwin" {
		return fmt.Errorf("casks specified but OS is %s (not darwin)", a.OS)
	}
	if _, ok := a.Manager.(CaskInstaller); !ok {
		return errors.New("casks specified but manager does not support casks")
	}

	return nil
}

func (a *PkgInstall) missingPackages(ctx context.Context) ([]string, error) {
	if len(a.Packages) == 0 {
		return nil, nil
	}

	installed, err := a.Manager.ListInstalled(ctx)
	if err != nil {
		return nil, err
	}

	return missingFrom(installed, a.Packages), nil
}

func (a *PkgInstall) missingCasks(ctx context.Context) ([]string, error) {
	if len(a.Casks) == 0 {
		return nil, nil
	}

	installed, err := a.Manager.(CaskInstaller).ListInstalledCasks(ctx)
	if err != nil {
		return nil, err
	}

	return missingFrom(installed, a.Casks), nil
}
