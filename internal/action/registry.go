package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"

	"gopkg.in/yaml.v3"
)

type Factory func(args yaml.Node, ctx *expr.Context) (Action, error)

func Registry() map[string]Factory {
	host := newHost(cmdexec.DefaultRunner())
	return map[string]Factory{
		"dir.create":          NewDirCreate,
		"symlink.create":      NewSymlinkCreate,
		"template.render":     NewTemplateRender,
		"pkg-manager.install": host.pkgManagerInstall,
		"pkg.install":         host.pkgInstall,
		"mise.use":            host.miseUse,
	}
}

type host struct {
	runner cmdexec.Runner
	plan   *helperPlan
}

func newHost(runner cmdexec.Runner) *host {
	return &host{runner: runner, plan: &helperPlan{}}
}

func (h *host) pkgManagerInstall(args yaml.Node, ctx *expr.Context) (Action, error) {
	act, err := NewPkgManagerInstall(args, ctx, h.runner)
	if err != nil {
		return nil, err
	}
	act.plan = h.plan
	return act, nil
}

func (h *host) pkgInstall(args yaml.Node, ctx *expr.Context) (Action, error) {
	act, err := NewPkgInstall(args, ctx, h.runner)
	if err != nil {
		return nil, err
	}
	if pacman, ok := act.Manager.(*PacmanManager); ok {
		pacman.plan = h.plan
	}
	return act, nil
}

func (h *host) miseUse(args yaml.Node, ctx *expr.Context) (Action, error) {
	return NewMiseUse(args, ctx, h.runner)
}
