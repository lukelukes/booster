package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"fmt"
	"strings"
)

type GitConfigItem struct {
	Key   string
	Value string
}

type GitConfigStore interface {
	GetGlobal(ctx context.Context, key string) (string, error)
	SetGlobal(ctx context.Context, key, value string) (string, error)
}

type GitCLIStore struct {
	Runner cmdexec.Runner
}

func (s *GitCLIStore) GetGlobal(ctx context.Context, key string) (string, error) {
	output, err := s.Runner.Run(ctx, "git", "config", "--global", "--get", key)
	return strings.TrimSpace(string(output)), err
}

func (s *GitCLIStore) SetGlobal(ctx context.Context, key, value string) (string, error) {
	output, err := s.Runner.Run(ctx, "git", "config", "--global", key, value)
	return string(output), err
}

type GitConfig struct {
	Store GitConfigStore
	Items []GitConfigItem
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

	if t.Store == nil {
		return Result{Status: StatusFailed, Error: errors.New("git config store is required")}
	}

	for _, item := range t.Items {
		existing, _ := t.Store.GetGlobal(ctx, item.Key)
		if existing == item.Value {
			skipped = append(skipped, item.Key)
			continue
		}

		setOutput, setErr := t.Store.SetGlobal(ctx, item.Key, item.Value)
		if setErr != nil {
			return Result{
				Status: StatusFailed,
				Error:  fmt.Errorf("set %s: %w", item.Key, setErr),
				Output: setOutput,
			}
		}
		configured = append(configured, item.Key)
	}

	if len(configured) == 0 {
		return Result{
			Status:  StatusSkipped,
			Message: "all keys already configured",
		}
	}

	msg := fmt.Sprintf("configured %d keys", len(configured))
	if len(skipped) > 0 {
		msg += fmt.Sprintf(" (skipped %d)", len(skipped))
	}

	return Result{
		Status:  StatusDone,
		Message: msg,
	}
}

func NewGitConfig(runner cmdexec.Runner) Factory {
	return func(args any) ([]Task, error) {
		items, err := parseGitConfigArgs(args)
		if err != nil {
			return nil, err
		}

		if len(items) == 0 {
			return nil, nil
		}

		return []Task{&GitConfig{
			Store: &GitCLIStore{Runner: runner},
			Items: items,
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

		if _, ok := m["prompt"]; ok {
			return nil, fmt.Errorf("arg %d: 'prompt' is no longer supported; define a variable with a prompt and use value: ${ vars.<name> }", i+1)
		}

		value, ok := m["value"].(string)
		if !ok || value == "" {
			return nil, fmt.Errorf("arg %d: 'value' is required and must be a string", i+1)
		}

		items = append(items, GitConfigItem{
			Key:   key,
			Value: value,
		})
	}

	return items, nil
}
