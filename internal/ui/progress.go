package ui

import (
	"booster/internal/model"
	"fmt"
	"io"
	"strings"
)

func Progress(w io.Writer, plan model.Plan) func(model.Event) {
	name := func(i int) string {
		if i >= 0 && i < len(plan.Steps) {
			return plan.Steps[i].Name
		}
		return fmt.Sprintf("step %d", i+1)
	}

	return func(e model.Event) {
		switch e := e.(type) {
		case model.RunStart:
			fmt.Fprintf(w, "\napplying %d steps\n", e.Total)
		case model.TaskStart:
			if e.Change.Kind != model.ChangeNone {
				fmt.Fprintf(w, "→ %s\n", e.Name)
			}
		case model.TaskOutput:
			fmt.Fprintln(w, strings.TrimRight("  │ "+e.Line, " "))
		case model.TaskDone:
			fmt.Fprintf(w, "%s %s  %s\n", resultSymbol(e.Result.Status), name(e.Index), describeResult(e.Result))
		case model.RunDone:
			fmt.Fprintf(w, "\n%s\n", describeSummary(e.Summary))
		}
	}
}

func resultSymbol(status model.Status) string {
	switch status {
	case model.StatusChanged:
		return "+"
	case model.StatusFailed:
		return "✗"
	case model.StatusSkipped:
		return "-"
	case model.StatusUnknown:
		return "?"
	default:
		return "✓"
	}
}

func describeResult(r model.Result) string {
	switch r.Status {
	case model.StatusChanged:
		return "changed"
	case model.StatusFailed:
		return fmt.Sprintf("failed: %v", r.Err)
	case model.StatusSkipped:
		return r.Reason
	case model.StatusUnknown:
		return "unknown"
	default:
		return "up to date"
	}
}

func describeSummary(s model.Summary) string {
	parts := []string{fmt.Sprintf("%d changed", s.Changed), fmt.Sprintf("%d up to date", s.OK)}
	if s.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", s.Failed))
	}
	if s.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d not run", s.Skipped))
	}
	return strings.Join(parts, " · ")
}
