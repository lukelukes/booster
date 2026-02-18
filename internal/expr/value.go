package expr

import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

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
	exprs := findExpressions(s)

	lastEnd := 0
	for _, e := range exprs {
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

	if lastEnd < len(s) {
		parts = append(parts, part{literal: s[lastEnd:]})
	}

	return parts, nil
}

type exprSpan struct {
	start int
	end   int
	inner string
}

func findExpressions(s string) []exprSpan {
	var spans []exprSpan
	i := 0

	for i < len(s)-1 {
		if s[i] == '$' && s[i+1] == '{' {
			start := i
			i += 2

			depth := 1
			exprStart := i

			for i < len(s) && depth > 0 {
				switch s[i] {
				case '{':
					depth++
				case '}':
					depth--
				case '"', '\'':
					quote := s[i]
					i++
					for i < len(s) && s[i] != quote {
						if s[i] == '\\' && i+1 < len(s) {
							i++
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
					end:   i + 1,
					inner: inner,
				})
			}
			i++
		} else {
			i++
		}
	}

	return spans
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
			fmt.Fprint(&sb, result)
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

func (v *Value) String() string {
	if v.isFullExpr {
		return fmt.Sprintf("Expr(%v)", v.raw)
	}
	if len(v.parts) > 0 {
		return fmt.Sprintf("Interpolated(%v)", v.raw)
	}
	return fmt.Sprintf("Literal(%v)", v.raw)
}
