package task

import (
	"errors"
	"fmt"
)

type SourceTarget struct {
	Source string
	Target string
}

func parseSourceTargetArgs(args any) ([]SourceTarget, error) {
	items, ok := args.([]any)
	if !ok {
		return nil, errors.New("args must be a list of {source, target} maps")
	}

	result := make([]SourceTarget, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("arg %d: must be a map with 'source' and 'target'", i+1)
		}

		sourceRaw, hasSource := m["source"]
		if !hasSource {
			return nil, fmt.Errorf("arg %d: missing 'source'", i+1)
		}
		source, ok := sourceRaw.(string)
		if !ok {
			return nil, fmt.Errorf("arg %d: 'source' must be a string", i+1)
		}

		targetRaw, hasTarget := m["target"]
		if !hasTarget {
			return nil, fmt.Errorf("arg %d: missing 'target'", i+1)
		}
		target, ok := targetRaw.(string)
		if !ok {
			return nil, fmt.Errorf("arg %d: 'target' must be a string", i+1)
		}

		result = append(result, SourceTarget{Source: source, Target: target})
	}

	return result, nil
}
