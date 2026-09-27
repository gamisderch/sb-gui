package tui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"signbox-cli/pkg/core"
	"signbox-cli/pkg/model"
	"signbox-cli/pkg/process"
	"signbox-cli/pkg/subscription"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type ViewType int

const (
	ViewMain ViewType = iota
	ViewProc
	ViewSettings
	ViewTheme
	ViewGeoRules
)

// Model is the main bubbletea state
type Model struct {
	view    ViewType
	width   int
	height  int
	state   *model.AppState
	procMgr *process.ProcessManager
	fetcher *subscription.Fetcher
	engine  *core.Engine

	// Status / Notification banner
	statusMessage string
	statusIsError bool

	// Add Subscription Text Input (with native paste support)
	textInput      textinput.Model
	addInputActive bool

	// Main View state
	selectedNodeIdx  int
	nodeScrollOffset int
	collapsedSubs    map[string]bool
	mainFilterActive bool
	mainFilterQuery  string
	rxHistory        []float64
	txHistory        []float64
	currentRxRate    string
	currentTxRate    string

	// Process View state
	vpnTree          []*process.ProcessNode
	bypassTree       []*process.ProcessNode
	procRightFocused bool
	procLeftIdx      int
	procRightIdx     int
	procLeftScroll   int // viewport scroll offset for left column
	procRightScroll  int // viewport scroll offset for right column
	selectedLeftPID  int // Frozen PID anchor (btop style)
	selectedRightPID int // Frozen PID anchor (btop style)
	procFilterActive bool
	procFilterQuery  string
	procTickCounter  int

	// Settings View state
	settingsCursor    int
	settingsUAIdx     int
	settingsModeIdx   int
	customUAActive    bool
	customUATextInput textinput.Model

	// Theme Studio state
	themeCursor       int  // Index in getThemeElements()
	themeSliderActive bool // true = adjusting RGB sliders, false = browsing elements
	themeRGBIndex     int  // 0 = R, 1 = G, 2 = B
	themeR            int  // Current Red (0..255)
	themeG            int  // Current Green (0..255)
	themeB            int  // Current Blue (0..255)

	// Geo Rules & Sync state
	geoInActionView bool // false = Level 1 (Direct/Proxy/Block menu), true = Level 2 (Action view)
	geoActionIdx    int  // 0 = Direct, 1 = Proxy, 2 = Block
	geoRuleCursor   int  // selected rule in Level 2 table
	geoCursor       int  // legacy
	geoModeAdd      bool
	geoTab          int // 0 = GeoSite, 1 = GeoIP
	geoSearchQuery  string
	geoSearchCursor int
	geoSearchScroll int
	geoSyncing      bool
	geoSyncMsg      string
	geoActionSelect model.GeoRuleAction

	// Confirmation modal state & background lifecycle
	confirmQuit     bool
	keepAliveOnExit bool
}


// NewModel initializes the TUI model
func NewModel(state *model.AppState, procMgr *process.ProcessManager, engine *core.Engine) *Model {
	ti := textinput.New()
	ti.Placeholder = "Paste URL (https://...) or direct link (vless://, ss://, etc.)"
	ti.Focus()
	ti.CharLimit = 2048
	ti.Width = 60

	uaInput := textinput.New()
	uaInput.Placeholder = "Enter custom User-Agent..."
	uaInput.CharLimit = 512
	uaInput.Width = 50

	m := &Model{
		view:              ViewMain,
		width:             100,
		height:            30,
		state:             state,
		procMgr:           procMgr,
		engine:            engine,
		fetcher:           subscription.NewFetcher(state.UserAgent),
		textInput:         ti,
		customUATextInput: uaInput,
		selectedNodeIdx:   0,
		nodeScrollOffset:  0,
		collapsedSubs:     make(map[string]bool),
		mainFilterActive:  false,
		rxHistory:         make([]float64, 30),
		txHistory:         make([]float64, 30),
		currentRxRate:     "0 B/s",
		currentTxRate:     "0 B/s",
		settingsUAIdx:     0,
		settingsModeIdx:   0,
		geoActionSelect:   model.GeoActionDirect,
		keepAliveOnExit:   false,
	}

	vpn, bypass, _ := procMgr.Refresh()
	m.vpnTree = vpn
	m.bypassTree = bypass

	if len(vpn) > 0 {
		m.selectedLeftPID = vpn[0].PID
	}
	if len(bypass) > 0 {
		m.selectedRightPID = bypass[0].PID
	}

	return m
}

type tickMsg time.Time
type pingResultMsg struct {
	node *model.ProxyNode
	ping int
}
type subFetchedMsg struct {
	sub *model.Subscription
	err error
}
type connectResultMsg struct {
	err error
}
type geoSyncDoneMsg struct {
	err error
}

func (m *Model) syncGeoDatabasesCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.engine.GetGeoManager().SyncDatabases(nil, nil)
		return geoSyncDoneMsg{err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, tickCmd())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.engine != nil {
			wasConnected := m.state.IsConnected
			m.state.IsConnected = m.engine.IsRunning()
			if m.state.IsConnected {
				if !wasConnected || m.state.ConnectedAt.IsZero() || time.Since(m.state.ConnectedAt) < 0 || time.Since(m.state.ConnectedAt) > 365*24*time.Hour {
					m.state.ConnectedAt = m.engine.GetConnectedAt()
				}
			}
		}

		if m.state.IsConnected {
			rx := float64(rand.Intn(3000) + 500)
			tx := float64(rand.Intn(500) + 50)
			m.rxHistory = append(m.rxHistory[1:], rx)
			m.txHistory = append(m.txHistory[1:], tx)
			m.currentRxRate = fmt.Sprintf("%.1f KB/s", rx/10)
			m.currentTxRate = fmt.Sprintf("%.1f KB/s", tx/10)
		} else {
			m.rxHistory = append(m.rxHistory[1:], 0)
			m.txHistory = append(m.txHistory[1:], 0)
			m.currentRxRate = "0 B/s"
			m.currentTxRate = "0 B/s"
		}

		// Refresh processes every 2 ticks (2 seconds) to avoid rapid jumping
		m.procTickCounter++
		if m.view == ViewProc && m.procTickCounter >= 2 {
			m.procTickCounter = 0
			m.refreshProcessesWithFrozenAnchor()
		}

		return m, tickCmd()

	case pingResultMsg:
		msg.node.Ping = msg.ping
		return m, nil

	case connectResultMsg:
		if msg.err != nil {
			if msg.err.Error() == "tun_perm_error" {
				m.statusMessage = "Connected (Proxy :2080). For TUN run: sudo setcap cap_net_admin=+ep ./bin/sing-box"
				m.statusIsError = false
				m.state.IsConnected = true
				m.state.ConnectedAt = time.Now()
			} else {
				m.statusMessage = fmt.Sprintf("Connect Error: %v", msg.err)
				m.statusIsError = true
				m.state.IsConnected = false
			}
		} else {
			m.statusMessage = fmt.Sprintf("Connected to %s", m.state.ActiveNode.Name)
			m.statusIsError = false
			m.state.IsConnected = true
			m.state.ConnectedAt = time.Now()
		}
		return m, nil

	case subFetchedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Subscription Error: %v", msg.err)
			m.statusIsError = true
		} else if msg.sub != nil {
			if len(msg.sub.Nodes) == 0 {
				m.statusMessage = fmt.Sprintf("Subscription Error: No valid nodes found in %s", msg.sub.Name)
				m.statusIsError = true
				return m, nil
			}
			m.state.Subscriptions = append(m.state.Subscriptions, msg.sub)
			m.statusMessage = fmt.Sprintf("Added %d nodes from %s", len(msg.sub.Nodes), msg.sub.Name)
			m.statusIsError = false
			if m.state.ActiveNode == nil && len(msg.sub.Nodes) > 0 {
				m.state.ActiveNode = msg.sub.Nodes[0]
			}
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		}
		return m, nil

	case geoSyncDoneMsg:
		m.geoSyncing = false
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Geo DB Sync Error: %v", msg.err)
			m.statusIsError = true
			m.geoSyncMsg = "Sync Failed"
		} else {
			m.statusMessage = "GeoSite and GeoIP databases successfully updated!"
			m.statusIsError = false
			m.geoSyncMsg = "Databases up to date"
			if m.state.IsConnected && m.engine != nil {
				_ = m.engine.UpdateGeoRules(m.state.GeoRules)
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		if m.view == ViewProc {
			colWidth := (m.width - 1) / 2
			isRight := (msg.X > colWidth)

			if msg.Button == tea.MouseButtonWheelUp {
				m.scrollWheel(isRight, -3)
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.scrollWheel(isRight, 3)
			} else if msg.Button == tea.MouseButtonLeft {
				m.procRightFocused = isRight
			}
			return m, nil
		} else if m.view == ViewMain {
			if msg.Button == tea.MouseButtonWheelUp {
				m.scrollNodeWheel(-3)
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.scrollNodeWheel(3)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) refreshProcessesWithFrozenAnchor() {
	vpn, bypass, _ := m.procMgr.Refresh()
	m.vpnTree = vpn
	m.bypassTree = bypass

	// Keep cursor FROZEN on selectedLeftPID (btop behavior)
	flatVPN := m.flattenTree(m.vpnTree, 0, "", false)
	foundLeft := false
	for idx, item := range flatVPN {
		if item.Node.PID == m.selectedLeftPID {
			m.procLeftIdx = idx
			foundLeft = true
			break
		}
	}
	if !foundLeft && len(flatVPN) > 0 {
		if m.procLeftIdx >= len(flatVPN) {
			m.procLeftIdx = len(flatVPN) - 1
		}
		m.selectedLeftPID = flatVPN[m.procLeftIdx].Node.PID
	}

	// Keep cursor FROZEN on selectedRightPID
	flatBypass := m.flattenTree(m.bypassTree, 0, "", true)
	foundRight := false
	for idx, item := range flatBypass {
		if item.Node.PID == m.selectedRightPID {
			m.procRightIdx = idx
			foundRight = true
			break
		}
	}
	if !foundRight && len(flatBypass) > 0 {
		if m.procRightIdx >= len(flatBypass) {
			m.procRightIdx = len(flatBypass) - 1
		}
		m.selectedRightPID = flatBypass[m.procRightIdx].Node.PID
	}
}

// Keyboard layout mapping to support Cyrillic/Russian/Ukrainian keyboard layouts seamlessly
var layoutMap = map[rune]string{
	'й': "q", 'Й': "Q",
	'ц': "w", 'Ц': "W",
	'у': "e", 'У': "E",
	'к': "r", 'К': "R",
	'е': "t", 'Е': "T",
	'н': "y", 'Н': "Y",
	'г': "u", 'Г': "U",
	'ш': "i", 'Ш': "I",
	'щ': "o", 'Щ': "O",
	'з': "p", 'З': "P",
	'х': "[", 'Х': "{",
	'ъ': "]", 'Ъ': "}",
	'ф': "a", 'Ф': "A",
	'ы': "s", 'Ы': "S",
	'в': "d", 'В': "D",
	'а': "f", 'А': "F",
	'п': "g", 'П': "G",
	'р': "h", 'Р': "H",
	'о': "j", 'О': "J",
	'л': "k", 'Л': "K",
	'д': "l", 'Д': "L",
	'ж': ";", 'Ж': ":",
	'э': "'", 'Э': "\"",
	'я': "z", 'Я': "Z",
	'ч': "x", 'Ч': "X",
	'с': "c", 'С': "C",
	'м': "v", 'М': "V",
	'и': "b", 'И': "B",
	'т': "n", 'Т': "N",
	'ь': "m", 'Ь': "M",
	'б': ",", 'Б': "<",
	'ю': ".", 'Ю': ">",
	'ё': "`", 'Ё': "~",
	'і': "s", 'І': "S",
	'ї': "]", 'Ї': "}",
	'є': "'", 'Є': "\"",
	'ґ': "g", 'Ґ': "G",
}

func normalizeKey(key string) string {
	r := []rune(key)
	if len(r) == 1 {
		if mapped, ok := layoutMap[r[0]]; ok {
			return mapped
		}
	}
	return key
}

func (m *Model) getNodeVisibleHeight() int {
	vis := m.height - 5
	if vis < 2 {
		vis = 2
	}
	return vis
}

func (m *Model) clampNodeState() {
	items := m.getServerListItems()
	total := len(items)
	vis := m.getNodeVisibleHeight()

	if total == 0 {
		m.selectedNodeIdx = 0
		m.nodeScrollOffset = 0
		return
	}

	if m.selectedNodeIdx >= total {
		m.selectedNodeIdx = total - 1
	}
	if m.selectedNodeIdx < 0 {
		m.selectedNodeIdx = 0
	}

	maxScroll := total - vis
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.nodeScrollOffset > maxScroll {
		m.nodeScrollOffset = maxScroll
	}
	if m.nodeScrollOffset < 0 {
		m.nodeScrollOffset = 0
	}
}

func (m *Model) scrollNodeWheel(delta int) {
	items := m.getServerListItems()
	vis := m.getNodeVisibleHeight()
	maxScroll := len(items) - vis
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.nodeScrollOffset += delta
	if m.nodeScrollOffset > maxScroll {
		m.nodeScrollOffset = maxScroll
	}
	if m.nodeScrollOffset < 0 {
		m.nodeScrollOffset = 0
	}
}

func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02dh %02dm %02ds", h, m, s)
	}
	return fmt.Sprintf("%02dm %02ds", m, s)
}

func (m *Model) ShouldKeepVPNAlive() bool {
	return m.keepAliveOnExit
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// 1. Text input modes (MUST NOT normalize layout so typing in Russian works!)
	if m.customUAActive {
		switch key {
		case "esc":
			m.customUAActive = false
			m.customUATextInput.Reset()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.customUATextInput.Value())
			m.customUAActive = false
			m.customUATextInput.Reset()
			if val != "" {
				m.state.UserAgent = val
				m.fetcher.UserAgent = val
				_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
				m.statusMessage = fmt.Sprintf("User-Agent updated: %s", val)
				m.statusIsError = false
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.customUATextInput, cmd = m.customUATextInput.Update(msg)
			return m, cmd
		}
	}

	if m.addInputActive {
		switch key {
		case "esc":
			m.addInputActive = false
			m.textInput.Reset()
			return m, nil
		case "enter":
			urlStr := strings.TrimSpace(m.textInput.Value())
			m.addInputActive = false
			m.textInput.Reset()
			if urlStr != "" {
				m.statusMessage = "Processing subscription / nodes..."
				m.statusIsError = false
				return m, func() tea.Msg {
					// 1. Direct proxy URI (vless, ss, trojan, vmess, hysteria2, hy2)
					if strings.HasPrefix(urlStr, "vless://") || strings.HasPrefix(urlStr, "ss://") ||
						strings.HasPrefix(urlStr, "trojan://") || strings.HasPrefix(urlStr, "vmess://") ||
						strings.HasPrefix(urlStr, "hysteria2://") || strings.HasPrefix(urlStr, "hy2://") {
						node, err := subscription.ParseNodeURI(urlStr, "Custom Nodes")
						if err != nil {
							return subFetchedMsg{err: err}
						}
						sub := &model.Subscription{
							ID:        urlStr,
							Name:      "Custom Nodes",
							URL:       "",
							Nodes:     []*model.ProxyNode{node},
							UpdatedAt: time.Now(),
						}
						return subFetchedMsg{sub: sub}
					}

					// 2. Local file on disk (JSON config, Clash YAML, or text file)
					if fileData, err := os.ReadFile(urlStr); err == nil {
						groupName := filepath.Base(urlStr)
						groupName = strings.TrimSuffix(groupName, filepath.Ext(groupName))
						if groupName == "" {
							groupName = "Local Config"
						}
						nodes, parseErr := subscription.ParseContent(string(fileData), groupName)
						if parseErr != nil {
							return subFetchedMsg{err: parseErr}
						}
						sub := &model.Subscription{
							ID:        urlStr,
							Name:      groupName,
							URL:       urlStr,
							Nodes:     nodes,
							UpdatedAt: time.Now(),
						}
						return subFetchedMsg{sub: sub}
					}

					// 3. HTTP / HTTPS subscription URL
					if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
						sub, err := m.fetcher.FetchSubscription(urlStr, "Subscription")
						return subFetchedMsg{sub: sub, err: err}
					}

					// 4. Raw JSON, YAML, or Base64 pasted directly
					nodes, err := subscription.ParseContent(urlStr, "Custom Nodes")
					if err != nil {
						return subFetchedMsg{err: fmt.Errorf("invalid subscription link, path or content: %w", err)}
					}
					sub := &model.Subscription{
						ID:        fmt.Sprintf("raw-%d", time.Now().Unix()),
						Name:      "Custom Nodes",
						URL:       "",
						Nodes:     nodes,
						UpdatedAt: time.Now(),
					}
					return subFetchedMsg{sub: sub}
				}
			}
			return m, nil
		default:
			// Let textinput handle all keys and pasted text!
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}

	// Handle search input if active
	if m.view == ViewProc && m.procFilterActive {
		switch key {
		case "delete", "del":
			m.procFilterQuery = ""
			m.clampProcState()
			return m, nil
		case "up", "down", "k", "j":
			m.procFilterActive = false
			m.clampProcState()
			return m.handleProcKeys(key)
		case "left":
			m.procFilterActive = false
			m.procRightFocused = false
			m.clampProcState()
			return m, nil
		case "right":
			m.procFilterActive = false
			m.procRightFocused = true
			m.clampProcState()
			return m, nil
		case "esc", "enter":
			m.procFilterActive = false
			m.clampProcState()
			return m, nil
		case "backspace":
			runes := []rune(m.procFilterQuery)
			if len(runes) > 0 {
				m.procFilterQuery = string(runes[:len(runes)-1])
			}
			m.clampProcState()
			return m, nil
		case "tab":
			m.procRightFocused = !m.procRightFocused
			m.clampProcState()
			return m, nil
		default:
			if utf8.RuneCountInString(key) == 1 {
				m.procFilterQuery += key
				m.clampProcState()
			}
			return m, nil
		}
	}

	if m.view == ViewMain && m.mainFilterActive {
		switch key {
		case "esc", "enter":
			m.mainFilterActive = false
		case "backspace":
			runes := []rune(m.mainFilterQuery)
			if len(runes) > 0 {
				m.mainFilterQuery = string(runes[:len(runes)-1])
			}
		default:
			if utf8.RuneCountInString(key) == 1 {
				m.mainFilterQuery += key
			}
		}
		return m, nil
	}

	// Intercept picker keys in Geo Add mode
	if m.view == ViewGeoRules && m.geoModeAdd {
		return m.handleGeoPickerKeys(key)
	}

	// 2. Normalize hotkeys across ALL keyboard layouts (Russian/Ukrainian -> English)
	key = normalizeKey(key)

	// 3. Handle Quit Confirmation Modal
	if m.confirmQuit {
		if m.state.IsConnected {
			switch key {
			case "enter", "y", "Y", "b", "B":
				m.keepAliveOnExit = true
				return m, tea.Quit
			case "d", "D":
				m.keepAliveOnExit = false
				if m.engine != nil {
					_ = m.engine.Stop()
				}
				return m, tea.Quit
			case "ctrl+c":
				m.keepAliveOnExit = false
				if m.engine != nil {
					_ = m.engine.Stop()
				}
				return m, tea.Quit
			case "n", "N", "esc", "q", "Q":
				m.confirmQuit = false
				return m, nil
			default:
				return m, nil
			}
		} else {
			switch key {
			case "y", "Y", "enter":
				m.keepAliveOnExit = false
				return m, tea.Quit
			case "ctrl+c":
				m.keepAliveOnExit = false
				return m, tea.Quit
			case "n", "N", "esc", "q", "Q":
				m.confirmQuit = false
				return m, nil
			default:
				return m, nil
			}
		}
	}

	// 4. Global hotkeys
	switch key {
	case "ctrl+c":
		m.confirmQuit = true
		return m, nil

	case "q":
		if m.view == ViewProc || m.view == ViewSettings || m.view == ViewTheme || m.view == ViewGeoRules {
			if m.view == ViewGeoRules {
				if m.geoInActionView {
					m.geoInActionView = false
					return m, nil
				}
				m.view = ViewSettings
				return m, nil
			}
			if m.view == ViewTheme {
				m.view = ViewSettings
				return m, nil
			}
			m.view = ViewMain
			return m, nil
		}
		// On ViewMain: prompt confirmation before quitting
		m.confirmQuit = true
		return m, nil

	case "p", "P":
		if m.view == ViewProc {
			m.view = ViewMain
		} else {
			m.view = ViewProc
			vpn, bypass, _ := m.procMgr.Refresh()
			m.vpnTree = vpn
			m.bypassTree = bypass
			m.refreshProcessesWithFrozenAnchor()
		}
		return m, nil

	case "`", "~":
		if m.view == ViewSettings {
			m.view = ViewMain
		} else {
			m.view = ViewSettings
		}
		return m, nil
	}

	// 5. View-specific keybindings
	switch m.view {
	case ViewMain:
		return m.handleMainKeys(key)
	case ViewProc:
		return m.handleProcKeys(key)
	case ViewSettings:
		return m.handleSettingsKeys(key)
	case ViewTheme:
		return m.handleThemeKeys(key)
	case ViewGeoRules:
		return m.handleGeoKeys(key)
	}

	return m, nil
}

func (m *Model) handleMainKeys(key string) (tea.Model, tea.Cmd) {
	m.clampNodeState()
	items := m.getServerListItems()
	vis := m.getNodeVisibleHeight()

	switch key {
	case "up", "k":
		if m.selectedNodeIdx > 0 {
			m.selectedNodeIdx--
			if m.selectedNodeIdx < m.nodeScrollOffset {
				m.nodeScrollOffset = m.selectedNodeIdx
			}
		}
	case "down", "j":
		if m.selectedNodeIdx < len(items)-1 {
			m.selectedNodeIdx++
			if m.selectedNodeIdx >= m.nodeScrollOffset+vis {
				m.nodeScrollOffset = m.selectedNodeIdx - vis + 1
			}
		}
	case "home":
		m.selectedNodeIdx = 0
		m.nodeScrollOffset = 0
	case "end":
		if len(items) > 0 {
			m.selectedNodeIdx = len(items) - 1
			maxScroll := len(items) - vis
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.nodeScrollOffset = maxScroll
		}
	case "pageup":
		m.selectedNodeIdx -= vis
		if m.selectedNodeIdx < 0 {
			m.selectedNodeIdx = 0
		}
		m.nodeScrollOffset -= vis
		if m.nodeScrollOffset < 0 {
			m.nodeScrollOffset = 0
		}
	case "pagedown":
		m.selectedNodeIdx += vis
		if m.selectedNodeIdx >= len(items) {
			m.selectedNodeIdx = len(items) - 1
		}
		m.nodeScrollOffset += vis
		maxScroll := len(items) - vis
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.nodeScrollOffset > maxScroll {
			m.nodeScrollOffset = maxScroll
		}
	case "f", "F":
		m.mainFilterActive = true

	case "a", "A", "+":
		// Open Add Subscription textinput
		m.addInputActive = true
		m.textInput.Reset()
		m.textInput.Focus()

	case "e":
		if len(items) > 0 && m.selectedNodeIdx < len(items) {
			curr := items[m.selectedNodeIdx]
			if curr.IsHeader {
				if m.collapsedSubs == nil {
					m.collapsedSubs = make(map[string]bool)
				}
				m.collapsedSubs[curr.Sub.ID] = !curr.Collapsed
			} else if curr.Sub != nil {
				if m.collapsedSubs == nil {
					m.collapsedSubs = make(map[string]bool)
				}
				m.collapsedSubs[curr.Sub.ID] = true
				// Locate header of this sub and move cursor there
				updatedItems := m.getServerListItems()
				for idx, it := range updatedItems {
					if it.IsHeader && it.Sub.ID == curr.Sub.ID {
						m.selectedNodeIdx = idx
						break
					}
				}
			}
			m.clampNodeState()
		}
		return m, nil

	case "E":
		// Toggle collapse/expand ALL subscriptions
		if m.collapsedSubs == nil {
			m.collapsedSubs = make(map[string]bool)
		}
		anyExpanded := false
		for _, sub := range m.state.Subscriptions {
			if !m.collapsedSubs[sub.ID] {
				anyExpanded = true
				break
			}
		}
		for _, sub := range m.state.Subscriptions {
			m.collapsedSubs[sub.ID] = anyExpanded
		}
		m.clampNodeState()
		return m, nil

	case " ", "enter":
		if len(items) > 0 && m.selectedNodeIdx < len(items) {
			curr := items[m.selectedNodeIdx]
			if curr.IsHeader {
				if m.collapsedSubs == nil {
					m.collapsedSubs = make(map[string]bool)
				}
				m.collapsedSubs[curr.Sub.ID] = !curr.Collapsed
				m.clampNodeState()
				return m, nil
			} else if curr.Node != nil {
				targetNode := curr.Node
				if m.state.IsConnected && m.state.ActiveNode != nil && m.state.ActiveNode.ID == targetNode.ID {
					_ = m.engine.Stop()
					m.state.IsConnected = false
					m.statusMessage = "Disconnected."
					m.statusIsError = false
				} else {
					m.state.ActiveNode = targetNode
					m.statusMessage = fmt.Sprintf("Connecting to %s...", targetNode.Name)
					m.statusIsError = false
					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)

					return m, func() tea.Msg {
						err := m.engine.Start(targetNode, m.procMgr.GetBypassNamesList(), m.state.GeoRules)
						return connectResultMsg{err: err}
					}
				}
			}
		}

	case "c", "C":
		if len(items) > 0 && m.selectedNodeIdx < len(items) {
			curr := items[m.selectedNodeIdx]
			var targetNode *model.ProxyNode
			if curr.IsHeader {
				if len(curr.Sub.Nodes) > 0 {
					targetNode = curr.Sub.Nodes[0]
				}
			} else {
				targetNode = curr.Node
			}

			if targetNode != nil {
				if m.state.IsConnected && m.state.ActiveNode != nil && m.state.ActiveNode.ID == targetNode.ID {
					_ = m.engine.Stop()
					m.state.IsConnected = false
					m.statusMessage = "Disconnected."
					m.statusIsError = false
				} else {
					m.state.ActiveNode = targetNode
					m.statusMessage = fmt.Sprintf("Connecting to %s...", targetNode.Name)
					m.statusIsError = false
					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)

					return m, func() tea.Msg {
						err := m.engine.Start(targetNode, m.procMgr.GetBypassNamesList(), m.state.GeoRules)
						return connectResultMsg{err: err}
					}
				}
			}
		}

	case "t", "T":
		var cmds []tea.Cmd
		for _, it := range items {
			if !it.IsHeader && it.Node != nil {
				node := it.Node
				cmds = append(cmds, func() tea.Msg {
					ms := subscription.PingNode(node, 2*time.Second)
					return pingResultMsg{node: node, ping: ms}
				})
			}
		}
		return m, tea.Batch(cmds...)

	case "u", "U":
		m.statusMessage = "Updating subscriptions..."
		m.statusIsError = false
		var cmds []tea.Cmd
		for _, sub := range m.state.Subscriptions {
			if sub.URL != "" {
				s := sub
				cmds = append(cmds, func() tea.Msg {
					updated, err := m.fetcher.FetchSubscription(s.URL, s.Name)
					if err != nil {
						return subFetchedMsg{err: err}
					}
					s.Nodes = updated.Nodes
					s.Upload = updated.Upload
					s.Download = updated.Download
					s.Total = updated.Total
					s.UpdatedAt = time.Now()
					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
					return subFetchedMsg{sub: nil}
				})
			}
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

func (m *Model) clampProcState() {
	vis := m.getProcVisibleHeight()

	// 1. Clamp Left column
	flatVPN := m.flattenTree(m.vpnTree, 0, "", false)
	totalLeft := len(flatVPN)
	if totalLeft == 0 {
		m.procLeftIdx = 0
		m.procLeftScroll = 0
	} else {
		if m.procLeftIdx >= totalLeft {
			m.procLeftIdx = totalLeft - 1
		}
		if m.procLeftIdx < 0 {
			m.procLeftIdx = 0
		}
		m.selectedLeftPID = flatVPN[m.procLeftIdx].Node.PID

		maxScroll := totalLeft - vis
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.procLeftScroll > maxScroll {
			m.procLeftScroll = maxScroll
		}
		if m.procLeftScroll < 0 {
			m.procLeftScroll = 0
		}
	}

	// 2. Clamp Right column
	flatBypass := m.flattenTree(m.bypassTree, 0, "", true)
	totalRight := len(flatBypass)
	if totalRight == 0 {
		m.procRightIdx = 0
		m.procRightScroll = 0
	} else {
		if m.procRightIdx >= totalRight {
			m.procRightIdx = totalRight - 1
		}
		if m.procRightIdx < 0 {
			m.procRightIdx = 0
		}
		m.selectedRightPID = flatBypass[m.procRightIdx].Node.PID

		maxScroll := totalRight - vis
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.procRightScroll > maxScroll {
			m.procRightScroll = maxScroll
		}
		if m.procRightScroll < 0 {
			m.procRightScroll = 0
		}
	}
}

func (m *Model) handleProcKeys(key string) (tea.Model, tea.Cmd) {
	m.clampProcState()
	vis := m.getProcVisibleHeight()

	switch key {
	case "tab":
		m.procRightFocused = !m.procRightFocused

	case "left", "h":
		m.procRightFocused = false

	case "right", "l":
		m.procRightFocused = true

	case "delete", "del":
		m.procFilterQuery = ""
		m.clampProcState()

	case "up", "k":
		if !m.procRightFocused {
			flat := m.flattenTree(m.vpnTree, 0, "", false)
			if len(flat) > 0 {
				if m.procLeftIdx >= m.procLeftScroll+vis {
					m.procLeftIdx = m.procLeftScroll + vis - 1
				} else if m.procLeftIdx < m.procLeftScroll {
					m.procLeftIdx = m.procLeftScroll
				} else if m.procLeftIdx > 0 {
					m.procLeftIdx--
				}
				if m.procLeftIdx >= len(flat) {
					m.procLeftIdx = len(flat) - 1
				}
				if m.procLeftIdx < 0 {
					m.procLeftIdx = 0
				}
				m.selectedLeftPID = flat[m.procLeftIdx].Node.PID
				if m.procLeftIdx < m.procLeftScroll {
					m.procLeftScroll = m.procLeftIdx
				}
			}
		} else {
			flat := m.flattenTree(m.bypassTree, 0, "", true)
			if len(flat) > 0 {
				if m.procRightIdx >= m.procRightScroll+vis {
					m.procRightIdx = m.procRightScroll + vis - 1
				} else if m.procRightIdx < m.procRightScroll {
					m.procRightIdx = m.procRightScroll
				} else if m.procRightIdx > 0 {
					m.procRightIdx--
				}
				if m.procRightIdx >= len(flat) {
					m.procRightIdx = len(flat) - 1
				}
				if m.procRightIdx < 0 {
					m.procRightIdx = 0
				}
				m.selectedRightPID = flat[m.procRightIdx].Node.PID
				if m.procRightIdx < m.procRightScroll {
					m.procRightScroll = m.procRightIdx
				}
			}
		}

	case "down", "j":
		if !m.procRightFocused {
			flat := m.flattenTree(m.vpnTree, 0, "", false)
			if len(flat) > 0 {
				if m.procLeftIdx < m.procLeftScroll {
					m.procLeftIdx = m.procLeftScroll
				} else if m.procLeftIdx >= m.procLeftScroll+vis {
					m.procLeftIdx = m.procLeftScroll + vis - 1
				} else if m.procLeftIdx < len(flat)-1 {
					m.procLeftIdx++
				}
				if m.procLeftIdx >= len(flat) {
					m.procLeftIdx = len(flat) - 1
				}
				if m.procLeftIdx < 0 {
					m.procLeftIdx = 0
				}
				m.selectedLeftPID = flat[m.procLeftIdx].Node.PID
				if m.procLeftIdx >= m.procLeftScroll+vis {
					m.procLeftScroll = m.procLeftIdx - vis + 1
				}
			}
		} else {
			flat := m.flattenTree(m.bypassTree, 0, "", true)
			if len(flat) > 0 {
				if m.procRightIdx < m.procRightScroll {
					m.procRightIdx = m.procRightScroll
				} else if m.procRightIdx >= m.procRightScroll+vis {
					m.procRightIdx = m.procRightScroll + vis - 1
				} else if m.procRightIdx < len(flat)-1 {
					m.procRightIdx++
				}
				if m.procRightIdx >= len(flat) {
					m.procRightIdx = len(flat) - 1
				}
				if m.procRightIdx < 0 {
					m.procRightIdx = 0
				}
				m.selectedRightPID = flat[m.procRightIdx].Node.PID
				if m.procRightIdx >= m.procRightScroll+vis {
					m.procRightScroll = m.procRightIdx - vis + 1
				}
			}
		}

	case "f", "F":
		m.procFilterActive = true

	case "s", "S":
		m.procMgr.HideSystem = !m.procMgr.HideSystem
		m.refreshProcessesWithFrozenAnchor()
		m.clampProcState()

	case "e", "E":
		m.procMgr.Collapse = !m.procMgr.Collapse
		m.refreshProcessesWithFrozenAnchor()
		m.clampProcState()

	case "enter":
		m.toggleSelectedProc(false)

	case "t", "T":
		m.toggleSelectedProc(true)

	case "esc", "q", "Q":
		m.view = ViewMain
	}

	return m, nil
}

func (m *Model) toggleSelectedProc(wholeTree bool) {
	var currentItem *flatProcItem
	if !m.procRightFocused {
		flat := m.flattenTree(m.vpnTree, 0, "", false)
		if len(flat) > 0 && m.procLeftIdx >= 0 && m.procLeftIdx < len(flat) {
			currentItem = &flat[m.procLeftIdx]
		}
	} else {
		flat := m.flattenTree(m.bypassTree, 0, "", true)
		if len(flat) > 0 && m.procRightIdx >= 0 && m.procRightIdx < len(flat) {
			currentItem = &flat[m.procRightIdx]
		}
	}

	if currentItem != nil && currentItem.Node != nil {
		m.procMgr.ToggleBypass(currentItem.Node, wholeTree)
		m.refreshProcessesWithFrozenAnchor()
		m.clampProcState()
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.engine != nil {
			if err := m.engine.UpdateBypass(m.procMgr.GetBypassNamesList()); err != nil {
				m.statusMessage = fmt.Sprintf("Bypass Error: %v", err)
				m.statusIsError = true
			}
		}
	}
}


func (m *Model) handleSettingsKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t", "T":
		m.view = ViewTheme
		m.themeSliderActive = false
		m.themeCursor = 0
		return m, nil

	case "g", "G":
		m.view = ViewGeoRules
		m.geoModeAdd = false
		m.geoInActionView = false
		m.geoActionIdx = 0
		return m, nil

	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if m.settingsCursor < 4 {
			m.settingsCursor++
		}
	case "e", "E":
		if m.settingsCursor == 1 && m.settingsUAIdx == 4 {
			m.customUAActive = true
			m.customUATextInput.SetValue(m.state.UserAgent)
			m.customUATextInput.Focus()
			return m, nil
		}
	case "enter", " ":
		if m.settingsCursor == 0 {
			m.view = ViewTheme
			m.themeSliderActive = false
			m.themeCursor = 0
			return m, nil
		} else if m.settingsCursor == 1 {
			m.settingsUAIdx = (m.settingsUAIdx + 1) % 5
			if m.settingsUAIdx == 4 {
				m.customUAActive = true
				m.customUATextInput.SetValue(m.state.UserAgent)
				m.customUATextInput.Focus()
				return m, nil
			} else {
				uas := []string{subscription.DefaultUserAgent, subscription.ClashUserAgent, subscription.V2RayNUserAgent, "Sing-box/1.10.0"}
				m.state.UserAgent = uas[m.settingsUAIdx]
				m.fetcher.UserAgent = m.state.UserAgent
				m.statusMessage = fmt.Sprintf("User-Agent set: %s", m.state.UserAgent)
				m.statusIsError = false
			}
		} else if m.settingsCursor == 2 {
			m.settingsModeIdx = (m.settingsModeIdx + 1) % 3
			modes := []model.TunnelMode{model.ModeTUN, model.ModeSystemProxy, model.ModeDirect}
			m.state.Mode = modes[m.settingsModeIdx]
		} else if m.settingsCursor == 3 {
			m.view = ViewGeoRules
			m.geoModeAdd = false
			m.geoInActionView = false
			m.geoActionIdx = 0
			return m, nil
		} else if m.settingsCursor == 4 {
			if core.IsSystemdServiceEnabled() {
				_ = core.UninstallSystemdService()
				m.statusMessage = "Systemd user service disabled."
				m.statusIsError = false
			} else {
				if path, err := core.InstallSystemdService(); err == nil {
					m.statusMessage = fmt.Sprintf("Systemd user service enabled (%s)", filepath.Base(path))
					m.statusIsError = false
				} else {
					m.statusMessage = fmt.Sprintf("Systemd Error: %v", err)
					m.statusIsError = true
				}
			}
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
	case "esc", "q", "Q":
		m.view = ViewMain
	}
	return m, nil
}

func (m *Model) handleGeoPickerKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.geoModeAdd = false
		return m, nil

	case "tab":
		m.geoTab = 1 - m.geoTab
		m.geoSearchCursor = 0
		m.geoSearchScroll = 0
		return m, nil

	case "ctrl+d":
		m.geoActionSelect = model.GeoActionDirect
		return m, nil

	case "ctrl+p":
		m.geoActionSelect = model.GeoActionProxy
		return m, nil

	case "ctrl+b":
		m.geoActionSelect = model.GeoActionBlock
		return m, nil

	case " ":
		switch m.geoActionSelect {
		case model.GeoActionDirect:
			m.geoActionSelect = model.GeoActionProxy
		case model.GeoActionProxy:
			m.geoActionSelect = model.GeoActionBlock
		case model.GeoActionBlock:
			m.geoActionSelect = model.GeoActionDirect
		}
		return m, nil

	case "up", "k":
		if m.geoSearchCursor > 0 {
			m.geoSearchCursor--
		}
		return m, nil

	case "down", "j":
		items := m.getPickerItems()
		if m.geoSearchCursor < len(items)-1 {
			m.geoSearchCursor++
		}
		return m, nil

	case "delete", "del":
		m.geoSearchQuery = ""
		m.geoSearchCursor = 0
		m.geoSearchScroll = 0
		return m, nil

	case "backspace":
		runes := []rune(m.geoSearchQuery)
		if len(runes) > 0 {
			m.geoSearchQuery = string(runes[:len(runes)-1])
			m.geoSearchCursor = 0
			m.geoSearchScroll = 0
		}
		return m, nil

	case "enter":
		items := m.getPickerItems()
		if len(items) > 0 && m.geoSearchCursor >= 0 && m.geoSearchCursor < len(items) {
			selectedName := items[m.geoSearchCursor]
			ruleType := model.GeoTypeSite
			if m.geoTab == 1 {
				ruleType = model.GeoTypeIP
			}

			found := false
			for _, r := range m.state.GeoRules {
				if r.Type == ruleType && strings.EqualFold(r.Name, selectedName) {
					if r.Action == m.geoActionSelect {
						switch r.Action {
						case model.GeoActionDirect:
							r.Action = model.GeoActionProxy
						case model.GeoActionProxy:
							r.Action = model.GeoActionBlock
						case model.GeoActionBlock:
							r.Action = model.GeoActionDirect
						}
					} else {
						r.Action = m.geoActionSelect
					}
					m.statusMessage = fmt.Sprintf("Updated rule %s [%s]", selectedName, strings.ToUpper(string(r.Action)))
					found = true
					break
				}
			}
			if !found {
				newRule := &model.GeoRule{
					Type:   ruleType,
					Name:   selectedName,
					Action: m.geoActionSelect,
				}
				m.state.GeoRules = append(m.state.GeoRules, newRule)
				m.geoCursor = len(m.state.GeoRules) - 1
				m.statusMessage = fmt.Sprintf("Added rule %s [%s]", selectedName, strings.ToUpper(string(m.geoActionSelect)))
			}

			// Keep m.geoModeAdd = true and preserve m.geoSearchQuery!
			m.statusIsError = false
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)

			if m.state.IsConnected && m.engine != nil {
				if err := m.engine.UpdateGeoRules(m.state.GeoRules); err != nil {
					m.statusMessage = fmt.Sprintf("Geo Rule Apply Error: %v", err)
					m.statusIsError = true
				}
			}
		}
		return m, nil

	case "q":
		if m.geoSearchQuery == "" {
			m.geoModeAdd = false
			return m, nil
		}
		m.geoSearchQuery += "q"
		m.geoSearchCursor = 0
		m.geoSearchScroll = 0
		return m, nil

	default:
		if utf8.RuneCountInString(key) == 1 {
			m.geoSearchQuery += key
			m.geoSearchCursor = 0
			m.geoSearchScroll = 0
		}
		return m, nil
	}
}

func (m *Model) handleGeoKeys(key string) (tea.Model, tea.Cmd) {
	if m.geoInActionView {
		return m.handleGeoActionViewKeys(key)
	}
	return m.handleGeoMenuKeys(key)
}

func (m *Model) handleGeoMenuKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.geoActionIdx > 0 {
			m.geoActionIdx--
		} else {
			m.geoActionIdx = 2
		}
		return m, nil

	case "down", "j":
		if m.geoActionIdx < 2 {
			m.geoActionIdx++
		} else {
			m.geoActionIdx = 0
		}
		return m, nil

	case "1":
		m.geoActionIdx = 0
		m.geoInActionView = true
		m.geoTab = 0
		m.geoRuleCursor = 0
		return m, nil

	case "2":
		m.geoActionIdx = 1
		m.geoInActionView = true
		m.geoTab = 0
		m.geoRuleCursor = 0
		return m, nil

	case "3":
		m.geoActionIdx = 2
		m.geoInActionView = true
		m.geoTab = 0
		m.geoRuleCursor = 0
		return m, nil

	case "enter", " ":
		m.geoInActionView = true
		m.geoTab = 0
		m.geoRuleCursor = 0
		return m, nil

	case "u", "U":
		m.geoSyncing = true
		m.geoSyncMsg = "Downloading GeoSite/GeoIP DBs from GitHub..."
		m.statusMessage = "Downloading GeoSite and GeoIP databases..."
		m.statusIsError = false
		return m, m.syncGeoDatabasesCmd()

	case "esc", "q", "Q":
		m.view = ViewSettings
		return m, nil
	}
	return m, nil
}

func (m *Model) handleGeoActionViewKeys(key string) (tea.Model, tea.Cmd) {
	currentAction := m.getCurrentGeoAction()
	currentType := model.GeoTypeSite
	if m.geoTab == 1 {
		currentType = model.GeoTypeIP
	}
	rules := m.getFilteredGeoRules(currentAction, currentType)

	switch key {
	case "tab":
		m.geoTab = 1 - m.geoTab
		m.geoRuleCursor = 0
		return m, nil

	case "1":
		m.geoTab = 0
		m.geoRuleCursor = 0
		return m, nil

	case "2":
		m.geoTab = 1
		m.geoRuleCursor = 0
		return m, nil

	case "a", "A":
		m.geoModeAdd = true
		m.geoActionSelect = currentAction
		items := m.getPickerItems()
		if m.geoSearchCursor >= len(items) {
			m.geoSearchCursor = 0
			m.geoSearchScroll = 0
		}
		return m, nil

	case "up", "k":
		if m.geoRuleCursor > 0 {
			m.geoRuleCursor--
		}
		return m, nil

	case "down", "j":
		if m.geoRuleCursor < len(rules)-1 {
			m.geoRuleCursor++
		}
		return m, nil

	case " ":
		// Move action of current rule (Direct -> Proxy -> Block -> Direct)
		if len(rules) > 0 && m.geoRuleCursor >= 0 && m.geoRuleCursor < len(rules) {
			r := rules[m.geoRuleCursor]
			switch r.Action {
			case model.GeoActionDirect:
				r.Action = model.GeoActionProxy
			case model.GeoActionProxy:
				r.Action = model.GeoActionBlock
			case model.GeoActionBlock:
				r.Action = model.GeoActionDirect
			}
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
			if m.state.IsConnected && m.engine != nil {
				_ = m.engine.UpdateGeoRules(m.state.GeoRules)
			}
			m.statusMessage = fmt.Sprintf("Moved rule %s to %s", r.Name, strings.ToUpper(string(r.Action)))
			m.statusIsError = false
			remaining := m.getFilteredGeoRules(currentAction, currentType)
			if m.geoRuleCursor >= len(remaining) && m.geoRuleCursor > 0 {
				m.geoRuleCursor--
			}
		}
		return m, nil

	case "d", "D", "delete", "del", "x", "X":
		if len(rules) > 0 && m.geoRuleCursor >= 0 && m.geoRuleCursor < len(rules) {
			target := rules[m.geoRuleCursor]
			m.removeGeoRule(target)
			remaining := m.getFilteredGeoRules(currentAction, currentType)
			if m.geoRuleCursor >= len(remaining) && m.geoRuleCursor > 0 {
				m.geoRuleCursor--
			}
			m.statusMessage = fmt.Sprintf("Removed rule %s", target.Name)
			m.statusIsError = false
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
			if m.state.IsConnected && m.engine != nil {
				_ = m.engine.UpdateGeoRules(m.state.GeoRules)
			}
		}
		return m, nil

	case "u", "U":
		m.geoSyncing = true
		m.geoSyncMsg = "Downloading GeoSite/GeoIP DBs from GitHub..."
		m.statusMessage = "Downloading GeoSite and GeoIP databases..."
		m.statusIsError = false
		return m, m.syncGeoDatabasesCmd()

	case "esc", "q", "Q":
		m.geoInActionView = false
		return m, nil
	}

	return m, nil
}

func (m *Model) handleThemeKeys(key string) (tea.Model, tea.Cmd) {
	elements := getThemeElements()

	if m.themeSliderActive {
		switch key {
		case "up", "k":
			if m.themeRGBIndex > 0 {
				m.themeRGBIndex--
			} else {
				m.themeRGBIndex = 2
			}
		case "down", "j", "tab":
			if m.themeRGBIndex < 2 {
				m.themeRGBIndex++
			} else {
				m.themeRGBIndex = 0
			}
		case "left", "h":
			m.adjustActiveRGB(-1)
		case "right", "l":
			m.adjustActiveRGB(1)
		case "shift+left", "H", "[":
			m.adjustActiveRGB(-10)
		case "shift+right", "L", "]":
			m.adjustActiveRGB(10)
		case "enter", "esc":
			m.themeSliderActive = false
			if m.state.ThemeColors == nil {
				m.state.ThemeColors = make(map[string]string)
			}
			m.state.ThemeColors = GetCurrentThemeMap()
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
			m.statusMessage = "Color updated."
			m.statusIsError = false
		case "r", "R":
			if m.themeCursor >= 0 && m.themeCursor < len(elements) {
				elem := elements[m.themeCursor]
				if def, ok := DefaultThemeColors[elem.ID]; ok {
					elem.SetColor(def)
					m.themeR, m.themeG, m.themeB = hexToRGB(def)
					if m.state.ThemeColors == nil {
						m.state.ThemeColors = make(map[string]string)
					}
					m.state.ThemeColors = GetCurrentThemeMap()
					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
				}
			}
		}
		return m, nil
	}

	// Browsing theme elements
	switch key {
	case "up", "k":
		if m.themeCursor > 0 {
			m.themeCursor--
		} else {
			m.themeCursor = len(elements) - 1
		}
	case "down", "j":
		if m.themeCursor < len(elements)-1 {
			m.themeCursor++
		} else {
			m.themeCursor = 0
		}
	case "enter", " ":
		m.themeSliderActive = true
		m.themeRGBIndex = 0
		if m.themeCursor >= 0 && m.themeCursor < len(elements) {
			curHex := elements[m.themeCursor].GetColor()
			m.themeR, m.themeG, m.themeB = hexToRGB(curHex)
		}
	case "r", "R":
		ApplyThemeColors(DefaultThemeColors)
		m.state.ThemeColors = GetCurrentThemeMap()
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		m.statusMessage = "Theme reset to default btop palette."
		m.statusIsError = false
	case "s", "S":
		m.state.ThemeColors = GetCurrentThemeMap()
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		m.statusMessage = "Theme saved to ~/.config/sb-cli/config.json."
		m.statusIsError = false
	case "esc", "q", "Q":
		m.view = ViewSettings
	}
	return m, nil
}

func (m *Model) adjustActiveRGB(delta int) {
	elements := getThemeElements()
	if m.themeCursor < 0 || m.themeCursor >= len(elements) {
		return
	}

	switch m.themeRGBIndex {
	case 0:
		m.themeR = clamp(m.themeR+delta, 0, 255)
	case 1:
		m.themeG = clamp(m.themeG+delta, 0, 255)
	case 2:
		m.themeB = clamp(m.themeB+delta, 0, 255)
	}

	newHex := rgbToHex(m.themeR, m.themeG, m.themeB)
	elements[m.themeCursor].SetColor(newHex)
	if m.state.ThemeColors == nil {
		m.state.ThemeColors = make(map[string]string)
	}
	m.state.ThemeColors = GetCurrentThemeMap()
}

func (m *Model) View() string {
	var body string
	switch m.view {
	case ViewMain:
		body = m.RenderMainView()
	case ViewProc:
		body = m.RenderProcView()
	case ViewSettings:
		body = m.RenderSettingsView()
	case ViewTheme:
		body = m.RenderThemeView()
	case ViewGeoRules:
		body = m.RenderGeoView()
	}

	header := m.renderTopBar()
	footer := m.renderFooter()

	raw := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	if m.confirmQuit {
		raw = m.overlayQuitConfirm(raw)
	}
	return m.paintSolidBlack(raw)
}

func (m *Model) overlayQuitConfirm(raw string) string {
	boxW := 56
	if boxW > m.width-4 {
		boxW = m.width - 4
	}
	if boxW < 24 {
		boxW = 24
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true)

	title := " Confirm Exit / Завершение работы "
	titleW := lipgloss.Width(title)
	fillW := boxW - 2 - titleW - 1
	if fillW < 0 {
		fillW = 0
	}
	topLine := borderStyle.Render("╭─") + titleStyle.Render(title) + borderStyle.Render(strings.Repeat("─", fillW)+"╮")

	innerW := boxW - 2
	var innerLines []string
	innerLines = append(innerLines, emptyBlackLine(innerW))

	msg1 := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg).Bold(true).Render("  Are you sure you want to quit signbox-cli?")
	innerLines = append(innerLines, padLine(msg1, innerW))

	bracketStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)
	textStyle := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorBg)
	mutedTextStyle := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg)

	if m.state.IsConnected {
		msg2 := lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render("  ● VPN tunnel is currently ACTIVE.")
		msg3 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  Choose how you want to exit:")
		innerLines = append(innerLines, padLine(msg2, innerW))
		innerLines = append(innerLines, padLine(msg3, innerW))
		innerLines = append(innerLines, emptyBlackLine(innerW))

		opt1Key := bracketStyle.Render("  [") + lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render("Enter") + bracketStyle.Render("/") + lipgloss.NewStyle().Foreground(ColorSuccess).Background(ColorBg).Bold(true).Render("y") + bracketStyle.Render("]")
		opt1 := opt1Key + textStyle.Render("  Keep VPN in background & exit")

		opt2Key := bracketStyle.Render("  [   ") + lipgloss.NewStyle().Foreground(ColorDanger).Background(ColorBg).Bold(true).Render("d") + bracketStyle.Render("   ]")
		opt2 := opt2Key + textStyle.Render("  Disconnect VPN and exit cleanly")

		opt3Key := bracketStyle.Render("  [  ") + mutedTextStyle.Render("Esc") + bracketStyle.Render("  ]")
		opt3 := opt3Key + mutedTextStyle.Render("  Cancel (stay in sb-cli)")

		innerLines = append(innerLines, padLine(opt1, innerW))
		innerLines = append(innerLines, padLine(opt2, innerW))
		innerLines = append(innerLines, padLine(opt3, innerW))
	} else {
		msg2 := lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBg).Render("  All background processes will be terminated cleanly.")
		innerLines = append(innerLines, padLine(msg2, innerW))
		innerLines = append(innerLines, emptyBlackLine(innerW))

		opt1Key := bracketStyle.Render("  [") + lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render("Enter") + bracketStyle.Render("/") + lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBg).Bold(true).Render("y") + bracketStyle.Render("]")
		opt1 := opt1Key + textStyle.Render("  Confirm quit")

		opt2Key := bracketStyle.Render("  [  ") + mutedTextStyle.Render("Esc") + bracketStyle.Render("  ]")
		opt2 := opt2Key + mutedTextStyle.Render("  Cancel")

		innerLines = append(innerLines, padLine(opt1, innerW))
		innerLines = append(innerLines, padLine(opt2, innerW))
	}
	innerLines = append(innerLines, emptyBlackLine(innerW))

	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", boxW-2) + "╯")

	var boxLines []string
	boxLines = append(boxLines, topLine)
	for _, l := range innerLines {
		boxLines = append(boxLines, borderStyle.Render("│")+l+borderStyle.Render("│"))
	}
	boxLines = append(boxLines, bottomLine)

	boxH := len(boxLines)
	screenLines := strings.Split(raw, "\n")
	if len(screenLines) < boxH {
		return raw
	}

	startY := (len(screenLines) - boxH) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (m.width - boxW) / 2
	if startX < 0 {
		startX = 0
	}

	for i, bLine := range boxLines {
		lineIdx := startY + i
		if lineIdx >= len(screenLines) {
			break
		}
		leftPad := strings.Repeat(" ", startX)
		rightPad := ""
		remaining := m.width - startX - boxW
		if remaining > 0 {
			rightPad = strings.Repeat(" ", remaining)
		}
		screenLines[lineIdx] = leftPad + bLine + rightPad
	}

	return strings.Join(screenLines, "\n")
}

func (m *Model) getProcVisibleHeight() int {
	h := m.height - 5
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) scrollWheel(isRight bool, delta int) {
	var flat []flatProcItem
	var pScroll *int
	if !isRight {
		flat = m.flattenTree(m.vpnTree, 0, "", false)
		pScroll = &m.procLeftScroll
	} else {
		flat = m.flattenTree(m.bypassTree, 0, "", true)
		pScroll = &m.procRightScroll
	}

	vis := m.getProcVisibleHeight()
	total := len(flat)
	maxScroll := total - vis
	if maxScroll < 0 {
		maxScroll = 0
	}

	newScroll := *pScroll + delta
	if newScroll < 0 {
		newScroll = 0
	}
	if newScroll > maxScroll {
		newScroll = maxScroll
	}
	*pScroll = newScroll
}

func (m *Model) paintSolidBlack(raw string) string {
	if m.width <= 0 || m.height <= 0 {
		return raw
	}

	r, g, b := HexToRGB(string(ColorBg))
	bgSeq := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)

	lines := strings.Split(raw, "\n")
	var result []string
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > m.width {
			line = ansi.Truncate(line, m.width, "")
			w = lipgloss.Width(line)
		}
		if w < m.width {
			line = line + strings.Repeat(" ", m.width-w)
		}

		// Ensure every reset is followed by bgSeq, and line starts with bgSeq
		line = strings.ReplaceAll(line, "\x1b[m", "\x1b[0m")
		line = strings.ReplaceAll(line, "\x1b[49m", bgSeq)
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bgSeq)
		line = bgSeq + line + "\x1b[0m"

		result = append(result, line)
	}

	blankLine := bgSeq + strings.Repeat(" ", m.width) + "\x1b[0m"
	for len(result) < m.height {
		result = append(result, blankLine)
	}
	if len(result) > m.height {
		result = result[:m.height]
	}
	return strings.Join(result, "\n")
}

func (m *Model) renderTopBar() string {
	title := StyleTitle.Render("sb-cli")
	version := lipgloss.NewStyle().Foreground(ColorMuted).Render("v1.0")

	modeBadge := lipgloss.NewStyle().Foreground(ColorSecondary).Render(fmt.Sprintf("[%s]", m.state.Mode))

	statusStr := StyleDanger.Render("● DISCONNECTED")
	if m.state.IsConnected {
		if m.state.ConnectedAt.IsZero() || time.Since(m.state.ConnectedAt) > 365*24*time.Hour || time.Since(m.state.ConnectedAt) < 0 {
			if m.engine != nil {
				m.state.ConnectedAt = m.engine.GetConnectedAt()
			} else {
				m.state.ConnectedAt = time.Now()
			}
		}
		uptime := formatUptime(time.Since(m.state.ConnectedAt))
		statusStr = StyleSuccess.Render(fmt.Sprintf("● CONNECTED (%s)", uptime))
	}

	statusBanner := ""
	if m.statusMessage != "" {
		msgStyle := StyleSuccess
		if m.statusIsError {
			msgStyle = StyleDanger
		}
		statusBanner = fmt.Sprintf(" │ %s", msgStyle.Render(m.statusMessage))
	}

	barContent := fmt.Sprintf(" %s %s │ Mode: %s │ %s%s", title, version, modeBadge, statusStr, statusBanner)
	return lipgloss.NewStyle().Width(m.width).Background(ColorBg).Render(barContent)
}

func (m *Model) renderFooter() string {
	if m.confirmQuit {
		return lipgloss.NewStyle().Width(m.width).Background(ColorSelection).Render(
			" Confirm Exit: Press [y/Enter] to stop VPN & quit, [n/Esc] to cancel",
		)
	}

	var buttons string

	if m.addInputActive {
		return lipgloss.NewStyle().Width(m.width).Background(ColorSelection).Render(
			fmt.Sprintf(" Add Subscription / Node: %s (Enter to submit, Esc to cancel)", m.textInput.View()),
		)
	}

	switch m.view {
	case ViewMain:
		buttons = strings.Join([]string{
			RenderFooterButton("c", "connect/stop"),
			RenderFooterButton("a", "add sub/node"),
			RenderFooterButton("t", "test ping"),
			RenderFooterButton("u", "update subs"),
			RenderFooterButton("p", "processes"),
			RenderFooterButton("`", "settings"),
			RenderFooterButton("q", "quit"),
		}, "")

	case ViewProc:
		buttons = strings.Join([]string{
			RenderFooterButton("↑/↓", "select"),
			RenderFooterButton("tab/←/→", "col"),
			RenderFooterButton("↵", "1 proc"),
			RenderFooterButton("t", "tree"),
			RenderFooterButton("s", "system"),
			RenderFooterButton("e", "collapse"),
			RenderFooterButton("f", "filter"),
			RenderFooterButton("del", "clear"),
			RenderFooterButton("q", "back"),
		}, "")

	case ViewSettings:
		buttons = strings.Join([]string{
			RenderFooterButton("↑/↓", "select option"),
			RenderFooterButton("↵", "select/toggle"),
			RenderFooterButton("g", "geo rules"),
			RenderFooterButton("t", "theme studio"),
			RenderFooterButton("`", "back to dashboard"),
			RenderFooterButton("q", "back"),
		}, "")

	case ViewTheme:
		if m.themeSliderActive {
			buttons = strings.Join([]string{
				RenderFooterButton("←/→", "±1"),
				RenderFooterButton("H/L", "±10"),
				RenderFooterButton("↑/↓", "R/G/B"),
				RenderFooterButton("r", "reset color"),
				RenderFooterButton("↵/esc", "done"),
			}, "")
		} else {
			buttons = strings.Join([]string{
				RenderFooterButton("↑/↓", "select element"),
				RenderFooterButton("↵", "edit RGB"),
				RenderFooterButton("r", "reset default"),
				RenderFooterButton("s", "save"),
				RenderFooterButton("q", "back"),
			}, "")
		}

	case ViewGeoRules:
		if m.geoModeAdd {
			buttons = strings.Join([]string{
				RenderFooterButton("↑/↓", "select"),
				RenderFooterButton("↵", "add/cycle"),
				RenderFooterButton("space", "target action"),
				RenderFooterButton("tab", "site/ip"),
				RenderFooterButton("del", "clear search"),
				RenderFooterButton("esc", "done"),
			}, "")
		} else if m.geoInActionView {
			buttons = strings.Join([]string{
				RenderFooterButton("↑/↓", "select rule"),
				RenderFooterButton("tab/1/2", "site/ip"),
				RenderFooterButton("a", "add rule"),
				RenderFooterButton("space", "move group"),
				RenderFooterButton("d/del", "delete"),
				RenderFooterButton("q/esc", "back to menu"),
			}, "")
		} else {
			buttons = strings.Join([]string{
				RenderFooterButton("↑/↓", "select group"),
				RenderFooterButton("↵/1..3", "open tabs"),
				RenderFooterButton("u", "sync databases"),
				RenderFooterButton("q/esc", "back to settings"),
			}, "")
		}
	}

	return lipgloss.NewStyle().Width(m.width).Background(ColorBg).Render(" " + buttons)
}

