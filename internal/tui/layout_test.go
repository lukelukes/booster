package tui

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLayout(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		wantMode  LayoutMode
		wantLeft  int
		wantRight int
	}{
		{
			name:      "narrow terminal - single column",
			width:     59,
			height:    24,
			wantMode:  LayoutSingleColumn,
			wantLeft:  59,
			wantRight: 0,
		},
		{
			name:      "boundary at 60 cols - two column",
			width:     60,
			height:    24,
			wantMode:  LayoutTwoColumn,
			wantLeft:  27,
			wantRight: 33,
		},
		{
			name:      "medium terminal - 45/55 split",
			width:     80,
			height:    24,
			wantMode:  LayoutTwoColumn,
			wantLeft:  36,
			wantRight: 44,
		},
		{
			name:      "medium terminal upper bound - 45/55 split",
			width:     100,
			height:    30,
			wantMode:  LayoutTwoColumn,
			wantLeft:  45,
			wantRight: 55,
		},
		{
			name:      "boundary at 101 cols - 50/50 split",
			width:     101,
			height:    30,
			wantMode:  LayoutTwoColumn,
			wantLeft:  50,
			wantRight: 51,
		},
		{
			name:      "wide terminal - 50/50 split",
			width:     120,
			height:    30,
			wantMode:  LayoutTwoColumn,
			wantLeft:  60,
			wantRight: 60,
		},
		{
			name:      "very wide terminal - 50/50 split",
			width:     200,
			height:    40,
			wantMode:  LayoutTwoColumn,
			wantLeft:  100,
			wantRight: 100,
		},
		{
			name:      "zero width - single column with zero",
			width:     0,
			height:    24,
			wantMode:  LayoutSingleColumn,
			wantLeft:  0,
			wantRight: 0,
		},
		{
			name:      "negative width - single column with zero",
			width:     -10,
			height:    24,
			wantMode:  LayoutSingleColumn,
			wantLeft:  0,
			wantRight: 0,
		},
		{
			name:      "zero height - still calculates width",
			width:     80,
			height:    0,
			wantMode:  LayoutTwoColumn,
			wantLeft:  36,
			wantRight: 44,
		},
		{
			name:      "negative height - still calculates width",
			width:     80,
			height:    -5,
			wantMode:  LayoutTwoColumn,
			wantLeft:  36,
			wantRight: 44,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := NewLayout(tt.width, tt.height)

			assert.Equal(t, tt.wantMode, layout.Mode, "layout mode mismatch")
			assert.Equal(t, tt.width, layout.Width, "width should be stored")
			assert.Equal(t, tt.height, layout.Height, "height should be stored")
			assert.Equal(t, tt.wantLeft, layout.LeftWidth, "left width mismatch")
			assert.Equal(t, tt.wantRight, layout.RightWidth, "right width mismatch")

			if layout.Mode == LayoutTwoColumn && tt.width > 0 {
				assert.Equal(t, tt.width, layout.LeftWidth+layout.RightWidth,
					"left + right should equal total width")
			}
		})
	}
}

func TestLayout_IsTwoColumn(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  bool
	}{
		{
			name:  "narrow terminal is not two column",
			width: 59,
			want:  false,
		},
		{
			name:  "boundary at 60 is two column",
			width: 60,
			want:  true,
		},
		{
			name:  "medium terminal is two column",
			width: 80,
			want:  true,
		},
		{
			name:  "wide terminal is two column",
			width: 120,
			want:  true,
		},
		{
			name:  "zero width is not two column",
			width: 0,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := NewLayout(tt.width, 24)
			assert.Equal(t, tt.want, layout.IsTwoColumn(), "IsTwoColumn() result mismatch")
		})
	}
}

func TestNewLayout_BreakpointBehavior(t *testing.T) {
	t.Run("breakpoint at 60 cols", func(t *testing.T) {
		layout59 := NewLayout(59, 24)
		assert.Equal(t, LayoutSingleColumn, layout59.Mode, "59 cols should be single column")

		layout60 := NewLayout(60, 24)
		assert.Equal(t, LayoutTwoColumn, layout60.Mode, "60 cols should be two column")
	})

	t.Run("breakpoint at 101 cols for split ratio change", func(t *testing.T) {
		layout100 := NewLayout(100, 24)
		assert.Equal(t, 45, layout100.LeftWidth, "100 cols should use 45/55 split")
		assert.Equal(t, 55, layout100.RightWidth, "100 cols should use 45/55 split")

		layout101 := NewLayout(101, 24)
		assert.Equal(t, 50, layout101.LeftWidth, "101 cols should use 50/50 split")
		assert.Equal(t, 51, layout101.RightWidth, "101 cols should use 50/50 split")
	})
}

func TestNewLayout_SplitRatios(t *testing.T) {
	t.Run("medium terminals use 45/55 split", func(t *testing.T) {
		widths := []int{60, 70, 80, 90, 100}
		for _, width := range widths {
			layout := NewLayout(width, 24)

			expectedLeft := width * 45 / 100
			expectedRight := width - expectedLeft

			assert.Equal(t, expectedLeft, layout.LeftWidth,
				"width %d should have 45%% left split", width)
			assert.Equal(t, expectedRight, layout.RightWidth,
				"width %d should have 55%% right split", width)
		}
	})

	t.Run("wide terminals use 50/50 split", func(t *testing.T) {
		widths := []int{101, 120, 150, 200}
		for _, width := range widths {
			layout := NewLayout(width, 24)

			expectedLeft := width / 2
			expectedRight := width - expectedLeft

			assert.Equal(t, expectedLeft, layout.LeftWidth,
				"width %d should have 50%% left split", width)
			assert.Equal(t, expectedRight, layout.RightWidth,
				"width %d should have 50%% right split", width)
		}
	})
}

func TestNewLayout_WidthConservation(t *testing.T) {
	widths := []int{60, 61, 62, 80, 99, 100, 101, 120, 121, 150}

	for _, width := range widths {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			layout := NewLayout(width, 24)

			if layout.IsTwoColumn() {
				sum := layout.LeftWidth + layout.RightWidth
				assert.Equal(t, width, sum,
					"left (%d) + right (%d) should equal total width (%d)",
					layout.LeftWidth, layout.RightWidth, width)
			}
		})
	}
}

func TestLayoutMode_String(t *testing.T) {
	assert.NotEqual(t, LayoutSingleColumn, LayoutTwoColumn,
		"layout modes should have distinct values")
}
