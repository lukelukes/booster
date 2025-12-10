package condition

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluator_Matches(t *testing.T) {
	tests := []struct {
		cond *Condition
		name string
		ctx  Context
		want bool
	}{
		{
			name: "nil condition always matches",
			ctx:  Context{OS: "arch"},
			cond: nil,
			want: true,
		},
		{
			name: "empty condition always matches",
			ctx:  Context{OS: "arch"},
			cond: &Condition{},
			want: true,
		},
		{
			name: "empty OS slice matches",
			ctx:  Context{OS: "arch"},
			cond: &Condition{OS: []string{}},
			want: true,
		},
		{
			name: "single OS match",
			ctx:  Context{OS: "arch"},
			cond: &Condition{OS: []string{"arch"}},
			want: true,
		},
		{
			name: "single OS no match",
			ctx:  Context{OS: "darwin"},
			cond: &Condition{OS: []string{"arch"}},
			want: false,
		},
		{
			name: "multiple OS match first",
			ctx:  Context{OS: "arch"},
			cond: &Condition{OS: []string{"arch", "darwin"}},
			want: true,
		},
		{
			name: "multiple OS match second",
			ctx:  Context{OS: "darwin"},
			cond: &Condition{OS: []string{"arch", "darwin"}},
			want: true,
		},
		{
			name: "multiple OS no match",
			ctx:  Context{OS: "ubuntu"},
			cond: &Condition{OS: []string{"arch", "darwin"}},
			want: false,
		},
		{
			name: "darwin matches darwin",
			ctx:  Context{OS: "darwin"},
			cond: &Condition{OS: []string{"darwin"}},
			want: true,
		},
		{
			name: "ubuntu matches ubuntu",
			ctx:  Context{OS: "ubuntu"},
			cond: &Condition{OS: []string{"ubuntu"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewEvaluator(tt.ctx)
			got := eval.Matches(tt.cond)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_Matches_Profile(t *testing.T) {
	tests := []struct {
		cond *Condition
		name string
		ctx  Context
		want bool
	}{
		{
			name: "profile matches single value",
			ctx:  Context{OS: "arch", Profile: "personal"},
			cond: &Condition{Profile: []string{"personal"}},
			want: true,
		},
		{
			name: "profile matches one of multiple",
			ctx:  Context{OS: "arch", Profile: "work"},
			cond: &Condition{Profile: []string{"personal", "work"}},
			want: true,
		},
		{
			name: "profile not in list fails",
			ctx:  Context{OS: "arch", Profile: "personal"},
			cond: &Condition{Profile: []string{"work"}},
			want: false,
		},
		{
			name: "empty profile condition always matches",
			ctx:  Context{OS: "arch", Profile: "personal"},
			cond: &Condition{Profile: []string{}},
			want: true,
		},
		{
			name: "nil profile condition always matches",
			ctx:  Context{OS: "arch", Profile: "personal"},
			cond: &Condition{},
			want: true,
		},
		{
			name: "empty context profile with profile condition fails",
			ctx:  Context{OS: "arch", Profile: ""},
			cond: &Condition{Profile: []string{"personal"}},
			want: false,
		},
		{
			name: "both OS and profile must match - both pass",
			ctx:  Context{OS: "arch", Profile: "personal"},
			cond: &Condition{OS: []string{"arch"}, Profile: []string{"personal"}},
			want: true,
		},
		{
			name: "both OS and profile must match - OS fails",
			ctx:  Context{OS: "darwin", Profile: "personal"},
			cond: &Condition{OS: []string{"arch"}, Profile: []string{"personal"}},
			want: false,
		},
		{
			name: "both OS and profile must match - profile fails",
			ctx:  Context{OS: "arch", Profile: "work"},
			cond: &Condition{OS: []string{"arch"}, Profile: []string{"personal"}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewEvaluator(tt.ctx)
			got := eval.Matches(tt.cond)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_FailureReason(t *testing.T) {
	tests := []struct {
		name    string
		ctx     Context
		cond    *Condition
		wantMsg string
	}{
		{
			name:    "nil condition returns empty",
			ctx:     Context{OS: "arch"},
			cond:    nil,
			wantMsg: "",
		},
		{
			name:    "matching condition returns empty",
			ctx:     Context{OS: "arch"},
			cond:    &Condition{OS: []string{"arch"}},
			wantMsg: "",
		},
		{
			name:    "single OS mismatch",
			ctx:     Context{OS: "darwin"},
			cond:    &Condition{OS: []string{"arch"}},
			wantMsg: "os=darwin, want arch",
		},
		{
			name:    "multiple OS mismatch",
			ctx:     Context{OS: "ubuntu"},
			cond:    &Condition{OS: []string{"arch", "darwin"}},
			wantMsg: "os=ubuntu, want arch or darwin",
		},
		{
			name:    "single profile mismatch",
			ctx:     Context{OS: "arch", Profile: "work"},
			cond:    &Condition{Profile: []string{"personal"}},
			wantMsg: "profile=work, want personal",
		},
		{
			name:    "multiple profile mismatch",
			ctx:     Context{OS: "arch", Profile: "gaming"},
			cond:    &Condition{Profile: []string{"personal", "work"}},
			wantMsg: "profile=gaming, want personal or work",
		},
		{
			name:    "OS fails before profile is checked",
			ctx:     Context{OS: "darwin", Profile: "work"},
			cond:    &Condition{OS: []string{"arch"}, Profile: []string{"personal"}},
			wantMsg: "os=darwin, want arch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := NewEvaluator(tt.ctx)
			got := eval.FailureReason(tt.cond)
			assert.Equal(t, tt.wantMsg, got)
		})
	}
}
