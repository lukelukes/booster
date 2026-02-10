package task

import (
	"booster/internal/expr"
	"errors"
	"fmt"
	"sort"
)

type argResolveError struct {
	path string
	err  error
}

func (e *argResolveError) Error() string {
	return e.err.Error()
}

func (e *argResolveError) Unwrap() error {
	return e.err
}

func resolveTaskArgs(args any, ctx *expr.Context) (any, error) {
	return resolveArgAtPath(args, ctx, "")
}

func resolveArgAtPath(v any, ctx *expr.Context, path string) (any, error) {
	switch typed := v.(type) {
	case nil, bool, int, int8, int16, int32, int64, float32, float64:
		return typed, nil
	case string:
		return resolveStringArg(typed, ctx, path)
	case []any:
		resolved := make([]any, len(typed))
		for i, item := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			r, err := resolveArgAtPath(item, ctx, childPath)
			if err != nil {
				return nil, err
			}
			resolved[i] = r
		}
		return resolved, nil
	case map[string]any:
		resolved := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := path + formatPathKey(key)
			r, err := resolveArgAtPath(typed[key], ctx, childPath)
			if err != nil {
				return nil, err
			}
			resolved[key] = r
		}
		return resolved, nil
	default:
		return typed, nil
	}
}

func resolveStringArg(raw string, ctx *expr.Context, path string) (any, error) {
	value, err := expr.NewValue(raw)
	if err != nil {
		return nil, &argResolveError{path: path, err: err}
	}

	if value.IsLiteral() {
		return raw, nil
	}

	if ctx == nil {
		return nil, &argResolveError{path: path, err: errors.New("expression context is required")}
	}

	resolved, err := value.Resolve(ctx)
	if err != nil {
		return nil, &argResolveError{path: path, err: err}
	}
	return resolved, nil
}

func formatPathKey(key string) string {
	if isPathIdentifier(key) {
		return "." + key
	}
	return fmt.Sprintf("[%q]", key)
}

func isPathIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
			continue
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

