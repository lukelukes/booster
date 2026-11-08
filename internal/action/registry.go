package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"

	"gopkg.in/yaml.v3"
)

type Factory func(args yaml.Node, ctx *expr.Context) (Action, error)

func Registry() map[string]Factory {
	return NewRegistry(cmdexec.DefaultRunner())
}

func NewRegistry(runner cmdexec.Runner) map[string]Factory {
	host := &host{runner: runner}
	return map[string]Factory{
		"dir.create":          NewDirCreate,
		"symlink.create":      NewSymlinkCreate,
		"template.render":     NewTemplateRender,
		"pkg-manager.install": host.pkgManagerInstall,
		"pkg.install":         host.pkgInstall,
		"mise.use":            host.miseUse,
		"git.config":          host.gitConfig,
	}
}

type host struct {
	runner        cmdexec.Runner
	helperPlanned bool
}

func (h *host) pkgManagerInstall(args yaml.Node, ctx *expr.Context) (Action, error) {
	act, err := NewPkgManagerInstall(args, ctx, h.runner)
	if err != nil {
		return nil, err
	}
	for _, manager := range act.Managers {
		if _, aur := aurHelperRepos[manager]; aur {
			h.helperPlanned = true
		}
	}
	return act, nil
}

func (h *host) pkgInstall(args yaml.Node, ctx *expr.Context) (Action, error) {
	act, err := NewPkgInstall(args, ctx, h.runner)
	if err != nil {
		return nil, err
	}
	if pacman, ok := act.Manager.(*PacmanManager); ok {
		pacman.helperPlanned = h.helperPlanned
	}
	return act, nil
}

func (h *host) miseUse(args yaml.Node, ctx *expr.Context) (Action, error) {
	return NewMiseUse(args, ctx, h.runner)
}

func (h *host) gitConfig(args yaml.Node, ctx *expr.Context) (Action, error) {
	return NewGitConfig(args, ctx, h.runner)
}
