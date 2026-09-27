package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BrailleLevels represents levels of height
var brailleRunes = []rune{' ', '⡀', '⡄', '⡆', '⡇', '⣇', '⣧', '⣷', '⣿'}

// Sparkline renders a clean speed graph using braille/blocks
func Sparkline(data []float64, width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if len(data) == 0 {
		return lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", width))
	}

	// Find max
	max := 0.0
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1.0
	}

	// Take the last `width` values
	start := len(data) - width
	if start < 0 {
		start = 0
	}
	slice := data[start:]

	var sb strings.Builder
	// Pad if slice is shorter than width
	pad := width - len(slice)
	for i := 0; i < pad; i++ {
		sb.WriteRune(' ')
	}

	for _, v := range slice {
		norm := v / max
		idx := int(norm * float64(len(brailleRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(brailleRunes) {
			idx = len(brailleRunes) - 1
		}
		sb.WriteRune(brailleRunes[idx])
	}

	return style.Copy().Background(ColorBg).Render(sb.String())
}
