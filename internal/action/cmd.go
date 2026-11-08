package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Cmd struct {
	Runner   cmdexec.Runner
	CheckCmd []string
	RunCmd   []string
	Sudo     bool
}

type cmdArgs struct {
	Check []expr.Str `yaml:"check"`
	Run   []expr.Str `yaml:"run"`
	Sudo  bool       `yaml:"sudo"`
}

func NewCmd(args yaml.Node, ctx *expr.Context, runner cmdexec.Runner) (Action, error) {
	if args.Kind != yaml.MappingNode {
		return nil, errors.New("args must be a mapping with 'run' and optional 'check' and 'sudo'")
	}

	for i := 0; i+1 < len(args.Content); i += 2 {
		key, value := args.Content[i].Value, args.Content[i+1]
		if (key == "check" || key == "run") && value.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("'%s' must be a list like [program, arg, ...]", key)
		}
	}

	var raw cmdArgs
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw.Run) == 0 {
		return nil, errors.New("missing 'run'")
	}

	check, err := expr.ResolveAll(raw.Check, ctx)
	if err != nil {
		return nil, fmt.Errorf("check%w", err)
	}
	run, err := expr.ResolveAll(raw.Run, ctx)
	if err != nil {
		return nil, fmt.Errorf("run%w", err)
	}

	return &Cmd{Runner: runner, CheckCmd: check, RunCmd: run, Sudo: raw.Sudo}, nil
}

func (a *Cmd) Name() string {
	return "cmd"
}

func (a *Cmd) Check(ctx context.Context) (model.Change, error) {
	change := model.Change{Verb: "run", Total: 1}

	if len(a.CheckCmd) > 0 {
		if _, err := a.Runner.Run(ctx, a.CheckCmd[0], a.CheckCmd[1:]...); err == nil {
			return change, nil
		}
		change.Kind = model.ChangeCreate
	} else {
		change.Kind = model.ChangeUnknown
	}

	change.Items = []model.Item{{Name: strings.Join(a.RunCmd, " ")}}
	change.NeedsSudo = a.Sudo
	return change, nil
}

func (a *Cmd) Apply(ctx context.Context) error {
	argv := a.RunCmd
	if a.Sudo {
		argv = append([]string{"sudo"}, argv...)
	}

	if output, err := a.Runner.Run(ctx, argv[0], argv[1:]...); err != nil {
		return fmt.Errorf("%s: %w: %s", strings.Join(a.RunCmd, " "), err, strings.TrimSpace(string(output)))
	}

	return nil
}
