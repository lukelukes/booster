package task

import (
	"booster/internal/pathutil"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// SymlinkCreate creates a symbolic link from Source to Target.
type SymlinkCreate struct {
	Source string
	Target string
}

func (t *SymlinkCreate) Name() string {
	return fmt.Sprintf("link %s → %s", t.Source, t.Target)
}

// NeedsSudo returns false - symlink creation in user space doesn't need sudo.
func (t *SymlinkCreate) NeedsSudo() bool {
	return false
}

func (t *SymlinkCreate) Run(ctx context.Context) Result {
	source := pathutil.Expand(t.Source)
	target := pathutil.Expand(t.Target)

	// Convert source to absolute path - relative symlinks resolve from the link location, not CWD
	if !filepath.IsAbs(source) {
		var err error
		source, err = filepath.Abs(source)
		if err != nil {
			return Result{Status: StatusFailed, Error: fmt.Errorf("failed to resolve source path: %w", err)}
		}
	}

	// 1. Check if source exists
	if _, err := os.Stat(source); err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("source does not exist: %s", source)}
	}

	// 2. Check if target already exists (Lstat doesn't follow symlinks)
	info, err := os.Lstat(target)
	if err == nil {
		// Target exists - check what it is
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - check where it points
			linkDest, err := os.Readlink(target)
			if err != nil {
				return Result{Status: StatusFailed, Error: err}
			}
			if linkDest == source {
				return Result{Status: StatusSkipped, Message: "already exists"}
			}
			return Result{Status: StatusFailed, Error: fmt.Errorf("symlink points to different source: %s", linkDest)}
		}
		// Not a symlink - it's a file or directory
		return Result{Status: StatusFailed, Error: fmt.Errorf("target exists but is not a symlink: %s", target)}
	}

	// 3. Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{Status: StatusFailed, Error: err}
	}

	// 4. Create the symlink
	if err := os.Symlink(source, target); err != nil {
		return Result{Status: StatusFailed, Error: err}
	}

	return Result{Status: StatusDone, Message: "created"}
}

// NewSymlinkCreate parses args and creates SymlinkCreate tasks.
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
