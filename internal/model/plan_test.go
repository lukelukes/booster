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
