// Package condition provides conditional execution logic for tasks.
package condition

import "slices"

// Context holds runtime environment facts for condition evaluation.
type Context struct {
	// OS is the detected operating system or Linux distribution.
	// Examples: "darwin", "arch", "ubuntu", "fedora"
	OS string
	// Profile is the user-selected profile (not auto-detected).
	Profile string
}

// Condition defines when a task should execute.
// All specified fields use OR logic within, AND logic across fields.
type Condition struct {
	// OS matches if the detected OS is any of these values.
	OS []string
	// Profile matches if the selected profile is any of these values.
	Profile []string
}

// Evaluator checks if conditions match the current context.
type Evaluator struct {
	ctx Context
}

// NewEvaluator creates an evaluator with the given context.
func NewEvaluator(ctx Context) *Evaluator {
	return &Evaluator{ctx: ctx}
}

// Matches returns true if the condition matches the current context.
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
