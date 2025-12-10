package tui

// LayoutMode represents the layout configuration for the TUI.
type LayoutMode int

const (
	// LayoutSingleColumn displays content in a single column (narrow terminals).
	LayoutSingleColumn LayoutMode = iota
	// LayoutTwoColumn displays content in two columns (wider terminals).
	LayoutTwoColumn
)

// Layout defines the responsive layout configuration for the TUI.
// It determines column widths based on terminal dimensions.
type Layout struct {
	Mode       LayoutMode // Single or two-column mode
	Width      int        // Total terminal width
	Height     int        // Total terminal height
	LeftWidth  int        // Width for task panel (or full width in single column)
	RightWidth int        // Width for log panel (0 in single column)
}

const (
	// Breakpoint for two-column layout
	minWidthTwoColumn = 60

	// Breakpoint for switching from 45/55 to 50/50 split
	wideTerminalWidth = 101

	// Split ratios for medium terminals (60-100 cols)
	mediumLeftPercent = 45

	// Split ratios for wide terminals (>100 cols)
	wideLeftPercent = 50
)

// NewLayout creates a layout based on terminal dimensions.
// Implements responsive breakpoints for optimal viewing:
//   - < 60 cols: Single column layout
//   - 60-100 cols: Two columns with 45/55 split (more space for logs)
//   - > 100 cols: Two columns with 50/50 split
func NewLayout(width, height int) Layout {
	// Handle invalid dimensions
	if width <= 0 {
		return Layout{
			Mode:       LayoutSingleColumn,
			Width:      width,
			Height:     height,
			LeftWidth:  0,
			RightWidth: 0,
		}
	}

	// Narrow terminal: single column
	if width < minWidthTwoColumn {
		return Layout{
			Mode:       LayoutSingleColumn,
			Width:      width,
			Height:     height,
			LeftWidth:  width,
			RightWidth: 0,
		}
	}

	// Two-column layout with responsive split
	var leftWidth, rightWidth int

	if width >= wideTerminalWidth {
		// Wide terminal: 50/50 split
		leftWidth = width * wideLeftPercent / 100
		rightWidth = width - leftWidth
	} else {
		// Medium terminal: 45/55 split (more space for logs)
		leftWidth = width * mediumLeftPercent / 100
		rightWidth = width - leftWidth
	}

	return Layout{
		Mode:       LayoutTwoColumn,
		Width:      width,
		Height:     height,
		LeftWidth:  leftWidth,
		RightWidth: rightWidth,
	}
}

// IsTwoColumn returns true if the layout uses two columns.
func (l Layout) IsTwoColumn() bool {
	return l.Mode == LayoutTwoColumn
}
