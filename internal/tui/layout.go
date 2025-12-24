package tui

type LayoutMode int

const (
	LayoutSingleColumn LayoutMode = iota

	LayoutTwoColumn
)

type Layout struct {
	Mode       LayoutMode
	Width      int
	Height     int
	LeftWidth  int
	RightWidth int
}

const (
	minWidthTwoColumn = 60

	wideTerminalWidth = 101

	mediumLeftPercent = 45

	wideLeftPercent = 50
)

func NewLayout(width, height int) Layout {
	if width <= 0 {
		return Layout{
			Mode:       LayoutSingleColumn,
			Width:      width,
			Height:     height,
			LeftWidth:  0,
			RightWidth: 0,
		}
	}

	if width < minWidthTwoColumn {
		return Layout{
			Mode:       LayoutSingleColumn,
			Width:      width,
			Height:     height,
			LeftWidth:  width,
			RightWidth: 0,
		}
	}

	var leftWidth, rightWidth int

	if width >= wideTerminalWidth {

		leftWidth = width * wideLeftPercent / 100
		rightWidth = width - leftWidth
	} else {

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

func (l Layout) IsTwoColumn() bool {
	return l.Mode == LayoutTwoColumn
}
