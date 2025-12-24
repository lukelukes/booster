package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"fmt"
	"strings"
)

type Prompter interface {
	Prompt(ctx context.Context, promptText string) (string, error)
}

type GitConfigItem struct {
	Key    string
	Value  string
	Prompt string
}

type GitConfig struct {
	Runner   cmdexec.Runner
	Prompter Prompter
	Items    []GitConfigItem
}

func (t *GitConfig) Name() string {
	if len(t.Items) == 0 {
		return "configure git: (none)"
	}
	if len(t.Items) == 1 {
		return "configure git: " + t.Items[0].Key
	}
	keys := make([]string, 0, len(t.Items))
	for _, item := range t.Items {
		keys = append(keys, item.Key)
	}
	return "configure git: " + strings.Join(keys, ", ")
}

func (t *GitConfig) NeedsSudo() bool {
	return false
}

func (t *GitConfig) Run(ctx context.Context) Result {
	if len(t.Items) == 0 {
		return Result{Status: StatusSkipped, Message: "no items to configure"}
	}

	var configured []string
	var skipped []string
	var allOutput strings.Builder

	for _, item := range t.Items {

		output, err := t.Runner.Run(ctx, "git", "config", "--global", "--get", item.Key)
		existing := strings.TrimSpace(string(output))

		if item.Value != "" {

			if existing == item.Value {
				skipped = append(skipped, item.Key)
				continue
			}

			setOutput, setErr := t.Runner.Run(ctx, "git", "config", "--global", item.Key, item.Value)
			if setErr != nil {
				if allOutput.Len() > 0 {
					allOutput.WriteString("\n")
				}
				allOutput.Write(setOutput)
				return Result{
					Status: StatusFailed,
					Error:  fmt.Errorf("set %s: %w", item.Key, setErr),
					Output: allOutput.String(),
				}
			}
			configured = append(configured, item.Key)
			continue
		}

		if err == nil && existing != "" {
			skipped = append(skipped, item.Key)
			continue
		}

		if item.Prompt != "" {
			if t.Prompter == nil {
				return Result{
					Status: StatusFailed,
					Error:  fmt.Errorf("cannot prompt for %s: no prompter configured", item.Key),
				}
			}

			value, promptErr := t.Prompter.Prompt(ctx, item.Prompt)
			if promptErr != nil {
				return Result{
					Status: StatusFailed,
					Error:  fmt.Errorf("prompt for %s: %w", item.Key, promptErr),
				}
			}

			setOutput, setErr := t.Runner.Run(ctx, "git", "config", "--global", item.Key, value)
			if setErr != nil {
				if allOutput.Len() > 0 {
					allOutput.WriteString("\n")
				}
				allOutput.Write(setOutput)
				return Result{
					Status: StatusFailed,
					Error:  fmt.Errorf("set %s: %w", item.Key, setErr),
					Output: allOutput.String(),
				}
			}
			configured = append(configured, item.Key)
		} else {
			skipped = append(skipped, item.Key)
		}
	}

	if len(configured) == 0 {
		return Result{
			Status:  StatusSkipped,
			Message: "all keys already configured",
			Output:  allOutput.String(),
		}
	}

	msg := fmt.Sprintf("configured %d keys", len(configured))
	if len(skipped) > 0 {
		msg += fmt.Sprintf(" (skipped %d)", len(skipped))
	}

	return Result{
		Status:  StatusDone,
		Message: msg,
		Output:  allOutput.String(),
	}
}

func NewGitConfig(runner cmdexec.Runner, prompter Prompter) Factory {
	return func(args any) ([]Task, error) {
		items, err := parseGitConfigArgs(args)
		if err != nil {
			return nil, err
		}

		if len(items) == 0 {
			return nil, nil
		}

		return []Task{&GitConfig{
			Runner:   runner,
			Prompter: prompter,
			Items:    items,
		}}, nil
	}
}

func parseGitConfigArgs(args any) ([]GitConfigItem, error) {
	list, ok := args.([]any)
	if !ok {
		return nil, errors.New("args must be a list")
	}

	var items []GitConfigItem
	for i, arg := range list {
		m, ok := arg.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("arg %d: must be a map with 'key' field", i+1)
		}

		key, ok := m["key"].(string)
		if !ok || key == "" {
			return nil, fmt.Errorf("arg %d: 'key' is required and must be a string", i+1)
		}

		value := ""
		if v, ok := m["value"].(string); ok {
			value = v
		}

		prompt := ""
		if p, ok := m["prompt"].(string); ok {
			prompt = p
		}

		items = append(items, GitConfigItem{
			Key:    key,
			Value:  value,
			Prompt: prompt,
		})
	}

	return items, nil
}
