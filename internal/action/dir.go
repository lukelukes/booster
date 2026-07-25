package action

import (
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

type DirCreate struct {
	Paths []string
}

func NewDirCreate(args yaml.Node, ctx *expr.Context) (Action, error) {
	if args.IsZero() || args.Kind != yaml.SequenceNode {
		return nil, errors.New("args must be a list of paths")
	}

	var raw []expr.Str
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}

	paths, err := expr.ResolveAll(raw, ctx)
	if err != nil {
		return nil, err
	}

	return &DirCreate{Paths: paths}, nil
}

func (a *DirCreate) Name() string {
	return "dir.create"
}

func (a *DirCreate) NeedsSudo() bool {
	return false
}

func (a *DirCreate) Check(_ context.Context) (model.Change, error) {
	change := model.Change{Verb: "create", Total: len(a.Paths)}

	for _, path := range a.Paths {
		info, err := os.Stat(expr.ExpandPath(path))
		switch {
		case err == nil && info.IsDir():
			continue
		case err == nil:
			return model.Change{}, fmt.Errorf("%s exists but is not a directory", path)
		case !errors.Is(err, fs.ErrNotExist):
			return model.Change{}, err
		}

		change.Items = append(change.Items, model.Item{Name: path})
	}

	if len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
	}

	return change, nil
}

func (a *DirCreate) Apply(_ context.Context) error {
	for _, path := range a.Paths {
		if err := os.MkdirAll(expr.ExpandPath(path), 0o755); err != nil {
			return err
		}
	}

	return nil
}
