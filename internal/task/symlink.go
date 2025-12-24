package task

import (
	"booster/internal/pathutil"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type SymlinkCreate struct {
	Source string
	Target string
}

func (t *SymlinkCreate) Name() string {
	return fmt.Sprintf("link %s → %s", t.Source, t.Target)
}

func (t *SymlinkCreate) NeedsSudo() bool {
	return false
}

func (t *SymlinkCreate) Run(ctx context.Context) Result {
	source := pathutil.Expand(t.Source)
	target := pathutil.Expand(t.Target)

	if !filepath.IsAbs(source) {
		var err error
		source, err = filepath.Abs(source)
		if err != nil {
			return Result{Status: StatusFailed, Error: fmt.Errorf("failed to resolve source path: %w", err)}
		}
	}

	if _, err := os.Stat(source); err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("source does not exist: %s", source)}
	}

	info, err := os.Lstat(target)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			linkDest, err := os.Readlink(target)
			if err != nil {
				return Result{Status: StatusFailed, Error: err}
			}
			if linkDest == source {
				return Result{Status: StatusSkipped, Message: "already exists"}
			}
			return Result{Status: StatusFailed, Error: fmt.Errorf("symlink points to different source: %s", linkDest)}
		}

		return Result{Status: StatusFailed, Error: fmt.Errorf("target exists but is not a symlink: %s", target)}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{Status: StatusFailed, Error: err}
	}

	if err := os.Symlink(source, target); err != nil {
		return Result{Status: StatusFailed, Error: err}
	}

	return Result{Status: StatusDone, Message: "created"}
}

func NewSymlinkCreate(args any) ([]Task, error) {
	pairs, err := parseSourceTargetArgs(args)
	if err != nil {
		return nil, err
	}

	tasks := make([]Task, 0, len(pairs))
	for _, pair := range pairs {
		tasks = append(tasks, &SymlinkCreate{Source: pair.Source, Target: pair.Target})
	}

	return tasks, nil
}
