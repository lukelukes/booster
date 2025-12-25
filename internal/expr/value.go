package expr

import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// Value represents a configuration value that may contain expressions.
// It can be:
//   - A literal (string, int, bool, list, map) with no expressions
//   - A full expression: entire value is ${ expr }
//   - An interpolated string: "prefix ${ expr } suffix"
type Value struct {
	raw any // Original value from YAML

	// For string values that may contain expressions
	parts      []part // Parsed parts (literal strings and expressions)
	isFullExpr bool   // True if entire value is a single ${ expr }

	// Compiled expression (for full expressions)
	program *vm.Program
}

type part struct {
	literal string      // Non-empty for literal parts
	program *vm.Program // Non-nil for expression parts
}

// NewValue creates a Value from a raw YAML value.
// It parses any ${ } expressions found in string values.
func NewValue(raw any) (*Value, error) {
	v := &Value{raw: raw}

	str, ok := raw.(string)
	if !ok {
		// Non-string values are literals (could contain nested Values in lists/maps)
		return v, nil
	}

	// Check if this is a full expression (entire string is ${ expr })
	trimmed := strings.TrimSpace(str)
	if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") {
		inner := strings.TrimSpace(trimmed[2 : len(trimmed)-1])
		// Verify it's a single expression (no other ${ in the middle)
		if !strings.Contains(inner, "${") {
			program, err := expr.Compile(inner, CompileOptions()...)
			if err != nil {
				return nil, fmt.Errorf("invalid expression %q: %w", inner, err)
			}
			v.program = program
			v.isFullExpr = true
			return v, nil
		}
	}

	// Parse as interpolated string
	parts, err := parseInterpolated(str)
	if err != nil {
		return nil, err
	}
	v.parts = parts

	return v, nil
}

// parseInterpolated splits a string into literal and expression parts.
// Uses brace-matching to correctly handle nested braces in expressions
// like ${ {"key": "value"}.key }.
func parseInterpolated(s string) ([]part, error) {
	var parts []part
	exprs := findExpressions(s)

	lastEnd := 0
	for _, e := range exprs {
		// Add literal part before this expression
		if e.start > lastEnd {
			parts = append(parts, part{literal: s[lastEnd:e.start]})
		}

		program, err := expr.Compile(e.inner, CompileOptions()...)
		if err != nil {
			return nil, fmt.Errorf("invalid expression %q: %w", e.inner, err)
		}
		parts = append(parts, part{program: program})
		lastEnd = e.end
	}

	// Add trailing literal if any
	if lastEnd < len(s) {
		parts = append(parts, part{literal: s[lastEnd:]})
	}

	return parts, nil
}

// exprSpan represents a ${ ... } expression's location in a string.
type exprSpan struct {
	start int    // Index of '$'
	end   int    // Index after closing '}'
	inner string // The expression content (without ${ })
}

// findExpressions locates all ${ ... } expressions in s, handling nested braces.
func findExpressions(s string) []exprSpan {
	var spans []exprSpan
	i := 0

	for i < len(s)-1 {
		// Look for ${
		if s[i] == '$' && s[i+1] == '{' {
			start := i
			i += 2 // Skip past ${

			// Count braces to find matching }
			depth := 1
			exprStart := i

			for i < len(s) && depth > 0 {
				switch s[i] {
				case '{':
					depth++
				case '}':
					depth--
				case '"', '\'':
					// Skip string literals to avoid counting braces inside them
					quote := s[i]
					i++
					for i < len(s) && s[i] != quote {
						if s[i] == '\\' && i+1 < len(s) {
							i++ // Skip escaped char
						}
						i++
					}
				}
				if depth > 0 {
					i++
				}
			}

			if depth == 0 {
				inner := strings.TrimSpace(s[exprStart:i])
				spans = append(spans, exprSpan{
					start: start,
					end:   i + 1, // Include the closing }
					inner: inner,
				})
			}
			i++ // Move past closing }
		} else {
			i++
		}
	}

	return spans
}

// IsLiteral returns true if this value contains no expressions.
func (v *Value) IsLiteral() bool {
	if v.program != nil {
		return false
	}
	for _, p := range v.parts {
		if p.program != nil {
			return false
		}
	}
	return true
}

// Resolve evaluates any expressions in this value against the given context.
func (v *Value) Resolve(ctx *Context) (any, error) {
	// Full expression: return typed result
	if v.isFullExpr && v.program != nil {
		return expr.Run(v.program, ctx)
	}

	// No expressions: return raw value
	if v.IsLiteral() {
		return v.raw, nil
	}

	// Interpolated string: evaluate parts and concatenate
	var sb strings.Builder
	for _, p := range v.parts {
		if p.program != nil {
			result, err := expr.Run(p.program, ctx)
			if err != nil {
				return nil, err
			}
			sb.WriteString(fmt.Sprint(result))
		} else {
			sb.WriteString(p.literal)
		}
	}
	return sb.String(), nil
}

// MustResolve is like Resolve but panics on error. For testing.
func (v *Value) MustResolve(ctx *Context) any {
	result, err := v.Resolve(ctx)
	if err != nil {
		panic(err)
	}
	return result
}

// ResolveCondition evaluates a when expression and returns whether
// the task should run. Returns (shouldRun, error).
//
// Rules:
//   - nil or literal empty string → always run (return true)
//   - expression must evaluate to bool
//   - non-bool result is an error
func ResolveCondition(when *Value, ctx *Context) (bool, error) {
	// nil or empty string means "always run"
	if when == nil {
		return true, nil
	}
	if when.IsLiteral() {
		if s, ok := when.raw.(string); ok && s == "" {
			return true, nil
		}
	}

	result, err := when.Resolve(ctx)
	if err != nil {
		return false, err
	}

	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("condition must evaluate to bool, got %T", result)
	}
	return b, nil
}

// String returns a string representation for debugging.
func (v *Value) String() string {
	if v.isFullExpr {
		return fmt.Sprintf("Expr(%v)", v.raw)
	}
	if len(v.parts) > 0 {
		return fmt.Sprintf("Interpolated(%v)", v.raw)
	}
	return fmt.Sprintf("Literal(%v)", v.raw)
}
