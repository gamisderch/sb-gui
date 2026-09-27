package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type themeElement struct {
	ID          string
	Name        string
	Description string
	IsIntensity bool
	GetColor    func() string
	SetColor    func(hex string)
	GetIntVal   func() int
	SetIntVal   func(val int)
}

var themeSectionNames = []string{
	"Text & Gradient",
	"Interface",
	"Logs Viewer",
}

func getThemeElementsForSection(sec int) []themeElement {
	switch sec {
	case 0:
		return []themeElement{
			{
				ID:          "text_normal",
				Name:        "Main Text",
				Description: "Default content text across all screens (#ffffff)",
				GetColor:    func() string { return string(ColorTextNormal) },
				SetColor: func(hex string) {
					ColorTextNormal = lipgloss.Color(hex)
					ColorWhite = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "text_muted",
				Name:        "Muted / Hints",
				Description: "Secondary hints, key shortcuts, subtitle descriptions",
				GetColor:    func() string { return string(ColorTextMuted) },
				SetColor: func(hex string) {
					ColorTextMuted = lipgloss.Color(hex)
					ColorMuted = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "primary",
				Name:        "Primary Accent",
				Description: "Window titles, primary headings, and bold labels",
				GetColor:    func() string { return string(ColorPrimary) },
				SetColor: func(hex string) {
					ColorPrimary = lipgloss.Color(hex)
					ColorTextPrimary = lipgloss.Color(hex)
					ColorBorderActive = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "secondary",
				Name:        "Header / Column Text",
				Description: "Table column headers, section titles, and mode badges",
				GetColor:    func() string { return string(ColorSecondary) },
				SetColor: func(hex string) {
					ColorSecondary = lipgloss.Color(hex)
					ColorTextSecondary = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "success",
				Name:        "Success / Connected",
				Description: "Connected status, low ping latency, and checkmarks",
				GetColor:    func() string { return string(ColorSuccess) },
				SetColor: func(hex string) {
					ColorSuccess = lipgloss.Color(hex)
					ColorTextSuccess = lipgloss.Color(hex)
					ColorGraphRx = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "warning",
				Name:        "Warning Text",
				Description: "Medium latency ping, cautions, and alert notices",
				GetColor:    func() string { return string(ColorWarning) },
				SetColor: func(hex string) {
					ColorWarning = lipgloss.Color(hex)
					ColorTextWarning = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "danger",
				Name:        "Danger / Error Text",
				Description: "High latency ping, error banners, disconnected status",
				GetColor:    func() string { return string(ColorDanger) },
				SetColor: func(hex string) {
					ColorDanger = lipgloss.Color(hex)
					ColorTextDanger = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "grad_color",
				Name:        "Gradient Base Color",
				Description: "Highlight base color for focused and near-cursor rows",
				GetColor:    func() string { return string(ColorGradBase) },
				SetColor: func(hex string) {
					ColorGradBase = lipgloss.Color(hex)
				},
			},
			{
				ID:          "grad_intensity",
				Name:        "Gradient Intensity",
				Description: "Fade curve steepness from cursor (1 = subtle, 10 = sharp)",
				IsIntensity: true,
				GetIntVal:   func() int { return ThemeGradIntensity },
				SetIntVal: func(val int) {
					if val < 1 {
						val = 1
					}
					if val > 10 {
						val = 10
					}
					ThemeGradIntensity = val
				},
			},
		}

	case 1:
		return []themeElement{
			{
				ID:          "bg",
				Name:        "Canvas Background",
				Description: "Main canvas background (Pure Pitch Black #000000)",
				GetColor:    func() string { return string(ColorBg) },
				SetColor: func(hex string) {
					ColorBg = lipgloss.Color(hex)
					ColorPanelBg = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "selection",
				Name:        "Row Selection",
				Description: "Highlight background for selected rows in lists",
				GetColor:    func() string { return string(ColorSelection) },
				SetColor: func(hex string) {
					ColorSelection = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "border_active",
				Name:        "Active Box Borders",
				Description: "Border color for focused panels and dialog frames",
				GetColor:    func() string { return string(ColorBorderActive) },
				SetColor: func(hex string) {
					ColorBorderActive = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "border_inactive",
				Name:        "Inactive Box Borders",
				Description: "Border color for unfocused panels and divider lines",
				GetColor:    func() string { return string(ColorBorderInactive) },
				SetColor: func(hex string) {
					ColorBorderInactive = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "tree",
				Name:        "Tree & Scrollbars",
				Description: "Process tree branch lines and scrollbar tracks",
				GetColor:    func() string { return string(ColorTreeLine) },
				SetColor: func(hex string) {
					ColorTreeLine = lipgloss.Color(hex)
					ReloadStyles()
				},
			},
			{
				ID:          "graph_rx",
				Name:        "Graph RX (Download)",
				Description: "Download speed sparkline Braille graph in dashboard",
				GetColor:    func() string { return string(ColorGraphRx) },
				SetColor: func(hex string) {
					ColorGraphRx = lipgloss.Color(hex)
				},
			},
			{
				ID:          "graph_tx",
				Name:        "Graph TX (Upload)",
				Description: "Upload speed sparkline Braille graph in dashboard",
				GetColor:    func() string { return string(ColorGraphTx) },
				SetColor: func(hex string) {
					ColorGraphTx = lipgloss.Color(hex)
				},
			},
		}

	case 2:
		return []themeElement{
			{
				ID:          "log_tab_active",
				Name:        "Active Log Tab",
				Description: "Background highlight of currently selected log tab",
				GetColor:    func() string { return string(ColorLogTabActive) },
				SetColor: func(hex string) {
					ColorLogTabActive = lipgloss.Color(hex)
				},
			},
			{
				ID:          "log_tab_inactive",
				Name:        "Inactive Log Tab",
				Description: "Text color for unselected tabs in Logs Viewer",
				GetColor:    func() string { return string(ColorLogTabInactive) },
				SetColor: func(hex string) {
					ColorLogTabInactive = lipgloss.Color(hex)
				},
			},
			{
				ID:          "log_info",
				Name:        "Log [INFO] / Connected",
				Description: "Syntax highlight for [INFO], connected, and active notices",
				GetColor:    func() string { return string(ColorLogInfo) },
				SetColor: func(hex string) {
					ColorLogInfo = lipgloss.Color(hex)
				},
			},
			{
				ID:          "log_warn",
				Name:        "Log [WARN] / Warning",
				Description: "Syntax highlight for [WARN] and warning messages",
				GetColor:    func() string { return string(ColorLogWarn) },
				SetColor: func(hex string) {
					ColorLogWarn = lipgloss.Color(hex)
				},
			},
			{
				ID:          "log_error",
				Name:        "Log [ERROR] / Failed",
				Description: "Syntax highlight for errors, failures, and fatal lines",
				GetColor:    func() string { return string(ColorLogError) },
				SetColor: func(hex string) {
					ColorLogError = lipgloss.Color(hex)
				},
			},
			{
				ID:          "log_inbound",
				Name:        "Log Inbound (TUN)",
				Description: "Syntax highlight for inbound/tun and local routing",
				GetColor:    func() string { return string(ColorLogInbound) },
				SetColor: func(hex string) {
					ColorLogInbound = lipgloss.Color(hex)
				},
			},
			{
				ID:          "log_outbound",
				Name:        "Log Outbound (Proxy)",
				Description: "Syntax highlight for outbound/proxy and socks connections",
				GetColor:    func() string { return string(ColorLogOutbound) },
				SetColor: func(hex string) {
					ColorLogOutbound = lipgloss.Color(hex)
				},
			},
			{
				ID:          "log_meta",
				Name:        "Log Meta / Sources",
				Description: "Log file sources, line counts, timestamps, follow status",
				GetColor:    func() string { return string(ColorLogMeta) },
				SetColor: func(hex string) {
					ColorLogMeta = lipgloss.Color(hex)
				},
			},
		}
	}
	return nil
}

func getThemeElements() []themeElement {
	return getThemeElementsForSection(0)
}

func (m *Model) getActiveThemeElements() []themeElement {
	return getThemeElementsForSection(m.themeSection)
}

// RenderThemeView renders the Theme Studio with 3 sections, RGB sliders, intensity control and live preview
func (m *Model) RenderThemeView() string {
	width := m.width
	height := m.height - 2

	if width < 70 {
		return lipgloss.NewStyle().Background(ColorBg).Foreground(ColorWarning).Render("Terminal too narrow for Theme Studio. Please resize.")
	}
	if height < 12 {
		height = 12
	}

	leftWidth := (width - 1) * 42 / 100
	if leftWidth < 34 {
		leftWidth = 34
	}
	rightWidth := width - leftWidth - 1

	leftPanel := m.renderThemeElementsPanel(leftWidth, height)
	rightPanel := m.renderThemeAdjustPanel(rightWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, StyleBg.Render(" "), rightPanel)
}

func (m *Model) renderThemeElementsPanel(width, height int) string {
	innerWidth := width - 2
	borderStyle := lipgloss.NewStyle().Foreground(ColorBorderInactive).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	// Top border
	headerLabel := " Theme Studio "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 3 Section Tabs at top
	var tabBtns []string
	for i, name := range themeSectionNames {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		if i == m.themeSection {
			tabBtns = append(tabBtns, lipgloss.NewStyle().
				Foreground(ColorWhite).
				Background(ColorSelection).
				Bold(true).
				Render(" "+label+" "))
		} else {
			tabBtns = append(tabBtns, lipgloss.NewStyle().
				Foreground(ColorMuted).
				Background(ColorBg).
				Render(" "+label+" "))
		}
	}
	tabsRow := " " + strings.Join(tabBtns, " ")
	innerLines = append(innerLines, padLine(tabsRow, innerWidth))

	dividerLine := lipgloss.NewStyle().Foreground(ColorBorderInactive).Background(ColorBg).Render("├" + strings.Repeat("─", innerWidth) + "┤")
	innerLines = append(innerLines, dividerLine)

	elements := m.getActiveThemeElements()
	for i, elem := range elements {
		isSelected := (i == m.themeCursor)

		cursor := "  "
		if isSelected {
			cursor = "▶ "
		}

		var swatchStr string
		var hexStyled string
		if elem.IsIntensity {
			swatchStr = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(fmt.Sprintf("[%2d/10]", elem.GetIntVal()))
			hexStyled = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("curve  ")
		} else {
			hexColor := elem.GetColor()
			swatchStr = lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Background(ColorBg).Bold(true).Render("██")
			hexStyled = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(fmt.Sprintf("%-7s", hexColor))
		}

		nameStr := elem.Name
		availName := innerWidth - lipgloss.Width(cursor) - lipgloss.Width(swatchStr) - lipgloss.Width(hexStyled) - 4
		if availName > 4 && len(nameStr) > availName {
			nameStr = nameStr[:availName-3] + "..."
		}

		nameStyled := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(fmt.Sprintf("%-*s", availName, nameStr))
		if isSelected {
			nameStyled = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(fmt.Sprintf("%-*s", availName, nameStr))
		}

		tag := ""
		if isSelected && m.themeSliderActive {
			tag = lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render(" [EDIT]")
		}

		sp := lipgloss.NewStyle().Background(ColorBg).Render(" ")
		cursorStyled := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(cursor)
		rowContent := cursorStyled + swatchStr + sp + hexStyled + sp + nameStyled + tag
		innerLines = append(innerLines, padLine(rowContent, innerWidth))
	}

	// Instructions at bottom
	innerLines = append(innerLines, emptyBlackLine(innerWidth))
	hint0 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [Tab/1..3] Switch section")
	hint1 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [↵/Space]  Edit RGB / Value")
	hint2 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [r]        Reset item to default")
	hint3 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [R]        Factory reset ALL")
	hint4 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [s]        Save theme to disk")
	innerLines = append(innerLines, padLine(hint0, innerWidth), padLine(hint1, innerWidth), padLine(hint2, innerWidth), padLine(hint3, innerWidth), padLine(hint4, innerWidth))

	// Pad
	for len(innerLines) < height-2 {
		innerLines = append(innerLines, emptyBlackLine(innerWidth))
	}
	if len(innerLines) > height-2 {
		innerLines = innerLines[:height-2]
	}

	var allLines []string
	allLines = append(allLines, topLine)
	for _, l := range innerLines {
		if strings.HasPrefix(l, "├") && strings.HasSuffix(l, "┤") {
			allLines = append(allLines, l)
		} else {
			allLines = append(allLines, borderStyle.Render("│")+l+borderStyle.Render("│"))
		}
	}
	allLines = append(allLines, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))

	return strings.Join(allLines, "\n")
}

func (m *Model) renderThemeAdjustPanel(width, height int) string {
	innerWidth := width - 2
	borderStyle := lipgloss.NewStyle().Foreground(ColorBorderActive).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	// Top border
	headerLabel := " RGB Adjust & Live Preview "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	elements := m.getActiveThemeElements()
	var currentElem themeElement
	if m.themeCursor >= 0 && m.themeCursor < len(elements) {
		currentElem = elements[m.themeCursor]
	}

	if m.themeSliderActive {
		if currentElem.IsIntensity {
			// Gradient intensity slider
			lblEditing := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Editing: ")
			sp := lipgloss.NewStyle().Background(ColorBg).Render("  ")
			hdr := lblEditing +
				lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(currentElem.Name) +
				sp +
				lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render(fmt.Sprintf("%d / 10", m.themeIntensity))
			innerLines = append(innerLines, padLine(hdr, innerWidth))
			desc := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  " + currentElem.Description)
			innerLines = append(innerLines, padLine(desc, innerWidth))
			innerLines = append(innerLines, emptyBlackLine(innerWidth))

			sliderWidth := innerWidth - 20
			if sliderWidth < 10 {
				sliderWidth = 10
			}
			bracketOpen := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("[")
			bracketClose := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("]")
			sp1 := lipgloss.NewStyle().Background(ColorBg).Render(" ")
			curStyled := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render("▶ ")
			lbl := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render("Intensity: ")
			bar := renderIntensityBar(m.themeIntensity, 10, sliderWidth)
			valStr := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(fmt.Sprintf("%2d", m.themeIntensity))
			sliderLine := curStyled + lbl + sp1 + bracketOpen + bar + bracketClose + sp1 + valStr
			innerLines = append(innerLines, padLine(sliderLine, innerWidth))

			innerLines = append(innerLines, emptyBlackLine(innerWidth))
			ctrls := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  ←/→: ±1   Shift+←/→: ±3   r: Reset (5)   ↵/Esc: Done")
			innerLines = append(innerLines, padLine(ctrls, innerWidth))
			innerLines = append(innerLines, emptyBlackLine(innerWidth))

			// Preview of all 7 distance steps
			stepsHdr := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("  Gradient Decay Preview by Distance from Cursor:")
			innerLines = append(innerLines, padLine(stepsHdr, innerWidth))
			var stepsLine strings.Builder
			stepsLine.WriteString("  ")
			for d := 0; d <= 6; d++ {
				c := GetGradientColor(d)
				block := lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(ColorBg).Bold(true).Render(fmt.Sprintf("d%d:██ %s ", d, c))
				stepsLine.WriteString(block)
			}
			innerLines = append(innerLines, padLine(stepsLine.String(), innerWidth))
			innerLines = append(innerLines, emptyBlackLine(innerWidth))

		} else {
			// RGB Sliders
			curHex := currentElem.GetColor()
			swatchBig := lipgloss.NewStyle().Foreground(lipgloss.Color(curHex)).Background(ColorBg).Bold(true).Render("██████")
			lblEditing := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Editing: ")
			sp := lipgloss.NewStyle().Background(ColorBg).Render("  ")
			hdr := lblEditing +
				lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(currentElem.Name) +
				sp + swatchBig + sp +
				lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(curHex)
			innerLines = append(innerLines, padLine(hdr, innerWidth))
			desc := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  " + currentElem.Description)
			innerLines = append(innerLines, padLine(desc, innerWidth))
			innerLines = append(innerLines, emptyBlackLine(innerWidth))

			sliderWidth := innerWidth - 28
			if sliderWidth < 10 {
				sliderWidth = 10
			}

			bracketOpen := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("[")
			bracketClose := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("]")
			sp1 := lipgloss.NewStyle().Background(ColorBg).Render(" ")

			// Red slider
			rCur := "  "
			rStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
			if m.themeRGBIndex == 0 {
				rCur = "▶ "
				rStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5577")).Background(ColorBg).Bold(true)
			}
			rBar := renderSliderBar(m.themeR, sliderWidth, "#ff3355")
			rCurStyled := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(rCur)
			rVal := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(fmt.Sprintf("%3d", m.themeR))
			rLine := rCurStyled + rStyle.Render("Red:  ") + sp1 + bracketOpen + rBar + bracketClose + sp1 + rVal
			innerLines = append(innerLines, padLine(rLine, innerWidth))

			// Green slider
			gCur := "  "
			gStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
			if m.themeRGBIndex == 1 {
				gCur = "▶ "
				gStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ee880")).Background(ColorBg).Bold(true)
			}
			gBar := renderSliderBar(m.themeG, sliderWidth, "#39e68c")
			gCurStyled := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(gCur)
			gVal := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(fmt.Sprintf("%3d", m.themeG))
			gLine := gCurStyled + gStyle.Render("Green:") + sp1 + bracketOpen + gBar + bracketClose + sp1 + gVal
			innerLines = append(innerLines, padLine(gLine, innerWidth))

			// Blue slider
			bCur := "  "
			bStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
			if m.themeRGBIndex == 2 {
				bCur = "▶ "
				bStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#55ccff")).Background(ColorBg).Bold(true)
			}
			bBar := renderSliderBar(m.themeB, sliderWidth, "#44ccff")
			bCurStyled := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(bCur)
			bVal := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(fmt.Sprintf("%3d", m.themeB))
			bLine := bCurStyled + bStyle.Render("Blue: ") + sp1 + bracketOpen + bBar + bracketClose + sp1 + bVal
			innerLines = append(innerLines, padLine(bLine, innerWidth))

			innerLines = append(innerLines, emptyBlackLine(innerWidth))
			ctrls := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  ←/→: ±1   Shift+←/→: ±10   ↑/↓/Tab: Switch R/G/B   ↵/Esc: Done")
			innerLines = append(innerLines, padLine(ctrls, innerWidth))
			innerLines = append(innerLines, emptyBlackLine(innerWidth))
		}

		// Live Preview Mockup Box
		prevHdr := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("─ Live Preview (Updates in Real-Time) ───────────")
		innerLines = append(innerLines, padLine(prevHdr, innerWidth))

		mockW := innerWidth - 4
		if mockW < 24 {
			mockW = 24
		}
		mockBorder := lipgloss.NewStyle().Foreground(ColorBorderActive).Background(ColorBg)

		switch m.themeSection {
		case 0:
			// Live Preview for Text & Gradient
			mockFill := strings.Repeat("─", max(2, mockW-18))
			mockTop := mockBorder.Render("  ╭─ Text & Gradient " + mockFill + "╮")
			mockTitle := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s  %s  %s",
				StyleTitle.Render("Title Accent"),
				StyleHeader.Render("Header Cyan"),
				StyleMuted.Render("Muted Text"),
			), mockW-1) + mockBorder.Render("│")

			mockBadges := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s  %s  %s",
				StyleSuccess.Render("[DIRECT]"),
				StyleWarning.Render("[PROXY]"),
				StyleDanger.Render("[BLOCK]"),
			), mockW-1) + mockBorder.Render("│")

			mockGrad0 := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s %s",
				lipgloss.NewStyle().Foreground(lipgloss.Color(GetGradientColor(0))).Background(ColorBg).Bold(true).Render("● 🇵🇱 Poland Warsaw"),
				lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("(Cursor dist 0, 12ms)"),
			), mockW-1) + mockBorder.Render("│")

			mockGrad1 := mockBorder.Render("  │") + padLine(fmt.Sprintf("    %s %s",
				lipgloss.NewStyle().Foreground(lipgloss.Color(GetGradientColor(1))).Background(ColorBg).Render("🇩🇪 Germany Frankfurt"),
				lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("(dist 1, 28ms)"),
			), mockW-1) + mockBorder.Render("│")

			mockGrad2 := mockBorder.Render("  │") + padLine(fmt.Sprintf("    %s %s",
				lipgloss.NewStyle().Foreground(lipgloss.Color(GetGradientColor(2))).Background(ColorBg).Render("🇳🇱 Netherlands"),
				lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("(dist 2, 35ms)"),
			), mockW-1) + mockBorder.Render("│")

			mockGrad6 := mockBorder.Render("  │") + padLine(fmt.Sprintf("    %s %s",
				lipgloss.NewStyle().Foreground(lipgloss.Color(GetGradientColor(6))).Background(ColorBg).Render("🇺🇸 United States"),
				lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("(Floor shade dist 6+)"),
			), mockW-1) + mockBorder.Render("│")

			mockBot := mockBorder.Render("  ╰" + strings.Repeat("─", mockW-2) + "╯")
			innerLines = append(innerLines, padLine(mockTop, innerWidth), mockTitle, mockBadges, mockGrad0, mockGrad1, mockGrad2, mockGrad6, padLine(mockBot, innerWidth))

		case 1:
			// Live Preview for Interface Elements
			mockFill := strings.Repeat("─", max(2, mockW-16))
			mockTop := mockBorder.Render("  ╭─ Active Box " + mockFill + "╮")
			mockStatus := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s  %s",
				StyleSuccess.Render("● CONNECTED"),
				lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("Tunnel Active (02:15:30)"),
			), mockW-1) + mockBorder.Render("│")

			mockProc := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s%s %s",
				StyleTreeBranch.Render("├─ "),
				lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render("5421"),
				lipgloss.NewStyle().Foreground(lipgloss.Color(GetGradientColor(0))).Background(ColorBg).Bold(true).Render("deep-rock-galactic.exe"),
			), mockW-1) + mockBorder.Render("│")

			mockSel := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s",
				StyleSelectedRow.Render("▶ Selected Item Highlight Sample              "),
			), mockW-1) + mockBorder.Render("│")

			sparkW := max(10, mockW-26)
			rxSpk := Sparkline([]float64{1, 3, 5, 8, 4, 9, 7, 10, 6, 8}, sparkW, lipgloss.NewStyle().Foreground(ColorGraphRx).Background(ColorBg))
			txSpk := Sparkline([]float64{2, 4, 3, 5, 2, 6, 4, 5, 3, 4}, sparkW, lipgloss.NewStyle().Foreground(ColorGraphTx).Background(ColorBg))
			mockGraph := mockBorder.Render("  │") + padLine(fmt.Sprintf("  RX: %s  TX: %s", rxSpk, txSpk), mockW-1) + mockBorder.Render("│")

			mockBot := mockBorder.Render("  ╰" + strings.Repeat("─", mockW-2) + "╯")
			innerLines = append(innerLines, padLine(mockTop, innerWidth), mockStatus, mockProc, mockSel, mockGraph, padLine(mockBot, innerWidth))

		case 2:
			// Live Preview for Logs Viewer
			mockFill := strings.Repeat("─", max(2, mockW-14))
			mockTop := mockBorder.Render("  ╭─ Logs Viewer " + mockFill + "╮")
			tab1 := lipgloss.NewStyle().Foreground(ColorTextNormal).Background(ColorLogTabActive).Bold(true).Render(" [1] main log ")
			tab2 := lipgloss.NewStyle().Foreground(ColorLogTabInactive).Background(ColorBg).Render(" [2] core log ")
			mockTabs := mockBorder.Render("  │") + padLine(" "+tab1+"  "+tab2, mockW-1) + mockBorder.Render("│")

			mockMeta := mockBorder.Render("  │") + padLine(lipgloss.NewStyle().Foreground(ColorLogMeta).Background(ColorBg).Render(" /tmp/sb-cli/main.log │ Lines: 128 │ Follow: ON"), mockW-1) + mockBorder.Render("│")

			lInfo := lipgloss.NewStyle().Foreground(ColorLogInfo).Background(ColorBg).Render("[INFO]  Connected to remote server: 188.114.97.1")
			lWarn := lipgloss.NewStyle().Foreground(ColorLogWarn).Background(ColorBg).Render("[WARN]  High latency detected on gateway: 165ms")
			lErr := lipgloss.NewStyle().Foreground(ColorLogError).Background(ColorBg).Render("[ERROR] Handshake timeout to remote node")
			lTun := lipgloss.NewStyle().Foreground(ColorLogInbound).Background(ColorBg).Render("inbound/tun[tun0]: direct bypass for cs2.exe")
			lProxy := lipgloss.NewStyle().Foreground(ColorLogOutbound).Background(ColorBg).Render("outbound/proxy[vless]: dial tcp 1.2.3.4:443")

			mockL1 := mockBorder.Render("  │") + padLine("  "+lInfo, mockW-1) + mockBorder.Render("│")
			mockL2 := mockBorder.Render("  │") + padLine("  "+lWarn, mockW-1) + mockBorder.Render("│")
			mockL3 := mockBorder.Render("  │") + padLine("  "+lErr, mockW-1) + mockBorder.Render("│")
			mockL4 := mockBorder.Render("  │") + padLine("  "+lTun, mockW-1) + mockBorder.Render("│")
			mockL5 := mockBorder.Render("  │") + padLine("  "+lProxy, mockW-1) + mockBorder.Render("│")

			mockBot := mockBorder.Render("  ╰" + strings.Repeat("─", mockW-2) + "╯")
			innerLines = append(innerLines, padLine(mockTop, innerWidth), mockTabs, mockMeta, mockL1, mockL2, mockL3, mockL4, mockL5, padLine(mockBot, innerWidth))
		}

	} else {
		// Welcome / Section Palette Overview
		welcome := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render(fmt.Sprintf("  Section %d: %s Palette", m.themeSection+1, themeSectionNames[m.themeSection]))
		innerLines = append(innerLines, padLine(welcome, innerWidth))
		innerLines = append(innerLines, emptyBlackLine(innerWidth))

		step1 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  1. Use ↑/↓ to choose any item on the left.")
		step2 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  2. Press [Enter] or [Space] to activate its sliders.")
		step3 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  3. Adjust values: ←/→ (±1), Shift+←/→ (±10), [r] Reset.")
		step4 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  4. Press [Tab] or [1..3] to switch between sections.")
		step5 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  5. Press [R] (Shift+R) to reset ALL colors to factory defaults!")
		innerLines = append(innerLines, padLine(step1, innerWidth), padLine(step2, innerWidth), padLine(step3, innerWidth), padLine(step4, innerWidth), padLine(step5, innerWidth))

		innerLines = append(innerLines, emptyBlackLine(innerWidth))
		pTitle := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("─ Current Palette Preview ────────────────────────")
		innerLines = append(innerLines, padLine(pTitle, innerWidth))

		for _, elem := range elements {
			var sw, hexStr string
			if elem.IsIntensity {
				sw = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(fmt.Sprintf("[%2d/10]", elem.GetIntVal()))
				hexStr = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("curve  ")
			} else {
				sw = lipgloss.NewStyle().Foreground(lipgloss.Color(elem.GetColor())).Background(ColorBg).Bold(true).Render("████")
				hexStr = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(fmt.Sprintf("%-7s", elem.GetColor()))
			}
			descStr := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(elem.Description)
			sp := lipgloss.NewStyle().Background(ColorBg).Render("  ")
			sp1 := lipgloss.NewStyle().Background(ColorBg).Render(" ")
			line := sp + sw + sp1 + hexStr + sp + descStr
			innerLines = append(innerLines, padLine(line, innerWidth))
		}
	}

	// Pad
	for len(innerLines) < height-2 {
		innerLines = append(innerLines, emptyBlackLine(innerWidth))
	}
	if len(innerLines) > height-2 {
		innerLines = innerLines[:height-2]
	}

	var allLines []string
	allLines = append(allLines, topLine)
	for _, l := range innerLines {
		allLines = append(allLines, borderStyle.Render("│")+l+borderStyle.Render("│"))
	}
	allLines = append(allLines, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))

	return strings.Join(allLines, "\n")
}

func renderSliderBar(val, totalWidth int, hexColor string) string {
	if totalWidth < 4 {
		totalWidth = 4
	}
	fillCount := (val * totalWidth) / 255
	if fillCount > totalWidth {
		fillCount = totalWidth
	}
	emptyCount := totalWidth - fillCount
	if emptyCount < 0 {
		emptyCount = 0
	}

	fillStr := strings.Repeat("█", fillCount)
	emptyStr := strings.Repeat("░", emptyCount)

	styledFill := lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Background(ColorBg).Bold(true).Render(fillStr)
	styledEmpty := lipgloss.NewStyle().Foreground(ColorTreeLine).Background(ColorBg).Render(emptyStr)

	return styledFill + styledEmpty
}

func renderIntensityBar(val, maxVal, totalWidth int) string {
	if totalWidth < 4 {
		totalWidth = 4
	}
	if maxVal <= 0 {
		maxVal = 10
	}
	fillCount := (val * totalWidth) / maxVal
	if fillCount > totalWidth {
		fillCount = totalWidth
	}
	emptyCount := totalWidth - fillCount
	if emptyCount < 0 {
		emptyCount = 0
	}

	fillStr := strings.Repeat("█", fillCount)
	emptyStr := strings.Repeat("░", emptyCount)

	styledFill := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render(fillStr)
	styledEmpty := lipgloss.NewStyle().Foreground(ColorTreeLine).Background(ColorBg).Render(emptyStr)

	return styledFill + styledEmpty
}

func hexToRGB(hex string) (int, int, int) {
	return HexToRGB(hex)
}

func rgbToHex(r, g, b int) string {
	return RGBToHex(r, g, b)
}
