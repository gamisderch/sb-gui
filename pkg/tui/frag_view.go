package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderFragView renders the flexible TLS Packet Fragmentation screen
func (m *Model) RenderFragView() string {
	width := m.width
	height := m.height - 2

	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	headerLabel := " TLS Packet Fragmentation Studio (Anti-DPI / Censorship Bypass) "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	descStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	innerLines = append(innerLines, padLine(descStyle.Render("  Splits initial TCP handshake packets (TLS ClientHello) into small fragments to prevent SNI detection by DPI firewalls."), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Helper to render toggle switch
	renderToggle := func(on bool, selected bool) string {
		if on {
			pill := lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render("[ ● Active / Enabled ]")
			if selected {
				return StyleSelectedRow.Render("[ ● Active / Enabled ]")
			}
			return pill
		}
		pill := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("[ ○ Disabled / Direct ]")
		if selected {
			return StyleSelectedRow.Render("[ ○ Disabled / Direct ]")
		}
		return pill
	}

	// Helper to render an item row with label on the left and control on the right
	renderRow := func(idx int, label string, control string) string {
		leftPadding := "    "
		isCurrent := (m.fragCursor == idx)

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

	// 1. Enable Fragmentation
	item0 := renderRow(0, "Enable Fragmentation", renderToggle(m.state.Fragmentation.Enabled, m.fragCursor == 0))
	innerLines = append(innerLines, padLine(item0, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 2. Packets Mode
	var packetsVal string
	if m.fragInputActive && m.fragCursor == 1 {
		packetsVal = fmt.Sprintf("[Edit: %s] (↵ save, Esc cancel)", m.fragTextInput.View())
	} else {
		packetsVal = fmt.Sprintf("[ %s ] (↵/Space cycle, 'e' edit)", m.state.Fragmentation.Packets)
	}
	item1 := renderRow(1, "Packets to Fragment", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(packetsVal))
	innerLines = append(innerLines, padLine(item1, innerWidth))
	innerLines = append(innerLines, padLine(descStyle.Render("      Options: 'tlshello' (Handshake SNI bypass - Recommended), '1-3' (First 3 TCP packets), '1-5'"), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 3. Fragment Length (bytes)
	var lengthVal string
	if m.fragInputActive && m.fragCursor == 2 {
		lengthVal = fmt.Sprintf("[Edit: %s] (↵ save, Esc cancel)", m.fragTextInput.View())
	} else {
		lengthVal = fmt.Sprintf("[ %s bytes ] (↵/Space cycle, 'e' edit)", m.state.Fragmentation.Length)
	}
	item2 := renderRow(2, "Fragment Length Range", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(lengthVal))
	innerLines = append(innerLines, padLine(item2, innerWidth))
	innerLines = append(innerLines, padLine(descStyle.Render("      Options: '100-200' (Balanced default), '50-100' (Medium chunks), '10-30' (Aggressive micro-chunks)"), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 4. Packet Interval (ms)
	var intervalVal string
	if m.fragInputActive && m.fragCursor == 3 {
		intervalVal = fmt.Sprintf("[Edit: %s] (↵ save, Esc cancel)", m.fragTextInput.View())
	} else {
		intervalVal = fmt.Sprintf("[ %s ms ] (↵/Space cycle, 'e' edit)", m.state.Fragmentation.Interval)
	}
	item3 := renderRow(3, "Delay Between Fragments", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(intervalVal))
	innerLines = append(innerLines, padLine(item3, innerWidth))
	innerLines = append(innerLines, padLine(descStyle.Render("      Options: '10-20' (Standard delay), '5-10' (Low latency), '1-5' (Fastest throughput)"), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Live JSON Preview box
	previewHdr := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("  [ Live Generated Outbound Config ]")
	previewContent := fmt.Sprintf(`      "tag": "fragment", "protocol": "freedom", "fragment": { "packets": %q, "length": %q, "interval": %q }`,
		m.state.Fragmentation.Packets, m.state.Fragmentation.Length, m.state.Fragmentation.Interval)
	innerLines = append(innerLines, padLine(previewHdr, innerWidth))
	innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(previewContent), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Footer instructions
	hint := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  [↑/↓] Select   [↵/Space] Toggle/Cycle   [e] Custom Input   [r] Reset Defaults   [Esc/q] Save & Back",
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
