package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlan_HasChanges(t *testing.T) {
	tests := []struct {
		name  string
		kinds []ChangeKind
		want  bool
	}{
		{name: "empty plan", want: false},
		{name: "everything up to date", kinds: []ChangeKind{ChangeNone, ChangeNone}, want: false},
		{name: "one create", kinds: []ChangeKind{ChangeNone, ChangeCreate}, want: true},
		{name: "one update", kinds: []ChangeKind{ChangeUpdate}, want: true},
		{name: "unknown counts as change", kinds: []ChangeKind{ChangeUnknown}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p Plan
			for _, k := range tt.kinds {
				p.Steps = append(p.Steps, PlannedStep{Change: Change{Kind: k}})
			}

			assert.Equal(t, tt.want, p.HasChanges())
		})
	}
}

func TestPlan_NeedsSudo(t *testing.T) {
	tests := []struct {
		name  string
		steps []PlannedStep
		want  bool
	}{
		{name: "empty plan", want: false},
		{name: "changes without sudo", steps: []PlannedStep{{Change: Change{Kind: ChangeCreate}}}, want: false},
		{name: "a change that needs sudo", steps: []PlannedStep{
			{Change: Change{Kind: ChangeCreate}},
			{Change: Change{Kind: ChangeUpdate, NeedsSudo: true}},
		}, want: true},
		{name: "an up-to-date step that would need sudo", steps: []PlannedStep{{Change: Change{NeedsSudo: true}}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Plan{Steps: tt.steps}.NeedsSudo())
		})
	}
}
