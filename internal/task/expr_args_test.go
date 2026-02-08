package task

import (
	"booster/internal/expr"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTaskArgs_TableDriven(t *testing.T) {
	ctx := expr.NewContext()
	ctx.Home = "/tmp/home"

	tests := []struct {
		name      string
		args      any
		ctx       *expr.Context
		want      any
		wantErr   string
		errorPath string
	}{
		{
			name: "literal string without context",
			args: "plain",
			ctx:  nil,
			want: "plain",
		},
		{
			name: "full expression resolves to non string",
			args: "${ 1 + 1 }",
			ctx:  ctx,
			want: 2,
		},
		{
			name: "interpolation resolves to string",
			args: "out-${ 1 + 1 }",
			ctx:  ctx,
			want: "out-2",
		},
		{
			name: "nested map and list shape preserved",
			args: map[string]any{
				"items": []any{
					map[string]any{"enabled": true, "name": "${ home }/a"},
					map[string]any{"enabled": false, "name": "literal"},
				},
				"count": 3,
			},
			ctx: ctx,
			want: map[string]any{
				"items": []any{
					map[string]any{"enabled": true, "name": "/tmp/home/a"},
					map[string]any{"enabled": false, "name": "literal"},
				},
				"count": 3,
			},
		},
		{
			name:      "expression without context fails fast",
			args:      []any{"${ home }/app"},
			ctx:       nil,
			wantErr:   "expression context is required",
			errorPath: "[0]",
		},
		{
			name: "invalid expression includes stable path",
			args: map[string]any{
				"items": []any{
					map[string]any{"name": "ok"},
					map[string]any{"name": "${ 1 + }"},
				},
			},
			ctx:       ctx,
			wantErr:   "invalid expression",
			errorPath: ".items[1].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTaskArgs(tt.args, tt.ctx)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				var argErr *argResolveError
				require.ErrorAs(t, err, &argErr)
				assert.Equal(t, tt.errorPath, argErr.path)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatPathKey_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "identifier", key: "items", want: ".items"},
		{name: "starts with number", key: "1name", want: "[\"1name\"]"},
		{name: "contains dash", key: "file-name", want: "[\"file-name\"]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatPathKey(tt.key))
		})
	}
}
