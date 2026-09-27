package tui

import (
	"fmt"
	"strings"

	"signbox-cli/pkg/model"

	"github.com/charmbracelet/lipgloss"
)

// RenderGeoView renders the Geo Rules & DB management screen with full hierarchy:
// Level 1: Menu with Direct / Proxy / Block
// Level 2: Inside selected Action, 2 Tabs for GeoSite and GeoIP
// Level 3: Add Rule Picker Modal
func (m *Model) RenderGeoView() string {
	width := m.width
	height := m.height - 2

	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	// Top border
	headerLabel := " GeoSite & GeoIP Routing Profiles "
	if m.geoModeAdd {
		action := m.getCurrentGeoAction()
		headerLabel = fmt.Sprintf(" [%s] > Add Rules to %s ", m.getActiveProfileName(), strings.ToUpper(string(action)))
	} else if m.geoInActionView {
		action := m.getCurrentGeoAction()
		switch action {
		case model.GeoActionDirect:
			headerLabel = fmt.Sprintf(" [%s] > DIRECT (Bypass VPN) ", m.getActiveProfileName())
		case model.GeoActionProxy:
			headerLabel = fmt.Sprintf(" [%s] > PROXY (Route via VPN) ", m.getActiveProfileName())
		case model.GeoActionBlock:
			headerLabel = fmt.Sprintf(" [%s] > BLOCK (Drop Traffic) ", m.getActiveProfileName())
		}
	} else if m.geoInProfileView {
		headerLabel = fmt.Sprintf(" Routing Rules: [%s] ", m.getActiveProfileName())
	}
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 1. Database Status & Sync bar
	siteStat, ipStat := m.engine.GetGeoManager().GetDBStats(m.engine.GetSingboxBin())

	siteInfo := "GeoSite DB: "
	if siteStat.Exists {
		siteInfo += StyleSuccess.Render(fmt.Sprintf("%d categories (%.1f MB)", siteStat.TotalCount, float64(siteStat.Size)/1024/1024))
	} else {
		siteInfo += StyleWarning.Render("Not downloaded (built-in fallbacks)")
	}

	ipInfo := "GeoIP DB: "
	if ipStat.Exists {
		ipInfo += StyleSuccess.Render(fmt.Sprintf("%d countries (%.1f MB)", ipStat.TotalCount, float64(ipStat.Size)/1024/1024))
	} else {
		ipInfo += StyleWarning.Render("Not downloaded")
	}

	syncBtn := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("[u] Sync / Download Fresh DBs")
	if m.geoSyncing {
		syncBtn = StyleWarning.Render(fmt.Sprintf("⏳ %s", m.geoSyncMsg))
	}

	dbLine := fmt.Sprintf("  %s   │   %s   │   %s", siteInfo, ipInfo, syncBtn)
	innerLines = append(innerLines, padLine(dbLine, innerWidth))
	innerLines = append(innerLines, padLine(borderStyle.Render("  "+strings.Repeat("─", innerWidth-4)), innerWidth))
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// 2. Body: Add-Rule Modal/Picker (L3) OR Action View 2 tabs (L2) OR Action Menu (L1) OR Profile Menu (L0)
	if m.geoModeAdd {
		innerLines = append(innerLines, m.renderGeoAddPicker(innerWidth, height-8)...)
	} else if m.geoInActionView {
		innerLines = append(innerLines, m.renderGeoActionView(innerWidth, height-8)...)
	} else if m.geoInProfileView {
		innerLines = append(innerLines, m.renderGeoActionMenu(innerWidth, height-8)...)
	} else {
		innerLines = append(innerLines, m.renderGeoProfileMenu(innerWidth, height-8)...)
	}

	// Pad or truncate
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

// renderGeoProfileMenu renders Level 0: list of routing profiles (Global + Subscriptions)
func (m *Model) renderGeoProfileMenu(innerWidth, maxRows int) []string {
	var lines []string

	hdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render(
		"  Routing Profiles (Select Global or a Subscription to configure its GeoSite/GeoIP rules):",
	)
	lines = append(lines, padLine(hdr, innerWidth), emptyBlackLine(innerWidth))

	totalProfiles := len(m.state.Subscriptions) + 1
	if m.geoProfileIdx >= totalProfiles {
		m.geoProfileIdx = totalProfiles - 1
	}
	if m.geoProfileIdx < 0 {
		m.geoProfileIdx = 0
	}

	// [0] Global / Default Rules entry
	gSite, gIP := 0, 0
	for _, r := range m.state.GeoRules {
		if r.Type == model.GeoTypeSite {
			gSite++
		} else {
			gIP++
		}
	}
	globalCountBadge := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		fmt.Sprintf("(%d GeoSite, %d GeoIP)", gSite, gIP),
	)
	globalTag := StyleSuccess.Render("[Global Default]")

	globalTitle := fmt.Sprintf("  [0] %s  %-26s  %s", globalTag, "Global / Default Rules", globalCountBadge)
	if m.geoProfileIdx == 0 {
		globalTitle = fmt.Sprintf("  ▶ [0] %s  %-26s  %s", globalTag, "Global / Default Rules", globalCountBadge)
		lines = append(lines, padLine(StyleSelectedRow.Render(globalTitle), innerWidth))
	} else {
		lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(globalTitle), innerWidth))
	}
	globalDesc := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"        Routing rules applied by default to all connections and subscriptions without custom rules.",
	)
	lines = append(lines, padLine(globalDesc, innerWidth), emptyBlackLine(innerWidth))

	// Section 2: Subscriptions
	secDivider := padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  ── Subscriptions Profiles ──────────────────────────────────────────────────"), innerWidth)
	lines = append(lines, secDivider, emptyBlackLine(innerWidth))

	if len(m.state.Subscriptions) == 0 {
		noSub := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
			"    No subscriptions added yet. (Press [a] in Main view to add subscriptions)",
		)
		lines = append(lines, padLine(noSub, innerWidth), emptyBlackLine(innerWidth))
	} else {
		for i, sub := range m.state.Subscriptions {
			profIdx := i + 1
			ruleCount := len(sub.GeoRules)

			var countBadge string
			if ruleCount == 0 {
				countBadge = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("(empty / inherits Global)")
			} else {
				dCnt, pCnt, bCnt := 0, 0, 0
				for _, r := range sub.GeoRules {
					switch r.Action {
					case model.GeoActionDirect:
						dCnt++
					case model.GeoActionProxy:
						pCnt++
					case model.GeoActionBlock:
						bCnt++
					}
				}
				countBadge = lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render(
					fmt.Sprintf("(%d rules: %d DIRECT, %d PROXY, %d BLOCK)", ruleCount, dCnt, pCnt, bCnt),
				)
			}

			subTag := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("[Subscription]")
			subTitle := fmt.Sprintf("  [%d] %s  %-26s  %s", profIdx, subTag, sub.Name, countBadge)
			if profIdx == m.geoProfileIdx {
				subTitle = fmt.Sprintf("  ▶ [%d] %s  %-26s  %s", profIdx, subTag, sub.Name, countBadge)
				lines = append(lines, padLine(StyleSelectedRow.Render(subTitle), innerWidth))
			} else {
				lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(subTitle), innerWidth))
			}

			nodeInfo := fmt.Sprintf("%d servers", len(sub.Nodes))
			if sub.URL != "" {
				nodeInfo += " │ " + sub.URL
			}
			if len(nodeInfo) > innerWidth-12 {
				nodeInfo = nodeInfo[:innerWidth-15] + "..."
			}
			subDesc := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
				fmt.Sprintf("        %s", nodeInfo),
			)
			lines = append(lines, padLine(subDesc, innerWidth), emptyBlackLine(innerWidth))
		}
	}

	lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  "+strings.Repeat("─", innerWidth-4)), innerWidth))
	hintBar := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  [↑/↓] Select   [Enter / 0..9] Rules   [J/K] Move   [d/Del] Delete/Clear   [u] Sync DBs   [q/Esc] Back",
	)
	lines = append(lines, padLine(hintBar, innerWidth))

	return lines
}

// renderGeoActionMenu renders Level 1: menu with Direct, Proxy, Block options
func (m *Model) renderGeoActionMenu(innerWidth, maxRows int) []string {
	var lines []string

	hdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render(
		fmt.Sprintf("  Profile: [%s]  (Select a group to configure GeoSite and GeoIP tabs):", m.getActiveProfileName()),
	)
	lines = append(lines, padLine(hdr, innerWidth), emptyBlackLine(innerWidth))

	type actionMenuItem struct {
		action      model.GeoRuleAction
		num         string
		title       string
		description string
		badgeStyle  lipgloss.Style
	}

	items := []actionMenuItem{
		{
			action:      model.GeoActionDirect,
			num:         "1",
			title:       "DIRECT Rules (Bypass VPN)",
			description: "Traffic routes directly via physical network interface, bypassing VPN tunnel.",
			badgeStyle:  StyleSuccess,
		},
		{
			action:      model.GeoActionProxy,
			num:         "2",
			title:       "PROXY Rules (Route via VPN)",
			description: "Traffic routes encrypted through the active VPN proxy node.",
			badgeStyle:  lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true),
		},
		{
			action:      model.GeoActionBlock,
			num:         "3",
			title:       "BLOCK Rules (Block / Drop Traffic)",
			description: "Matched domains and IP addresses are terminated immediately (Ads / Tracker shield).",
			badgeStyle:  StyleDanger,
		},
	}

	for i, item := range items {
		siteCnt, ipCnt := m.getGeoActionCount(item.action)
		badge := item.badgeStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(string(item.action))))
		countStr := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
			fmt.Sprintf("(%d GeoSite categories, %d GeoIP countries)", siteCnt, ipCnt),
		)

		prefix := "    "
		titleLine := fmt.Sprintf("  [%s] %s  %-32s  %s", item.num, badge, item.title, countStr)
		if i == m.geoActionIdx {
			prefix = "  ▶ "
			titleLine = fmt.Sprintf("%s[%s] %s  %-32s  %s", prefix, item.num, badge, item.title, countStr)
			lines = append(lines, padLine(StyleSelectedRow.Render(titleLine), innerWidth))
		} else {
			lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(titleLine), innerWidth))
		}

		descLine := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
			fmt.Sprintf("        %s", item.description),
		)
		lines = append(lines, padLine(descLine, innerWidth))
		lines = append(lines, emptyBlackLine(innerWidth))
	}

	lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  "+strings.Repeat("─", innerWidth-4)), innerWidth))
	hintBar := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  [↑/↓] Select Group   [Enter / 1..3] Open Tabs   [u] Sync DBs   [q/Esc] Back to Profiles",
	)
	lines = append(lines, padLine(hintBar, innerWidth))

	return lines
}

// renderGeoActionView renders Level 2: inside selected action, 2 Tabs for GeoSite and GeoIP
func (m *Model) renderGeoActionView(innerWidth, maxRows int) []string {
	var lines []string

	action := m.getCurrentGeoAction()
	actionBadge := StyleSuccess.Render("[ DIRECT ]")
	actionTitle := "DIRECT Rules (Bypass VPN)"
	switch action {
	case model.GeoActionProxy:
		actionBadge = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("[  PROXY ]")
		actionTitle = "PROXY Rules (Route via VPN)"
	case model.GeoActionBlock:
		actionBadge = StyleDanger.Render("[  BLOCK ]")
		actionTitle = "BLOCK Rules (Block / Drop Traffic)"
	}

	siteCnt, ipCnt := m.getGeoActionCount(action)

	// Top action indicator
	titleLine := lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Background(ColorBg).Render(
		fmt.Sprintf("  Profile: %s   │   Group: %s %s", StylePrimary.Render(m.getActiveProfileName()), actionBadge, actionTitle),
	)
	lines = append(lines, padLine(titleLine, innerWidth))

	// 2 Tabs: GeoSite and GeoIP
	tabSite := fmt.Sprintf("○ [1] GeoSite (Domains) (%d active)", siteCnt)
	tabIP := fmt.Sprintf("○ [2] GeoIP (Country IPs) (%d active)", ipCnt)
	if m.geoTab == 0 {
		tabSite = fmt.Sprintf("● [1] GeoSite (Domains) (%d active)", siteCnt)
	} else {
		tabIP = fmt.Sprintf("● [2] GeoIP (Country IPs) (%d active)", ipCnt)
	}

	tabLine := fmt.Sprintf("  %s    %s    (Press [Tab] or [1]/[2] to switch tab)",
		lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render(tabSite),
		lipgloss.NewStyle().Foreground(ColorWarning).Background(ColorBg).Bold(true).Render(tabIP),
	)
	lines = append(lines, padLine(tabLine, innerWidth))
	lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  "+strings.Repeat("─", innerWidth-4)), innerWidth))

	// Table for the selected Tab
	currentType := model.GeoTypeSite
	typeLabel := "GeoSite"
	if m.geoTab == 1 {
		currentType = model.GeoTypeIP
		typeLabel = "GeoIP"
	}

	rules := m.getFilteredGeoRules(action, currentType)

	tblHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render(
		fmt.Sprintf("  %-4s %-12s %-34s %-15s", "#", "TYPE", "CATEGORY / TAG", "ROUTING ACTION"),
	)
	lines = append(lines, padLine(tblHdr, innerWidth))

	if len(rules) == 0 {
		lines = append(lines, emptyBlackLine(innerWidth))
		noRules1 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
			fmt.Sprintf("    No %s rules configured in %s yet.", typeLabel, actionBadge),
		)
		noRules2 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(
			fmt.Sprintf("    Press [a] to search & add %s rules from database.", typeLabel),
		)
		lines = append(lines, padLine(noRules1, innerWidth), padLine(noRules2, innerWidth))
	} else {
		total := len(rules)
		visRows := maxRows - 8
		if visRows < 5 {
			visRows = 5
		}

		if m.geoRuleCursor >= total {
			m.geoRuleCursor = total - 1
		}
		if m.geoRuleCursor < 0 {
			m.geoRuleCursor = 0
		}

		// Adjust scroll offset
		if m.geoRuleCursor < m.geoRuleScroll {
			m.geoRuleScroll = m.geoRuleCursor
		}
		if m.geoRuleCursor >= m.geoRuleScroll+visRows {
			m.geoRuleScroll = m.geoRuleCursor - visRows + 1
		}

		end := m.geoRuleScroll + visRows
		if end > total {
			end = total
		}

		for i := m.geoRuleScroll; i < end; i++ {
			r := rules[i]
			tBadge := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("[GeoSite]")
			if r.Type == model.GeoTypeIP {
				tBadge = lipgloss.NewStyle().Foreground(ColorWarning).Background(ColorBg).Bold(true).Render("[ GeoIP ]")
			}
			numStr := fmt.Sprintf("%3d.", i+1)
			nameStr := r.Name
			if len(nameStr) > 34 {
				nameStr = nameStr[:31] + "..."
			}

			rowText := fmt.Sprintf("  %-4s %s  %-34s %s", numStr, tBadge, nameStr, actionBadge)
			if i == m.geoRuleCursor {
				lines = append(lines, padLine(StyleSelectedRow.Render(rowText), innerWidth))
			} else {
				lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(rowText), innerWidth))
			}
		}

		if total > visRows {
			scrollInfo := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
				fmt.Sprintf("    Showing %d..%d of %d rules  (Press PgUp/PgDn to scroll)", m.geoRuleScroll+1, end, total),
			)
			lines = append(lines, padLine(scrollInfo, innerWidth))
		}
	}

	lines = append(lines, emptyBlackLine(innerWidth))
	hintBar := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  [a] Add Rule   [Tab] Switch Tab   [Space] Move Action   [Del / d] Delete   [q/Esc] Back to Menu",
	)
	lines = append(lines, padLine(hintBar, innerWidth))

	return lines
}

// renderGeoAddPicker renders Level 3: Add Rule Picker Modal with live search and green highlight
func (m *Model) renderGeoAddPicker(innerWidth, maxRows int) []string {
	var lines []string

	currAction := m.getCurrentGeoAction()
	currBadge := StyleSuccess.Render("[ DIRECT ]")
	switch currAction {
	case model.GeoActionProxy:
		currBadge = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render("[  PROXY ]")
	case model.GeoActionBlock:
		currBadge = StyleDanger.Render("[  BLOCK ]")
	}

	boxTitle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Background(ColorBg).Render(
		fmt.Sprintf("  ── Add Rules to [%s] > %s (Search Database) ──", m.getActiveProfileName(), currBadge),
	)
	lines = append(lines, padLine(boxTitle, innerWidth))

	// Tab switcher
	tabSite := "○ [1] GeoSite (Domains)"
	tabIP := "○ [2] GeoIP (Country IPs)"
	if m.geoTab == 0 {
		tabSite = "● [1] GeoSite (Domains)"
	} else {
		tabIP = "● [2] GeoIP (Country IPs)"
	}

	tabLine := fmt.Sprintf("  %s    %s    (Press [Tab] or [1]/[2] to switch tab)",
		lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render(tabSite),
		lipgloss.NewStyle().Foreground(ColorWarning).Background(ColorBg).Bold(true).Render(tabIP))
	lines = append(lines, padLine(tabLine, innerWidth))
	lines = append(lines, emptyBlackLine(innerWidth))

	// Search Input
	queryDisplay := m.geoSearchQuery
	if queryDisplay == "" {
		queryDisplay = "Type to search (e.g. ru, youtube, ads, steam, google)..."
	}
	searchPrompt := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Render("  Search: [ ")
	searchVal := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render(m.geoSearchQuery + "█")
	if m.geoSearchQuery == "" {
		searchVal = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Italic(true).Render(queryDisplay)
	}
	searchEnd := lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Render(" ]  ([Del] Clear)")
	lines = append(lines, padLine(searchPrompt+searchVal+searchEnd, innerWidth))

	// Action selector
	actionDirect := "○ DIRECT"
	actionProxy := "○ PROXY"
	actionBlock := "○ BLOCK"
	switch m.geoActionSelect {
	case model.GeoActionDirect:
		actionDirect = "● DIRECT"
	case model.GeoActionProxy:
		actionProxy = "● PROXY"
	case model.GeoActionBlock:
		actionBlock = "● BLOCK"
	}
	actionLine := fmt.Sprintf("  Target Action: %s   %s   %s   (Press [Space] to change action)",
		StyleSuccess.Render(actionDirect),
		lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render(actionProxy),
		StyleDanger.Render(actionBlock))
	lines = append(lines, padLine(actionLine, innerWidth))
	lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  "+strings.Repeat("─", innerWidth-4)), innerWidth))

	// Search results
	items := m.getPickerItems()
	total := len(items)

	visRows := maxRows - 6
	if visRows < 5 {
		visRows = 5
	}

	if total == 0 {
		lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("    No matching categories found."), innerWidth))
	} else {
		// Clamp cursor
		if m.geoSearchCursor >= total {
			m.geoSearchCursor = total - 1
		}
		if m.geoSearchCursor < 0 {
			m.geoSearchCursor = 0
		}

		// Adjust scroll
		if m.geoSearchCursor < m.geoSearchScroll {
			m.geoSearchScroll = m.geoSearchCursor
		}
		if m.geoSearchCursor >= m.geoSearchScroll+visRows {
			m.geoSearchScroll = m.geoSearchCursor - visRows + 1
		}

		end := m.geoSearchScroll + visRows
		if end > total {
			end = total
		}

		countHdr := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(fmt.Sprintf("    Showing %d..%d of %d items (Green [✓] are active):", m.geoSearchScroll+1, end, total))
		lines = append(lines, padLine(countHdr, innerWidth))

		for i := m.geoSearchScroll; i < end; i++ {
			item := items[i]
			ruleType := model.GeoTypeSite
			if m.geoTab == 1 {
				ruleType = model.GeoTypeIP
			}

			var existingRule *model.GeoRule
			for _, r := range m.getActiveProfileRules() {
				if r.Type == ruleType && strings.EqualFold(r.Name, item) {
					existingRule = r
					break
				}
			}

			if existingRule != nil {
				actionStr := fmt.Sprintf("[%s]", strings.ToUpper(string(existingRule.Action)))
				if i == m.geoSearchCursor {
					text := fmt.Sprintf("  ▶ %-30s  %-10s  ↵ Enter to change", item, actionStr)
					style := lipgloss.NewStyle().Background(ColorSelection).Foreground(ColorSuccess).Bold(true)
					lines = append(lines, padLine(style.Render(text), innerWidth))
				} else {
					text := fmt.Sprintf("  ✓ %-30s  %-10s", item, actionStr)
					style := lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true)
					lines = append(lines, padLine(style.Render(text), innerWidth))
				}
			} else {
				if i == m.geoSearchCursor {
					text := fmt.Sprintf("  ▶ %-30s  ↵ Enter to add", item)
					lines = append(lines, padLine(StyleSelectedRow.Render(text), innerWidth))
				} else {
					text := fmt.Sprintf("    %s", item)
					lines = append(lines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(text), innerWidth))
				}
			}
		}
	}

	lines = append(lines, emptyBlackLine(innerWidth))
	hintBar := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  [↑/↓] Select   [Enter] Add / Change Action   [Space] Cycle Action   [Tab] Site/IP   [Esc] Done",
	)
	lines = append(lines, padLine(hintBar, innerWidth))

	return lines
}

func (m *Model) getPickerItems() []string {
	var all []string
	if m.geoTab == 0 {
		cats, _ := m.engine.GetGeoManager().GetGeoSiteCategories(m.engine.GetSingboxBin())
		all = cats
	} else {
		ips, _ := m.engine.GetGeoManager().GetGeoIPCodes(m.engine.GetSingboxBin())
		all = ips
	}

	query := strings.ToLower(strings.TrimSpace(m.geoSearchQuery))
	if query == "" {
		return all
	}

	var filtered []string
	for _, item := range all {
		if strings.Contains(strings.ToLower(item), query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// getActiveProfileRules returns the slice of GeoRules for the currently selected profile
func (m *Model) getActiveProfileRules() []*model.GeoRule {
	if m.geoProfileIdx <= 0 || m.geoProfileIdx > len(m.state.Subscriptions) {
		return m.state.GeoRules
	}
	sub := m.state.Subscriptions[m.geoProfileIdx-1]
	return sub.GeoRules
}

// setActiveProfileRules sets the slice of GeoRules for the currently selected profile
func (m *Model) setActiveProfileRules(rules []*model.GeoRule) {
	if m.geoProfileIdx <= 0 || m.geoProfileIdx > len(m.state.Subscriptions) {
		m.state.GeoRules = rules
	} else {
		sub := m.state.Subscriptions[m.geoProfileIdx-1]
		sub.GeoRules = rules
	}
}

// getActiveProfileName returns the display name for the currently selected profile
func (m *Model) getActiveProfileName() string {
	if m.geoProfileIdx <= 0 || m.geoProfileIdx > len(m.state.Subscriptions) {
		return "Global / Default"
	}
	return m.state.Subscriptions[m.geoProfileIdx-1].Name
}

func (m *Model) getGeoActionCount(action model.GeoRuleAction) (siteCount, ipCount int) {
	for _, r := range m.getActiveProfileRules() {
		if r.Action == action {
			if r.Type == model.GeoTypeSite {
				siteCount++
			} else {
				ipCount++
			}
		}
	}
	return siteCount, ipCount
}

func (m *Model) getFilteredGeoRules(action model.GeoRuleAction, rType model.GeoRuleType) []*model.GeoRule {
	var list []*model.GeoRule
	for _, r := range m.getActiveProfileRules() {
		if r.Action == action && r.Type == rType {
			list = append(list, r)
		}
	}
	return list
}

func (m *Model) getCurrentGeoAction() model.GeoRuleAction {
	switch m.geoActionIdx {
	case 0:
		return model.GeoActionDirect
	case 1:
		return model.GeoActionProxy
	case 2:
		return model.GeoActionBlock
	default:
		return model.GeoActionDirect
	}
}

func (m *Model) removeGeoRule(target *model.GeoRule) {
	rules := m.getActiveProfileRules()
	for i, r := range rules {
		if r == target || (r.Type == target.Type && strings.EqualFold(r.Name, target.Name) && r.Action == target.Action) {
			rules = append(rules[:i], rules[i+1:]...)
			m.setActiveProfileRules(rules)
			break
		}
	}
}
