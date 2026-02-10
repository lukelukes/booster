package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type DeferredFactoryTask struct {
	name      string
	factory   func() ([]Task, error)
	loadOnce  sync.Once
	loaded    []Task
	loadError error
}

func NewDeferredFactoryTask(name string, factory func() ([]Task, error)) *DeferredFactoryTask {
	return &DeferredFactoryTask{name: name, factory: factory}
}

func (t *DeferredFactoryTask) Name() string {
	if t.name != "" {
		return t.name
	}
	return "task"
}

func (t *DeferredFactoryTask) NeedsSudo() bool {
	loaded, err := t.load()
	if err != nil {
		return false
	}
	return AnyNeedsSudo(loaded)
}

func (t *DeferredFactoryTask) Run(ctx context.Context) Result {
	loaded, err := t.load()
	if err != nil {
		return Result{Status: StatusFailed, Error: err, Message: err.Error()}
	}

	if len(loaded) == 0 {
		return Result{Status: StatusDone, Message: "no tasks generated"}
	}

	var messages []string
	anyDone := false
	allSkipped := len(loaded) > 0
	for _, inner := range loaded {
		result := inner.Run(ctx)
		if result.Status == StatusFailed {
			return result
		}
		if result.Status == StatusDone {
			anyDone = true
		}
		if result.Status != StatusSkipped {
			allSkipped = false
		}
		if result.Message != "" {
			messages = append(messages, result.Message)
		}
	}

	finalStatus := StatusDone
	if !anyDone && allSkipped {
		finalStatus = StatusSkipped
	}

	if len(messages) == 0 {
		return Result{Status: finalStatus}
	}

	return Result{Status: finalStatus, Message: strings.Join(messages, "; ")}
}

func (t *DeferredFactoryTask) load() ([]Task, error) {
	t.loadOnce.Do(func() {
		t.loaded, t.loadError = t.factory()
		if t.loadError != nil {
			t.loadError = fmt.Errorf("prepare task %q: %w", t.Name(), t.loadError)
		}
	})

	if t.loadError != nil {
		return nil, t.loadError
	}

	return t.loaded, nil
}
