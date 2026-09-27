package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors matching btop + Happ dark aesthetic (Pure Pitch Black #000000)
var (
	ColorBg        = lipgloss.Color("#000000") // pure black background
	ColorPanelBg   = lipgloss.Color("#000000")
	ColorSelection = lipgloss.Color("#501a24") // btop crimson wine row selection highlight
	ColorPrimary   = lipgloss.Color("#ff4772") // btop red/magenta accent
	ColorSecondary = lipgloss.Color("#44ccff") // cyan accent
	ColorSuccess   = lipgloss.Color("#39e68c") // bright green (low ping, connected)
	ColorWarning   = lipgloss.Color("#ffaa33") // yellow/orange (medium ping)
	ColorDanger    = lipgloss.Color("#ff3355") // high ping, disconnected
	ColorMuted     = lipgloss.Color("#666a80") // muted gray
	ColorTreeLine  = lipgloss.Color("#555970") // tree branch color
	ColorWhite     = lipgloss.Color("#ffffff")
)

// Base Styles - every style has Background(ColorBg) explicitly set!
var (
	StyleBg = lipgloss.NewStyle().Background(ColorBg)

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBg)

	StylePrimary = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBg)

	StyleSecondary = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary).
			Background(ColorBg)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary).
			Background(ColorBg)

	StyleNormal = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorBg)

	StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Background(ColorBg).
			Padding(0, 1)

	StyleActiveBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorBg).
			Padding(0, 1)

	StyleSelectedRow = lipgloss.NewStyle().
				Background(ColorSelection).
				Foreground(ColorWhite).
				Bold(true)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(ColorBg)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Background(ColorBg).
			Bold(true)

	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Background(ColorBg)

	StyleDanger = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Background(ColorBg)

	StyleTreeBranch = lipgloss.NewStyle().
			Foreground(ColorTreeLine).
			Background(ColorBg)

	StyleProcessWine = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Background(ColorBg).
			Bold(true)

	StyleProcessSystem = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(ColorBg)

	// Btop style button in footer: e.g. [f] filter
	StyleFooterKey = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Background(ColorBg).
			Bold(true)

	StyleFooterDesc = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorBg)
)

// RenderFooterButton renders btop style shortcut: [k] action
func RenderFooterButton(key, desc string) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("["),
		StyleFooterKey.Copy().Background(ColorBg).Render(key),
		lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("] "),
		StyleFooterDesc.Copy().Background(ColorBg).Render(desc),
		lipgloss.NewStyle().Background(ColorBg).Render("  "),
	)
}

// GetGreenGradient returns a subtle green shade that dims with distance from cursor (like in btop)
// Floor is #28984c so it NEVER turns dark or black!
func GetGreenGradient(dist int) string {
	switch dist {
	case 0:
		return "#69ff94" // bright highlight
	case 1:
		return "#4ee880" // bright fresh green
	case 2:
		return "#44d672"
	case 3:
		return "#3cc668"
	case 4:
		return "#34b65e"
	case 5:
		return "#2ea656"
	case 6:
		return "#2a9e50"
	default:
		return "#28984c" // rich readable forest green floor
	}
}

// Abs returns absolute value of integer
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

var DefaultThemeColors = map[string]string{
	"bg":        "#000000",
	"selection": "#501a24",
	"primary":   "#ff4772",
	"secondary": "#44ccff",
	"success":   "#39e68c",
	"warning":   "#ffaa33",
	"danger":    "#ff3355",
	"muted":     "#666a80",
	"tree":      "#555970",
}

// ReloadStyles refreshes all derived Lipgloss styles when theme colors change
func ReloadStyles() {
	StyleBg = lipgloss.NewStyle().Background(ColorBg)
	StyleTitle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Background(ColorBg)
	StylePrimary = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Background(ColorBg)
	StyleSecondary = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg)
	StyleHeader = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg)
	StyleNormal = lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg)
	StyleBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorMuted).Background(ColorBg).Padding(0, 1)
	StyleActiveBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Background(ColorBg).Padding(0, 1)
	StyleSelectedRow = lipgloss.NewStyle().Background(ColorSelection).Foreground(ColorWhite).Bold(true)
	StyleMuted = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning).Background(ColorBg)
	StyleDanger = lipgloss.NewStyle().Foreground(ColorDanger).Background(ColorBg)
	StyleTreeBranch = lipgloss.NewStyle().Foreground(ColorTreeLine).Background(ColorBg)
	StyleProcessWine = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true)
	StyleProcessSystem = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	StyleFooterKey = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)
	StyleFooterDesc = lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg)
}

// ApplyThemeColors overrides active colors from a saved hex map
func ApplyThemeColors(theme map[string]string) {
	if v, ok := theme["bg"]; ok && v != "" {
		ColorBg = lipgloss.Color(v)
		ColorPanelBg = lipgloss.Color(v)
	}
	if v, ok := theme["selection"]; ok && v != "" {
		ColorSelection = lipgloss.Color(v)
	}
	if v, ok := theme["primary"]; ok && v != "" {
		ColorPrimary = lipgloss.Color(v)
	}
	if v, ok := theme["secondary"]; ok && v != "" {
		ColorSecondary = lipgloss.Color(v)
	}
	if v, ok := theme["success"]; ok && v != "" {
		ColorSuccess = lipgloss.Color(v)
	}
	if v, ok := theme["warning"]; ok && v != "" {
		ColorWarning = lipgloss.Color(v)
	}
	if v, ok := theme["danger"]; ok && v != "" {
		ColorDanger = lipgloss.Color(v)
	}
	if v, ok := theme["muted"]; ok && v != "" {
		ColorMuted = lipgloss.Color(v)
	}
	if v, ok := theme["tree"]; ok && v != "" {
		ColorTreeLine = lipgloss.Color(v)
	}
	ReloadStyles()
}

// GetCurrentThemeMap returns active colors in map form
func GetCurrentThemeMap() map[string]string {
	return map[string]string{
		"bg":        string(ColorBg),
		"selection": string(ColorSelection),
		"primary":   string(ColorPrimary),
		"secondary": string(ColorSecondary),
		"success":   string(ColorSuccess),
		"warning":   string(ColorWarning),
		"danger":    string(ColorDanger),
		"muted":     string(ColorMuted),
		"tree":      string(ColorTreeLine),
	}
}

// HexToRGB parses a hex color string like "#000000" or "#fff" into r, g, b components
func HexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 0, 0, 0
	}
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// RGBToHex formats r, g, b integers (0-255) into a 6-digit hex string "#rrggbb"
func RGBToHex(r, g, b int) string {
	r = clamp(r, 0, 255)
	g = clamp(g, 0, 255)
	b = clamp(b, 0, 255)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
