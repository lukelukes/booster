package variable

import (
	"os"
)

// PromptCollector collects variable values from the user.
// This interface enables testing without TUI interaction.
type PromptCollector interface {
	// Collect prompts user for values and returns collected map.
	// defs contains only variables that need prompting.
	Collect(defs []Definition) (map[string]string, error)
}

// Resolver handles the variable resolution pipeline.
// Resolution order: environment → stored → prompt
type Resolver struct {
	store     *FileStore
	collector PromptCollector
	envLookup func(string) string
}

// ResolverOption configures a Resolver.
type ResolverOption func(*Resolver)

// WithEnvLookup sets a custom environment variable lookup function.
// Useful for testing.
func WithEnvLookup(fn func(string) string) ResolverOption {
	return func(r *Resolver) {
		r.envLookup = fn
	}
}

// WithCollector sets the prompt collector.
func WithCollector(c PromptCollector) ResolverOption {
	return func(r *Resolver) {
		r.collector = c
	}
}

// NewResolver creates a resolver with the given store.
func NewResolver(store *FileStore, opts ...ResolverOption) *Resolver {
	r := &Resolver{
		store:     store,
		envLookup: os.Getenv, // default to real env lookup
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve resolves all variables from definitions.
// Returns a map of variable name to resolved value.
func (r *Resolver) Resolve(defs []Definition) (map[string]string, error) {
	if len(defs) == 0 {
		return make(map[string]string), nil
	}

	result := make(map[string]string)

	// Load stored values
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
