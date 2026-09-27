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

	// Section 1: Text & Gradient Colors
	ColorTextNormal    = lipgloss.Color("#ffffff")
	ColorTextMuted     = lipgloss.Color("#666a80")
	ColorTextPrimary   = lipgloss.Color("#ff4772")
	ColorTextSecondary = lipgloss.Color("#44ccff")
	ColorTextSuccess   = lipgloss.Color("#39e68c")
	ColorTextWarning   = lipgloss.Color("#ffaa33")
	ColorTextDanger    = lipgloss.Color("#ff3355")
	ColorGradBase      = lipgloss.Color("#69ff94")
	ThemeGradIntensity = 5

	// Section 2: Interface Elements Overrides
	ColorBorderActive   = lipgloss.Color("#ff4772")
	ColorBorderInactive = lipgloss.Color("#666a80")
	ColorGraphRx        = lipgloss.Color("#39e68c")
	ColorGraphTx        = lipgloss.Color("#44ccff")

	// Section 3: Logs Viewer Overrides
	ColorLogTabActive   = lipgloss.Color("#501a24")
	ColorLogTabInactive = lipgloss.Color("#666a80")
	ColorLogInfo        = lipgloss.Color("#39e68c")
	ColorLogWarn        = lipgloss.Color("#ffaa33")
	ColorLogError       = lipgloss.Color("#ff3355")
	ColorLogInbound     = lipgloss.Color("#ff4772")
	ColorLogOutbound    = lipgloss.Color("#44ccff")
	ColorLogMeta        = lipgloss.Color("#666a80")
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

// GetGradientColor calculates a gradient shade that dims with distance from cursor based on base color and intensity
func GetGradientColor(dist int) string {
	baseHex := string(ColorGradBase)
	intensity := ThemeGradIntensity
	if intensity < 1 {
		intensity = 1
	}
	if intensity > 10 {
		intensity = 10
	}

	// Exact btop green palette when using factory default green #69ff94 at intensity 5
	if strings.EqualFold(baseHex, "#69ff94") && intensity == 5 {
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

	r, g, b := HexToRGB(baseHex)
	floorFactor := 0.75 - float64(intensity)*0.05
	if floorFactor < 0.25 {
		floorFactor = 0.25
	}

	factor := 1.0
	if dist > 0 {
		d := dist
		if d > 6 {
			d = 6
		}
		ratio := float64(d) / 6.0
		factor = 1.0 - ratio*(1.0-floorFactor)
	}

	curR := int(float64(r) * factor)
	curG := int(float64(g) * factor)
	curB := int(float64(b) * factor)

	// Ensure readable brightness floor so text never becomes unreadable on #000000 background
	minFloor := 40
	if curR < minFloor && curG < minFloor && curB < minFloor {
		if r >= g && r >= b {
			curR = minFloor
		} else if g >= r && g >= b {
			curG = minFloor
		} else {
			curB = minFloor
		}
	}

	return RGBToHex(curR, curG, curB)
}

// GetGreenGradient returns a subtle green shade that dims with distance from cursor (like in btop)
// Floor is #28984c so it NEVER turns dark or black!
func GetGreenGradient(dist int) string {
	return GetGradientColor(dist)
}

// Abs returns absolute value of integer
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

var DefaultThemeColors = map[string]string{
	// Section 1: Text & Gradient
	"text_normal":    "#ffffff",
	"text_muted":     "#666a80",
	"text_primary":   "#ff4772",
	"text_secondary": "#44ccff",
	"text_success":   "#39e68c",
	"text_warning":   "#ffaa33",
	"text_danger":    "#ff3355",
	"grad_color":     "#69ff94",
	"grad_intensity": "5",

	// Section 2: Interface Elements
	"bg":              "#000000",
	"selection":       "#501a24",
	"primary":         "#ff4772",
	"secondary":       "#44ccff",
	"muted":           "#666a80",
	"tree":            "#555970",
	"border_active":   "#ff4772",
	"border_inactive": "#666a80",
	"graph_rx":        "#39e68c",
	"graph_tx":        "#44ccff",

	// Section 3: Logs Viewer
	"log_tab_active":   "#501a24",
	"log_tab_inactive": "#666a80",
	"log_info":         "#39e68c",
	"log_warn":         "#ffaa33",
	"log_error":        "#ff3355",
	"log_inbound":      "#ff4772",
	"log_outbound":     "#44ccff",
	"log_meta":         "#666a80",

	// Compatibility aliases
	"success": "#39e68c",
	"warning": "#ffaa33",
	"danger":  "#ff3355",
}

// ReloadStyles refreshes all derived Lipgloss styles when theme colors change
func ReloadStyles() {
	StyleBg = lipgloss.NewStyle().Background(ColorBg)
	StyleTitle = lipgloss.NewStyle().Bold(true).Foreground(ColorTextPrimary).Background(ColorBg)
	StylePrimary = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Background(ColorBg)
	StyleSecondary = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg)
	StyleHeader = lipgloss.NewStyle().Bold(true).Foreground(ColorTextSecondary).Background(ColorBg)
	StyleNormal = lipgloss.NewStyle().Foreground(ColorTextNormal).Background(ColorBg)
	StyleBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorderInactive).Background(ColorBg).Padding(0, 1)
	StyleActiveBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorderActive).Background(ColorBg).Padding(0, 1)
	StyleSelectedRow = lipgloss.NewStyle().Background(ColorSelection).Foreground(ColorTextNormal).Bold(true)
	StyleMuted = lipgloss.NewStyle().Foreground(ColorTextMuted).Background(ColorBg)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorTextSuccess).Background(ColorBg).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorTextWarning).Background(ColorBg)
	StyleDanger = lipgloss.NewStyle().Foreground(ColorTextDanger).Background(ColorBg)
	StyleTreeBranch = lipgloss.NewStyle().Foreground(ColorTreeLine).Background(ColorBg)
	StyleProcessWine = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true)
	StyleProcessSystem = lipgloss.NewStyle().Foreground(ColorTextMuted).Background(ColorBg)
	StyleFooterKey = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)
	StyleFooterDesc = lipgloss.NewStyle().Foreground(ColorTextNormal).Background(ColorBg)
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
		ColorBorderActive = lipgloss.Color(v)
		ColorTextPrimary = lipgloss.Color(v)
	}
	if v, ok := theme["secondary"]; ok && v != "" {
		ColorSecondary = lipgloss.Color(v)
		ColorTextSecondary = lipgloss.Color(v)
	}
	if v, ok := theme["success"]; ok && v != "" {
		ColorSuccess = lipgloss.Color(v)
		ColorTextSuccess = lipgloss.Color(v)
		ColorGraphRx = lipgloss.Color(v)
	}
	if v, ok := theme["warning"]; ok && v != "" {
		ColorWarning = lipgloss.Color(v)
		ColorTextWarning = lipgloss.Color(v)
	}
	if v, ok := theme["danger"]; ok && v != "" {
		ColorDanger = lipgloss.Color(v)
		ColorTextDanger = lipgloss.Color(v)
	}
	if v, ok := theme["muted"]; ok && v != "" {
		ColorMuted = lipgloss.Color(v)
		ColorTextMuted = lipgloss.Color(v)
		ColorBorderInactive = lipgloss.Color(v)
	}
	if v, ok := theme["tree"]; ok && v != "" {
		ColorTreeLine = lipgloss.Color(v)
	}

	// Specific text overrides
	if v, ok := theme["text_normal"]; ok && v != "" {
		ColorTextNormal = lipgloss.Color(v)
		ColorWhite = lipgloss.Color(v)
	}
	if v, ok := theme["text_muted"]; ok && v != "" {
		ColorTextMuted = lipgloss.Color(v)
	}
	if v, ok := theme["text_primary"]; ok && v != "" {
		ColorTextPrimary = lipgloss.Color(v)
	}
	if v, ok := theme["text_secondary"]; ok && v != "" {
		ColorTextSecondary = lipgloss.Color(v)
	}
	if v, ok := theme["text_success"]; ok && v != "" {
		ColorTextSuccess = lipgloss.Color(v)
	}
	if v, ok := theme["text_warning"]; ok && v != "" {
		ColorTextWarning = lipgloss.Color(v)
	}
	if v, ok := theme["text_danger"]; ok && v != "" {
		ColorTextDanger = lipgloss.Color(v)
	}

	// Gradient
	if v, ok := theme["grad_color"]; ok && v != "" {
		ColorGradBase = lipgloss.Color(v)
	}
	if v, ok := theme["grad_intensity"]; ok && v != "" {
		var intVal int
		if _, err := fmt.Sscanf(v, "%d", &intVal); err == nil && intVal >= 1 && intVal <= 10 {
			ThemeGradIntensity = intVal
		}
	}

	// Interface overrides
	if v, ok := theme["border_active"]; ok && v != "" {
		ColorBorderActive = lipgloss.Color(v)
	}
	if v, ok := theme["border_inactive"]; ok && v != "" {
		ColorBorderInactive = lipgloss.Color(v)
	}
	if v, ok := theme["graph_rx"]; ok && v != "" {
		ColorGraphRx = lipgloss.Color(v)
	}
	if v, ok := theme["graph_tx"]; ok && v != "" {
		ColorGraphTx = lipgloss.Color(v)
	}

	// Logs overrides
	if v, ok := theme["log_tab_active"]; ok && v != "" {
		ColorLogTabActive = lipgloss.Color(v)
	}
	if v, ok := theme["log_tab_inactive"]; ok && v != "" {
		ColorLogTabInactive = lipgloss.Color(v)
	}
	if v, ok := theme["log_info"]; ok && v != "" {
		ColorLogInfo = lipgloss.Color(v)
	}
	if v, ok := theme["log_warn"]; ok && v != "" {
		ColorLogWarn = lipgloss.Color(v)
	}
	if v, ok := theme["log_error"]; ok && v != "" {
		ColorLogError = lipgloss.Color(v)
	}
	if v, ok := theme["log_inbound"]; ok && v != "" {
		ColorLogInbound = lipgloss.Color(v)
	}
	if v, ok := theme["log_outbound"]; ok && v != "" {
		ColorLogOutbound = lipgloss.Color(v)
	}
	if v, ok := theme["log_meta"]; ok && v != "" {
		ColorLogMeta = lipgloss.Color(v)
	}

	ReloadStyles()
}

// GetCurrentThemeMap returns active colors in map form
func GetCurrentThemeMap() map[string]string {
	return map[string]string{
		"text_normal":         string(ColorTextNormal),
		"text_muted":          string(ColorTextMuted),
		"text_primary":        string(ColorTextPrimary),
		"text_secondary":      string(ColorTextSecondary),
		"text_success":        string(ColorTextSuccess),
		"text_warning":        string(ColorTextWarning),
		"text_danger":         string(ColorTextDanger),
		"grad_color":          string(ColorGradBase),
		"grad_intensity":      fmt.Sprintf("%d", ThemeGradIntensity),
		"bg":                  string(ColorBg),
		"selection":           string(ColorSelection),
		"primary":             string(ColorPrimary),
		"secondary":           string(ColorSecondary),
		"success":             string(ColorSuccess),
		"warning":             string(ColorWarning),
		"danger":              string(ColorDanger),
		"muted":               string(ColorMuted),
		"tree":                string(ColorTreeLine),
		"border_active":       string(ColorBorderActive),
		"border_inactive":     string(ColorBorderInactive),
		"graph_rx":            string(ColorGraphRx),
		"graph_tx":            string(ColorGraphTx),
		"log_tab_active":      string(ColorLogTabActive),
		"log_tab_inactive":    string(ColorLogTabInactive),
		"log_info":            string(ColorLogInfo),
		"log_warn":            string(ColorLogWarn),
		"log_error":           string(ColorLogError),
		"log_inbound":         string(ColorLogInbound),
		"log_outbound":        string(ColorLogOutbound),
		"log_meta":            string(ColorLogMeta),
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
