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
	ViewLogs
	ViewAdvancedSettings
	ViewFragmentation
	ViewMux
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

	// Advanced Settings state
	advCursor      int
	advInputActive bool
	advTextInput   textinput.Model

	// Fragmentation Studio state
	fragCursor      int
	fragInputActive bool
	fragTextInput   textinput.Model

	// Mux Studio state
	muxCursor int

	// Theme Studio state
	themeCursor       int  // Index in getThemeElements()
	themeSliderActive bool // true = adjusting RGB sliders, false = browsing elements
	themeRGBIndex     int  // 0 = R, 1 = G, 2 = B
	themeR            int  // Current Red (0..255)
	themeG            int  // Current Green (0..255)
	themeB            int  // Current Blue (0..255)

	// Geo Rules & Sync state
	geoInProfileView bool // false = Level 0 (Profile Selection), true = Level 1 (Direct/Proxy/Block menu)
	geoProfileIdx    int  // 0 = Global / Default, 1..N = Subscriptions[0..N-1]
	geoInActionView  bool // false = Level 1 (Direct/Proxy/Block menu), true = Level 2 (Action view)
	geoActionIdx     int  // 0 = Direct, 1 = Proxy, 2 = Block
	geoRuleCursor   int  // selected rule in Level 2 table
	geoRuleScroll   int  // scroll offset in Level 2 table
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

	// Logs View state
	logActiveTab     LogTab
	logScrollOffsets map[LogTab]int
	logAutoFollow    map[LogTab]bool
	logCache         map[LogTab][]string
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

	advInput := textinput.New()
	advInput.CharLimit = 256
	advInput.Width = 40

	fragInput := textinput.New()
	fragInput.CharLimit = 64
	fragInput.Width = 30

	if state != nil {
		if state.IPType == "" {
			state.IPType = model.IPTypePreferIPv4
		}
		if state.Fragmentation.Packets == "" {
			state.Fragmentation = model.DefaultFragmentationConfig()
		}
		if state.Mux.Concurrency == 0 {
			state.Mux = model.DefaultMuxConfig()
		}
		if state.Advanced.ResolveServerIP == "" {
			state.Advanced = model.DefaultAdvancedTunnelConfig()
		}
	}

	if engine != nil && state != nil {
		engine.SetTunnelOptions(state.IPType, state.Fragmentation, state.Mux, state.Advanced)
	}

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
		advTextInput:      advInput,
		fragTextInput:     fragInput,
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
		logActiveTab:      LogTabMain,
		logScrollOffsets:  make(map[LogTab]int),
		logAutoFollow: map[LogTab]bool{
			LogTabMain:   true,
			LogTabCore:   true,
			LogTabTunnel: true,
			LogTabSub:    true,
			LogTabDaemon: true,
		},
		logCache: make(map[LogTab][]string),
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

	m.selectActiveNode()

	return m
}

func (m *Model) selectActiveNode() {
	if m.state == nil || m.state.ActiveNode == nil {
		return
	}
	items := m.getServerListItems()
	for idx, it := range items {
		if !it.IsHeader && it.Node != nil {
			if it.Node.ID == m.state.ActiveNode.ID || (it.Node.RawURI != "" && it.Node.RawURI == m.state.ActiveNode.RawURI) {
				m.selectedNodeIdx = idx
				if m.selectedNodeIdx > 5 {
					m.nodeScrollOffset = m.selectedNodeIdx - 5
				}
				return
			}
		}
	}
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

		if m.view == ViewLogs && m.logCache != nil {
			delete(m.logCache, m.logActiveTab)
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
				rulesToUse := model.ResolveGeoRules(m.state.ActiveNode, m.state.Subscriptions, m.state.GeoRules)
				_ = m.engine.UpdateGeoRules(rulesToUse)
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
		} else if m.view == ViewLogs {
			if msg.Button == tea.MouseButtonWheelUp {
				m.scrollLogsWheel(-3)
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.scrollLogsWheel(3)
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

	if m.advInputActive {
		switch key {
		case "esc":
			m.advInputActive = false
			m.advTextInput.Reset()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.advTextInput.Value())
			m.advInputActive = false
			m.advTextInput.Reset()
			if val != "" {
				if m.advCursor == 3 {
					m.state.Advanced.ResolveServerIP = val
					m.statusMessage = fmt.Sprintf("Resolve Server IP set: %s", val)
				} else if m.advCursor == 4 {
					m.state.Advanced.ResolveServerDomain = val
					m.statusMessage = fmt.Sprintf("Resolve Server Domain set: %s", val)
				}
				m.statusIsError = false
				_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
				if m.state.IsConnected && m.engine != nil {
					_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
				}
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.advTextInput, cmd = m.advTextInput.Update(msg)
			return m, cmd
		}
	}

	if m.fragInputActive {
		switch key {
		case "esc":
			m.fragInputActive = false
			m.fragTextInput.Reset()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.fragTextInput.Value())
			m.fragInputActive = false
			m.fragTextInput.Reset()
			if val != "" {
				if m.fragCursor == 1 {
					m.state.Fragmentation.Packets = val
					m.statusMessage = fmt.Sprintf("Fragmentation Packets set: %s", val)
				} else if m.fragCursor == 2 {
					m.state.Fragmentation.Length = val
					m.statusMessage = fmt.Sprintf("Fragmentation Length set: %s", val)
				} else if m.fragCursor == 3 {
					m.state.Fragmentation.Interval = val
					m.statusMessage = fmt.Sprintf("Fragmentation Interval set: %s", val)
				}
				m.statusIsError = false
				_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
				if m.state.IsConnected && m.engine != nil {
					_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
				}
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.fragTextInput, cmd = m.fragTextInput.Update(msg)
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
						nodes, rules, parseErr := subscription.ParseContentWithRules(string(fileData), groupName)
						if parseErr != nil {
							return subFetchedMsg{err: parseErr}
						}
						sub := &model.Subscription{
							ID:        urlStr,
							Name:      groupName,
							URL:       urlStr,
							Nodes:     nodes,
							GeoRules:  rules,
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
					nodes, rules, err := subscription.ParseContentWithRules(urlStr, "Custom Nodes")
					if err != nil {
						return subFetchedMsg{err: fmt.Errorf("invalid subscription link, path or content: %w", err)}
					}
					sub := &model.Subscription{
						ID:        fmt.Sprintf("raw-%d", time.Now().Unix()),
						Name:      "Custom Nodes",
						URL:       "",
						Nodes:     nodes,
						GeoRules:  rules,
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
		if m.view == ViewProc || m.view == ViewSettings || m.view == ViewTheme || m.view == ViewGeoRules || m.view == ViewLogs || m.view == ViewAdvancedSettings || m.view == ViewFragmentation || m.view == ViewMux {
			if m.view == ViewGeoRules {
				if m.geoInActionView {
					m.geoInActionView = false
					return m, nil
				}
				if m.geoInProfileView {
					m.geoInProfileView = false
					return m, nil
				}
				m.view = ViewSettings
				return m, nil
			}
			if m.view == ViewTheme || m.view == ViewAdvancedSettings || m.view == ViewFragmentation || m.view == ViewMux {
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

	case "L":
		if (m.view == ViewTheme && m.themeSliderActive) || (m.view == ViewMux && (m.muxCursor == 1 || m.muxCursor == 2)) {
			break
		}
		if m.view == ViewLogs {
			m.view = ViewMain
		} else {
			m.view = ViewLogs
		}
		return m, nil

	case "`", "~":
		if m.view == ViewSettings || m.view == ViewAdvancedSettings || m.view == ViewFragmentation || m.view == ViewMux {
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
	case ViewLogs:
		return m.handleLogsKeys(key)
	case ViewAdvancedSettings:
		return m.handleAdvancedKeys(key)
	case ViewFragmentation:
		return m.handleFragKeys(key)
	case ViewMux:
		return m.handleMuxKeys(key)
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

	case "d", "D", "delete", "del":
		if len(items) > 0 && m.selectedNodeIdx < len(items) {
			curr := items[m.selectedNodeIdx]
			if curr.Sub != nil {
				if curr.IsHeader || key == "D" || key == "delete" || key == "del" {
					targetSub := curr.Sub
					subName := targetSub.Name
					nodeCount := len(targetSub.Nodes)

					// If connected node is from this subscription, stop VPN
					if m.state.IsConnected && m.state.ActiveNode != nil {
						for _, n := range targetSub.Nodes {
							if n.ID == m.state.ActiveNode.ID || n == m.state.ActiveNode {
								_ = m.engine.Stop()
								m.state.IsConnected = false
								m.state.ActiveNode = nil
								break
							}
						}
					}

					// Remove from m.state.Subscriptions
					for i, s := range m.state.Subscriptions {
						if s == targetSub || s.ID == targetSub.ID {
							m.state.Subscriptions = append(m.state.Subscriptions[:i], m.state.Subscriptions[i+1:]...)
							break
						}
					}

					// Reassign active node if needed
					if m.state.ActiveNode != nil {
						for _, n := range targetSub.Nodes {
							if n.ID == m.state.ActiveNode.ID || n == m.state.ActiveNode {
								m.state.ActiveNode = nil
								break
							}
						}
					}
					if m.state.ActiveNode == nil && len(m.state.Subscriptions) > 0 && len(m.state.Subscriptions[0].Nodes) > 0 {
						m.state.ActiveNode = m.state.Subscriptions[0].Nodes[0]
					}

					if m.collapsedSubs != nil {
						delete(m.collapsedSubs, targetSub.ID)
					}

					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
					m.statusMessage = fmt.Sprintf("Deleted subscription %s (%d servers)", subName, nodeCount)
					m.statusIsError = false
					m.clampNodeState()
					return m, nil
				} else {
					m.statusMessage = fmt.Sprintf("Tip: Press [e] to focus header, then [d] to delete %s (or [Del]/[Shift+D])", curr.Sub.Name)
					m.statusIsError = false
					return m, nil
				}
			}
		}
		return m, nil

	case "shift+up", "ctrl+up", "K", "[", "<":
		if len(items) > 0 && m.selectedNodeIdx < len(items) {
			curr := items[m.selectedNodeIdx]
			if curr.Sub != nil {
				targetSub := curr.Sub
				subIdx := -1
				for i, s := range m.state.Subscriptions {
					if s == targetSub || s.ID == targetSub.ID {
						subIdx = i
						break
					}
				}
				if subIdx > 0 {
					m.state.Subscriptions[subIdx], m.state.Subscriptions[subIdx-1] = m.state.Subscriptions[subIdx-1], m.state.Subscriptions[subIdx]
					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
					m.statusMessage = fmt.Sprintf("Moved subscription [%s] up", targetSub.Name)
					m.statusIsError = false
					m.selectSubscriptionHeader(targetSub.ID)
					return m, nil
				} else if subIdx == 0 {
					m.statusMessage = fmt.Sprintf("Subscription [%s] is already at the top", targetSub.Name)
					m.statusIsError = false
					return m, nil
				}
			}
		}
		return m, nil

	case "shift+down", "ctrl+down", "J", "]", ">":
		if len(items) > 0 && m.selectedNodeIdx < len(items) {
			curr := items[m.selectedNodeIdx]
			if curr.Sub != nil {
				targetSub := curr.Sub
				subIdx := -1
				for i, s := range m.state.Subscriptions {
					if s == targetSub || s.ID == targetSub.ID {
						subIdx = i
						break
					}
				}
				if subIdx >= 0 && subIdx < len(m.state.Subscriptions)-1 {
					m.state.Subscriptions[subIdx], m.state.Subscriptions[subIdx+1] = m.state.Subscriptions[subIdx+1], m.state.Subscriptions[subIdx]
					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
					m.statusMessage = fmt.Sprintf("Moved subscription [%s] down", targetSub.Name)
					m.statusIsError = false
					m.selectSubscriptionHeader(targetSub.ID)
					return m, nil
				} else if subIdx == len(m.state.Subscriptions)-1 {
					m.statusMessage = fmt.Sprintf("Subscription [%s] is already at the bottom", targetSub.Name)
					m.statusIsError = false
					return m, nil
				}
			}
		}
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
						rulesToUse := model.ResolveGeoRules(targetNode, m.state.Subscriptions, m.state.GeoRules)
						err := m.engine.Start(targetNode, m.procMgr.GetBypassNamesList(), rulesToUse)
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
						rulesToUse := model.ResolveGeoRules(targetNode, m.state.Subscriptions, m.state.GeoRules)
						err := m.engine.Start(targetNode, m.procMgr.GetBypassNamesList(), rulesToUse)
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
		core.LogSub("User triggered subscription update...")
		var cmds []tea.Cmd
		for _, sub := range m.state.Subscriptions {
			if sub.URL != "" {
				s := sub
				cmds = append(cmds, func() tea.Msg {
					updated, err := m.fetcher.FetchSubscription(s.URL, s.Name)
					if err != nil {
						core.LogSub("Failed to update subscription %s: %v", s.Name, err)
						return subFetchedMsg{err: err}
					}
					s.Nodes = updated.Nodes
					s.Upload = updated.Upload
					s.Download = updated.Download
					s.Total = updated.Total
					s.UpdatedAt = time.Now()
					if len(updated.GeoRules) > 0 && len(s.GeoRules) == 0 {
						s.GeoRules = updated.GeoRules
					}
					core.LogSub("Successfully updated subscription %s: %d nodes", s.Name, len(s.Nodes))
					if m.state.ActiveNode != nil {
						for _, n := range updated.Nodes {
							if n.ID == m.state.ActiveNode.ID || (n.RawURI != "" && n.RawURI == m.state.ActiveNode.RawURI) || (n.Name == m.state.ActiveNode.Name && n.Address == m.state.ActiveNode.Address && n.Port == m.state.ActiveNode.Port) {
								m.state.ActiveNode = n
								break
							}
						}
					}
					_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
					return subFetchedMsg{sub: nil}
				})
			}
		}
		return m, tea.Batch(cmds...)

	case "l", "L":
		m.view = ViewLogs
		return m, nil
	}

	return m, nil
}

func (m *Model) selectSubscriptionHeader(subID string) {
	items := m.getServerListItems()
	for idx, it := range items {
		if it.IsHeader && it.Sub != nil && it.Sub.ID == subID {
			m.selectedNodeIdx = idx
			vis := m.getNodeVisibleHeight()
			if m.selectedNodeIdx < m.nodeScrollOffset {
				m.nodeScrollOffset = m.selectedNodeIdx
			}
			if m.selectedNodeIdx >= m.nodeScrollOffset+vis {
				m.nodeScrollOffset = m.selectedNodeIdx - vis + 1
			}
			break
		}
	}
	m.clampNodeState()
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
		m.geoInProfileView = false
		m.geoProfileIdx = 0
		m.geoActionIdx = 0
		return m, nil

	case "l", "L":
		m.view = ViewLogs
		return m, nil

	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if m.settingsCursor < 8 {
			m.settingsCursor++
		}
	case "e", "E":
		if m.settingsCursor == 2 {
			m.customUAActive = true
			m.customUATextInput.SetValue(m.state.UserAgent)
			m.customUATextInput.Focus()
			return m, nil
		} else if m.settingsCursor == 6 {
			m.view = ViewFragmentation
			m.fragCursor = 0
			m.fragInputActive = false
			return m, nil
		} else if m.settingsCursor == 7 {
			m.view = ViewMux
			m.muxCursor = 0
			return m, nil
		} else if m.settingsCursor == 8 {
			m.view = ViewAdvancedSettings
			m.advCursor = 0
			m.advInputActive = false
			return m, nil
		}
	case "enter", " ":
		switch m.settingsCursor {
		case 0:
			m.view = ViewTheme
			m.themeSliderActive = false
			m.themeCursor = 0
			return m, nil

		case 1:
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

		case 2:
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

		case 3:
			m.settingsModeIdx = (m.settingsModeIdx + 1) % 3
			modes := []model.TunnelMode{model.ModeTUN, model.ModeSystemProxy, model.ModeDirect}
			m.state.Mode = modes[m.settingsModeIdx]

		case 4:
			switch m.state.IPType {
			case model.IPTypePreferIPv4:
				m.state.IPType = model.IPTypePreferIPv6
			case model.IPTypePreferIPv6:
				m.state.IPType = model.IPTypeIPv4Only
			case model.IPTypeIPv4Only:
				m.state.IPType = model.IPTypeIPv6Only
			default:
				m.state.IPType = model.IPTypePreferIPv4
			}
			m.statusMessage = fmt.Sprintf("IP strategy set: %s", m.state.IPType)
			m.statusIsError = false
			if m.state.IsConnected && m.engine != nil {
				_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
			}

		case 5:
			m.view = ViewGeoRules
			m.geoModeAdd = false
			m.geoInActionView = false
			m.geoInProfileView = false
			m.geoProfileIdx = 0
			m.geoActionIdx = 0
			return m, nil

		case 6:
			if key == " " {
				m.state.Fragmentation.Enabled = !m.state.Fragmentation.Enabled
				m.statusMessage = fmt.Sprintf("Fragmentation: enabled = %t", m.state.Fragmentation.Enabled)
				m.statusIsError = false
				if m.state.IsConnected && m.engine != nil {
					_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
				}
			} else {
				m.view = ViewFragmentation
				m.fragCursor = 0
				m.fragInputActive = false
				return m, nil
			}

		case 7:
			if key == " " {
				m.state.Mux.Enabled = !m.state.Mux.Enabled
				m.statusMessage = fmt.Sprintf("Mux: enabled = %t", m.state.Mux.Enabled)
				m.statusIsError = false
				if m.state.IsConnected && m.engine != nil {
					_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
				}
			} else {
				m.view = ViewMux
				m.muxCursor = 0
				return m, nil
			}

		case 8:
			m.view = ViewAdvancedSettings
			m.advCursor = 0
			m.advInputActive = false
			return m, nil
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
	case "esc", "q", "Q":
		m.view = ViewMain
	}
	return m, nil
}

func (m *Model) handleAdvancedKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.advCursor > 0 {
			m.advCursor--
		}
	case "down", "j":
		if m.advCursor < 5 {
			m.advCursor++
		}
	case "e", "E":
		if m.advCursor == 3 {
			m.advInputActive = true
			m.advTextInput.SetValue(m.state.Advanced.ResolveServerIP)
			m.advTextInput.Focus()
			return m, nil
		} else if m.advCursor == 4 {
			m.advInputActive = true
			m.advTextInput.SetValue(m.state.Advanced.ResolveServerDomain)
			m.advTextInput.Focus()
			return m, nil
		}
	case "enter", " ":
		switch m.advCursor {
		case 0:
			m.state.Advanced.UseLocalDNS = !m.state.Advanced.UseLocalDNS
		case 1:
			m.state.Advanced.UseDNSFromJSON = !m.state.Advanced.UseDNSFromJSON
		case 2:
			m.state.Advanced.EnableResolveServer = !m.state.Advanced.EnableResolveServer
		case 3:
			m.advInputActive = true
			m.advTextInput.SetValue(m.state.Advanced.ResolveServerIP)
			m.advTextInput.Focus()
			return m, nil
		case 4:
			m.advInputActive = true
			m.advTextInput.SetValue(m.state.Advanced.ResolveServerDomain)
			m.advTextInput.Focus()
			return m, nil
		case 5:
			m.state.Advanced.EnableSniffing = !m.state.Advanced.EnableSniffing
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "esc", "q", "Q":
		m.view = ViewSettings
		m.settingsCursor = 8
	}
	return m, nil
}

func (m *Model) handleFragKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.fragCursor > 0 {
			m.fragCursor--
		}
	case "down", "j":
		if m.fragCursor < 3 {
			m.fragCursor++
		}
	case "e", "E":
		switch m.fragCursor {
		case 1:
			m.fragInputActive = true
			m.fragTextInput.SetValue(m.state.Fragmentation.Packets)
			m.fragTextInput.Focus()
			return m, nil
		case 2:
			m.fragInputActive = true
			m.fragTextInput.SetValue(m.state.Fragmentation.Length)
			m.fragTextInput.Focus()
			return m, nil
		case 3:
			m.fragInputActive = true
			m.fragTextInput.SetValue(m.state.Fragmentation.Interval)
			m.fragTextInput.Focus()
			return m, nil
		}
	case "enter", " ":
		switch m.fragCursor {
		case 0:
			m.state.Fragmentation.Enabled = !m.state.Fragmentation.Enabled
		case 1:
			switch m.state.Fragmentation.Packets {
			case "tlshello":
				m.state.Fragmentation.Packets = "1-3"
			case "1-3":
				m.state.Fragmentation.Packets = "1-5"
			default:
				m.state.Fragmentation.Packets = "tlshello"
			}
		case 2:
			switch m.state.Fragmentation.Length {
			case "100-200":
				m.state.Fragmentation.Length = "50-100"
			case "50-100":
				m.state.Fragmentation.Length = "10-30"
			default:
				m.state.Fragmentation.Length = "100-200"
			}
		case 3:
			switch m.state.Fragmentation.Interval {
			case "10-20":
				m.state.Fragmentation.Interval = "5-10"
			case "5-10":
				m.state.Fragmentation.Interval = "1-5"
			default:
				m.state.Fragmentation.Interval = "10-20"
			}
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "r", "R":
		m.state.Fragmentation = model.DefaultFragmentationConfig()
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "esc", "q", "Q":
		m.view = ViewSettings
		m.settingsCursor = 6
	}
	return m, nil
}

func (m *Model) handleMuxKeys(key string) (tea.Model, tea.Cmd) {
	clamp := func(val, min, max int) int {
		if val < min {
			return min
		}
		if val > max {
			return max
		}
		return val
	}

	switch key {
	case "up", "k":
		if m.muxCursor > 0 {
			m.muxCursor--
		}
	case "down", "j":
		if m.muxCursor < 3 {
			m.muxCursor++
		}
	case "left", "h":
		if m.muxCursor == 1 {
			m.state.Mux.Concurrency = clamp(m.state.Mux.Concurrency-1, 1, 128)
		} else if m.muxCursor == 2 {
			m.state.Mux.XUDPConcurrency = clamp(m.state.Mux.XUDPConcurrency-1, 1, 128)
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "right", "l":
		if m.muxCursor == 1 {
			m.state.Mux.Concurrency = clamp(m.state.Mux.Concurrency+1, 1, 128)
		} else if m.muxCursor == 2 {
			m.state.Mux.XUDPConcurrency = clamp(m.state.Mux.XUDPConcurrency+1, 1, 128)
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "H", "shift+left", "[":
		if m.muxCursor == 1 {
			m.state.Mux.Concurrency = clamp(m.state.Mux.Concurrency-8, 1, 128)
		} else if m.muxCursor == 2 {
			m.state.Mux.XUDPConcurrency = clamp(m.state.Mux.XUDPConcurrency-8, 1, 128)
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "L", "shift+right", "]":
		if m.muxCursor == 1 {
			m.state.Mux.Concurrency = clamp(m.state.Mux.Concurrency+8, 1, 128)
		} else if m.muxCursor == 2 {
			m.state.Mux.XUDPConcurrency = clamp(m.state.Mux.XUDPConcurrency+8, 1, 128)
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "enter", " ":
		if m.muxCursor == 0 {
			m.state.Mux.Enabled = !m.state.Mux.Enabled
		} else if m.muxCursor == 3 {
			switch m.state.Mux.XUDPProxyUDP443 {
			case "reject":
				m.state.Mux.XUDPProxyUDP443 = "allow"
			case "allow":
				m.state.Mux.XUDPProxyUDP443 = "skip"
			default:
				m.state.Mux.XUDPProxyUDP443 = "reject"
			}
		}
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "r", "R":
		m.state.Mux = model.DefaultMuxConfig()
		_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
		if m.state.IsConnected && m.engine != nil {
			_ = m.engine.UpdateTunnelConfig(m.state.IPType, m.state.Fragmentation, m.state.Mux, m.state.Advanced)
		}
	case "esc", "q", "Q":
		m.view = ViewSettings
		m.settingsCursor = 7
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

			rules := m.getActiveProfileRules()
			found := false
			for _, r := range rules {
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
					m.statusMessage = fmt.Sprintf("Updated rule %s [%s] in %s", selectedName, strings.ToUpper(string(r.Action)), m.getActiveProfileName())
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
				rules = append(rules, newRule)
				m.setActiveProfileRules(rules)
				m.geoCursor = len(rules) - 1
				m.statusMessage = fmt.Sprintf("Added rule %s [%s] to %s", selectedName, strings.ToUpper(string(m.geoActionSelect)), m.getActiveProfileName())
			}

			// Keep m.geoModeAdd = true and preserve m.geoSearchQuery!
			m.statusIsError = false
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)

			if m.state.IsConnected && m.engine != nil {
				rulesToUse := model.ResolveGeoRules(m.state.ActiveNode, m.state.Subscriptions, m.state.GeoRules)
				if err := m.engine.UpdateGeoRules(rulesToUse); err != nil {
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
	if m.geoModeAdd {
		return m.handleGeoPickerKeys(key)
	}
	if m.geoInActionView {
		return m.handleGeoActionViewKeys(key)
	}
	if m.geoInProfileView {
		return m.handleGeoMenuKeys(key)
	}
	return m.handleGeoProfileKeys(key)
}

func (m *Model) handleGeoProfileKeys(key string) (tea.Model, tea.Cmd) {
	totalProfiles := len(m.state.Subscriptions) + 1
	switch key {
	case "up", "k":
		if m.geoProfileIdx > 0 {
			m.geoProfileIdx--
		} else {
			m.geoProfileIdx = totalProfiles - 1
		}
		return m, nil

	case "down", "j":
		if m.geoProfileIdx < totalProfiles-1 {
			m.geoProfileIdx++
		} else {
			m.geoProfileIdx = 0
		}
		return m, nil

	case "0":
		m.geoProfileIdx = 0
		m.geoInProfileView = true
		m.geoActionIdx = 0
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(key[0] - '0')
		if idx < totalProfiles {
			m.geoProfileIdx = idx
			m.geoInProfileView = true
			m.geoActionIdx = 0
		}
		return m, nil

	case "enter", " ":
		m.geoInProfileView = true
		m.geoActionIdx = 0
		return m, nil

	case "u", "U":
		m.geoSyncing = true
		m.geoSyncMsg = "Downloading GeoSite/GeoIP DBs from GitHub..."
		m.statusMessage = "Downloading GeoSite and GeoIP databases..."
		m.statusIsError = false
		return m, m.syncGeoDatabasesCmd()

	case "shift+up", "ctrl+up", "K", "[", "<":
		if m.geoProfileIdx > 1 {
			subIdx := m.geoProfileIdx - 1
			m.state.Subscriptions[subIdx], m.state.Subscriptions[subIdx-1] = m.state.Subscriptions[subIdx-1], m.state.Subscriptions[subIdx]
			m.geoProfileIdx--
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
			m.statusMessage = fmt.Sprintf("Moved subscription [%s] up", m.state.Subscriptions[m.geoProfileIdx-1].Name)
			m.statusIsError = false
		}
		return m, nil

	case "shift+down", "ctrl+down", "J", "]", ">":
		if m.geoProfileIdx > 0 && m.geoProfileIdx < len(m.state.Subscriptions) {
			subIdx := m.geoProfileIdx - 1
			m.state.Subscriptions[subIdx], m.state.Subscriptions[subIdx+1] = m.state.Subscriptions[subIdx+1], m.state.Subscriptions[subIdx]
			m.geoProfileIdx++
			_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
			m.statusMessage = fmt.Sprintf("Moved subscription [%s] down", m.state.Subscriptions[m.geoProfileIdx-1].Name)
			m.statusIsError = false
		}
		return m, nil

	case "d", "D", "delete", "del":
		if m.geoProfileIdx > 0 && m.geoProfileIdx <= len(m.state.Subscriptions) {
			sub := m.state.Subscriptions[m.geoProfileIdx-1]
			if len(sub.GeoRules) > 0 {
				sub.GeoRules = nil
				_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
				if m.state.IsConnected && m.engine != nil {
					rulesToUse := model.ResolveGeoRules(m.state.ActiveNode, m.state.Subscriptions, m.state.GeoRules)
					_ = m.engine.UpdateGeoRules(rulesToUse)
				}
				m.statusMessage = fmt.Sprintf("Cleared custom rules for [%s] (now inherits Global)", sub.Name)
				m.statusIsError = false
				return m, nil
			} else {
				// Delete empty subscription
				targetSub := sub
				subName := targetSub.Name
				nodeCount := len(targetSub.Nodes)

				if m.state.IsConnected && m.state.ActiveNode != nil {
					for _, n := range targetSub.Nodes {
						if n.ID == m.state.ActiveNode.ID || n == m.state.ActiveNode {
							_ = m.engine.Stop()
							m.state.IsConnected = false
							m.state.ActiveNode = nil
							break
						}
					}
				}

				subIdx := m.geoProfileIdx - 1
				m.state.Subscriptions = append(m.state.Subscriptions[:subIdx], m.state.Subscriptions[subIdx+1:]...)
				if m.geoProfileIdx > len(m.state.Subscriptions) {
					m.geoProfileIdx = len(m.state.Subscriptions)
				}

				if m.state.ActiveNode != nil {
					for _, n := range targetSub.Nodes {
						if n.ID == m.state.ActiveNode.ID || n == m.state.ActiveNode {
							m.state.ActiveNode = nil
							break
						}
					}
				}
				if m.state.ActiveNode == nil && len(m.state.Subscriptions) > 0 && len(m.state.Subscriptions[0].Nodes) > 0 {
					m.state.ActiveNode = m.state.Subscriptions[0].Nodes[0]
				}

				if m.collapsedSubs != nil {
					delete(m.collapsedSubs, targetSub.ID)
				}

				_ = core.SaveConfig(m.state, m.procMgr.BypassNames)
				m.statusMessage = fmt.Sprintf("Deleted subscription %s (%d servers)", subName, nodeCount)
				m.statusIsError = false
				return m, nil
			}
		}
		return m, nil

	case "esc", "q", "Q":
		m.view = ViewSettings
		return m, nil
	}
	return m, nil
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
		m.geoInProfileView = false
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
		m.geoRuleScroll = 0
		return m, nil

	case "1":
		m.geoTab = 0
		m.geoRuleCursor = 0
		m.geoRuleScroll = 0
		return m, nil

	case "2":
		m.geoTab = 1
		m.geoRuleCursor = 0
		m.geoRuleScroll = 0
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

	case "pgup", "pageup":
		m.geoRuleCursor -= 10
		if m.geoRuleCursor < 0 {
			m.geoRuleCursor = 0
		}
		return m, nil

	case "pgdown", "pagedown":
		m.geoRuleCursor += 10
		if m.geoRuleCursor >= len(rules) {
			m.geoRuleCursor = len(rules) - 1
		}
		if m.geoRuleCursor < 0 {
			m.geoRuleCursor = 0
		}
		return m, nil

	case "home":
		m.geoRuleCursor = 0
		return m, nil

	case "end":
		if len(rules) > 0 {
			m.geoRuleCursor = len(rules) - 1
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
				rulesToUse := model.ResolveGeoRules(m.state.ActiveNode, m.state.Subscriptions, m.state.GeoRules)
				_ = m.engine.UpdateGeoRules(rulesToUse)
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
				rulesToUse := model.ResolveGeoRules(m.state.ActiveNode, m.state.Subscriptions, m.state.GeoRules)
				_ = m.engine.UpdateGeoRules(rulesToUse)
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
	case ViewLogs:
		body = m.RenderLogsView()
	case ViewAdvancedSettings:
		body = m.RenderAdvancedSettingsView()
	case ViewFragmentation:
		body = m.RenderFragView()
	case ViewMux:
		body = m.RenderMuxView()
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
		origLine := screenLines[lineIdx]

		// Extract left part of the background line to preserve borders and panels
		leftPart := ansi.Truncate(origLine, startX, "")
		leftW := lipgloss.Width(leftPart)
		if leftW < startX {
			leftPart += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", startX-leftW))
		}

		// Extract right part of the background line to preserve borders and panels
		expectedRightW := m.width - (startX + boxW)
		if expectedRightW < 0 {
			expectedRightW = 0
		}
		rightPart := ansi.TruncateLeft(origLine, startX+boxW, "")
		rightW := lipgloss.Width(rightPart)
		if rightW < expectedRightW {
			rightPart += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", expectedRightW-rightW))
		} else if rightW > expectedRightW {
			rightPart = ansi.Truncate(rightPart, expectedRightW, "")
		}

		screenLines[lineIdx] = leftPart + bLine + rightPart
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
			RenderFooterButton("L", "logs"),
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
			RenderFooterButton("L", "logs"),
			RenderFooterButton("q", "back"),
		}, "")

	case ViewSettings:
		buttons = strings.Join([]string{
			RenderFooterButton("↑/↓", "select option"),
			RenderFooterButton("↵", "select/toggle"),
			RenderFooterButton("g", "geo rules"),
			RenderFooterButton("t", "theme studio"),
			RenderFooterButton("L", "logs"),
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

	case ViewLogs:
		buttons = strings.Join([]string{
			RenderFooterButton("tab", "next tab"),
			RenderFooterButton("↑/↓", "scroll"),
			RenderFooterButton("pgup/dn", "page"),
			RenderFooterButton("f", "follow"),
			RenderFooterButton("r", "refresh"),
			RenderFooterButton("c", "clear"),
			RenderFooterButton("q/esc", "back"),
		}, "")

	case ViewAdvancedSettings:
		buttons = strings.Join([]string{
			RenderFooterButton("↑/↓", "select"),
			RenderFooterButton("↵/space", "toggle/edit"),
			RenderFooterButton("e", "edit text"),
			RenderFooterButton("q/esc", "back to settings"),
		}, "")

	case ViewFragmentation:
		buttons = strings.Join([]string{
			RenderFooterButton("↑/↓", "select"),
			RenderFooterButton("↵/space", "toggle/cycle"),
			RenderFooterButton("e", "custom input"),
			RenderFooterButton("r", "defaults"),
			RenderFooterButton("q/esc", "save & back"),
		}, "")

	case ViewMux:
		buttons = strings.Join([]string{
			RenderFooterButton("↑/↓", "select"),
			RenderFooterButton("←/→", "±1"),
			RenderFooterButton("H/L", "±8"),
			RenderFooterButton("↵/space", "toggle/cycle"),
			RenderFooterButton("r", "defaults"),
			RenderFooterButton("q/esc", "save & back"),
		}, "")
	}

	return lipgloss.NewStyle().Width(m.width).Background(ColorBg).Render(" " + buttons)
}

