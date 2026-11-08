package ui

import (
	"booster/internal/model"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgress(t *testing.T) {
	create := model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 2, Items: []model.Item{{Name: "/a"}}}
	events := []model.Event{
		model.RunStart{Total: 4},
		model.TaskStart{Index: 0, Name: "dirs", Change: create},
		model.TaskDone{Index: 0, Result: model.Result{Status: model.StatusChanged, Change: create}},
		model.TaskStart{Index: 1, Name: "links", Change: model.Change{Verb: "link", Total: 1}},
		model.TaskDone{Index: 1, Result: model.Result{Status: model.StatusOK}},
		model.TaskStart{Index: 2, Name: "more dirs", Change: create},
		model.TaskDone{Index: 2, Result: model.Result{Status: model.StatusFailed, Err: errors.New("permission denied")}},
		model.TaskDone{Index: 3, Result: model.Result{Status: model.StatusSkipped, Reason: "not run"}},
		model.RunDone{Summary: model.Summary{Total: 4, Changed: 1, OK: 1, Failed: 1, Skipped: 1}},
	}
	plan := model.Plan{Steps: []model.PlannedStep{
		{Name: "dirs", Change: create},
		{Name: "links"},
		{Name: "more dirs", Change: create},
		{Name: "later", Change: create},
	}}
	var sb strings.Builder
	emit := Progress(&sb, plan)

	for _, e := range events {
		emit(e)
	}

	assert.Equal(t, `
applying 4 steps
→ dirs
+ dirs  changed
✓ links  up to date
→ more dirs
✗ more dirs  failed: permission denied
- later  not run

1 changed · 1 up to date · 1 failed · 1 not run
`, sb.String())
}
