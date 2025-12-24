package tui

import (
	"booster/internal/variable"
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/huh"
)

type PromptCollector struct {
	input      io.Reader
	accessible bool
}

func NewPromptCollector() *PromptCollector {
	return &PromptCollector{}
}

func (p *PromptCollector) WithInput(r io.Reader) *PromptCollector {
	p.input = r
	p.accessible = true
	return p
}

func (p *PromptCollector) Collect(defs []variable.Definition) (map[string]string, error) {
	if len(defs) == 0 {
		return make(map[string]string), nil
	}

	values := make([]string, len(defs))
	for i, def := range defs {
		values[i] = def.Default
	}

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

	form := huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(huh.ThemeCatppuccin())

	if p.input != nil {
		form = form.WithInput(p.input)
	}
	if p.accessible {
		form = form.WithAccessible(true)
	}

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt cancelled: %w", err)
	}

	result := make(map[string]string)
	for i, def := range defs {
		result[def.Name] = values[i]
	}

	return result, nil
}

type HuhPrompter struct {
	input      io.Reader
	accessible bool
}

func NewHuhPrompter() *HuhPrompter {
	return &HuhPrompter{}
}

func (p *HuhPrompter) WithInput(r io.Reader) *HuhPrompter {
	p.input = r
	p.accessible = true
	return p
}

func (p *HuhPrompter) Prompt(ctx context.Context, promptText string) (string, error) {
	var value string

	input := huh.NewInput().
		Title(promptText).
		Value(&value)

	form := huh.NewForm(
		huh.NewGroup(input),
	).WithTheme(huh.ThemeCatppuccin())

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
