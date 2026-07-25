package expr

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Str string

func (s *Str) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == 0 {
		return nil
	}
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a scalar value, got %s", nodeKindName(node.Kind))
	}

	*s = Str(node.Value)
	return nil
}

func (s Str) Resolve(ctx *Context) (string, error) {
	value, err := NewValue(string(s))
	if err != nil {
		return "", err
	}

	resolved, err := value.Resolve(ctx)
	if err != nil {
		return "", err
	}

	switch v := resolved.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	default:
		return fmt.Sprint(v), nil
	}
}

func ResolveAll(values []Str, ctx *Context) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	resolved := make([]string, 0, len(values))
	for i, v := range values {
		s, err := v.Resolve(ctx)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		resolved = append(resolved, s)
	}
	return resolved, nil
}

func nodeKindName(kind yaml.Kind) string {
	switch kind {
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a sequence"
	case yaml.AliasNode:
		return "an alias"
	case yaml.DocumentNode:
		return "a document"
	case yaml.ScalarNode:
		return "a scalar"
	default:
		return "an unknown node"
	}
}
