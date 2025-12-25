package expr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// exprPattern matches ${ ... } expressions, handling nested braces.
var exprPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

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
func parseInterpolated(s string) ([]part, error) {
	var parts []part
	lastEnd := 0

	matches := exprPattern.FindAllStringSubmatchIndex(s, -1)
	for _, match := range matches {
		// match[0]:match[1] is the full match ${ ... }
		// match[2]:match[3] is the capture group (the expression)
		if match[0] > lastEnd {
			parts = append(parts, part{literal: s[lastEnd:match[0]]})
		}

		exprStr := s[match[2]:match[3]]
		program, err := expr.Compile(exprStr, CompileOptions()...)
		if err != nil {
			return nil, fmt.Errorf("invalid expression %q: %w", exprStr, err)
		}
		parts = append(parts, part{program: program})
		lastEnd = match[1]
	}

	if lastEnd < len(s) {
		parts = append(parts, part{literal: s[lastEnd:]})
	}

	return parts, nil
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
