package expr

import (
	"fmt"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"
)

func ResolveMatches(node yaml.Node, ctx *Context) (yaml.Node, error) {
	resolved, err := resolveMatchesAt(&node, ctx, "")
	if err != nil {
		return yaml.Node{}, err
	}
	return *resolved, nil
}

func resolveMatchesAt(node *yaml.Node, ctx *Context, path string) (*yaml.Node, error) {
	switch node.Kind {
	case yaml.AliasNode:
		return resolveMatchesAt(node.Alias, ctx, path)
	case yaml.MappingNode:
		if len(node.Content) == 2 && node.Content[0].Value == "match" {
			return resolveMatch(node.Content[1], ctx, path+".match")
		}
		out := copyNode(node)
		for i := 1; i < len(out.Content); i += 2 {
			resolved, err := resolveMatchesAt(out.Content[i], ctx, path+pathKey(out.Content[i-1].Value))
			if err != nil {
				return nil, err
			}
			out.Content[i] = resolved
		}
		return out, nil
	case yaml.SequenceNode, yaml.DocumentNode:
		out := copyNode(node)
		for i, child := range out.Content {
			resolved, err := resolveMatchesAt(child, ctx, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			out.Content[i] = resolved
		}
		return out, nil
	default:
		return node, nil
	}
}

func copyNode(node *yaml.Node) *yaml.Node {
	out := *node
	out.Content = slices.Clone(node.Content)
	return &out
}

func resolveMatch(block *yaml.Node, ctx *Context, path string) (*yaml.Node, error) {
	if block.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: must be a mapping with 'on', 'cases' and optional 'default'", path)
	}

	var on, cases, fallback *yaml.Node
	for i := 0; i < len(block.Content); i += 2 {
		value := block.Content[i+1]
		switch key := block.Content[i].Value; key {
		case "on":
			on = value
		case "cases":
			cases = value
		case "default":
			fallback = value
		default:
			return nil, fmt.Errorf("%s: unknown key %q", path, key)
		}
	}
	if on == nil {
		return nil, fmt.Errorf("%s: missing 'on'", path)
	}
	if cases == nil {
		return nil, fmt.Errorf("%s: missing 'cases'", path)
	}

	var raw Str
	if err := raw.UnmarshalYAML(on); err != nil {
		return nil, fmt.Errorf("%s.on: %w", path, err)
	}
	if ctx == nil {
		return nil, fmt.Errorf("%s.on: expression context is required", path)
	}
	selector, err := raw.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s.on: %w", path, err)
	}

	branch, branchPath, err := selectCase(cases, selector, path+".cases")
	if err != nil {
		return nil, err
	}
	if branch == nil {
		if fallback == nil {
			return nil, fmt.Errorf("%s: no case for %q and no default", path, selector)
		}
		branch, branchPath = fallback, path+".default"
	}

	return resolveMatchesAt(branch, ctx, branchPath)
}

func selectCase(cases *yaml.Node, selector, path string) (*yaml.Node, string, error) {
	if cases.Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("%s: must be a mapping of case values to branches", path)
	}

	seen := make(map[string]bool, len(cases.Content)/2)
	var branch *yaml.Node
	for i := 0; i < len(cases.Content); i += 2 {
		key := cases.Content[i]
		if key.Kind != yaml.ScalarNode {
			return nil, "", fmt.Errorf("%s: case keys must be scalars, got %s", path, nodeKindName(key.Kind))
		}
		if seen[key.Value] {
			return nil, "", fmt.Errorf("%s: duplicate case %q", path, key.Value)
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
