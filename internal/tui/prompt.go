package tui

import (
	"booster/internal/variable"
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/huh"
)

// PromptCollector implements variable.PromptCollector using huh forms.
type PromptCollector struct {
	input      io.Reader
	accessible bool
}

// NewPromptCollector creates a new TUI-based prompt collector.
func NewPromptCollector() *PromptCollector {
	return &PromptCollector{}
}

// WithInput configures the collector to read from the given reader.
// This enables programmatic testing without interactive TUI.
// When input is set, accessible mode is automatically enabled.
func (p *PromptCollector) WithInput(r io.Reader) *PromptCollector {
	p.input = r
	p.accessible = true
	return p
}

// Collect prompts the user for variable values using huh forms.
func (p *PromptCollector) Collect(defs []variable.Definition) (map[string]string, error) {
	if len(defs) == 0 {
		return make(map[string]string), nil
	}

	// Use a slice to hold values (maps aren't addressable)
	values := make([]string, len(defs))
	for i, def := range defs {
		values[i] = def.Default
	}

	// Build form fields
	var fields []huh.Field
	for i, def := range defs {
		prompt := def.Prompt
		if prompt == "" {
			prompt = "Enter " + def.Name
		}

		input := huh.NewInput().
			Title(prompt).
			Value(&values[i]).
			Placeholder(def.Default)

		fields = append(fields, input)
	}

	// Create form
	form := huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(huh.ThemeCatppuccin())

	// Enable programmatic input for testing
	if p.input != nil {
		form = form.WithInput(p.input)
	}
	if p.accessible {
		form = form.WithAccessible(true)
	}

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt cancelled: %w", err)
	}

	// Copy values back to map
	result := make(map[string]string)
	for i, def := range defs {
		result[def.Name] = values[i]
	}

	return result, nil
}

// HuhPrompter implements task.Prompter using huh.
// It prompts users for single values using charmbracelet/huh.
type HuhPrompter struct {
	input      io.Reader
	accessible bool
}

// NewHuhPrompter creates a new HuhPrompter for prompting users.
func NewHuhPrompter() *HuhPrompter {
	return &HuhPrompter{}
}

// WithInput configures the prompter to read from the given reader.
// This enables programmatic testing without interactive TUI.
// When input is set, accessible mode is automatically enabled.
func (p *HuhPrompter) WithInput(r io.Reader) *HuhPrompter {
	p.input = r
	p.accessible = true
	return p
}

// Prompt asks the user for a single value using huh.
func (p *HuhPrompter) Prompt(ctx context.Context, promptText string) (string, error) {
	var value string

	input := huh.NewInput().
		Title(promptText).
		Value(&value)

	form := huh.NewForm(
		huh.NewGroup(input),
	).WithTheme(huh.ThemeCatppuccin())

	// Enable programmatic input for testing
	if p.input != nil {
		form = form.WithInput(p.input)
	}
	if p.accessible {
		form = form.WithAccessible(true)
	}

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("prompt cancelled: %w", err)
	}

	return value, nil
}
