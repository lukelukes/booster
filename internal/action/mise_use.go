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

type ToolSpec struct {
	Name    string
	Version string
}

func (t ToolSpec) String() string {
	return t.Name + "@" + t.Version
}

func parseToolSpec(s string) (ToolSpec, error) {
	name, version, found := strings.Cut(s, "@")
	if !found || name == "" || version == "" {
		return ToolSpec{}, fmt.Errorf("invalid tool spec %q: expected format tool@version", s)
	}

	return ToolSpec{Name: name, Version: version}, nil
}

type MiseUse struct {
	Runner cmdexec.Runner
	Tools  []ToolSpec
}

func NewMiseUse(args yaml.Node, ctx *expr.Context, runner cmdexec.Runner) (Action, error) {
	if args.IsZero() || args.Kind != yaml.SequenceNode {
		return nil, errors.New("args must be a list of tool@version specs")
	}

	var raw []expr.Str
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}

	specs, err := expr.ResolveAll(raw, ctx)
	if err != nil {
		return nil, err
	}

	tools := make([]ToolSpec, 0, len(specs))
	for i, spec := range specs {
		tool, err := parseToolSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		tools = append(tools, tool)
	}

	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}

	return &MiseUse{Runner: runner, Tools: tools}, nil
}

func (a *MiseUse) Name() string {
	return "mise.use"
}

func (a *MiseUse) NeedsSudo() bool {
	return false
}

func (a *MiseUse) Check(ctx context.Context) (model.Change, error) {
	change := model.Change{Verb: "use", Total: len(a.Tools)}

	if !a.available() {
		change.Kind = model.ChangeUnknown
		return change, nil
	}

	for _, tool := range a.Tools {
		current := a.currentVersion(ctx, tool.Name)
		if current == tool.Version {
			continue
		}

		change.Items = append(change.Items, model.Item{Name: tool.Name, From: current, To: tool.Version})
		if current != "" {
			change.Kind = model.ChangeUpdate
		}
	}

	if change.Kind == model.ChangeNone && len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
	}

	return change, nil
}

func (a *MiseUse) Apply(ctx context.Context) error {
	if !a.available() {
		return errors.New("mise not found in PATH; install mise first (e.g., via pkg.install)")
	}

	for _, tool := range a.Tools {
		if a.currentVersion(ctx, tool.Name) == tool.Version {
			continue
		}

		if _, err := a.Runner.Run(ctx, "mise", "use", "--global", tool.String()); err != nil {
			return fmt.Errorf("mise use %s: %w", tool, err)
		}
	}

	return nil
}

func (a *MiseUse) available() bool {
	_, err := a.Runner.LookPath("mise")
	return err == nil
}

func (a *MiseUse) currentVersion(ctx context.Context, tool string) string {
	output, err := a.Runner.Run(ctx, "mise", "current", tool)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}
