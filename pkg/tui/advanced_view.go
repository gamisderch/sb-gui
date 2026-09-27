package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderAdvancedSettingsView renders the Advanced Settings screen matching the UI screenshot
func (m *Model) RenderAdvancedSettingsView() string {
	width := m.width
	height := m.height - 2

	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	headerLabel := " Advanced Tunnel & DNS Settings "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	descStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	innerLines = append(innerLines, padLine(descStyle.Render("  Configure low-level DNS resolution, bootstrap resolvers, and inbound traffic sniffing."), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Helper to render toggle switch
	renderToggle := func(on bool, selected bool) string {
		if on {
			pill := lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render("[ ● On  ]")
			if selected {
				return StyleSelectedRow.Render("[ ● On  ]")
			}
			return pill
		}
		pill := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("[ ○ Off ]")
		if selected {
			return StyleSelectedRow.Render("[ ○ Off ]")
		}
		return pill
	}

	// Helper to render an item row with label on the left and control on the right
	renderRow := func(idx int, label string, control string) string {
		leftPadding := "    "
		isCurrent := (m.advCursor == idx)

		labelStyle := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg)
		if isCurrent {
			labelStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true)
		}

		lbl := leftPadding + labelStyle.Render(label)
		lblW := lipgloss.Width(lbl)
		ctrlW := lipgloss.Width(control)

		spaceW := innerWidth - lblW - ctrlW - 4
		if spaceW < 1 {
			spaceW = 1
		}

		line := lbl + strings.Repeat(" ", spaceW) + control + "  "
		if isCurrent {
			return StyleSelectedRow.Render(line)
		}
		return line
	}

	// 1. Use local DNS
	item0 := renderRow(0, "Use local DNS", renderToggle(m.state.Advanced.UseLocalDNS, m.advCursor == 0))
	innerLines = append(innerLines, padLine(item0, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 2. Use DNS from JSON
	item1 := renderRow(1, "Use DNS from JSON", renderToggle(m.state.Advanced.UseDNSFromJSON, m.advCursor == 1))
	innerLines = append(innerLines, padLine(item1, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 3. Enable Resolve Server
	item2 := renderRow(2, "Enable Resolve Server", renderToggle(m.state.Advanced.EnableResolveServer, m.advCursor == 2))
	innerLines = append(innerLines, padLine(item2, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 4. DNS IP for resolve Server
	var ipVal string
	if m.advInputActive && m.advCursor == 3 {
		ipVal = fmt.Sprintf("[Edit: %s] (↵ save, Esc cancel)", m.advTextInput.View())
	} else {
		val := m.state.Advanced.ResolveServerIP
		if val == "" {
			val = "1.1.1.1"
		}
		ipVal = fmt.Sprintf("[ %s ]", val)
	}
	item3 := renderRow(3, "DNS IP for resolve Server", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(ipVal))
	innerLines = append(innerLines, padLine(item3, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 5. DNS domain for resolve Server
	var domainVal string
	if m.advInputActive && m.advCursor == 4 {
		domainVal = fmt.Sprintf("[Edit: %s] (↵ save, Esc cancel)", m.advTextInput.View())
	} else {
		val := m.state.Advanced.ResolveServerDomain
		if val == "" {
			val = "https://1.1.1.1/dns-query"
		}
		domainVal = fmt.Sprintf("[ %s ]", val)
	}
	item4 := renderRow(4, "DNS domain for resolve Server", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(domainVal))
	innerLines = append(innerLines, padLine(item4, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 6. Enable sniffing
	item5 := renderRow(5, "Enable sniffing", renderToggle(m.state.Advanced.EnableSniffing, m.advCursor == 5))
	innerLines = append(innerLines, padLine(item5, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Footer instructions
	hint := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  [↑/↓] Select option   [↵/Space] Toggle / Edit   [Esc/q] Back to Tunnel Settings",
	)
	innerLines = append(innerLines, padLine(hint, innerWidth))

	// Pad or truncate to fit height
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
