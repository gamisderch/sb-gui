package tui

import (
	"fmt"
	"strings"

	"signbox-cli/pkg/process"

	"github.com/charmbracelet/lipgloss"
)

// RenderProcView renders the 2-column btop-style process tree view
func (m *Model) RenderProcView() string {
	width := m.width
	height := m.height - 2 // Reserve 1 line for top bar, 1 line for footer

	if width < 70 {
		return lipgloss.NewStyle().Background(ColorBg).Foreground(ColorWarning).Render("Terminal width is too small for 2-column process view. Please resize.")
	}
	if height < 10 {
		height = 10
	}

	colWidth := (width - 1) / 2
	rightWidth := width - colWidth - 1

	leftColumn := m.renderTreeColumn("VPN (Default / All Traffic)", m.vpnTree, colWidth, height, !m.procRightFocused, false)
	rightColumn := m.renderTreeColumn("Direct (Bypass / No VPN)", m.bypassTree, rightWidth, height, m.procRightFocused, true)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, StyleBg.Render(" "), rightColumn)
}

func (m *Model) renderTreeColumn(title string, roots []*process.ProcessNode, width, height int, isFocused bool, isRightCol bool) string {
	var lines []string

	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Border & Title Styling
	borderColor := ColorMuted
	if isFocused {
		if isRightCol {
			borderColor = ColorSecondary
		} else {
			borderColor = ColorPrimary
		}
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(borderColor).Background(ColorBg).Bold(isFocused)

	searchIndicator := ""
	if isFocused && m.procFilterActive {
		searchIndicator = fmt.Sprintf(" [f %s_]", m.procFilterQuery)
	} else if m.procFilterQuery != "" {
		searchIndicator = fmt.Sprintf(" [f %s]", m.procFilterQuery)
	}

	// 1. Top Border with embedded Title and Filter
	headerLabel := fmt.Sprintf(" %s%s ", title, searchIndicator)
	labelW := lipgloss.Width(headerLabel)
	if labelW > width-4 {
		headerLabel = headerLabel[:max(4, width-7)] + "... "
		labelW = lipgloss.Width(headerLabel)
	}
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")
	lines = append(lines, topLine)

	// 2. Table Column Headers (btop style)
	rightMetaWidth := 20
	treeHeaderWidth := innerWidth - rightMetaWidth - 1
	if treeHeaderWidth < 10 {
		treeHeaderWidth = 10
	}
	headerText := fmt.Sprintf(" %-*s %-6s %4s %7s", treeHeaderWidth, "PID  Program", "User", "Conn", "Down/Up")
	if lipgloss.Width(headerText) > innerWidth {
		headerText = headerText[:innerWidth]
	}
	colHeaderLine := borderStyle.Render("│") +
		lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render(fmt.Sprintf("%-*s", innerWidth, headerText)) +
		borderStyle.Render("│")
	lines = append(lines, colHeaderLine)

	// Flatten process tree
	flatRows := m.flattenTree(roots, 0, "", isRightCol)
	totalItems := len(flatRows)

	visibleHeight := height - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Current cursor and scroll offset
	cursorIdx := m.procLeftIdx
	scrollOffset := m.procLeftScroll
	if isRightCol {
		cursorIdx = m.procRightIdx
		scrollOffset = m.procRightScroll
	}

	// Clamp boundaries
	if totalItems == 0 {
		cursorIdx = 0
		scrollOffset = 0
	} else {
		if cursorIdx >= totalItems {
			cursorIdx = totalItems - 1
		}
		if cursorIdx < 0 {
			cursorIdx = 0
		}
		maxScroll := totalItems - visibleHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if scrollOffset > maxScroll {
			scrollOffset = maxScroll
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
	}

	// Synchronize clamped state back to model so key handlers never see stale out-of-range indices
	if isRightCol {
		m.procRightIdx = cursorIdx
		m.procRightScroll = scrollOffset
		if totalItems > 0 && cursorIdx < totalItems {
			m.selectedRightPID = flatRows[cursorIdx].Node.PID
		}
	} else {
		m.procLeftIdx = cursorIdx
		m.procLeftScroll = scrollOffset
		if totalItems > 0 && cursorIdx < totalItems {
			m.selectedLeftPID = flatRows[cursorIdx].Node.PID
		}
	}

	// 3. Scrollbar Calculation (btop style thumb and track)
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
			thumbStart = (scrollOffset * maxThumbStart) / maxScroll
		}
		if thumbStart < 0 {
			thumbStart = 0
		}
		if thumbStart > maxThumbStart {
			thumbStart = maxThumbStart
		}
	}

	// 4. Content Rows
	rowWidth := innerWidth - 1 // Leave 1 char for right scrollbar
	if rowWidth < 10 {
		rowWidth = 10
	}

	for r := 0; r < visibleHeight; r++ {
		itemIdx := scrollOffset + r

		// Scrollbar char for this row
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
			row := flatRows[itemIdx]
			isSelected := isFocused && (itemIdx == cursorIdx)
			rowStr = m.formatProcRow(row, itemIdx, cursorIdx, rowWidth, isSelected)
		} else {
			// Empty filler line with solid black background
			rowStr = lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", rowWidth))
		}

		lines = append(lines, borderStyle.Render("│")+rowStr+scrollChar+borderStyle.Render("│"))
	}

	// 5. Bottom Border with [curr/total] counter
	counterStr := " [0/0] "
	if totalItems > 0 {
		counterStr = fmt.Sprintf(" [%d/%d] ", cursorIdx+1, totalItems)
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
	lines = append(lines, bottomLine)

	return strings.Join(lines, "\n")
}

type flatProcItem struct {
	Node   *process.ProcessNode
	Prefix string
	Depth  int
}

func (m *Model) flattenTree(nodes []*process.ProcessNode, depth int, prefix string, isTargetCol bool) []flatProcItem {
	var result []flatProcItem
	query := strings.ToLower(m.procFilterQuery)

	for i, node := range nodes {
		isLast := (i == len(nodes)-1)

		branch := "├─"
		nextPrefix := prefix + "│ "
		if isLast {
			branch = "└─"
			nextPrefix = prefix + "  "
		}
		if depth == 0 {
			hasChildren := len(node.Children) > 0 || (m.procMgr.Collapse && node.ChildCount > 1)
			if hasChildren {
				if m.procMgr.Collapse && (node.IsCollapsed || node.ChildCount > 1) {
					branch = "[+] "
				} else {
					branch = "[-] "
				}
			} else {
				branch = "    "
			}
			nextPrefix = "  "
		} else if depth >= 4 {
			branch = "├"
			nextPrefix = prefix + " "
			if isLast {
				branch = "└"
			}
		}

		fullPrefix := prefix + branch

		showThisNode := true
		if query != "" {
			showThisNode = nodeMatchesOrHasDescendantMatch(node, query)
		}

		if showThisNode {
			result = append(result, flatProcItem{
				Node:   node,
				Prefix: fullPrefix,
				Depth:  depth,
			})

			if len(node.Children) > 0 && (!m.procMgr.Collapse || !node.IsCollapsed) {
				childItems := m.flattenTree(node.Children, depth+1, nextPrefix, isTargetCol)
				result = append(result, childItems...)
			}
		}
	}

	return result
}

func nodeMatchesOrHasDescendantMatch(node *process.ProcessNode, query string) bool {
	if strings.Contains(strings.ToLower(node.CleanName), query) ||
		strings.Contains(fmt.Sprintf("%d", node.PID), query) ||
		strings.Contains(strings.ToLower(node.User), query) {
		return true
	}
	for _, ch := range node.Children {
		if nodeMatchesOrHasDescendantMatch(ch, query) {
			return true
		}
	}
	return false
}

func (m *Model) formatProcRow(item flatProcItem, itemIdx, cursorIdx int, rowWidth int, isSelected bool) string {
	node := item.Node

	displayName := node.CleanName
	if m.procMgr.Collapse && node.ChildCount > 1 {
		displayName = fmt.Sprintf("%s (%d)", node.CleanName, node.ChildCount)
	}

	isOffline := (node.PID <= 0)
	pidStr := fmt.Sprintf("%-6d", node.PID)
	if isOffline {
		pidStr = "  -   "
		displayName = node.CleanName + " [saved]"
	}

	// Columns on the right: User(6) + Conns(4) + Speed(7) + 3 spaces = 20 chars
	userWidth := 6
	connsWidth := 4
	speedWidth := 7
	rightMetaWidth := userWidth + connsWidth + speedWidth + 3

	availForTree := rowWidth - rightMetaWidth - 1
	if availForTree < 12 {
		availForTree = 12
	}

	// Wine prefix if running under Wine/Proton
	prefix := item.Prefix
	if node.IsWine {
		displayName = "🍷" + displayName
	}

	// Distance gradient for process name (btop green gradient)
	dist := Abs(cursorIdx - itemIdx)
	procGreenColor := lipgloss.Color(GetGreenGradient(dist))

	// Truncate process name if too long for tree column
	prefixW := lipgloss.Width(prefix)
	pidW := lipgloss.Width(pidStr)
	nameW := lipgloss.Width(displayName)

	if prefixW+pidW+nameW > availForTree {
		availName := availForTree - prefixW - pidW
		if availName < 6 && prefixW > 4 {
			prefix = ".." + prefix[prefixW-4:]
			prefixW = lipgloss.Width(prefix)
			availName = availForTree - prefixW - pidW
		}
		if availName > 3 && len(displayName) > availName {
			displayName = displayName[:availName-3] + "..."
			nameW = lipgloss.Width(displayName)
		}
	}

	treeW := prefixW + pidW + nameW
	treeSlack := availForTree - treeW
	if treeSlack < 0 {
		treeSlack = 0
	}

	// Format Right Metadata values
	userStr := node.User
	if len(userStr) > userWidth {
		userStr = userStr[:userWidth]
	}

	connVal := "-"
	if node.Sockets > 0 {
		connVal = fmt.Sprintf("%d", node.Sockets)
	}

	speedVal := "0/0"
	if node.DownRate != "0 B/s" || node.UpRate != "0 B/s" {
		speedVal = fmt.Sprintf("%s/%s", node.DownRate, node.UpRate)
	}
	if len(speedVal) > speedWidth {
		speedVal = speedVal[:speedWidth]
	}

	if isSelected {
		// Entire row highlighted in crimson wine (#501a24)
		selBg := ColorSelection
		sPrefix := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff99aa")).Background(selBg).Render(prefix)
		sPID := lipgloss.NewStyle().Foreground(ColorWhite).Background(selBg).Bold(true).Render(pidStr)
		sName := lipgloss.NewStyle().Foreground(lipgloss.Color("#69ff94")).Background(selBg).Bold(true).Render(displayName)
		sSlack := lipgloss.NewStyle().Background(selBg).Render(strings.Repeat(" ", treeSlack))
		sUser := lipgloss.NewStyle().Foreground(ColorWhite).Background(selBg).Render(fmt.Sprintf("%-*s", userWidth, userStr))
		sConn := lipgloss.NewStyle().Foreground(ColorWhite).Background(selBg).Bold(true).Render(fmt.Sprintf("%*s", connsWidth, connVal))
		sSpeed := lipgloss.NewStyle().Foreground(ColorWhite).Background(selBg).Bold(true).Render(fmt.Sprintf("%*s", speedWidth, speedVal))
		sp := lipgloss.NewStyle().Background(selBg).Render(" ")

		return lipgloss.NewStyle().Background(selBg).Render(" ") +
			sPrefix + sPID + sName + sSlack + sp + sUser + sp + sConn + sp + sSpeed
	}

	// Unselected row: Solid pitch black background (#000000)
	sPrefix := lipgloss.NewStyle().Foreground(ColorTreeLine).Background(ColorBg).Render(prefix)
	pidColor := ColorWhite
	nameColor := procGreenColor
	if isOffline {
		pidColor = ColorMuted
		nameColor = ColorMuted
	}
	sPID := lipgloss.NewStyle().Foreground(pidColor).Background(ColorBg).Render(pidStr)
	sName := lipgloss.NewStyle().Foreground(nameColor).Background(ColorBg).Render(displayName)
	sSlack := lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", treeSlack))

	sUser := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(fmt.Sprintf("%-*s", userWidth, userStr))

	connStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	if node.Sockets > 0 {
		connStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true)
	}
	sConn := connStyle.Render(fmt.Sprintf("%*s", connsWidth, connVal))

	speedStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	if node.DownRate != "0 B/s" || node.UpRate != "0 B/s" {
		speedStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg)
	}
	sSpeed := speedStyle.Render(fmt.Sprintf("%*s", speedWidth, speedVal))

	sp := lipgloss.NewStyle().Background(ColorBg).Render(" ")

	return lipgloss.NewStyle().Background(ColorBg).Render(" ") +
		sPrefix + sPID + sName + sSlack + sp + sUser + sp + sConn + sp + sSpeed
}
