package run

import (
	"booster/internal/model"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanResult_Without_SkippedStepsAreNotApplied(t *testing.T) {
	create := model.Change{Kind: model.ChangeCreate, Verb: "create", Total: 1, Items: []model.Item{{Name: "/a"}}}
	actions := []*fakeAction{{change: create}, {change: create}, {change: create}}
	plan, err := Plan(context.Background(), Compiled{
		Steps: []Step{
			{Name: "dirs", Action: actions[0]},
			{Name: "links", Action: actions[1]},
			{Name: "packages", Action: actions[2]},
		},
		Excluded: 1,
	})
	require.NoError(t, err)

	kept := plan.Without([]int{0, 2})
	summary := Apply(context.Background(), kept, func(model.Event) {})

	assert.Equal(t, model.Plan{Steps: []model.PlannedStep{{Name: "links", Change: create}}, Excluded: 1}, kept.View())
	assert.Equal(t, 1, summary.Changed)
	assert.Equal(t, []int{0, 1, 0}, []int{actions[0].applied, actions[1].applied, actions[2].applied})
	assert.Len(t, plan.Steps, 3)
}

func TestPlanResult_Without_NothingSkipped(t *testing.T) {
	plan := PlanResult{Steps: []Planned{{PlannedStep: model.PlannedStep{Name: "dirs"}}}}

	assert.Equal(t, plan, plan.Without(nil))
}
