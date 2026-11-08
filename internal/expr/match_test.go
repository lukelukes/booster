package expr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func yamlNode(t *testing.T, src string) yaml.Node {
	t.Helper()

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &doc))
	require.NotEmpty(t, doc.Content)

	return *doc.Content[0]
}

func decodeNode(t *testing.T, node yaml.Node) any {
	t.Helper()

	var out any
	require.NoError(t, node.Decode(&out))
	return out
}

func TestResolveMatches(t *testing.T) {
	ctx := &Context{OS: "darwin", Profile: "work", Vars: map[string]any{"count": 3}}

	tests := []struct {
		name    string
		src     string
		want    string
		wantErr string
	}{
		{
			name: "picks the case matching on",
			src: `
match:
  on: ${ os }
  cases:
    arch: [git-delta, ghostty]
    darwin:
      - packages: [delta, gh]
      - casks: [ghostty]
  default: []
`,
			want: `
- packages: [delta, gh]
- casks: [ghostty]
`,
		},
		{
			name: "falls back to default",
			src: `
match:
  on: ${ profile }
  cases:
    personal: me@example.com
  default: me@work.example.com
`,
			want: `me@work.example.com`,
		},
		{
			name: "a null default is still a default",
			src: `
match:
  on: ${ os }
  cases:
    arch: x
  default: ~
`,
			want: `~`,
		},
		{
			name: "compares non-string results as strings",
			src: `
match:
  on: ${ vars.count }
  cases:
    "3": three
    "4": four
`,
			want: `three`,
		},
		{
			name: "accepts a literal on",
			src: `
match:
  on: darwin
  cases:
    darwin: yes-literal
`,
			want: `yes-literal`,
		},
		{
			name: "resolves at any depth",
			src: `
- git
- name: tools
  items:
    match:
      on: ${ os }
      cases:
        darwin: [a, b]
`,
			want: `
- git
- name: tools
  items: [a, b]
`,
		},
		{
			name: "resolves matches nested in the selected branch",
			src: `
match:
  on: ${ os }
  cases:
    darwin:
      match:
        on: ${ profile }
        cases:
          work: [slack]
`,
			want: `[slack]`,
		},
		{
			name: "does not evaluate branches that were not selected",
			src: `
match:
  on: ${ os }
  cases:
    arch:
      match:
        on: ${ nope() }
        cases: {}
    darwin: ok
`,
			want: `ok`,
		},
		{
			name: "leaves mappings with other keys alongside match alone",
			src: `
match: literal
other: value
`,
			want: `
match: literal
other: value
`,
		},
		{
			name: "resolves a match behind an alias",
			src: `
- &pick
  match:
    on: ${ os }
    cases:
      darwin: brew
- *pick
`,
			want: `[brew, brew]`,
		},
		{
			name: "dereferences an aliased match body",
			src: `
- &body
  on: ${ os }
  cases: {darwin: brew}
- match: *body
`,
			want: `[{on: "${ os }", cases: {darwin: brew}}, brew]`,
		},
		{
			name: "dereferences an aliased on",
			src: `
- &selector ${ os }
- match:
    on: *selector
    cases: {darwin: brew}
`,
			want: `["${ os }", brew]`,
		},
		{
			name: "dereferences aliased cases",
			src: `
- &table {darwin: brew, arch: pacman}
- match:
    on: ${ os }
    cases: *table
`,
			want: `[{darwin: brew, arch: pacman}, brew]`,
		},
		{
			name: "rejects a repeated on",
			src: `
match:
  on: linux
  on: darwin
  cases: {darwin: a}
`,
			wantErr: `.match: duplicate key "on"`,
		},
		{
			name: "rejects a repeated cases",
			src: `
match:
  on: ${ os }
  cases: {darwin: a}
  cases: {darwin: b}
`,
			wantErr: `.match: duplicate key "cases"`,
		},
		{
			name: "rejects a repeated default",
			src: `
match:
  on: ${ os }
  cases: {arch: a}
  default: b
  default: c
`,
			wantErr: `.match: duplicate key "default"`,
		},
		{
			name: "errors with the path when nothing matches and there is no default",
			src: `
- packages:
    match:
      on: ${ os }
      cases:
        arch: [pacman]
`,
			wantErr: `[0].packages.match: no case for "darwin" and no default`,
		},
		{
			name:    "rejects a non-mapping match block",
			src:     `match: [a, b]`,
			wantErr: ".match: must be a mapping with 'on', 'cases' and optional 'default'",
		},
		{
			name: "rejects unknown keys",
			src: `
match:
  on: ${ os }
  cases: {darwin: a}
  otherwise: b
`,
			wantErr: `.match: unknown key "otherwise"`,
		},
		{
			name: "requires on",
			src: `
match:
  cases: {darwin: a}
`,
			wantErr: ".match: missing 'on'",
		},
		{
			name: "requires on to be a scalar",
			src: `
match:
  on: [darwin]
  cases: {darwin: a}
`,
			wantErr: ".match.on: expected a scalar value, got a sequence",
		},
		{
			name: "reports a failing on expression",
			src: `
match:
  on: ${ nope() }
  cases: {darwin: a}
`,
			wantErr: ".match.on: ",
		},
		{
			name: "requires cases",
			src: `
match:
  on: ${ os }
  default: a
`,
			wantErr: ".match: missing 'cases'",
		},
		{
			name: "requires cases to be a mapping",
			src: `
match:
  on: ${ os }
  cases: [darwin]
`,
			wantErr: ".match.cases: must be a mapping of case values to branches",
		},
		{
			name: "rejects duplicate case keys",
			src: `
match:
  on: ${ os }
  cases:
    darwin: a
    darwin: b
`,
			wantErr: `.match.cases: duplicate case "darwin"`,
		},
		{
			name: "reports the path of errors inside the selected branch",
			src: `
match:
  on: ${ os }
  cases:
    darwin:
      match:
        on: ${ profile }
        cases: {personal: a}
`,
			wantErr: `.match.cases.darwin.match: no case for "work" and no default`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveMatches(yamlNode(t, tt.src), ctx)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, decodeNode(t, yamlNode(t, tt.want)), decodeNode(t, got))
		})
	}
}

func TestResolveMatchesRejectsCycles(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantPath string
	}{
		{
			name:     "a sequence that contains itself",
			src:      `&loop [a, *loop]`,
			wantPath: "[1][1]",
		},
		{
			name: "a mapping that contains itself",
			src: `
outer: &loop
  inner: *loop
`,
			wantPath: ".outer.inner",
		},
		{
			name: "a selected branch that contains its match",
			src: `
&loop
match:
  on: ${ os }
  cases:
    darwin: [*loop]
`,
			wantPath: ".match.cases.darwin[0].match",
		},
		{
			name: "a selected branch that matches on its own body",
			src: `
match: &body
  on: ${ os }
  cases:
    darwin:
      match: *body
`,
			wantPath: ".match.cases.darwin.match",
		},
		{
			name: "a selected branch that matches on its own cases",
			src: `
match:
  on: ${ os }
  cases: &cases
    darwin:
      match:
        on: ${ os }
        cases: *cases
`,
			wantPath: ".match.cases.darwin.match.cases",
		},
		{
			name: "a default that matches on its own body",
			src: `
match: &body
  on: ${ os }
  cases: {arch: a}
  default:
    match: *body
`,
			wantPath: ".match.default.match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveMatches(yamlNode(t, tt.src), &Context{OS: "darwin"})

			pathErr, ok := errors.AsType[*PathError](err)
			require.True(t, ok, "want a *PathError, got %v", err)
			assert.Equal(t, tt.wantPath, pathErr.Path)
			assert.EqualError(t, pathErr.Err, "alias refers to itself")
		})
	}
}

const expandingAliases = `
- &a [x, x, x, x, x, x, x, x, x, x]
- &b [*a, *a, *a, *a, *a, *a, *a, *a, *a, *a]
- &c [*b, *b, *b, *b, *b, *b, *b, *b, *b, *b]
- &d [*c, *c, *c, *c, *c, *c, *c, *c, *c, *c]
- &e [*d, *d, *d, *d, *d, *d, *d, *d, *d, *d]
`

func TestResolveMatchesBoundsAliasExpansion(t *testing.T) {
	t.Run("a shared graph within the budget resolves", func(t *testing.T) {
		got, err := ResolveMatches(yamlNode(t, `
- &a [x, x, x, x, x, x, x, x, x, x]
- &b [*a, *a, *a, *a, *a, *a, *a, *a, *a, *a]
- &c [*b, *b, *b, *b, *b, *b, *b, *b, *b, *b]
`), &Context{OS: "darwin"})

		require.NoError(t, err)
		require.Len(t, got.Content, 3)
		assert.Len(t, got.Content[2].Content, 10)
		assert.Len(t, got.Content[2].Content[9].Content, 10)
		assert.Equal(t, "x", got.Content[2].Content[9].Content[9].Content[9].Value)
	})

	t.Run("a compact graph that expands past the budget is rejected", func(t *testing.T) {
		_, err := ResolveMatches(yamlNode(t, expandingAliases), &Context{OS: "darwin"})

		pathErr, ok := errors.AsType[*PathError](err)
		require.True(t, ok, "want a *PathError, got %v", err)
		assert.Equal(t, "[3][7][9][0][1]", pathErr.Path)
		assert.EqualError(t, pathErr.Err, "too many alias expansions")
	})
}

func TestResolveMatchesEvaluatesOnOnce(t *testing.T) {
	calls := 0
	ctx := &Context{Vars: map[string]any{"pick": func() string {
		calls++
		return "c"
	}}}

	_, err := ResolveMatches(yamlNode(t, `
match:
  on: ${ vars.pick() }
  cases: {a: 1, b: 2, c: 3}
`), ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestResolveMatchesLeavesInputUntouched(t *testing.T) {
	src := `
- match:
    on: ${ os }
    cases: {darwin: brew}
`
	input := yamlNode(t, src)

	_, err := ResolveMatches(input, &Context{OS: "darwin"})

	require.NoError(t, err)
	assert.Equal(t, decodeNode(t, yamlNode(t, src)), decodeNode(t, input))
}

func TestResolveMatchesReportsThePathSeparately(t *testing.T) {
	_, err := ResolveMatches(yamlNode(t, `
- packages:
    match:
      on: ${ os }
      cases: {arch: [pacman]}
`), &Context{OS: "darwin"})

	pathErr, ok := errors.AsType[*PathError](err)
	require.True(t, ok)
	assert.Equal(t, "[0].packages.match", pathErr.Path)
	assert.EqualError(t, pathErr.Err, `no case for "darwin" and no default`)
}
