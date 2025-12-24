package task

import (
	"booster/internal/pathutil"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

type TemplateSystem struct {
	OS string

	Profile string
}

type TemplateContext struct {
	Vars map[string]string

	System TemplateSystem
}

type TemplateRender struct {
	Context TemplateContext
	Source  string
	Target  string
}

func (t *TemplateRender) Name() string {
	return fmt.Sprintf("render %s → %s", filepath.Base(t.Source), filepath.Base(t.Target))
}

func (t *TemplateRender) NeedsSudo() bool {
	return false
}

func (t *TemplateRender) Run(ctx context.Context) Result {
	source := pathutil.Expand(t.Source)
	target := pathutil.Expand(t.Target)

	tmplContent, err := os.ReadFile(source)
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("read template: %w", err)}
	}

	tmpl, err := template.New(filepath.Base(source)).Parse(string(tmplContent))
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("parse template: %w", err)}
	}

	var buf bytes.Buffer
	if execErr := tmpl.Execute(&buf, t.Context); execErr != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("execute template: %w", execErr)}
	}
	rendered := buf.Bytes()

	existing, err := os.ReadFile(target)
	if err == nil && bytes.Equal(existing, rendered) {
		return Result{Status: StatusSkipped, Message: "already up to date"}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("create directories: %w", err)}
	}

	if err := os.WriteFile(target, rendered, 0o644); err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("write output: %w", err)}
	}

	return Result{Status: StatusDone, Message: "rendered"}
}

type TemplateRenderConfig struct {
	Vars    map[string]string
	OS      string
	Profile string
}

func NewTemplateRenderFactory(cfg TemplateRenderConfig) Factory {
	return func(args any) ([]Task, error) {
		pairs, err := parseSourceTargetArgs(args)
		if err != nil {
			return nil, err
		}

		ctx := TemplateContext{
			Vars: cfg.Vars,
			System: TemplateSystem{
				OS:      cfg.OS,
				Profile: cfg.Profile,
			},
		}

		tasks := make([]Task, 0, len(pairs))
		for _, pair := range pairs {
			tasks = append(tasks, &TemplateRender{
				Source:  pair.Source,
				Target:  pair.Target,
				Context: ctx,
			})
		}

		return tasks, nil
	}
}
