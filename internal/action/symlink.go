package action

import (
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type SymlinkCreate struct {
	Links []SourceTarget
}

func NewSymlinkCreate(args yaml.Node, ctx *expr.Context) (Action, error) {
	links, err := decodeSourceTargets(args, ctx)
	if err != nil {
		return nil, err
	}

	return &SymlinkCreate{Links: links}, nil
}

func (a *SymlinkCreate) Name() string {
	return "symlink.create"
}

func (a *SymlinkCreate) NeedsSudo() bool {
	return false
}

func (a *SymlinkCreate) Check(_ context.Context) (model.Change, error) {
	change := model.Change{Verb: "link", Total: len(a.Links)}

	for _, link := range a.Links {
		source, target, err := resolvePaths(link)
		if err != nil {
			return model.Change{}, err
		}

		current, err := readLink(target)
		if err != nil {
			return model.Change{}, err
		}
		if current == source {
			continue
		}

		change.Items = append(change.Items, model.Item{Name: target, From: current, To: source})
		if current != "" {
			change.Kind = model.ChangeUpdate
		}
	}

	if change.Kind == model.ChangeNone && len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
	}

	return change, nil
}

func (a *SymlinkCreate) Apply(_ context.Context) error {
	for _, link := range a.Links {
		source, target, err := resolvePaths(link)
		if err != nil {
			return err
		}

		current, err := readLink(target)
		if err != nil {
			return err
		}
		if current == source {
			continue
		}
		if current != "" {
			if err := os.Remove(target); err != nil {
				return err
			}
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(source, target); err != nil {
			return err
		}
	}

	return nil
}

func resolvePaths(link SourceTarget) (source, target string, err error) {
	source = expr.ExpandPath(link.Source)
	if !filepath.IsAbs(source) {
		source, err = filepath.Abs(source)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve source path: %w", err)
		}
	}

	if _, err := os.Stat(source); err != nil {
		return "", "", fmt.Errorf("source does not exist: %s", source)
	}

	return source, expr.ExpandPath(link.Target), nil
}

func readLink(target string) (string, error) {
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("target exists but is not a symlink: %s", target)
	}

	return os.Readlink(target)
}
