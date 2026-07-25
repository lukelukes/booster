package action

import (
	"booster/internal/expr"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

type SourceTarget struct {
	Source string
	Target string
}

type sourceTargetArgs struct {
	Source expr.Str `yaml:"source"`
	Target expr.Str `yaml:"target"`
}

func decodeSourceTargets(args yaml.Node, ctx *expr.Context) ([]SourceTarget, error) {
	if args.IsZero() || args.Kind != yaml.SequenceNode {
		return nil, errors.New("args must be a list of {source, target} entries")
	}

	for i, item := range args.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("[%d]: must be a mapping with 'source' and 'target'", i)
		}
	}

	var raw []sourceTargetArgs
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}

	pairs := make([]SourceTarget, 0, len(raw))
	for i, item := range raw {
		if item.Source == "" {
			return nil, fmt.Errorf("[%d]: missing 'source'", i)
		}
		if item.Target == "" {
			return nil, fmt.Errorf("[%d]: missing 'target'", i)
		}

		source, err := item.Source.Resolve(ctx)
		if err != nil {
			return nil, fmt.Errorf("[%d].source: %w", i, err)
		}
		target, err := item.Target.Resolve(ctx)
		if err != nil {
			return nil, fmt.Errorf("[%d].target: %w", i, err)
		}

		pairs = append(pairs, SourceTarget{Source: source, Target: target})
	}

	return pairs, nil
}
