package tui

import (
	"fmt"
	"strings"

	"signbox-cli/pkg/core"
	"signbox-cli/pkg/model"

	"github.com/charmbracelet/lipgloss"
)

// RenderSettingsView renders the redesigned settings screen with App Settings and Tunnel Settings sections
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

	// =========================================================================
	// SECTION 1: APP SETTINGS
	// =========================================================================
	appHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [1] APP SETTINGS")
	innerLines = append(innerLines, padLine(appHdr, innerWidth))

	// Item 0: Theme & Color Studio
	themeText := "      ▶ [Theme & Color Studio] Customize Palette, 3 RGB Sliders & Live Preview (Press ↵ or [t])"
	if m.settingsCursor == 0 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(themeText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Render(themeText), innerWidth))
	}

	// Item 1: Background Daemon & Systemd Autostart
	systemdStatus := "Disabled"
	if core.IsSystemdServiceEnabled() {
		systemdStatus = "Enabled"
	}
	systemdText := fmt.Sprintf("      ▶ [Auto-start on Boot (systemd --user)] Current: [%s] (Press ↵ to toggle)", systemdStatus)
	if m.settingsCursor == 1 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(systemdText), innerWidth))
	} else {
		statusColor := ColorMuted
		if systemdStatus == "Enabled" {
			statusColor = ColorSuccess
		}
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(statusColor).Background(ColorBg).Render(systemdText), innerWidth))
	}

	// Item 2: Subscription User-Agent
	var uaSummary string
	if m.customUAActive {
		uaSummary = fmt.Sprintf("      ▶ Custom UA: %s (Enter to save, Esc to cancel)", m.customUATextInput.View())
	} else {
		currentUA := m.state.UserAgent
		if currentUA == "" {
			currentUA = "Happ/1.8.0 (Linux; x86_64)"
		}
		uaSummary = fmt.Sprintf("      ▶ [Subscription User-Agent] %s [Press ↵ to cycle, 'e' to edit]", currentUA)
	}
	if m.settingsCursor == 2 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(uaSummary), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(uaSummary), innerWidth))
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// =========================================================================
	// SECTION 2: TUNNEL SETTINGS
	// =========================================================================
	tunnelHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Background(ColorBg).Render("  [2] TUNNEL SETTINGS")
	innerLines = append(innerLines, padLine(tunnelHdr, innerWidth))

	// Item 3: Routing / Tunnel Mode
	modeText := fmt.Sprintf("      ▶ [Tunnel Mode] Current: %s (Press ↵ or Space to cycle TUN/Proxy/Direct)", m.state.Mode)
	if m.settingsCursor == 3 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(modeText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(modeText), innerWidth))
	}

	// Item 4: Preferred IP Type
	ipTypeLabel := "Prefer IPv4 (Recommended)"
	switch m.state.IPType {
	case model.IPTypePreferIPv6:
		ipTypeLabel = "Prefer IPv6"
	case model.IPTypeIPv4Only:
		ipTypeLabel = "IPv4 Only"
	case model.IPTypeIPv6Only:
		ipTypeLabel = "IPv6 Only"
	}
	ipTypeText := fmt.Sprintf("      ▶ [Preferred IP Type] Strategy: %s (Press ↵ or Space to cycle)", ipTypeLabel)
	if m.settingsCursor == 4 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(ipTypeText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Render(ipTypeText), innerWidth))
	}

	// Item 5: GeoSite & GeoIP Rules
	rulesCount := len(m.state.GeoRules)
	subProfiles := len(m.state.Subscriptions)
	geoText := fmt.Sprintf("      ▶ [GeoSite & GeoIP Rules] Direct/Proxy/Block rules & DB Sync (%d global, %d subs) (Press ↵ or [g])", rulesCount, subProfiles)
	if m.settingsCursor == 5 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(geoText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Render(geoText), innerWidth))
	}

	// Item 6: TLS Packet Fragmentation
	fragStatus := "Disabled"
	if m.state.Fragmentation.Enabled {
		fragStatus = "Enabled"
	}
	fragText := fmt.Sprintf("      ▶ [TLS Packet Fragmentation] Status: [%s] (Packets: %s, Length: %s, Delay: %sms) [Space=toggle, ↵=configure]",
		fragStatus, m.state.Fragmentation.Packets, m.state.Fragmentation.Length, m.state.Fragmentation.Interval)
	if m.settingsCursor == 6 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(fragText), innerWidth))
	} else {
		statusColor := ColorMuted
		if m.state.Fragmentation.Enabled {
			statusColor = ColorSuccess
		}
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(statusColor).Background(ColorBg).Render(fragText), innerWidth))
	}

	// Item 7: TCP / UDP Multiplexing (Mux)
	muxStatus := "Disabled"
	if m.state.Mux.Enabled {
		muxStatus = "Enabled"
	}
	muxText := fmt.Sprintf("      ▶ [TCP/UDP Multiplexing (Mux)] Status: [%s] (Concurrency: %d, XUDP: %d, UDP443: %s) [Space=toggle, ↵=configure]",
		muxStatus, m.state.Mux.Concurrency, m.state.Mux.XUDPConcurrency, m.state.Mux.XUDPProxyUDP443)
	if m.settingsCursor == 7 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(muxText), innerWidth))
	} else {
		statusColor := ColorMuted
		if m.state.Mux.Enabled {
			statusColor = ColorSuccess
		}
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(statusColor).Background(ColorBg).Render(muxText), innerWidth))
	}

	// Item 8: Advanced Settings
	advText := "      ▶ [Advanced Settings] DNS Resolvers, Local DNS, JSON DNS & Inbound Sniffing (Press ↵ to open)"
	if m.settingsCursor == 8 {
		innerLines = append(innerLines, padLine(StyleSelectedRow.Render(advText), innerWidth))
	} else {
		innerLines = append(innerLines, padLine(lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBg).Bold(true).Render(advText), innerWidth))
	}
	innerLines = append(innerLines, emptyBlackLine(innerWidth))

	// System & Status line
	statusLine := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render(
		"  cgroup v2: /sys/fs/cgroup/game-vpn │ nftables: inet game_tunnel │ DNS Shield: UDP/TCP 53 -> DoH",
	)
	innerLines = append(innerLines, padLine(statusLine, innerWidth))

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
