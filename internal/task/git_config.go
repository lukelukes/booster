package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"fmt"
	"strings"
)

// Prompter is an interface for prompting users for input.
// This abstraction enables testability by allowing mock implementations.
type Prompter interface {
	// Prompt asks the user for input using the given prompt text.
	// Returns the user's response or an error if the prompt was cancelled.
	Prompt(ctx context.Context, promptText string) (string, error)
}

// GitConfigItem represents a single git configuration key-value pair.
type GitConfigItem struct {
	Key    string // Git config key (e.g., "user.name")
	Value  string // Explicit value (if set, no prompt needed)
	Prompt string // Prompt text (used if no Value and key doesn't exist)
}

// GitConfig sets global git configuration values.
// It is idempotent: only sets values that don't already exist or have explicit values.
type GitConfig struct {
	Runner   cmdexec.Runner
	Prompter Prompter
	Items    []GitConfigItem
}

// Name returns a human-readable description for display.
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

// Run executes the git configuration. It is idempotent.
// For each item:
//   - If Value is set explicitly: set it unconditionally
//   - If Value is empty: check if key exists
//   - If exists: skip
//   - If doesn't exist and Prompt is set: prompt user
//   - If doesn't exist and no Prompt: skip
func (t *GitConfig) Run(ctx context.Context) Result {
	if len(t.Items) == 0 {
		return Result{Status: StatusSkipped, Message: "no items to configure"}
	}

	var configured []string
	var skipped []string
	var allOutput strings.Builder

	for _, item := range t.Items {
		// Check if key already exists
		output, err := t.Runner.Run(ctx, "git", "config", "--global", "--get", item.Key)
		existing := strings.TrimSpace(string(output))

		// If explicit value is provided, always set it (even if it already exists)
		if item.Value != "" {
			// Only set if different from existing value
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

		// No explicit value provided
		// If key already exists, skip it (respect user's existing config)
		if err == nil && existing != "" {
			skipped = append(skipped, item.Key)
			continue
		}

		// Key doesn't exist - prompt if prompt text is provided
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

			// Set the prompted value
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
			// No prompt provided and key doesn't exist - skip
			skipped = append(skipped, item.Key)
		}
	}

	// Build result message
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

// NewGitConfig creates a factory for GitConfig tasks.
// The factory parses YAML in the following format:
//
//	args:
//	  - key: "user.name"
//	    prompt: "What is your name for git commits?"
//	  - key: "user.email"
//	    prompt: "What is your email for git commits?"
//	  - key: "init.defaultBranch"
//	    value: "main"
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

// parseGitConfigArgs parses the YAML args into GitConfigItem structs.
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

		// Key is required
		key, ok := m["key"].(string)
		if !ok || key == "" {
			return nil, fmt.Errorf("arg %d: 'key' is required and must be a string", i+1)
		}

		// Value is optional
		value := ""
		if v, ok := m["value"].(string); ok {
			value = v
		}

		// Prompt is optional
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
