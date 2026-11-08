package run

import (
	"booster/internal/cmdexec"
	"booster/internal/model"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	events []model.Event
}

func (r *recorder) emit(e model.Event) {
	switch e := e.(type) {
	case model.TaskDone:
		e.Result.Duration = 0
		r.events = append(r.events, e)
	case model.RunDone:
		e.Summary.Duration = 0
		r.events = append(r.events, e)
	default:
		r.events = append(r.events, e)
	}
}

func planOf(steps ...Planned) PlanResult {
	return PlanResult{Steps: steps}
}

func TestApply_EmitsEventsInOrderAndSkipsUpToDateSteps(t *testing.T) {
	create := model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1, Items: []model.Item{{Name: "/a"}}}
	none := model.Change{Verb: "link", Total: 2}
	unknown := model.Change{Kind: model.ChangeUnknown, Verb: "install"}
	dirs, links, tools := &fakeAction{}, &fakeAction{}, &fakeAction{}
	var rec recorder

	summary := Apply(context.Background(), planOf(
		Planned{PlannedStep: model.PlannedStep{Name: "dirs", Change: create}, Action: dirs},
		Planned{PlannedStep: model.PlannedStep{Name: "links", Change: none}, Action: links},
		Planned{PlannedStep: model.PlannedStep{Name: "tools", Change: unknown}, Action: tools},
	), rec.emit)

	assert.Equal(t, []model.Event{
		model.RunStart{Total: 3},
		model.TaskStart{Index: 0, Name: "dirs", Change: create},
		model.TaskDone{Index: 0, Result: model.Result{Status: model.StatusChanged, Change: create}},
		model.TaskStart{Index: 1, Name: "links", Change: none},
		model.TaskDone{Index: 1, Result: model.Result{Status: model.StatusOK, Change: none}},
		model.TaskStart{Index: 2, Name: "tools", Change: unknown},
		model.TaskDone{Index: 2, Result: model.Result{Status: model.StatusChanged, Change: unknown}},
		model.RunDone{Summary: model.Summary{Total: 3, Changed: 2, OK: 1}},
	}, rec.events)
	assert.Equal(t, 1, dirs.applied)
	assert.Zero(t, links.applied)
	assert.Equal(t, 1, tools.applied)
	assert.Equal(t, model.Summary{Total: 3, Changed: 2, OK: 1}, withoutDuration(summary))
}

func TestApply_StopsAtTheFirstFailure(t *testing.T) {
	create := model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1}
	boom := errors.New("permission denied")
	first, failing, later := &fakeAction{}, &fakeAction{applyErr: boom}, &fakeAction{}
	var rec recorder

	summary := Apply(context.Background(), planOf(
		Planned{PlannedStep: model.PlannedStep{Name: "dirs", Change: create}, Action: first},
		Planned{PlannedStep: model.PlannedStep{Name: "links", Change: create}, Action: failing},
		Planned{PlannedStep: model.PlannedStep{Name: "more", Change: create}, Action: later},
	), rec.emit)

	require.Len(t, rec.events, 7)
	assert.Equal(t, model.TaskStart{Index: 1, Name: "links", Change: create}, rec.events[3])
	assert.Equal(t, model.TaskDone{Index: 1, Result: model.Result{Status: model.StatusFailed, Change: create, Err: boom}}, rec.events[4])
	assert.Equal(t, model.TaskDone{Index: 2, Result: model.Result{Status: model.StatusSkipped, Change: create, Reason: "not run"}}, rec.events[5])
	assert.Equal(t, model.RunDone{Summary: model.Summary{Total: 3, Changed: 1, Failed: 1, Skipped: 1}}, rec.events[6])
	assert.Zero(t, later.applied)
	assert.Equal(t, 1, withoutDuration(summary).Failed)
}

func TestApply_EmptyPlan(t *testing.T) {
	var rec recorder

	Apply(context.Background(), PlanResult{}, rec.emit)

	assert.Equal(t, []model.Event{model.RunStart{}, model.RunDone{}}, rec.events)
}

func withoutDuration(s model.Summary) model.Summary {
	s.Duration = 0
	return s
}

type printingAction struct {
	fakeAction
	script string
}

func (p *printingAction) Apply(ctx context.Context) error {
	p.applied++
	_, err := cmdexec.DefaultRunner().Run(ctx, "sh", "-c", p.script)
	return err
}

func TestApply_StreamsCommandOutputBetweenStartAndDone(t *testing.T) {
	create := model.Change{Kind: model.ChangeCreate, Verb: "install", Total: 1}
	var rec recorder

	Apply(context.Background(), planOf(
		Planned{PlannedStep: model.PlannedStep{Name: "tools", Change: create}, Action: &printingAction{script: "echo resolving; echo 'warning: x' >&2; echo done"}},
		Planned{PlannedStep: model.PlannedStep{Name: "apps", Change: create}, Action: &printingAction{script: "echo fetched; exit 1"}},
	), rec.emit)

	require.Len(t, rec.events, 10)
	assert.Equal(t, []model.Event{
		model.TaskStart{Index: 0, Name: "tools", Change: create},
		model.TaskOutput{Index: 0, Line: "resolving"},
		model.TaskOutput{Index: 0, Line: "warning: x"},
		model.TaskOutput{Index: 0, Line: "done"},
		model.TaskDone{Index: 0, Result: model.Result{Status: model.StatusChanged, Change: create}},
		model.TaskStart{Index: 1, Name: "apps", Change: create},
		model.TaskOutput{Index: 1, Line: "fetched"},
	}, rec.events[1:8])
	done, ok := rec.events[8].(model.TaskDone)
	require.True(t, ok)
	assert.Equal(t, model.StatusFailed, done.Result.Status)
}
