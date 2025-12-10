package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourceTargetArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    any
		want    []SourceTarget
		wantErr string
	}{
		{
			name: "valid single pair",
			args: []any{
				map[string]any{"source": "src", "target": "dst"},
			},
			want: []SourceTarget{
				{Source: "src", Target: "dst"},
			},
		},
		{
			name: "valid multiple pairs",
			args: []any{
				map[string]any{"source": "a", "target": "b"},
				map[string]any{"source": "c", "target": "d"},
				map[string]any{"source": "e", "target": "f"},
			},
			want: []SourceTarget{
				{Source: "a", Target: "b"},
				{Source: "c", Target: "d"},
				{Source: "e", Target: "f"},
			},
		},
		{
			name: "empty list",
			args: []any{},
			want: []SourceTarget{},
		},
		{
			name:    "not a list",
			args:    "not a list",
			wantErr: "must be a list",
		},
		{
			name:    "not a map",
			args:    []any{"not a map"},
			wantErr: "arg 1: must be a map",
		},
		{
			name: "missing source",
			args: []any{
				map[string]any{"target": "dst"},
			},
			wantErr: "arg 1: missing 'source'",
		},
		{
			name: "missing target",
			args: []any{
				map[string]any{"source": "src"},
			},
			wantErr: "arg 1: missing 'target'",
		},
		{
			name: "source not string",
			args: []any{
				map[string]any{"source": 123, "target": "dst"},
			},
			wantErr: "arg 1: 'source' must be a string",
		},
		{
			name: "target not string",
			args: []any{
				map[string]any{"source": "src", "target": 456},
			},
			wantErr: "arg 1: 'target' must be a string",
		},
		{
			name: "error on second item",
			args: []any{
				map[string]any{"source": "a", "target": "b"},
				map[string]any{"source": "c"}, // missing target
			},
			wantErr: "arg 2: missing 'target'",
		},
		{
			name: "error on third item",
			args: []any{
				map[string]any{"source": "a", "target": "b"},
				map[string]any{"source": "c", "target": "d"},
				map[string]any{"source": 123, "target": "e"}, // invalid source
			},
			wantErr: "arg 3: 'source' must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSourceTargetArgs(tt.args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseSourceTargetArgs_ErrorIndices(t *testing.T) {
	// Ensures error messages use 1-indexed positions
	tests := []struct {
		name          string
		args          []any
		expectedIndex string
	}{
		{
			name: "first arg error shows arg 1",
			args: []any{
				"not a map",
			},
			expectedIndex: "arg 1:",
		},
		{
			name: "second arg error shows arg 2",
			args: []any{
				map[string]any{"source": "a", "target": "b"},
				map[string]any{"target": "c"}, // missing source
			},
			expectedIndex: "arg 2:",
		},
		{
			name: "third arg error shows arg 3",
			args: []any{
				map[string]any{"source": "a", "target": "b"},
				map[string]any{"source": "c", "target": "d"},
				map[string]any{"source": "e"}, // missing target
			},
			expectedIndex: "arg 3:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSourceTargetArgs(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedIndex,
				"error message must show correct 1-indexed position")
		})
	}
}
