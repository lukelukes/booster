package action

import (
	"booster/internal/expr"
	"booster/internal/model"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
)

type TemplateSystem struct {
	OS string

	Profile string
}

type TemplateContext struct {
	Vars map[string]any

	System TemplateSystem
}

type TemplateRender struct {
	Context TemplateContext
	Files   []SourceTarget
}

func NewTemplateRender(args yaml.Node, ctx *expr.Context) (Action, error) {
	files, err := decodeSourceTargets(args, ctx)
	if err != nil {
		return nil, err
	}

	render := &TemplateRender{Files: files}
	if ctx != nil {
		render.Context = TemplateContext{
			Vars:   ctx.Vars,
			System: TemplateSystem{OS: ctx.OS, Profile: ctx.Profile},
		}
	}

	return render, nil
}

func (a *TemplateRender) Name() string {
	return "template.render"
}

func (a *TemplateRender) Check(_ context.Context) (model.Change, error) {
	change := model.Change{Verb: "render", Total: len(a.Files)}

	for _, file := range a.Files {
		rendered, err := a.render(file)
		if err != nil {
			return model.Change{}, err
		}

		target := expr.ExpandPath(file.Target)
		existing, err := os.ReadFile(target)
		switch {
		case err == nil && bytes.Equal(existing, rendered):
			continue
		case err == nil:
			change.Items = append(change.Items, model.Item{Name: target})
			change.Kind = model.ChangeUpdate
			continue
		case !errors.Is(err, fs.ErrNotExist):
			return model.Change{}, err
		}

		change.Items = append(change.Items, model.Item{Name: target})
	}

	if change.Kind == model.ChangeNone && len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
	}

	return change, nil
}

func (a *TemplateRender) Apply(_ context.Context) error {
	for _, file := range a.Files {
		rendered, err := a.render(file)
		if err != nil {
			return err
		}

		target := expr.ExpandPath(file.Target)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create directories: %w", err)
		}
		if err := os.WriteFile(target, rendered, 0o644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}

	return nil
}

func (a *TemplateRender) render(file SourceTarget) ([]byte, error) {
	source := expr.ExpandPath(file.Source)

	content, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}

	tmpl, err := template.New(filepath.Base(source)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, a.Context); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), nil
}
