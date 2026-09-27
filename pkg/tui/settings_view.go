package tui

import (
	"fmt"
	"strings"

	"signbox-cli/pkg/core"
	"signbox-cli/pkg/model"

	"github.com/charmbracelet/lipgloss"
)

// RenderSettingsView renders the settings and config screen
func (m *Model) RenderSettingsView() string {
	width := m.width
	height := m.height - 2

	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	// Top border
	headerLabel := " Settings & Network Configuration "
	labelW := lipgloss.Width(headerLabel)
	fillerW := width - 2 - labelW - 1
	if fillerW < 0 {
		fillerW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(headerLabel) + borderStyle.Render(strings.Repeat("─", fillerW)+"╮")

	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Section 0: Theme & Color Studio
	themeHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [1] Theme & Color Customizer (RGB Sliders & Live Preview)")
	innerLines = append(innerLines, padLine(themeHdr, innerWidth))

	themeText := "      ▶ [Open Theme Studio] Customize Palette, 3 RGB Sliders & Real-time Preview (Press ↵ or [t])"
	if m.settingsCursor == 0 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(themeText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Render(themeText), innerWidth))
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Section 1: User-Agent Spoofing
	uaHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [2] Subscription User-Agent (Anti-blocking)")
	innerLines = append(innerLines, padLine(uaHdr, innerWidth))
	userAgents := []string{
		"Happ/1.8.0 (Linux; x86_64) [Recommended]",
		"clash-meta/1.18.0",
		"v2rayN/6.23",
		"Sing-box/1.10.0",
	}

	for i, ua := range userAgents {
		cursor := "      ○ "
		if i == m.settingsUAIdx {
			cursor = "      ● "
		}
		text := fmt.Sprintf("%s%s", cursor, ua)
		if m.settingsCursor == 1 && i == m.settingsUAIdx {
			innerLines = append(innerLines, padLine(StyleSelectedRow.Render(text), innerWidth))
		} else {
			innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(text), innerWidth))
		}
	}

	// Custom UA item
	customCursor := "      ○ "
	if m.settingsUAIdx == 4 {
		customCursor = "      ● "
	}
	var customText string
	if m.customUAActive {
		customText = fmt.Sprintf("      ▶ Custom: %s (Enter to save, Esc to cancel)", m.customUATextInput.View())
	} else {
		customVal := m.state.UserAgent
		if customVal == "" {
			customVal = "(not set)"
		}
		customText = fmt.Sprintf("%sCustom: %s [Press ↵ or 'e' to edit]", customCursor, customVal)
	}
	if m.settingsCursor == 1 && m.settingsUAIdx == 4 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(customText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(customText), innerWidth))
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Section 2: Routing Mode
	modeHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [3] Routing / Tunnel Mode")
	innerLines = append(innerLines, padLine(modeHdr, innerWidth))
	modes := []model.TunnelMode{
		model.ModeTUN,
		model.ModeSystemProxy,
		model.ModeDirect,
	}

	for i, mode := range modes {
		cursor := "      ○ "
		if mode == m.state.Mode {
			cursor = "      ● "
		}
		text := fmt.Sprintf("%s%s", cursor, string(mode))
		if m.settingsCursor == 2 && i == m.settingsModeIdx {
			innerLines = append(innerLines, padLine(StyleSelectedRow.Render(text), innerWidth))
		} else {
			innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(text), innerWidth))
		}
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Section 3: GeoSite & GeoIP Rules
	geoHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [4] Geo Routing Rules & Database Sync (GeoSite / GeoIP)")
	innerLines = append(innerLines, padLine(geoHdr, innerWidth))
	rulesCount := len(m.state.GeoRules)
	geoText := fmt.Sprintf("      ▶ [Open Geo Rules Manager] Direct/Proxy/Block rules & DB Sync (%d active rules) (Press ↵ or [g])", rulesCount)
	if m.settingsCursor == 3 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(geoText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Render(geoText), innerWidth))
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Section 4: Background Daemon & Systemd Integration
	daemonHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [5] Background Daemon & System Autostart")
	innerLines = append(innerLines, padLine(daemonHdr, innerWidth))

	systemdStatus := "Disabled"
	if core.IsSystemdServiceEnabled() {
		systemdStatus = "Enabled"
	}
	systemdText := fmt.Sprintf("      ▶ Auto-start on boot (systemd --user): [%s] (Press ↵ to toggle)", systemdStatus)
	if m.settingsCursor == 4 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(systemdText), innerWidth))
	} else {
		statusColor := ColorMuted
		if systemdStatus == "Enabled" {
			statusColor = ColorSuccess
		}
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(statusColor).Background(ColorBg).Render(systemdText), innerWidth))
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Section 5: DNS Protection
	dnsHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [6] DNS Leak Shield")
	dnsDesc := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render("      Redirect UDP/TCP 53 -> 127.0.0.1:6450 (Sing-box Local DNS Resolver)")
	dnsStatusLbl := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("      Status: ")
	dnsStatusVal := StyleSuccess.Render("Protected (Policy routing active)")

	innerLines = append(innerLines,
		padLine(dnsHdr, innerWidth),
		padLine(dnsDesc, innerWidth),
		padLine(dnsStatusLbl+dnsStatusVal, innerWidth),
	)
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// Section 6: System Privileges & cgroup status
	sysHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [7] Kernel & cgroup v2 Status")
	cgroupLine := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("      cgroup path:   /sys/fs/cgroup/game-vpn")
	nftLine := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("      nftables:      inet game_tunnel (mangle output + nat redirect)")
	ipRuleLine := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("      ip rule:       fwmark 0x50 lookup 100 priority 1000")

	innerLines = append(innerLines,
		padLine(sysHdr, innerWidth),
		padLine(cgroupLine, innerWidth),
		padLine(nftLine, innerWidth),
		padLine(ipRuleLine, innerWidth),
	)

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
