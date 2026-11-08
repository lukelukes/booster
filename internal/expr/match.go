package expr

import (
	"fmt"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"
)

type PathError struct {
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return e.Path + ": " + e.Err.Error()
}

func (e *PathError) Unwrap() error {
	return e.Err
}

func pathErrorf(path, format string, args ...any) error {
	return &PathError{Path: path, Err: fmt.Errorf(format, args...)}
}

const maxAliasExpansions = 10000

func ResolveMatches(node yaml.Node, ctx *Context) (yaml.Node, error) {
	m := &matcher{ctx: ctx, active: map[*yaml.Node]bool{}}
	resolved, err := m.resolve(&node, "")
	if err != nil {
		return yaml.Node{}, err
	}
	return *resolved, nil
}

type matcher struct {
	ctx        *Context
	active     map[*yaml.Node]bool
	aliasDepth int
	expansions int
}

func isContainer(node *yaml.Node) bool {
	return node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode || node.Kind == yaml.DocumentNode
}

func (m *matcher) enter(node *yaml.Node, path string) (*yaml.Node, func(), error) {
	aliased := node.Kind == yaml.AliasNode
	node = deref(node)
	if aliased || m.aliasDepth > 0 {
		m.expansions++
		if m.expansions > maxAliasExpansions {
			return nil, nil, pathErrorf(path, "too many alias expansions")
		}
	}
	if !isContainer(node) {
		return node, func() {}, nil
	}
	if m.active[node] {
		return nil, nil, pathErrorf(path, "alias refers to itself")
	}
	m.active[node] = true
	if aliased {
		m.aliasDepth++
	}
	return node, func() {
		delete(m.active, node)
		if aliased {
			m.aliasDepth--
		}
	}, nil
}

func (m *matcher) resolve(node *yaml.Node, path string) (*yaml.Node, error) {
	node, leave, err := m.enter(node, path)
	if err != nil {
		return nil, err
	}
	defer leave()
	if !isContainer(node) {
		return node, nil
	}

	if node.Kind == yaml.MappingNode && len(node.Content) == 2 && node.Content[0].Value == "match" {
		return m.resolveMatch(node.Content[1], path+".match")
	}

	out := copyNode(node)
	if node.Kind == yaml.MappingNode {
		for i := 1; i < len(out.Content); i += 2 {
			resolved, err := m.resolve(out.Content[i], path+pathKey(out.Content[i-1].Value))
			if err != nil {
				return nil, err
			}
			out.Content[i] = resolved
		}
		return out, nil
	}

	for i, child := range out.Content {
		resolved, err := m.resolve(child, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		out.Content[i] = resolved
	}
	return out, nil
}

func copyNode(node *yaml.Node) *yaml.Node {
	out := *node
	out.Content = slices.Clone(node.Content)
	return &out
}

func deref(node *yaml.Node) *yaml.Node {
	for node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

func (m *matcher) resolveMatch(block *yaml.Node, path string) (*yaml.Node, error) {
	block, leave, err := m.enter(block, path)
	if err != nil {
		return nil, err
	}
	defer leave()
	if block.Kind != yaml.MappingNode {
		return nil, pathErrorf(path, "must be a mapping with 'on', 'cases' and optional 'default'")
	}

	fields := map[string]*yaml.Node{}
	for i := 0; i < len(block.Content); i += 2 {
		key := block.Content[i].Value
		switch key {
		case "on", "cases", "default":
		default:
			return nil, pathErrorf(path, "unknown key %q", key)
		}
		if fields[key] != nil {
			return nil, pathErrorf(path, "duplicate key %q", key)
		}
		fields[key] = block.Content[i+1]
	}
	on, cases, fallback := fields["on"], fields["cases"], fields["default"]
	if on == nil {
		return nil, pathErrorf(path, "missing 'on'")
	}
	if cases == nil {
		return nil, pathErrorf(path, "missing 'cases'")
	}

	var raw Str
	if err := raw.UnmarshalYAML(deref(on)); err != nil {
		return nil, &PathError{Path: path + ".on", Err: err}
	}
	if m.ctx == nil {
		return nil, pathErrorf(path+".on", "expression context is required")
	}
	selector, err := raw.Resolve(m.ctx)
	if err != nil {
		return nil, &PathError{Path: path + ".on", Err: err}
	}

	cases, leaveCases, err := m.enter(cases, path+".cases")
	if err != nil {
		return nil, err
	}
	defer leaveCases()

	branch, branchPath, err := selectCase(cases, selector, path+".cases")
	if err != nil {
		return nil, err
	}
	if branch == nil {
		if fallback == nil {
			return nil, pathErrorf(path, "no case for %q and no default", selector)
		}
		branch, branchPath = fallback, path+".default"
	}

	return m.resolve(branch, branchPath)
}

func selectCase(cases *yaml.Node, selector, path string) (*yaml.Node, string, error) {
	if cases.Kind != yaml.MappingNode {
		return nil, "", pathErrorf(path, "must be a mapping of case values to branches")
	}

	seen := make(map[string]bool, len(cases.Content)/2)
	var branch *yaml.Node
	for i := 0; i < len(cases.Content); i += 2 {
		key := cases.Content[i]
		if key.Kind != yaml.ScalarNode {
			return nil, "", pathErrorf(path, "case keys must be scalars, got %s", nodeKindName(key.Kind))
		}
		if seen[key.Value] {
			return nil, "", pathErrorf(path, "duplicate case %q", key.Value)
		}
		seen[key.Value] = true
		if key.Value == selector {
			branch = cases.Content[i+1]
		}
	}

	return branch, path + pathKey(selector), nil
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func pathKey(key string) string {
	if identifier.MatchString(key) {
		return "." + key
	}
	return fmt.Sprintf("[%q]", key)
}
