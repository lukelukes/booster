package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type strArgs struct {
	Path     Str   `yaml:"path"`
	Packages []Str `yaml:"packages"`
}

func testContext() *Context {
	ctx := NewContext()
	ctx.OS = "arch"
	ctx.Home = "/home/tester"
	ctx.Vars = map[string]any{"editor": "nvim"}
	return ctx
}

func TestStr_UnmarshalYAML_RetainsRawForm(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want Str
	}{
		{
			name: "expression is not evaluated at decode time",
			yaml: "path: ${ home }/config",
			want: "${ home }/config",
		},
		{
			name: "plain string",
			yaml: "path: /etc/hosts",
			want: "/etc/hosts",
		},
		{
			name: "quoted expression",
			yaml: `path: "${ vars.editor }"`,
			want: "${ vars.editor }",
		},
		{
			name: "number keeps its source text",
			yaml: "path: 3",
			want: "3",
		},
		{
			name: "bool keeps its source text",
			yaml: "path: true",
			want: "true",
		},
		{
			name: "absent field is empty",
			yaml: "packages: []",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args strArgs
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &args))

			assert.Equal(t, tt.want, args.Path)
		})
	}
}

func TestStr_UnmarshalYAML_RejectsNonScalar(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "mapping",
			yaml: "path:\n  os: arch\n",
			want: "got a mapping",
		},
		{
			name: "sequence",
			yaml: "path:\n  - a\n  - b\n",
			want: "got a sequence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args strArgs
			err := yaml.Unmarshal([]byte(tt.yaml), &args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestStr_Resolve(t *testing.T) {
	tests := []struct {
		name string
		str  Str
		want string
	}{
		{
			name: "literal passes through",
			str:  "/etc/hosts",
			want: "/etc/hosts",
		},
		{
			name: "empty string",
			str:  "",
			want: "",
		},
		{
			name: "interpolation",
			str:  "${ home }/.config",
			want: "/home/tester/.config",
		},
		{
			name: "interpolation among literals",
			str:  "prefix-${ os }-suffix",
			want: "prefix-arch-suffix",
		},
		{
			name: "variable namespace",
			str:  "${ vars.editor }",
			want: "nvim",
		},
		{
			name: "non string result is stringified",
			str:  "${ 1 + 1 }",
			want: "2",
		},
		{
			name: "bool result is stringified",
			str:  `${ os == "arch" }`,
			want: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.str.Resolve(testContext())

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStr_Resolve_Error(t *testing.T) {
	tests := []struct {
		name string
		str  Str
		want string
	}{
		{
			name: "malformed expression",
			str:  "${ 1 + }",
			want: "invalid expression",
		},
		{
			name: "unknown field",
			str:  "${ nope }",
			want: "nope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.str.Resolve(testContext())

			require.Error(t, err)
			assert.Empty(t, got)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestResolveAll(t *testing.T) {
	var args strArgs
	require.NoError(t, yaml.Unmarshal([]byte("packages:\n  - git\n  - ${ vars.editor }\n  - ${ os }-tools\n"), &args))

	got, err := ResolveAll(args.Packages, testContext())

	require.NoError(t, err)
	assert.Equal(t, []string{"git", "nvim", "arch-tools"}, got)
}

func TestResolveAll_Empty(t *testing.T) {
	got, err := ResolveAll(nil, testContext())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestResolveAll_ErrorCarriesIndex(t *testing.T) {
	got, err := ResolveAll([]Str{"git", "${ 1 + }"}, testContext())

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "[1]")
}
