package tui

import (
	"fmt"
	"strings"
	"time"

	"signbox-cli/pkg/model"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// RenderMainView renders the main dashboard (Happ structure + btop aesthetics)
func (m *Model) RenderMainView() string {
	width := m.width
	height := m.height - 2 // Reserve 1 line for top bar, 1 line for footer

	if width < 60 {
		return lipgloss.NewStyle().Background(ColorBg).Foreground(ColorWarning).Render("Terminal too narrow. Please resize to at least 60 columns.")
	}
	if height < 10 {
		height = 10
	}

	leftWidth := (width - 1) / 2
	rightWidth := width - leftWidth - 1

	leftPanel := m.renderServersPanel(leftWidth, height)
	rightPanel := m.renderDashboardPanel(rightWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, StyleBg.Render(" "), rightPanel)
}

func (m *Model) renderServersPanel(width, height int) string {
	innerWidth := width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true)

	// Search bar indicator
	searchBar := ""
	if m.addInputActive {
		searchBar = fmt.Sprintf(" [Add: %s]", m.textInput.View())
	} else if m.mainFilterActive {
		searchBar = fmt.Sprintf(" [f %s_]", m.mainFilterQuery)
	} else if m.mainFilterQuery != "" {
		searchBar = fmt.Sprintf(" [f %s]", m.mainFilterQuery)
	}

	// 1. Top border
	headerLabel := fmt.Sprintf(" Servers & Subscriptions%s ", searchBar)
	if lipgloss.Width(headerLabel) > width-4 {
		headerLabel = headerLabel[:max(4, width-7)] + "... "
	}
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string

	nodes := m.getFilteredNodes()
	totalItems := len(nodes)

	// Subheader info: Show current group & traffic if selected, or filter/add hint
	subHeaderStr := "  [F] Filter  [A] Add Subscription/Node"
	if totalItems > 0 && m.selectedNodeIdx < totalItems {
		currNode := nodes[m.selectedNodeIdx]
		if sub := m.findSubscription(currNode.Group); sub != nil {
			subHeaderStr = fmt.Sprintf("  ▼ [%s] %s  [F] Filter", sub.Name, sub.FormatTraffic())
		} else if currNode.Group != "" {
			subHeaderStr = fmt.Sprintf("  ▼ [%s]  [F] Filter", currNode.Group)
		}
	}
	if lipgloss.Width(subHeaderStr) > innerWidth {
		subHeaderStr = ansi.Truncate(subHeaderStr, innerWidth-2, "") + ".."
	}
	subHeader := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render(subHeaderStr)
	innerLines = append(innerLines, padLine(subHeader, innerWidth))

	// Height available for nodes: height minus top border(1), subheader(1), bottom border(1)
	visibleHeight := height - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	m.clampNodeState()

	// Scrollbar calculation (same as proc_view)
	thumbSize := 0
	thumbStart := 0
	if totalItems > visibleHeight && visibleHeight > 0 {
		thumbSize = (visibleHeight * visibleHeight) / totalItems
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > visibleHeight-1 {
			thumbSize = visibleHeight - 1
		}
		maxThumbStart := visibleHeight - thumbSize
		maxScroll := totalItems - visibleHeight
		if maxScroll > 0 {
			thumbStart = (m.nodeScrollOffset * maxThumbStart) / maxScroll
		}
		if thumbStart < 0 {
			thumbStart = 0
		}
		if thumbStart > maxThumbStart {
			thumbStart = maxThumbStart
		}
	}

	rowWidth := innerWidth - 1 // 1 char for scrollbar
	if rowWidth < 10 {
		rowWidth = 10
	}

	for r := 0; r < visibleHeight; r++ {
		itemIdx := m.nodeScrollOffset + r

		// Scrollbar char
		var scrollChar string
		if totalItems > visibleHeight {
			if r >= thumbStart && r < thumbStart+thumbSize {
				scrollChar = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render("█")
			} else {
				scrollChar = lipgloss.NewStyle().Foreground(ColorTreeLine).Background(ColorBg).Render("│")
			}
		} else {
			scrollChar = lipgloss.NewStyle().Background(ColorBg).Render(" ")
		}

		var rowStr string
		if itemIdx < totalItems {
			node := nodes[itemIdx]
			isSelected := (itemIdx == m.selectedNodeIdx)
			rowStr = m.formatNodeRow(node, itemIdx, m.selectedNodeIdx, rowWidth, isSelected)
		} else {
			if totalItems == 0 && r == 0 {
				rowStr = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(fmt.Sprintf("%-*s", rowWidth, "  No servers. Press [a] to add."))
			} else {
				rowStr = lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", rowWidth))
			}
		}

		innerLines = append(innerLines, rowStr+scrollChar)
	}

	// Pad or truncate to exactly height - 2 lines
	for len(innerLines) < height-2 {
		innerLines = append(innerLines, emptyBlackLine(innerWidth))
	}
	if len(innerLines) > height-2 {
		innerLines = innerLines[:height-2]
	}

	// 2. Assemble rows with side borders
	var allLines []string
	allLines = append(allLines, topLine)
	for _, l := range innerLines {
		allLines = append(allLines, borderStyle.Render("│")+l+borderStyle.Render("│"))
	}

	// 3. Bottom border with [curr/total] counter (same as proc_view)
	counterStr := " [0/0] "
	if totalItems > 0 {
		counterStr = fmt.Sprintf(" [%d/%d] ", m.selectedNodeIdx+1, totalItems)
	}
	cW := lipgloss.Width(counterStr)
	botFiller := width - 2 - cW
	if botFiller < 2 {
		botFiller = 2
	}
	leftFiller := strings.Repeat("─", botFiller-2)
	bottomLine := borderStyle.Render("╰"+leftFiller) +
		lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(counterStr) +
		borderStyle.Render("──╯")
	allLines = append(allLines, bottomLine)

	return strings.Join(allLines, "\n")
}

func (m *Model) formatNodeRow(node *model.ProxyNode, itemIdx, cursorIdx, width int, isSelected bool) string {
	isActive := (m.state.ActiveNode != nil && m.state.ActiveNode.ID == node.ID)

	pingStr := node.FormatPing()
	pingW := 6
	protoStr := fmt.Sprintf("[%s]", node.Protocol)
	protoW := lipgloss.Width(protoStr)

	markerStr := "○"
	if isActive {
		markerStr = "●"
	} else if isSelected {
		markerStr = "❯"
	}
	markerW := lipgloss.Width(markerStr)

	// Available width for node name: width - (markerW + 1 + 1 + protoW + 1 + pingW)
	rightMetaW := protoW + 1 + pingW
	availNameW := width - markerW - 2 - rightMetaW
	if availNameW < 6 {
		availNameW = 6
	}

	name := node.Name
	if lipgloss.Width(name) > availNameW {
		name = ansi.Truncate(name, availNameW-2, "") + ".."
	}
	nameW := lipgloss.Width(name)
	nameSlack := availNameW - nameW
	if nameSlack < 0 {
		nameSlack = 0
	}

	if isSelected {
		selBg := ColorSelection
		sMarker := lipgloss.NewStyle().Foreground(ColorWhite).Background(selBg).Bold(true).Render(markerStr)
		if isActive {
			sMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Background(selBg).Bold(true).Render("●")
		}
		sName := lipgloss.NewStyle().Foreground(lipgloss.Color("#69ff94")).Background(selBg).Bold(true).Render(name)
		sSlack := lipgloss.NewStyle().Background(selBg).Render(strings.Repeat(" ", nameSlack))
		sProto := lipgloss.NewStyle().Foreground(ColorWhite).Background(selBg).Bold(true).Render(protoStr)

		pingStyle := lipgloss.NewStyle().Foreground(ColorWhite).Background(selBg).Bold(true)
		if node.Ping > 0 && node.Ping < 100 {
			pingStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Background(selBg).Bold(true)
		} else if node.Ping >= 100 && node.Ping < 250 {
			pingStyle = lipgloss.NewStyle().Foreground(ColorWarning).Background(selBg).Bold(true)
		} else if node.Ping == -2 || node.Ping >= 250 {
			pingStyle = lipgloss.NewStyle().Foreground(ColorDanger).Background(selBg).Bold(true)
		}
		sPing := pingStyle.Render(fmt.Sprintf("%*s", pingW, pingStr))
		sp := lipgloss.NewStyle().Background(selBg).Render(" ")

		return sp + sMarker + sp + sName + sSlack + sp + sProto + sp + sPing
	}

	// Unselected row: distance gradient on node name (same as btop proc_view)
	dist := Abs(cursorIdx - itemIdx)
	procGreenColor := lipgloss.Color(GetGreenGradient(dist))

	sMarker := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(markerStr)
	if isActive {
		sMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render("●")
	}
	sName := lipgloss.NewStyle().Foreground(procGreenColor).Background(ColorBg).Render(name)
	sSlack := lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", nameSlack))
	sProto := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Render(protoStr)

	pingStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	if node.Ping > 0 {
		if node.Ping < 100 {
			pingStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg)
		} else if node.Ping < 250 {
			pingStyle = lipgloss.NewStyle().Foreground(ColorWarning).Background(ColorBg)
		} else {
			pingStyle = lipgloss.NewStyle().Foreground(ColorDanger).Background(ColorBg)
		}
	} else if node.Ping == -2 {
		pingStr = "err"
		pingStyle = lipgloss.NewStyle().Foreground(ColorDanger).Background(ColorBg)
	}
	sPing := pingStyle.Render(fmt.Sprintf("%*s", pingW, pingStr))
	sp := lipgloss.NewStyle().Background(ColorBg).Render(" ")

	return sp + sMarker + sp + sName + sSlack + sp + sProto + sp + sPing
}

func (m *Model) renderDashboardPanel(width, height int) string {
	innerWidth := width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true)

	// Top border
	headerLabel := " Connection Dashboard "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Big status button (Happ style)
	if m.state.IsConnected {
		uptime := time.Since(m.state.ConnectedAt).Truncate(time.Second)
		b1 := StyleSuccess.Copy().Background(ColorBg).Bold(true).Render("  ╭────────────────────────╮")
		b2 := StyleSuccess.Copy().Background(ColorBg).Bold(true).Render("  │      ● CONNECTED       │")
		b3 := StyleSuccess.Copy().Background(ColorBg).Bold(true).Render(fmt.Sprintf("  │        %-14s  │", uptime.String()))
		b4 := StyleSuccess.Copy().Background(ColorBg).Bold(true).Render("  ╰────────────────────────╯")
		innerLines = append(innerLines, padLine(b1, innerWidth), padLine(b2, innerWidth), padLine(b3, innerWidth), padLine(b4, innerWidth))
	} else {
		b1 := StyleMuted.Copy().Background(ColorBg).Render("  ╭────────────────────────╮")
		b2 := StyleMuted.Copy().Background(ColorBg).Render("  │     ○ DISCONNECTED     │")
		b3 := StyleMuted.Copy().Background(ColorBg).Render("  │  Press [c] to connect  │")
		b4 := StyleMuted.Copy().Background(ColorBg).Render("  ╰────────────────────────╯")
		innerLines = append(innerLines, padLine(b1, innerWidth), padLine(b2, innerWidth), padLine(b3, innerWidth), padLine(b4, innerWidth))
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Mode & Active Node info
	lblMode := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Mode:        ")
	modeLine := lblMode + StyleSecondary.Copy().Background(ColorBg).Render(string(m.state.Mode))

	lblProxy := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Local Proxy: ")
	proxyLine := lblProxy + StyleSuccess.Copy().Background(ColorBg).Render("127.0.0.1:2080 (SOCKS5/HTTP)")
	innerLines = append(innerLines, padLine(modeLine, innerWidth), padLine(proxyLine, innerWidth))

	if m.state.ActiveNode != nil {
		node := m.state.ActiveNode
		lblNode := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Active Node: ")
		lblProto := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Protocol:    ")
		lblEnd := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Endpoint:    ")
		lblLat := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Latency:     ")

		pingVal := node.FormatPing()
		pingStyle := StyleMuted
		if node.Ping > 0 {
			if node.Ping < 100 {
				pingStyle = StyleSuccess
			} else if node.Ping < 250 {
				pingStyle = StyleWarning
			} else {
				pingStyle = StyleDanger
			}
		}

		innerLines = append(innerLines,
			padLine(lblNode+StyleTitle.Copy().Background(ColorBg).Render(node.Name), innerWidth),
			padLine(lblProto+StyleNormal.Render(node.Protocol), innerWidth),
			padLine(lblEnd+StyleNormal.Render(fmt.Sprintf("%s:%d", node.Address, node.Port)), innerWidth),
			padLine(lblLat+pingStyle.Render(pingVal), innerWidth),
		)
	} else {
		lblNode := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Active Node: ")
		innerLines = append(innerLines,
			padLine(lblNode+StyleMuted.Copy().Background(ColorBg).Render("None (Select and press [c])"), innerWidth),
		)
	}

	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Speed section header
	speedHeader := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("─ Real-time Tunnel Speed (RX / TX) ──────")
	innerLines = append(innerLines, padLine(speedHeader, innerWidth))

	// Sparkline graphs with EXACT black background
	sparkWidth := innerWidth - 22
	if sparkWidth < 6 {
		sparkWidth = 6
	}
	rxGraph := Sparkline(m.rxHistory, sparkWidth, StyleSuccess)
	txGraph := Sparkline(m.txHistory, sparkWidth, StyleSecondary)

	downLabel := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Down ")
	downSpeed := StyleSuccess.Copy().Background(ColorBg).Bold(true).Render(fmt.Sprintf("%9s", m.currentRxRate))
	sp := lipgloss.NewStyle().Background(ColorBg).Render(" ")

	upLabel := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Up   ")
	upSpeed := StyleSecondary.Copy().Background(ColorBg).Bold(true).Render(fmt.Sprintf("%9s", m.currentTxRate))

	downLine := downLabel + rxGraph + sp + downSpeed
	upLine := upLabel + txGraph + sp + upSpeed
	innerLines = append(innerLines, padLine(downLine, innerWidth), padLine(upLine, innerWidth))

	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Process Split-Tunneling status
	splitHeader := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("─ Process Split-Tunneling ───────────────")
	bypassCount := len(m.procMgr.BypassNames)
	lblBypass := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("  Processes in Direct (Bypass): ")
	valBypass := StylePrimary.Copy().Background(ColorBg).Render(fmt.Sprintf("%d", bypassCount))
	bypassLine := lblBypass + valBypass
	hintLine := StyleMuted.Copy().Background(ColorBg).Render("  Press [p] to view/manage process tree")
	innerLines = append(innerLines, padLine(splitHeader, innerWidth), padLine(bypassLine, innerWidth), padLine(hintLine, innerWidth))

	// Pad or truncate to exactly height - 2
	for len(innerLines) < height-2 {
		innerLines = append(innerLines, emptyBlackLine(innerWidth))
	}
	if len(innerLines) > height-2 {
		innerLines = innerLines[:height-2]
	}

	// Assemble rows with side borders
	var allLines []string
	allLines = append(allLines, topLine)
	for _, l := range innerLines {
		allLines = append(allLines, borderStyle.Render("│")+l+borderStyle.Render("│"))
	}

	// Bottom border
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")
	allLines = append(allLines, bottomLine)

	return strings.Join(allLines, "\n")
}

func padLine(content string, targetWidth int) string {
	w := lipgloss.Width(content)
	if w > targetWidth {
		content = ansi.Truncate(content, targetWidth, "")
		w = lipgloss.Width(content)
	}
	if w < targetWidth {
		content = content + lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", targetWidth-w))
	}
	return content
}

func emptyBlackLine(width int) string {
	return lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", width))
}

func (m *Model) getFilteredNodes() []*model.ProxyNode {
	var result []*model.ProxyNode
	query := strings.ToLower(m.mainFilterQuery)

	for _, sub := range m.state.Subscriptions {
		for _, node := range sub.Nodes {
			if query == "" || strings.Contains(strings.ToLower(node.Name), query) || strings.Contains(strings.ToLower(node.Protocol), query) {
				result = append(result, node)
			}
		}
	}
	return result
}

func (m *Model) findSubscription(group string) *model.Subscription {
	for _, sub := range m.state.Subscriptions {
		if sub.Name == group {
			return sub
		}
	}
	return nil
}
