package task

import (
	"booster/internal/pathutil"
	"context"
	"errors"
	"fmt"
	"os"
)

type DirCreate struct {
	Path string
}

func (t *DirCreate) Name() string {
	return "create " + t.Path
}

func (t *DirCreate) NeedsSudo() bool {
	return false
}

func (t *DirCreate) Run(ctx context.Context) Result {
	expanded := pathutil.Expand(t.Path)

	info, err := os.Stat(expanded)
	if err == nil {
		if info.IsDir() {
			return Result{Status: StatusSkipped, Message: "already exists"}
		}
		return Result{
			Status: StatusFailed,
			Error:  errors.New("path exists but is not a directory"),
		}
	}

	if err := os.MkdirAll(expanded, 0o755); err != nil {
		return Result{Status: StatusFailed, Error: err}
	}

	return Result{Status: StatusDone, Message: "created"}
}

func NewDirCreate(args any) ([]Task, error) {
	paths, ok := args.([]any)
	if !ok {
		return nil, errors.New("args must be a list of paths")
	}

	var tasks []Task
	for i, p := range paths {
		path, ok := p.(string)
		if !ok {
			return nil, fmt.Errorf("arg %d: path must be a string", i+1)
		}
		tasks = append(tasks, &DirCreate{Path: path})
	}

	return tasks, nil
}
