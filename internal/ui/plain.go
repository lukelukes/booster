package ui

import (
	"booster/internal/model"
	"fmt"
	"io"
	"strings"
)

func RenderPlan(w io.Writer, p model.Plan) error {
	var sb strings.Builder

	width := 0
	for _, s := range p.Steps {
		width = max(width, len(s.Name))
	}

	changed := 0
	for _, s := range p.Steps {
		if s.Change.Kind != model.ChangeNone {
			changed++
		}
		fmt.Fprintf(&sb, "%s %-*s  %s\n", symbol(s.Change.Kind), width, s.Name, describe(s.Change))
		for _, item := range s.Change.Items {
			fmt.Fprintf(&sb, "    %s\n", describeItem(item))
		}
	}

	if len(p.Steps) > 0 {
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "%d to change · %d up to date", changed, len(p.Steps)-changed)
	if p.Excluded > 0 {
		fmt.Fprintf(&sb, " · %d excluded", p.Excluded)
	}
	sb.WriteString("\n")

	_, err := io.WriteString(w, sb.String())
	return err
}

func symbol(kind model.ChangeKind) string {
	switch kind {
	case model.ChangeCreate:
		return "+"
	case model.ChangeUpdate:
		return "~"
	case model.ChangeUnknown:
		return "?"
	default:
		return "✓"
	}
}

func describe(c model.Change) string {
	switch c.Kind {
	case model.ChangeNone:
		return "up to date"
	case model.ChangeUnknown:
		return "known after apply"
	}

	text := fmt.Sprintf("%s %d", c.Verb, len(c.Items))
	if ok := c.Total - len(c.Items); ok > 0 {
		text += fmt.Sprintf(" · %d up to date", ok)
	}
	return text
}

func describeItem(item model.Item) string {
	switch {
	case item.From != "":
		return fmt.Sprintf("%s  %s → %s", item.Name, item.From, item.To)
	case item.To != "":
		return fmt.Sprintf("%s  → %s", item.Name, item.To)
	default:
		return item.Name
	}
}
