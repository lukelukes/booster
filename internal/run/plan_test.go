package run

import (
	"booster/internal/model"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAction struct {
	change   model.Change
	err      error
	applyErr error
	checked  int
	applied  int
}

func (f *fakeAction) Name() string { return "fake" }

func (f *fakeAction) Check(context.Context) (model.Change, error) {
	f.checked++
	return f.change, f.err
}

func (f *fakeAction) Apply(context.Context) error {
	f.applied++
	return f.applyErr
}

func TestPlan_ChecksEveryStepInOrder(t *testing.T) {
	create := model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1, Items: []model.Item{{Name: "/a"}}}
	none := model.Change{Verb: "link", Total: 2}
	update := model.Change{Kind: model.ChangeUpdate, Verb: "link", Total: 1, Items: []model.Item{{Name: "/b", From: "/x", To: "/y"}}}

	actions := []*fakeAction{{change: create}, {change: none}, {change: update}}
	compiled := Compiled{
		Steps: []Step{
			{Name: "dirs", Action: actions[0]},
			{Name: "links", Action: actions[1]},
			{Name: "more links", Action: actions[2]},
		},
		Excluded: 3,
	}

	plan, err := Plan(context.Background(), compiled)

	require.NoError(t, err)
	assert.Equal(t, model.Plan{
		Steps: []model.PlannedStep{
			{Name: "dirs", Change: create},
			{Name: "links", Change: none},
			{Name: "more links", Change: update},
		},
		Excluded: 3,
	}, plan.View())
	for _, a := range actions {
		assert.Equal(t, 1, a.checked)
		assert.Zero(t, a.applied)
	}
}

func TestPlan_KeepsActionsForApply(t *testing.T) {
	a := &fakeAction{}

	plan, err := Plan(context.Background(), Compiled{Steps: []Step{{Name: "dirs", Action: a}}})

	require.NoError(t, err)
	require.Len(t, plan.Steps, 1)
	assert.Same(t, a, plan.Steps[0].Action)
}

func TestPlan_FailsNamingTheStepWhenCheckFails(t *testing.T) {
	compiled := Compiled{Steps: []Step{
		{Name: "dirs", Action: &fakeAction{}},
		{Name: "links", Action: &fakeAction{err: errors.New("source does not exist: /nope")}},
	}}

	_, err := Plan(context.Background(), compiled)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"links"`)
	assert.Contains(t, err.Error(), "source does not exist: /nope")
}
