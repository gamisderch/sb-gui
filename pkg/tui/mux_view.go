package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderMuxView renders the flexible TCP/UDP Multiplexing (Mux) configuration screen
func (m *Model) RenderMuxView() string {
	width := m.width
	height := m.height - 2

	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	headerLabel := " TCP / UDP Multiplexing Studio (Mux & XUDP) "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	descStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	innerLines = append(innerLines, padLine(descStyle.Render("  Mux allows multiple virtual TCP/UDP connections to share a single physical connection, reducing handshake latency."), innerWidth))
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
		pill := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("[ ○ Disabled / Off ]")
		if selected {
			return StyleSelectedRow.Render("[ ○ Disabled / Off ]")
		}
		return pill
	}

	// Helper to render an item row with label on the left and control on the right
	renderRow := func(idx int, label string, control string) string {
		leftPadding := "    "
		isCurrent := (m.muxCursor == idx)

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

	// Helper to render a slider bar
	renderSlider := func(val int, max int) string {
		barLen := 16
		fill := (val * barLen) / max
		if fill > barLen {
			fill = barLen
		}
		if fill < 1 && val > 0 {
			fill = 1
		}
		empty := barLen - fill
		return fmt.Sprintf("[%s%s] %3d / %d", strings.Repeat("■", fill), strings.Repeat("□", empty), val, max)
	}

	// 1. Enable Mux
	item0 := renderRow(0, "Enable Multiplexing (Mux)", renderToggle(m.state.Mux.Enabled, m.muxCursor == 0))
	innerLines = append(innerLines, padLine(item0, innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 2. TCP Concurrency
	concStr := renderSlider(m.state.Mux.Concurrency, 128)
	item1 := renderRow(1, "TCP Concurrency (1-128)", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(concStr))
	innerLines = append(innerLines, padLine(item1, innerWidth))
	innerLines = append(innerLines, padLine(descStyle.Render("      Controls maximum concurrent TCP sub-streams per connection (Recommended: 8, use ←/→ ±1, H/L ±8)"), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 3. XUDP Concurrency
	xudpStr := renderSlider(m.state.Mux.XUDPConcurrency, 128)
	item2 := renderRow(2, "XUDP Concurrency (1-128)", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(xudpStr))
	innerLines = append(innerLines, padLine(item2, innerWidth))
	innerLines = append(innerLines, padLine(descStyle.Render("      Controls concurrent UDP-over-TCP channels for gaming/VoIP (Recommended: 16, use ←/→ ±1, H/L ±8)"), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 4. UDP 443 Policy
	policyVal := fmt.Sprintf("[ %s ] (↵/Space to cycle)", m.state.Mux.XUDPProxyUDP443)
	item3 := renderRow(3, "UDP 443 (QUIC / HTTP3) Policy", lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(policyVal))
	innerLines = append(innerLines, padLine(item3, innerWidth))
	innerLines = append(innerLines, padLine(descStyle.Render("      'reject' forces browser TCP fallback to avoid ISP UDP throttling; 'allow' permits QUIC; 'skip' bypasses"), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Live JSON Preview box
	previewHdr := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("  [ Live Generated Outbound Config ]")
	previewContent := fmt.Sprintf(`      "mux": { "enabled": %t, "concurrency": %d, "xudpConcurrency": %d, "xudpProxyUDP443": %q }`,
		m.state.Mux.Enabled, m.state.Mux.Concurrency, m.state.Mux.XUDPConcurrency, m.state.Mux.XUDPProxyUDP443)
	innerLines = append(innerLines, padLine(previewHdr, innerWidth))
	innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(previewContent), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Footer instructions
	hint := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  [↑/↓] Select   [←/→] ±1   [H/L] ±8   [↵/Space] Toggle/Cycle   [r] Reset Defaults   [Esc/q] Save & Back",
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
