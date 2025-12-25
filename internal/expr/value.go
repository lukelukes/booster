package expr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

var exprPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

type Value struct {
	raw any

	parts      []part
	isFullExpr bool

	program *vm.Program
}

type part struct {
	literal string
	program *vm.Program
}

func NewValue(raw any) (*Value, error) {
	v := &Value{raw: raw}

	str, ok := raw.(string)
	if !ok {
		return v, nil
	}

	trimmed := strings.TrimSpace(str)
	if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") {
		inner := strings.TrimSpace(trimmed[2 : len(trimmed)-1])
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

	parts, err := parseInterpolated(str)
	if err != nil {
		return nil, err
	}
	v.parts = parts

	return v, nil
}

func parseInterpolated(s string) ([]part, error) {
	var parts []part
	lastEnd := 0

	matches := exprPattern.FindAllStringSubmatchIndex(s, -1)
	for _, match := range matches {
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

func (v *Value) Resolve(ctx *Context) (any, error) {
	if v.isFullExpr && v.program != nil {
		return expr.Run(v.program, ctx)
	}

	if v.IsLiteral() {
		return v.raw, nil
	}

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

func (v *Value) MustResolve(ctx *Context) any {
	result, err := v.Resolve(ctx)
	if err != nil {
		panic(err)
	}
	return result
}

func ResolveCondition(when *Value, ctx *Context) (bool, error) {
	// TODO: Implement this
	panic("not implemented")
}

func (v *Value) String() string {
	if v.isFullExpr {
		return fmt.Sprintf("Expr(%v)", v.raw)
	}
	if len(v.parts) > 0 {
		return fmt.Sprintf("Interpolated(%v)", v.raw)
	}
	return fmt.Sprintf("Literal(%v)", v.raw)
}
