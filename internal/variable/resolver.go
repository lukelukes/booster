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

	var needsPrompt []Definition

	for _, def := range defs {
		if val := r.envLookup(def.Name); val != "" {
			result[def.Name] = val
			continue
		}

		if val, ok := stored[def.Name]; ok {
			result[def.Name] = val
			continue
		}

		needsPrompt = append(needsPrompt, def)
	}

	if len(needsPrompt) > 0 && r.collector != nil {
		prompted, err := r.collector.Collect(needsPrompt)
		if err != nil {
			return nil, err
		}

		for _, def := range needsPrompt {
			val := prompted[def.Name]
			if val == "" && def.Default != "" {
				val = def.Default
			}
			result[def.Name] = val

			stored[def.Name] = val
		}

		if err := r.store.Save(stored); err != nil {
			return nil, err
		}
	}

	return result, nil
}
