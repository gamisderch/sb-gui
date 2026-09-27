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
	GetColor    func() string
	SetColor    func(hex string)
}

func getThemeElements() []themeElement {
	return []themeElement{
		{
			ID:          "bg",
			Name:        "Background",
			Description: "Main canvas background (Pure Pitch Black)",
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
			ID:          "primary",
			Name:        "Primary Accent",
			Description: "Active box borders, titles, and highlight badges",
			GetColor:    func() string { return string(ColorPrimary) },
			SetColor: func(hex string) {
				ColorPrimary = lipgloss.Color(hex)
				ReloadStyles()
			},
		},
		{
			ID:          "secondary",
			Name:        "Secondary Accent",
			Description: "Cyan headers, column titles, and mode badges",
			GetColor:    func() string { return string(ColorSecondary) },
			SetColor: func(hex string) {
				ColorSecondary = lipgloss.Color(hex)
				ReloadStyles()
			},
		},
		{
			ID:          "success",
			Name:        "Success / Connected",
			Description: "Connected status, download graph, low latency ping",
			GetColor:    func() string { return string(ColorSuccess) },
			SetColor: func(hex string) {
				ColorSuccess = lipgloss.Color(hex)
				ReloadStyles()
			},
		},
		{
			ID:          "warning",
			Name:        "Warning / Medium",
			Description: "Medium latency ping, notifications, cautions",
			GetColor:    func() string { return string(ColorWarning) },
			SetColor: func(hex string) {
				ColorWarning = lipgloss.Color(hex)
				ReloadStyles()
			},
		},
		{
			ID:          "danger",
			Name:        "Danger / Disconnected",
			Description: "Disconnected indicator, high ping, errors",
			GetColor:    func() string { return string(ColorDanger) },
			SetColor: func(hex string) {
				ColorDanger = lipgloss.Color(hex)
				ReloadStyles()
			},
		},
		{
			ID:          "muted",
			Name:        "Muted / Inactive",
			Description: "Inactive box borders, subtle subtitles and hints",
			GetColor:    func() string { return string(ColorMuted) },
			SetColor: func(hex string) {
				ColorMuted = lipgloss.Color(hex)
				ReloadStyles()
			},
		},
		{
			ID:          "tree",
			Name:        "Tree Lines",
			Description: "Process tree branch lines and scrollbar track",
			GetColor:    func() string { return string(ColorTreeLine) },
			SetColor: func(hex string) {
				ColorTreeLine = lipgloss.Color(hex)
				ReloadStyles()
			},
		},
	}
}

// RenderThemeView renders the Theme & Color Studio
func (m *Model) RenderThemeView() string {
	width := m.width
	height := m.height - 2

	if width < 70 {
		return lipgloss.NewStyle().Background(ColorBg).Foreground(ColorWarning).Render("Terminal too narrow for Theme Studio. Please resize.")
	}
	if height < 12 {
		height = 12
	}

	leftWidth := (width - 1) * 38 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := width - leftWidth - 1

	leftPanel := m.renderThemeElementsPanel(leftWidth, height)
	rightPanel := m.renderThemeAdjustPanel(rightWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, StyleBg.Render(" "), rightPanel)
}

func (m *Model) renderThemeElementsPanel(width, height int) string {
	innerWidth := width - 2
	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	// Top border
	headerLabel := " Theme Elements "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	elements := getThemeElements()
	for i, elem := range elements {
		isSelected := (i == m.themeCursor)

		cursor := "  "
		if isSelected {
			cursor = "▶ "
		}

		hexColor := elem.GetColor()
		swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Background(ColorBg).Bold(true).Render("██")
		hexStyled := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(fmt.Sprintf("%-7s", hexColor))

		nameStr := elem.Name
		availName := innerWidth - lipgloss.Width(cursor) - lipgloss.Width(swatch) - lipgloss.Width(hexStyled) - 4
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
		rowContent := cursorStyled + swatch + sp + hexStyled + sp + nameStyled + tag
		innerLines = append(innerLines, padLine(rowContent, innerWidth))
	}

	// Instructions at bottom
	innerLines = append(innerLines, emptyBlackLine(innerWidth))
	hint1 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [↵] Adjust RGB sliders")
	hint2 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [r] Reset to default")
	hint3 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  [s] Save theme to disk")
	innerLines = append(innerLines, padLine(hint1, innerWidth), padLine(hint2, innerWidth), padLine(hint3, innerWidth))

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

func (m *Model) renderThemeAdjustPanel(width, height int) string {
	innerWidth := width - 2
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg)
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

	elements := getThemeElements()
	var currentElem themeElement
	if m.themeCursor >= 0 && m.themeCursor < len(elements) {
		currentElem = elements[m.themeCursor]
	}

	if m.themeSliderActive {
		// Active editing mode with 3 sliders and live preview
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

		// 3 RGB Sliders
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

		// Live Preview Mockup Box
		prevHdr := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("─ Live Preview (Updates in Real-Time) ───────────")
		innerLines = append(innerLines, padLine(prevHdr, innerWidth))

		mockW := innerWidth - 4
		if mockW < 20 {
			mockW = 20
		}
		mockBorder := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg)
		mockFill := strings.Repeat("─", max(2, mockW-16))
		mockTop := mockBorder.Render("  ╭─ Active Box " + mockFill + "╮")
		mockStatus := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s  %s",
			StyleSuccess.Render("● CONNECTED"),
			lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("Tunnel Active (02:15:30)"),
		), mockW-1) + mockBorder.Render("│")

		mockProc := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s%s %s",
			StyleTreeBranch.Render("├─ "),
			lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render("5421"),
			lipgloss.NewStyle().Foreground(lipgloss.Color(GetGreenGradient(0))).Background(ColorBg).Bold(true).Render("deep-rock-galactic.exe"),
		), mockW-1) + mockBorder.Render("│")

		mockSel := mockBorder.Render("  │") + padLine(fmt.Sprintf("  %s",
			StyleSelectedRow.Render("▶ Selected Item Highlight Sample              "),
		), mockW-1) + mockBorder.Render("│")

		mockBot := mockBorder.Render("  ╰" + strings.Repeat("─", mockW-2) + "╯")

		innerLines = append(innerLines, padLine(mockTop, innerWidth), mockStatus, mockProc, mockSel, padLine(mockBot, innerWidth))
	} else {
		// Welcome / Instructions mode
		welcome := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  Interactive RGB Theme Customizer")
		innerLines = append(innerLines, padLine(welcome, innerWidth))
		innerLines = append(innerLines, emptyBlackLine(innerWidth))

		step1 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  1. Use ↑/↓ to choose any theme element on the left.")
		step2 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  2. Press [Enter] to activate its 3 RGB sliders.")
		step3 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  3. Adjust Red, Green, and Blue values with ←/→ arrows.")
		step4 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  4. Every single tick updates the UI instantly in real-time!")
		innerLines = append(innerLines, padLine(step1, innerWidth), padLine(step2, innerWidth), padLine(step3, innerWidth), padLine(step4, innerWidth))

		innerLines = append(innerLines, emptyBlackLine(innerWidth))
		pTitle := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("─ Current Palette Preview ────────────────────────")
		innerLines = append(innerLines, padLine(pTitle, innerWidth))

		for _, elem := range elements {
			sw := lipgloss.NewStyle().Foreground(lipgloss.Color(elem.GetColor())).Background(ColorBg).Bold(true).Render("████")
			hexStr := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(fmt.Sprintf("%-7s", elem.GetColor()))
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

func hexToRGB(hex string) (int, int, int) {
	return HexToRGB(hex)
}

func rgbToHex(r, g, b int) string {
	return RGBToHex(r, g, b)
}
