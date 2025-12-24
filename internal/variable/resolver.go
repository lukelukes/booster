package variable

import (
	"os"
)

type PromptCollector interface {
	Collect(defs []Definition) (map[string]string, error)
}

type Resolver struct {
	store     *FileStore
	collector PromptCollector
	envLookup func(string) string
}

type ResolverOption func(*Resolver)

func WithEnvLookup(fn func(string) string) ResolverOption {
	return func(r *Resolver) {
		r.envLookup = fn
	}
}

func WithCollector(c PromptCollector) ResolverOption {
	return func(r *Resolver) {
		r.collector = c
	}
}

func NewResolver(store *FileStore, opts ...ResolverOption) *Resolver {
	r := &Resolver{
		store:     store,
		envLookup: os.Getenv,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Resolver) Resolve(defs []Definition) (map[string]string, error) {
	if len(defs) == 0 {
		return make(map[string]string), nil
	}

	result := make(map[string]string)

	stored, err := r.store.Load()
	if err != nil {
		return nil, err
	}

	// Track which variables need prompting
	var needsPrompt []Definition

	for _, def := range defs {
		// 1. Check environment
		if val := r.envLookup(def.Name); val != "" {
			result[def.Name] = val
			continue
		}

		// 2. Check stored values
		if val, ok := stored[def.Name]; ok {
			result[def.Name] = val
			continue
		}

		// 3. Need to prompt
		needsPrompt = append(needsPrompt, def)
	}

	// Prompt for missing values if any
	if len(needsPrompt) > 0 && r.collector != nil {
		prompted, err := r.collector.Collect(needsPrompt)
		if err != nil {
			return nil, err
		}

		// Apply prompted values (with default fallback)
		for _, def := range needsPrompt {
			val := prompted[def.Name]
			if val == "" && def.Default != "" {
				val = def.Default
			}
			result[def.Name] = val

			// Save prompted value for next time
			stored[def.Name] = val
		}

		// Persist newly prompted values
		if err := r.store.Save(stored); err != nil {
			return nil, err
		}
	}

	return result, nil
}
