package condition

import "slices"

type Context struct {
	OS string

	Profile string
}

type Condition struct {
	OS []string

	Profile []string
}

type Evaluator struct {
	ctx Context
}

func NewEvaluator(ctx Context) *Evaluator {
	return &Evaluator{ctx: ctx}
}

// A nil or empty condition always matches.
func (e *Evaluator) Matches(c *Condition) bool {
	if c == nil {
		return true
	}

	if len(c.OS) > 0 && !contains(c.OS, e.ctx.OS) {
		return false
	}

	if len(c.Profile) > 0 && !contains(c.Profile, e.ctx.Profile) {
		return false
	}

	return true
}

// FailureReason returns a human-readable reason why the condition failed.
// Returns empty string if condition matches.
func (e *Evaluator) FailureReason(c *Condition) string {
	if c == nil || e.Matches(c) {
		return ""
	}

	if len(c.OS) > 0 && !contains(c.OS, e.ctx.OS) {
		return "os=" + e.ctx.OS + ", want " + joinStrings(c.OS)
	}

	if len(c.Profile) > 0 && !contains(c.Profile, e.ctx.Profile) {
		return "profile=" + e.ctx.Profile + ", want " + joinStrings(c.Profile)
	}

	return ""
}

func contains(slice []string, val string) bool {
	return slices.Contains(slice, val)
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	if len(ss) == 1 {
		return ss[0]
	}

	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += " or " + ss[i]
	}
	return result
}
