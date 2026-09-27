package tui

import (
	"fmt"
	"strings"

	"signbox-cli/pkg/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type LogTab int

const (
	LogTabMain LogTab = iota
	LogTabCore
	LogTabTunnel
	LogTabSub
	LogTabDaemon
)

var logTabTitles = []string{
	"main log",
	"core log",
	"tunnel log",
	"subscribtion log",
	"daemon log",
}

var logTabSources = []string{
	"/tmp/sb-cli/main.log",
	"/tmp/sb-cli/xray.log",
	"/tmp/sb-cli/sing-box.log",
	"/tmp/sb-cli/subscription.log",
	"systemd (journalctl) / daemon.log",
}

// RenderLogsView renders the Logs Viewer body with 5 tabs and btop aesthetic
func (m *Model) RenderLogsView() string {
	width := m.width
	height := m.height - 2 // Reserve 1 line for top bar, 1 line for footer
	if width < 40 {
		width = 40
	}
	if height < 8 {
		height = 8
	}

	bg := ColorBg
	var lines []string

	// 1. Header border
	titleStr := " LOGS VIEWER "
	titleStyled := lipgloss.NewStyle().Foreground(ColorPrimary).Background(bg).Bold(true).Render(titleStr)
	availDash := width - lipgloss.Width(titleStr) - 4
	if availDash < 0 {
		availDash = 0
	}
	leftDashes := 3
	rightDashes := availDash - leftDashes
	if rightDashes < 0 {
		rightDashes = 0
	}
	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(bg)
	headerLine := borderStyle.Render("┌" + strings.Repeat("─", leftDashes)) +
		titleStyled +
		borderStyle.Render(strings.Repeat("─", rightDashes) + "┐")
	lines = append(lines, padLineWithBg(headerLine, width, bg))

	// 2. Tab Bar Line
	var tabBtns []string
	for i, name := range logTabTitles {
		tabNum := fmt.Sprintf("[%d] %s", i+1, name)
		if LogTab(i) == m.logActiveTab {
			tabBtns = append(tabBtns, lipgloss.NewStyle().
				Foreground(ColorTextNormal).
				Background(ColorLogTabActive).
				Bold(true).
				Render(" "+tabNum+" "))
		} else {
			tabBtns = append(tabBtns, lipgloss.NewStyle().
				Foreground(ColorLogTabInactive).
				Background(bg).
				Render(" "+tabNum+" "))
		}
	}
	tabsRow := " " + strings.Join(tabBtns, "  ")
	tabsW := lipgloss.Width(tabsRow)
	tabSlack := width - tabsW - 2
	if tabSlack < 0 {
		tabSlack = 0
	}
	tabLine := borderStyle.Render("│") +
		tabsRow +
		lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", tabSlack)) +
		borderStyle.Render("│")
	lines = append(lines, padLineWithBg(tabLine, width, bg))

	// 3. Subheader line (source, line count, follow state)
	tabLogs := m.getLogLinesForTab(m.logActiveTab)
	totalLogs := len(tabLogs)
	followStr := "OFF"
	if m.logAutoFollow[m.logActiveTab] {
		followStr = "ON"
	}
	subLeft := fmt.Sprintf(" %s ", logTabSources[m.logActiveTab])
	subRight := fmt.Sprintf(" Lines: %d │ Follow: %s [f] │ [Tab] Next Tab ", totalLogs, followStr)
	subLeftStyled := lipgloss.NewStyle().Foreground(ColorSecondary).Background(bg).Bold(true).Render(subLeft)
	subRightStyled := lipgloss.NewStyle().Foreground(ColorLogMeta).Background(bg).Render(subRight)
	subW := lipgloss.Width(subLeft) + lipgloss.Width(subRight)
	dashSlack := width - subW - 2
	if dashSlack < 0 {
		dashSlack = 0
	}
	subLine := borderStyle.Render("├") +
		subLeftStyled +
		borderStyle.Render(strings.Repeat("─", dashSlack)) +
		subRightStyled +
		borderStyle.Render("┤")
	lines = append(lines, padLineWithBg(subLine, width, bg))

	// 4. Log content viewport
	// Reserved vertical lines in body: headerLine(1), tabLine(1), subLine(1), bottomBorder(1), statusLine(1) = 5 lines
	viewportHeight := height - 5
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	// Calculate scroll offset (auto-follow scrolls to bottom)
	offset := m.logScrollOffsets[m.logActiveTab]
	if m.logAutoFollow[m.logActiveTab] {
		if totalLogs > viewportHeight {
			offset = totalLogs - viewportHeight
		} else {
			offset = 0
		}
		m.logScrollOffsets[m.logActiveTab] = offset
	} else {
		// Clamp offset
		maxOffset := totalLogs - viewportHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset < 0 {
			offset = 0
		}
		m.logScrollOffsets[m.logActiveTab] = offset
	}

	// Render log rows
	innerWidth := width - 3 // 1 char left border, 1 char right border/scrollbar, 1 char margin
	for i := 0; i < viewportHeight; i++ {
		lineIdx := offset + i
		var rowText string
		if lineIdx < totalLogs {
			rowText = m.formatLogLine(tabLogs[lineIdx], innerWidth)
		} else if totalLogs == 0 && i == 0 {
			emptyMsg := fmt.Sprintf(" [No log entries found for %s. Logs will stream here automatically.]", logTabTitles[m.logActiveTab])
			rowText = lipgloss.NewStyle().Foreground(ColorMuted).Background(bg).Render(emptyMsg)
		} else {
			rowText = ""
		}

		// Scrollbar indicator
		scrollChar := "│"
		if totalLogs > viewportHeight {
			thumbSize := max(1, (viewportHeight*viewportHeight)/totalLogs)
			maxScroll := totalLogs - viewportHeight
			thumbPos := 0
			if maxScroll > 0 {
				thumbPos = (offset * (viewportHeight - thumbSize)) / maxScroll
			}
			if i >= thumbPos && i < thumbPos+thumbSize {
				scrollChar = "█"
			}
		}

		scrollStyled := borderStyle.Render(scrollChar)
		rowW := lipgloss.Width(rowText)
		rSlack := innerWidth - rowW
		if rSlack < 0 {
			rSlack = 0
		}
		contentLine := borderStyle.Render("│ ") +
			rowText +
			lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", rSlack)) +
			scrollStyled
		lines = append(lines, padLineWithBg(contentLine, width, bg))
	}

	// 5. Bottom border
	bottomLine := borderStyle.Render("└" + strings.Repeat("─", width-2) + "┘")
	lines = append(lines, padLineWithBg(bottomLine, width, bg))

	// 6. Status notification line
	var statusLine string
	if m.statusMessage != "" {
		stColor := ColorSuccess
		if m.statusIsError {
			stColor = ColorDanger
		}
		statusLine = lipgloss.NewStyle().Foreground(stColor).Background(bg).Bold(true).Render(" " + m.statusMessage)
	} else {
		statusLine = lipgloss.NewStyle().Foreground(ColorMuted).Background(bg).Render(
			fmt.Sprintf(" Viewing %s (%s)", logTabTitles[m.logActiveTab], logTabSources[m.logActiveTab]),
		)
	}
	lines = append(lines, padLineWithBg(statusLine, width, bg))

	// Ensure exact height
	for len(lines) < height {
		lines = append(lines, padLineWithBg("", width, bg))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (m *Model) getLogLinesForTab(tab LogTab) []string {
	if m.logCache == nil {
		m.logCache = make(map[LogTab][]string)
	}

	// Always refresh logs for active tab
	switch tab {
	case LogTabMain:
		m.logCache[tab] = core.GetMainLogs(1000)
	case LogTabCore:
		m.logCache[tab] = core.GetCoreLogs(1000)
	case LogTabTunnel:
		m.logCache[tab] = core.GetTunnelLogs(1000)
	case LogTabSub:
		m.logCache[tab] = core.GetSubLogs(1000)
	case LogTabDaemon:
		m.logCache[tab] = core.GetDaemonLogs(1000)
	}
	return m.logCache[tab]
}

// formatLogLine applies syntax highlighting to log lines
func (m *Model) formatLogLine(raw string, maxWidth int) string {
	bg := ColorBg
	if maxWidth <= 0 {
		maxWidth = 80
	}

	clean := strings.TrimRight(raw, "\r\n")
	if len(clean) == 0 {
		return ""
	}

	// Truncate to fit screen if overly long
	if lipgloss.Width(clean) > maxWidth {
		clean = ansi.Truncate(clean, maxWidth-2, "") + ".."
	}

	lower := strings.ToLower(clean)

	// Determine log level color
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "failure") || strings.Contains(lower, "fatal") {
		return lipgloss.NewStyle().Foreground(ColorLogError).Background(bg).Render(clean)
	}
	if strings.Contains(lower, "warn") || strings.Contains(lower, "warning") {
		return lipgloss.NewStyle().Foreground(ColorLogWarn).Background(bg).Render(clean)
	}
	if strings.Contains(clean, "[INFO]") || strings.Contains(lower, "connected") || strings.Contains(lower, "active") {
		return lipgloss.NewStyle().Foreground(ColorLogInfo).Background(bg).Render(clean)
	}
	if strings.Contains(clean, "inbound/tun") {
		return lipgloss.NewStyle().Foreground(ColorLogInbound).Background(bg).Render(clean)
	}
	if strings.Contains(clean, "outbound/direct") {
		return lipgloss.NewStyle().Foreground(ColorLogOutbound).Background(bg).Render(clean)
	}
	if strings.Contains(clean, "outbound/socks") || strings.Contains(clean, "proxy") {
		return lipgloss.NewStyle().Foreground(ColorLogOutbound).Background(bg).Render(clean)
	}

	return lipgloss.NewStyle().Foreground(ColorTextNormal).Background(bg).Render(clean)
}

// handleLogsKeys handles keyboard input in ViewLogs
func (m *Model) handleLogsKeys(key string) (tea.Model, tea.Cmd) {
	tabCount := len(logTabTitles)

	switch key {
	case "tab":
		m.logActiveTab = LogTab((int(m.logActiveTab) + 1) % tabCount)
		return m, nil

	case "shift+tab":
		m.logActiveTab = LogTab((int(m.logActiveTab) + tabCount - 1) % tabCount)
		return m, nil

	case "1":
		m.logActiveTab = LogTabMain
		return m, nil
	case "2":
		m.logActiveTab = LogTabCore
		return m, nil
	case "3":
		m.logActiveTab = LogTabTunnel
		return m, nil
	case "4":
		m.logActiveTab = LogTabSub
		return m, nil
	case "5":
		m.logActiveTab = LogTabDaemon
		return m, nil

	case "left", "h":
		m.logActiveTab = LogTab((int(m.logActiveTab) + tabCount - 1) % tabCount)
		return m, nil

	case "right":
		m.logActiveTab = LogTab((int(m.logActiveTab) + 1) % tabCount)
		return m, nil

	case "up", "k":
		m.logAutoFollow[m.logActiveTab] = false
		if m.logScrollOffsets[m.logActiveTab] > 0 {
			m.logScrollOffsets[m.logActiveTab]--
		}
		return m, nil

	case "down", "j":
		lines := m.getLogLinesForTab(m.logActiveTab)
		maxOffset := len(lines) - 5
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.logScrollOffsets[m.logActiveTab] < maxOffset {
			m.logScrollOffsets[m.logActiveTab]++
		} else {
			m.logAutoFollow[m.logActiveTab] = true
		}
		return m, nil

	case "pgup":
		m.logAutoFollow[m.logActiveTab] = false
		m.logScrollOffsets[m.logActiveTab] -= 10
		if m.logScrollOffsets[m.logActiveTab] < 0 {
			m.logScrollOffsets[m.logActiveTab] = 0
		}
		return m, nil

	case "pgdown":
		m.logScrollOffsets[m.logActiveTab] += 10
		lines := m.getLogLinesForTab(m.logActiveTab)
		maxOffset := len(lines) - 5
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.logScrollOffsets[m.logActiveTab] >= maxOffset {
			m.logScrollOffsets[m.logActiveTab] = maxOffset
			m.logAutoFollow[m.logActiveTab] = true
		}
		return m, nil

	case "home", "g":
		m.logAutoFollow[m.logActiveTab] = false
		m.logScrollOffsets[m.logActiveTab] = 0
		return m, nil

	case "end", "G":
		m.logAutoFollow[m.logActiveTab] = true
		return m, nil

	case "f", "F":
		m.logAutoFollow[m.logActiveTab] = !m.logAutoFollow[m.logActiveTab]
		if m.logAutoFollow[m.logActiveTab] {
			m.statusMessage = "Auto-follow enabled (streaming live)"
		} else {
			m.statusMessage = "Auto-follow disabled (scrolling history)"
		}
		m.statusIsError = false
		return m, nil

	case "r", "R":
		// Clear cache to force reload
		m.logCache[m.logActiveTab] = nil
		_ = m.getLogLinesForTab(m.logActiveTab)
		m.statusMessage = fmt.Sprintf("Reloaded %s", logTabTitles[m.logActiveTab])
		m.statusIsError = false
		return m, nil

	case "c", "C":
		var err error
		switch m.logActiveTab {
		case LogTabMain:
			err = core.ClearLogFile("main.log")
		case LogTabCore:
			err = core.ClearLogFile("xray.log")
		case LogTabTunnel:
			err = core.ClearLogFile("sing-box.log")
		case LogTabSub:
			err = core.ClearLogFile("subscription.log")
		case LogTabDaemon:
			err = core.ClearLogFile("daemon.log")
		}
		if err != nil {
			m.statusMessage = fmt.Sprintf("Error clearing log: %v", err)
			m.statusIsError = true
		} else {
			m.logCache[m.logActiveTab] = nil
			m.logScrollOffsets[m.logActiveTab] = 0
			m.statusMessage = fmt.Sprintf("Cleared %s", logTabTitles[m.logActiveTab])
			m.statusIsError = false
		}
		return m, nil

	case "q", "esc", "l", "L":
		m.view = ViewMain
		return m, nil
	}

	return m, nil
}

// scrollLogsWheel handles mouse wheel scrolling inside logs
func (m *Model) scrollLogsWheel(delta int) {
	lines := m.getLogLinesForTab(m.logActiveTab)
	total := len(lines)
	if delta < 0 {
		m.logAutoFollow[m.logActiveTab] = false
		m.logScrollOffsets[m.logActiveTab] += delta
		if m.logScrollOffsets[m.logActiveTab] < 0 {
			m.logScrollOffsets[m.logActiveTab] = 0
		}
	} else if delta > 0 {
		maxOffset := total - 5
		if maxOffset < 0 {
			maxOffset = 0
		}
		m.logScrollOffsets[m.logActiveTab] += delta
		if m.logScrollOffsets[m.logActiveTab] >= maxOffset {
			m.logScrollOffsets[m.logActiveTab] = maxOffset
			m.logAutoFollow[m.logActiveTab] = true
		}
	}
}
