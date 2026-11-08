package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

type GitConfigEntry struct {
	Key   string
	Value string
}

type GitConfig struct {
	Runner  cmdexec.Runner
	Entries []GitConfigEntry
}

type gitConfigArgs struct {
	Key    string    `yaml:"key"`
	Value  *expr.Str `yaml:"value"`
	Prompt string    `yaml:"prompt"`
}

func NewGitConfig(args yaml.Node, ctx *expr.Context, runner cmdexec.Runner) (Action, error) {
	if args.IsZero() || args.Kind != yaml.SequenceNode {
		return nil, errors.New("args must be a list of {key, value} entries")
	}

	for i, item := range args.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("[%d]: must be a mapping with 'key' and 'value'", i)
		}
	}

	var raw []gitConfigArgs
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}

	entries := make([]GitConfigEntry, 0, len(raw))
	for i, item := range raw {
		if item.Key == "" {
			return nil, fmt.Errorf("[%d]: missing 'key'", i)
		}
		if item.Prompt != "" {
			return nil, fmt.Errorf("[%d]: 'prompt' is no longer supported; define a variable with a prompt and use value: ${ vars.<name> }", i)
		}
		if item.Value == nil {
			return nil, fmt.Errorf("[%d]: missing 'value'", i)
		}

		value, err := item.Value.Resolve(ctx)
		if err != nil {
			return nil, fmt.Errorf("[%d].value: %w", i, err)
		}

		entries = append(entries, GitConfigEntry{Key: item.Key, Value: value})
	}

	return &GitConfig{Runner: runner, Entries: entries}, nil
}

func (a *GitConfig) Name() string {
	return "git.config"
}

func (a *GitConfig) Check(ctx context.Context) (model.Change, error) {
	change := model.Change{Verb: "set", Total: len(a.Entries)}

	if !a.available() {
		change.Kind = model.ChangeUnknown
		return change, nil
	}

	for _, entry := range a.Entries {
		current, exists, err := a.current(ctx, entry.Key)
		if err != nil {
			return model.Change{}, err
		}
		if exists && current == entry.Value {
			continue
		}

		change.Items = append(change.Items, model.Item{Name: entry.Key, From: current, To: entry.Value})
		if exists {
			change.Kind = model.ChangeUpdate
		}
	}

	if change.Kind == model.ChangeNone && len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
	}

	return change, nil
}

func (a *GitConfig) Apply(ctx context.Context) error {
	if !a.available() {
		return errors.New("git not found in PATH; install git first (e.g., via pkg.install)")
	}

	for _, entry := range a.Entries {
		current, exists, err := a.current(ctx, entry.Key)
		if err != nil {
			return err
		}
		if exists && current == entry.Value {
			continue
		}

		if output, err := a.Runner.Run(ctx, "git", "config", "--global", entry.Key, entry.Value); err != nil {
			return fmt.Errorf("git config %s: %w: %s", entry.Key, err, strings.TrimSpace(string(output)))
		}
	}

	return nil
}

func (a *GitConfig) available() bool {
	_, err := a.Runner.LookPath("git")
	return err == nil
}

func (a *GitConfig) current(ctx context.Context, key string) (string, bool, error) {
	output, err := a.Runner.Run(ctx, "git", "config", "--global", "--get", key)
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read git config %s: %w: %s", key, err, strings.TrimSpace(string(output)))
	}

	return strings.TrimSuffix(string(output), "\n"), true, nil
}
