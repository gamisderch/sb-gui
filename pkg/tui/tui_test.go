package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"signbox-cli/pkg/core"
	"signbox-cli/pkg/model"
	"signbox-cli/pkg/process"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func init() {
	testCfgPath := filepath.Join(os.TempDir(), "sb-cli-test-config.json")
	_ = os.Setenv("SB_CONFIG_PATH", testCfgPath)
}


func createTestModel() *Model {
	state := &model.AppState{
		Mode:      model.ModeTUN,
		UserAgent: "Happ/1.8.0",
	}
	pm := process.NewProcessManager()
	engine := core.NewEngine()

	m := NewModel(state, pm, engine)
	m.width = 120
	m.height = 30
	m.view = ViewProc

	// Populate mock processes
	var vpnList []*process.ProcessNode
	for i := 1; i <= 50; i++ {
		vpnList = append(vpnList, &process.ProcessNode{
			PID:       1000 + i,
			CleanName: "mock-process",
			User:      "user",
			Sockets:   1,
		})
	}
	m.vpnTree = vpnList
	m.selectedLeftPID = vpnList[0].PID
	m.procLeftIdx = 0
	m.procLeftScroll = 0

	return m
}

func TestDelKeyClearsSearch(t *testing.T) {
	m := createTestModel()
	m.procFilterActive = true
	m.procFilterQuery = "wine"

	// Press "delete"
	m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	if m.procFilterQuery != "" {
		t.Fatalf("expected empty filter query after delete, got %q", m.procFilterQuery)
	}

	m.procFilterQuery = "steam"
	// Also test "del" key string
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("del")})
	// In Bubble Tea, delete key can be named "delete" or "del"
	m.procFilterActive = true
	m.procFilterQuery = "steam"
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDelete})
	if m.procFilterQuery != "" {
		t.Fatalf("expected empty filter query after delete, got %q", m.procFilterQuery)
	}
}

func TestArrowKeysAutoExitSearch(t *testing.T) {
	m := createTestModel()
	m.procFilterActive = true
	m.procFilterQuery = "mock"

	// Press "down" arrow
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})

	if m.procFilterActive {
		t.Fatalf("expected procFilterActive to be false after arrow key, but was true")
	}
	if m.procLeftIdx != 1 {
		t.Fatalf("expected cursor to move to index 1, got %d", m.procLeftIdx)
	}
}

func TestNaturalScrollingAndCursorMovement(t *testing.T) {
	m := createTestModel()
	vis := m.getProcVisibleHeight() // e.g. 25 lines

	// Move down past viewport height
	for i := 0; i < vis+5; i++ {
		_, _ = m.handleProcKeys("down")
	}

	expectedIdx := vis + 5
	if m.procLeftIdx != expectedIdx {
		t.Fatalf("expected procLeftIdx to be %d, got %d", expectedIdx, m.procLeftIdx)
	}
	if m.procLeftScroll != 6 {
		t.Fatalf("expected procLeftScroll to be 6, got %d", m.procLeftScroll)
	}

	// Now press UP: cursor must move up without changing scroll offset!
	initialScroll := m.procLeftScroll
	_, _ = m.handleProcKeys("up")

	if m.procLeftIdx != expectedIdx-1 {
		t.Fatalf("expected procLeftIdx to decrement to %d, got %d", expectedIdx-1, m.procLeftIdx)
	}
	if m.procLeftScroll != initialScroll {
		t.Fatalf("expected procLeftScroll to remain %d, but changed to %d", initialScroll, m.procLeftScroll)
	}
}

func TestMouseWheelScrollDoesNotMoveCursor(t *testing.T) {
	m := createTestModel()
	m.procLeftIdx = 5
	m.selectedLeftPID = m.vpnTree[5].PID
	m.procLeftScroll = 0

	// Scroll wheel down
	m.scrollWheel(false, 3)

	if m.procLeftScroll != 3 {
		t.Fatalf("expected procLeftScroll to be 3, got %d", m.procLeftScroll)
	}
	if m.procLeftIdx != 5 {
		t.Fatalf("expected procLeftIdx to stay frozen at 5, got %d", m.procLeftIdx)
	}
	if m.selectedLeftPID != m.vpnTree[5].PID {
		t.Fatalf("expected selectedLeftPID to stay frozen, got %d", m.selectedLeftPID)
	}

	// Scroll wheel up
	m.scrollWheel(false, -3)
	if m.procLeftScroll != 0 {
		t.Fatalf("expected procLeftScroll to return to 0, got %d", m.procLeftScroll)
	}
	if m.procLeftIdx != 5 {
		t.Fatalf("expected procLeftIdx to stay frozen at 5, got %d", m.procLeftIdx)
	}
}

func TestGradientColorsNeverDark(t *testing.T) {
	for dist := 0; dist < 50; dist++ {
		color := GetGreenGradient(dist)
		if color == "" || color == "#000000" || color == "#111111" {
			t.Fatalf("gradient color at dist %d is too dark: %s", dist, color)
		}
	}

	if GetGreenGradient(100) != "#28984c" {
		t.Fatalf("expected floor to be #28984c, got %s", GetGreenGradient(100))
	}
}

func TestFilterShrinkArrowKeyPanic(t *testing.T) {
	m := createTestModel()
	// Create 200 processes
	var list []*process.ProcessNode
	for i := 1; i <= 200; i++ {
		name := "common"
		if i <= 8 {
			name = "rare-target"
		}
		list = append(list, &process.ProcessNode{
			PID:       1000 + i,
			CleanName: name,
			User:      "user",
		})
	}
	m.vpnTree = list

	// User navigated down to index 188
	m.procLeftIdx = 188
	m.procLeftScroll = 170

	// User types "rare" in filter (now only 8 items match!)
	m.procFilterActive = true
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})

	// User presses arrow key "up" or "down" - must NOT panic!
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	if m.procLeftIdx >= 8 {
		t.Fatalf("expected procLeftIdx to be clamped < 8, got %d", m.procLeftIdx)
	}

	// Also test down
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.procLeftIdx >= 8 {
		t.Fatalf("expected procLeftIdx to be clamped < 8, got %d", m.procLeftIdx)
	}
}

func TestThemeStudioSlidersAndPreview(t *testing.T) {
	m := createTestModel()
	m.view = ViewSettings
	m.settingsCursor = 0 // Option 0 is Theme Studio

	// Press Enter to open Theme Studio
	_, _ = m.handleSettingsKeys("enter")
	if m.view != ViewTheme {
		t.Fatalf("expected view to be ViewTheme, got %v", m.view)
	}

	// Move cursor to "Primary Accent" (index 2)
	m.themeCursor = 2
	_, _ = m.handleThemeKeys("enter")
	if !m.themeSliderActive {
		t.Fatalf("expected themeSliderActive to be true")
	}

	// Adjust Red slider (-10)
	initialR := m.themeR
	m.handleThemeKeys("[")
	if m.themeR != initialR-10 {
		t.Fatalf("expected themeR to be %d, got %d", initialR-10, m.themeR)
	}


	// Check that ColorPrimary was updated in real time
	expectedHex := rgbToHex(m.themeR, m.themeG, m.themeB)
	if string(ColorPrimary) != expectedHex {
		t.Fatalf("expected ColorPrimary to be %s, got %s", expectedHex, string(ColorPrimary))
	}

	// Render view during slider editing - must NOT panic and must contain live preview
	rendered := m.RenderThemeView()
	if !strings.Contains(rendered, "Live Preview") {
		t.Fatalf("expected rendered Theme view to contain Live Preview section")
	}

	// Reset color with 'r'
	m.handleThemeKeys("r")
	if string(ColorPrimary) != DefaultThemeColors["primary"] {
		t.Fatalf("expected ColorPrimary to be reset to %s, got %s", DefaultThemeColors["primary"], string(ColorPrimary))
	}

	// Press Esc to exit slider mode
	m.handleThemeKeys("esc")
	if m.themeSliderActive {
		t.Fatalf("expected themeSliderActive to be false after esc")
	}
}

func TestMainViewRenderingExactDimensions(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain
	m.width = 100
	m.height = 30

	rendered := m.View()
	lines := strings.Split(rendered, "\n")
	if len(lines) != m.height {
		t.Fatalf("expected View to produce exactly %d lines, got %d", m.height, len(lines))
	}

	for i, l := range lines {
		w := lipgloss.Width(l)
		if w != m.width {
			t.Fatalf("line %d has width %d, expected %d", i, w, m.width)
		}
	}
}

func TestAllViewsSolidBackgroundAndDimensions(t *testing.T) {
	views := []ViewType{
		ViewMain, ViewProc, ViewSettings, ViewTheme, ViewGeoRules, ViewLogs,
		ViewAdvancedSettings, ViewFragmentation, ViewMux,
	}
	viewNames := map[ViewType]string{
		ViewMain:             "ViewMain",
		ViewProc:             "ViewProc",
		ViewSettings:         "ViewSettings",
		ViewTheme:            "ViewTheme",
		ViewGeoRules:         "ViewGeoRules",
		ViewLogs:             "ViewLogs",
		ViewAdvancedSettings: "ViewAdvancedSettings",
		ViewFragmentation:    "ViewFragmentation",
		ViewMux:              "ViewMux",
	}

	for _, v := range views {
		t.Run(viewNames[v], func(t *testing.T) {
			m := createTestModel()
			m.view = v
			m.width = 110
			m.height = 32

			rendered := m.View()
			lines := strings.Split(rendered, "\n")
			if len(lines) != m.height {
				t.Fatalf("[%s] expected %d lines, got %d", viewNames[v], m.height, len(lines))
			}

			r, g, b := HexToRGB(string(ColorBg))
			bgSeq := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)

			for lineIdx, line := range lines {
				w := lipgloss.Width(line)
				if w != m.width {
					t.Fatalf("[%s] line %d visual width is %d, expected %d", viewNames[v], lineIdx, w, m.width)
				}
				if !strings.HasPrefix(line, bgSeq) {
					t.Fatalf("[%s] line %d missing background prefix %q", viewNames[v], lineIdx, bgSeq)
				}
				if strings.Contains(line, "\x1b[49m") {
					t.Fatalf("[%s] line %d contains default background reset \\x1b[49m", viewNames[v], lineIdx)
				}
				// Verify no naked \x1b[0m without bgSeq reinstatement inside the line
				parts := strings.Split(line, "\x1b[0m")
				// All parts except the very last one (which ends the line) must start with bgSeq
				for pIdx, part := range parts[1:] {
					if pIdx < len(parts)-2 && len(part) > 0 && !strings.HasPrefix(part, bgSeq) && !strings.HasPrefix(part, "\x1b[48;") {
						t.Fatalf("[%s] line %d part %d after reset does not reinstate background: %q", viewNames[v], lineIdx, pIdx, part)
					}
				}
			}
		})
	}

	// Also test ViewGeoRules in Add/Picker mode
	t.Run("ViewGeoRulesAddPicker", func(t *testing.T) {
		m := createTestModel()
		m.view = ViewGeoRules
		m.geoModeAdd = true
		m.geoSearchQuery = "steam"
		m.width = 110
		m.height = 32

		rendered := m.View()
		lines := strings.Split(rendered, "\n")
		if len(lines) != m.height {
			t.Fatalf("expected %d lines, got %d", m.height, len(lines))
		}
	})

	// Also test ViewGeoRules in Level 2 Action View mode
	t.Run("ViewGeoRulesActionView", func(t *testing.T) {
		m := createTestModel()
		m.view = ViewGeoRules
		m.geoInActionView = true
		m.geoActionIdx = 0
		m.geoTab = 0
		m.width = 110
		m.height = 32

		rendered := m.View()
		lines := strings.Split(rendered, "\n")
		if len(lines) != m.height {
			t.Fatalf("expected %d lines, got %d", m.height, len(lines))
		}
	})

	// Also test overlay quit confirm modal
	t.Run("ConfirmQuitModalSolidBackground", func(t *testing.T) {
		m := createTestModel()
		m.view = ViewMain
		m.confirmQuit = true
		m.width = 110
		m.height = 32

		rendered := m.View()
		lines := strings.Split(rendered, "\n")
		if len(lines) != m.height {
			t.Fatalf("expected %d lines, got %d", m.height, len(lines))
		}

		r, g, b := HexToRGB(string(ColorBg))
		bgSeq := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
		for lineIdx, line := range lines {
			w := lipgloss.Width(line)
			if w != m.width {
				t.Fatalf("line %d width %d != %d", lineIdx, w, m.width)
			}
			if !strings.HasPrefix(line, bgSeq) {
				t.Fatalf("line %d missing bg prefix", lineIdx)
			}
		}
	})
}

func TestGeoRulesNavigationAndActions(t *testing.T) {
	m := createTestModel()
	m.view = ViewSettings
	m.settingsCursor = 5

	// Press "enter" to open Geo Rules -> starts at Level 0 Profile Menu
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.view != ViewGeoRules {
		t.Fatalf("expected ViewGeoRules, got %v", m.view)
	}
	if m.geoInProfileView || m.geoInActionView {
		t.Fatalf("expected to start in Level 0 Profile menu")
	}

	// Press "enter" on Global Profile -> enters Level 1 Menu
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.geoInProfileView || m.geoInActionView {
		t.Fatalf("expected to enter Level 1 Action menu")
	}

	// Press "enter" to enter Level 2: DIRECT Action View (geoActionIdx 0)
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.geoInActionView {
		t.Fatalf("expected geoInActionView to be true after selecting action")
	}
	if m.geoActionIdx != 0 {
		t.Fatalf("expected geoActionIdx 0 (DIRECT), got %d", m.geoActionIdx)
	}

	// Press "a" to open Level 3 Add Picker
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.geoModeAdd {
		t.Fatalf("expected geoModeAdd to be true")
	}

	// Type "r", "u"
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if m.geoSearchQuery != "ru" {
		t.Fatalf("expected search query 'ru', got %q", m.geoSearchQuery)
	}

	// Test Action Cycling with Space
	if m.geoActionSelect != model.GeoActionDirect {
		t.Fatalf("expected default action to be DIRECT, got %v", m.geoActionSelect)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.geoActionSelect != model.GeoActionProxy {
		t.Fatalf("expected action PROXY after space, got %v", m.geoActionSelect)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.geoActionSelect != model.GeoActionBlock {
		t.Fatalf("expected action BLOCK after space, got %v", m.geoActionSelect)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.geoActionSelect != model.GeoActionDirect {
		t.Fatalf("expected action DIRECT after ctrl+d, got %v", m.geoActionSelect)
	}

	// Tab switch to GeoIP and back
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if m.geoTab != 1 {
		t.Fatalf("expected geoTab 1 (GeoIP), got %d", m.geoTab)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if m.geoTab != 0 {
		t.Fatalf("expected geoTab 0 (GeoSite), got %d", m.geoTab)
	}

	// Press Enter to add selected rule: Picker MUST remain open and search query MUST NOT reset!
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.geoModeAdd {
		t.Fatalf("expected geoModeAdd to remain true so user can continue adding rules")
	}
	if m.geoSearchQuery != "ru" {
		t.Fatalf("expected search query 'ru' to NOT be reset, got %q", m.geoSearchQuery)
	}
	if len(m.state.GeoRules) != 1 {
		t.Fatalf("expected 1 geo rule, got %d", len(m.state.GeoRules))
	}
	rule := m.state.GeoRules[0]
	if rule.Action != model.GeoActionDirect {
		t.Fatalf("expected rule action DIRECT, got %v", rule.Action)
	}

	// Press Esc to exit picker mode back to Level 2 Action View
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if m.geoModeAdd {
		t.Fatalf("expected geoModeAdd to be false after esc")
	}
	if !m.geoInActionView {
		t.Fatalf("expected geoInActionView to remain true")
	}

	// Switch tabs in Level 2 Action View
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if m.geoTab != 1 {
		t.Fatalf("expected geoTab 1 after Tab, got %d", m.geoTab)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if m.geoTab != 0 {
		t.Fatalf("expected geoTab 0 after Tab, got %d", m.geoTab)
	}

	// Move rule between action groups with space (Direct -> Proxy)
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.GeoRules[0].Action != model.GeoActionProxy {
		t.Fatalf("expected rule action PROXY after space, got %v", m.state.GeoRules[0].Action)
	}

	// Press "q" to return from Level 2 Action View to Level 1 Menu
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.geoInActionView {
		t.Fatalf("expected geoInActionView to be false after 'q'")
	}
	if m.view != ViewGeoRules {
		t.Fatalf("expected ViewGeoRules, got %v", m.view)
	}

	// Open Proxy group with key '2'
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if !m.geoInActionView || m.geoActionIdx != 1 {
		t.Fatalf("expected geoInActionView true and geoActionIdx 1")
	}

	// Delete rule using DEL key
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDelete})
	if len(m.state.GeoRules) != 0 {
		t.Fatalf("expected 0 geo rules after delete, got %d", len(m.state.GeoRules))
	}

	// Press "q" to return from Level 2 Action View to Level 1 Menu
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.geoInActionView || !m.geoInProfileView {
		t.Fatalf("expected geoInActionView false and geoInProfileView true")
	}

	// Press "q" on Level 1 Menu to return to Level 0 Profile Menu
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.geoInProfileView {
		t.Fatalf("expected geoInProfileView false after 'q' on Level 1")
	}
	if m.view != ViewGeoRules {
		t.Fatalf("expected ViewGeoRules, got %v", m.view)
	}

	// Press "q" on Level 0 Profile Menu to return to ViewSettings
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.view != ViewSettings {
		t.Fatalf("expected ViewSettings after 'q' on profile menu, got %v", m.view)
	}
}

func TestConfirmQuitOnCtrlCAndQ(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain

	// 1. Press "q" on ViewMain -> should trigger confirmQuit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !m.confirmQuit {
		t.Fatalf("expected confirmQuit to be true after pressing 'q' on ViewMain")
	}

	// 2. Press "n" -> cancel quit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.confirmQuit {
		t.Fatalf("expected confirmQuit to be false after pressing 'n'")
	}

	// 3. Press "ctrl+c" -> should trigger confirmQuit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.confirmQuit {
		t.Fatalf("expected confirmQuit to be true after pressing ctrl+c")
	}

	// 4. Press "esc" -> cancel quit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if m.confirmQuit {
		t.Fatalf("expected confirmQuit to be false after pressing esc")
	}

	// 5. Press "ctrl+c" then "enter" -> should return tea.Quit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected tea.Quit command after confirming exit with Enter")
	}

	// 6. Double ctrl+c forces immediate exit
	m.confirmQuit = false
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.confirmQuit {
		t.Fatalf("expected confirmQuit to be true after first ctrl+c")
	}
}

func TestUniversalKeyboardLayoutSupport(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain

	// 1. Cyrillic 'з' ('p') -> switch to ViewProc
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'з'}})
	if m.view != ViewProc {
		t.Fatalf("expected ViewProc after pressing Cyrillic 'з', got %v", m.view)
	}

	// 2. Cyrillic 'ё' ('`') -> switch to ViewSettings
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'ё'}})
	if m.view != ViewSettings {
		t.Fatalf("expected ViewSettings after pressing Cyrillic 'ё', got %v", m.view)
	}

	// 3. Cyrillic 'й' ('q') -> back to ViewMain
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'й'}})
	if m.view != ViewMain {
		t.Fatalf("expected ViewMain after pressing Cyrillic 'й' in settings, got %v", m.view)
	}

	// 4. Cyrillic 'й' ('q') on ViewMain -> confirmQuit modal
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'й'}})
	if !m.confirmQuit {
		t.Fatalf("expected confirmQuit to be true after pressing Cyrillic 'й' on ViewMain")
	}

	// 5. Cyrillic 'т' ('n') -> cancel quit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'т'}})
	if m.confirmQuit {
		t.Fatalf("expected confirmQuit to be false after pressing Cyrillic 'т'")
	}

	// 6. Typing in text input fields must NOT normalize layout
	m.mainFilterActive = true
	m.mainFilterQuery = ""
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'р'}})
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'о'}})
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'с'}})
	if m.mainFilterQuery != "рос" {
		t.Fatalf("expected text input to retain Russian letters 'рос', got %q", m.mainFilterQuery)
	}
}

func TestBackgroundDaemonQuitConfirmOptions(t *testing.T) {
	// Case A: Disconnected -> Enter quits with ShouldKeepVPNAlive = false
	mA := createTestModel()
	mA.view = ViewMain
	mA.state.IsConnected = false
	mA.confirmQuit = true

	_, cmdA := mA.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if cmdA == nil {
		t.Fatalf("expected tea.Quit command on Enter")
	}
	if mA.ShouldKeepVPNAlive() {
		t.Fatalf("expected ShouldKeepVPNAlive to be false when disconnected")
	}

	// Case B: Connected -> Enter keeps VPN alive in background
	mB := createTestModel()
	mB.view = ViewMain
	mB.state.IsConnected = true
	mB.confirmQuit = true

	_, cmdB := mB.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if cmdB == nil {
		t.Fatalf("expected tea.Quit command on Enter")
	}
	if !mB.ShouldKeepVPNAlive() {
		t.Fatalf("expected ShouldKeepVPNAlive to be true on Enter when connected")
	}

	// Case C: Connected -> 'd' disconnects and does not keep alive
	mC := createTestModel()
	mC.view = ViewMain
	mC.state.IsConnected = true
	mC.confirmQuit = true

	_, cmdC := mC.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmdC == nil {
		t.Fatalf("expected tea.Quit command on 'd'")
	}
	if mC.ShouldKeepVPNAlive() {
		t.Fatalf("expected ShouldKeepVPNAlive to be false on 'd' when connected")
	}

	// Case D: Connected -> Russian 'в' ('d') disconnects and does not keep alive
	mD := createTestModel()
	mD.view = ViewMain
	mD.state.IsConnected = true
	mD.confirmQuit = true

	_, cmdD := mD.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'в'}})
	if cmdD == nil {
		t.Fatalf("expected tea.Quit command on Russian 'в'")
	}
	if mD.ShouldKeepVPNAlive() {
		t.Fatalf("expected ShouldKeepVPNAlive to be false on Russian 'в'")
	}
}

func TestNodeSelectionAndScrolling(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain
	m.height = 20
	m.width = 100

	// Add 30 dummy nodes
	var nodes []*model.ProxyNode
	for i := 0; i < 30; i++ {
		nodes = append(nodes, &model.ProxyNode{
			ID:       fmt.Sprintf("node-%d", i),
			Name:     fmt.Sprintf("Server Node #%d", i),
			Protocol: "vless",
			Address:  "1.2.3.4",
			Port:     443,
		})
	}
	m.state.Subscriptions = []*model.Subscription{
		{
			ID:    "sub-1",
			Name:  "Test Subscription",
			Nodes: nodes,
		},
	}

	m.clampNodeState()
	if m.selectedNodeIdx != 0 {
		t.Fatalf("expected selectedNodeIdx 0, got %d", m.selectedNodeIdx)
	}

	// Move down with 'down' arrow
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.selectedNodeIdx != 1 {
		t.Fatalf("expected selectedNodeIdx 1, got %d", m.selectedNodeIdx)
	}

	// Move down with Russian 'о' ('j')
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'о'}})
	if m.selectedNodeIdx != 2 {
		t.Fatalf("expected selectedNodeIdx 2 after Russian 'о' (j), got %d", m.selectedNodeIdx)
	}

	// Move up with Russian 'л' ('k')
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'л'}})
	if m.selectedNodeIdx != 1 {
		t.Fatalf("expected selectedNodeIdx 1 after Russian 'л' (k), got %d", m.selectedNodeIdx)
	}

	// Mouse wheel scrolling
	initialScroll := m.nodeScrollOffset
	m.scrollNodeWheel(3)
	if m.nodeScrollOffset != initialScroll+3 {
		t.Fatalf("expected nodeScrollOffset to increment by 3, got %d", m.nodeScrollOffset)
	}
	m.scrollNodeWheel(-3)
	if m.nodeScrollOffset != initialScroll {
		t.Fatalf("expected nodeScrollOffset to return to %d, got %d", initialScroll, m.nodeScrollOffset)
	}
}

func TestSubscriptionCollapseAndExpand(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain

	// 2 subscriptions
	sub1 := &model.Subscription{
		ID:   "sub-alpha",
		Name: "Alpha VPN",
		Nodes: []*model.ProxyNode{
			{ID: "n1", Name: "Node 1", Protocol: "vless"},
			{ID: "n2", Name: "Node 2", Protocol: "vless"},
		},
	}
	sub2 := &model.Subscription{
		ID:   "sub-beta",
		Name: "Beta VPN",
		Nodes: []*model.ProxyNode{
			{ID: "n3", Name: "Node 3", Protocol: "vless"},
		},
	}
	m.state.Subscriptions = []*model.Subscription{sub1, sub2}
	m.clampNodeState()

	// Initial items: 2 headers + 3 nodes = 5 items total
	items := m.getServerListItems()
	if len(items) != 5 {
		t.Fatalf("expected 5 items initially, got %d", len(items))
	}
	if !items[0].IsHeader || items[0].Sub.ID != "sub-alpha" {
		t.Fatalf("expected first item to be header of sub-alpha")
	}

	// 1. Cursor is on index 0 (sub-alpha header) -> press 'e' to collapse
	m.selectedNodeIdx = 0
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !m.collapsedSubs["sub-alpha"] {
		t.Fatalf("expected sub-alpha to be collapsed")
	}
	// After collapse: 1 header (sub-alpha) + 1 header (sub-beta) + 1 node (sub-beta) = 3 items total
	itemsAfter := m.getServerListItems()
	if len(itemsAfter) != 3 {
		t.Fatalf("expected 3 items after collapsing sub-alpha, got %d", len(itemsAfter))
	}

	// 2. Press 'e' on sub-alpha header again -> expands back to 5 items
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.collapsedSubs["sub-alpha"] {
		t.Fatalf("expected sub-alpha to be expanded")
	}
	if len(m.getServerListItems()) != 5 {
		t.Fatalf("expected 5 items after expanding sub-alpha, got %d", len(m.getServerListItems()))
	}

	// 3. Move cursor to node inside sub-alpha (index 1) -> press Russian 'у' ('e')
	m.selectedNodeIdx = 1
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'у'}})
	if !m.collapsedSubs["sub-alpha"] {
		t.Fatalf("expected sub-alpha to collapse from inside a node using Russian 'у'")
	}
	// Cursor should have jumped to the sub-alpha header (index 0)
	if m.selectedNodeIdx != 0 {
		t.Fatalf("expected cursor to jump to header (0), got %d", m.selectedNodeIdx)
	}

	// 4. Shift+E ('E') -> collapse all / expand all toggle
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	// All should now be collapsed: 2 items total (2 headers)
	itemsCollapsedAll := m.getServerListItems()
	if len(itemsCollapsedAll) != 2 {
		t.Fatalf("expected 2 headers when all collapsed, got %d", len(itemsCollapsedAll))
	}

	// Shift+E again -> expands all
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	itemsExpandedAll := m.getServerListItems()
	if len(itemsExpandedAll) != 5 {
		t.Fatalf("expected 5 items when all expanded, got %d", len(itemsExpandedAll))
	}
}

func TestUptimeFormattingAndReconnection(t *testing.T) {
	// 1. Test formatUptime
	if s := formatUptime(45 * time.Second); s != "00m 45s" {
		t.Fatalf("expected '00m 45s', got %q", s)
	}
	if s := formatUptime(15*time.Minute + 30*time.Second); s != "15m 30s" {
		t.Fatalf("expected '15m 30s', got %q", s)
	}
	if s := formatUptime(2*time.Hour + 10*time.Minute + 5*time.Second); s != "02h 10m 05s" {
		t.Fatalf("expected '02h 10m 05s', got %q", s)
	}
	if s := formatUptime(-5 * time.Second); s != "00m 00s" {
		t.Fatalf("expected negative to clamp to '00m 00s', got %q", s)
	}

	// 2. Test GetConnectedAt does not return zero time
	m := createTestModel()
	connAt := m.engine.GetConnectedAt()
	if connAt.IsZero() {
		t.Fatalf("expected GetConnectedAt to not be zero")
	}

	// 3. Test that renderTopBar recovers a zero ConnectedAt
	m.state.IsConnected = true
	m.state.ConnectedAt = time.Time{} // zero time bug scenario
	_ = m.renderTopBar()
	if m.state.ConnectedAt.IsZero() {
		t.Fatalf("expected ConnectedAt to be recovered, but remained zero")
	}
	if time.Since(m.state.ConnectedAt) > 24*time.Hour {
		t.Fatalf("expected sane ConnectedAt, got %v", m.state.ConnectedAt)
	}
}

func TestMainViewBorderAlignmentWithLongHeader(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain
	m.width = 80
	m.height = 24

	// Add subscription with very long name and traffic that previously caused overflow
	longSub := &model.Subscription{
		ID:   "sub-long",
		Name: "177.1.202.124:2096-extremely-long-domain-name-that-overflows",
		Nodes: []*model.ProxyNode{
			{ID: "n1", Name: "tcp-tcp-pc", Protocol: "vless"},
		},
		Upload:   100 * 1024 * 1024 * 1024,
		Download: 110 * 1024 * 1024 * 1024,
		Total:    0,
	}
	m.state.Subscriptions = []*model.Subscription{longSub}
	m.selectedNodeIdx = 0 // Selected on the header row

	rendered := m.RenderMainView()
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w != m.width {
			t.Fatalf("line %d width %d != %d: %q", i, w, m.width, line)
		}
		// Check that the line ends with right border │
		stripped := ansi.Strip(line)
		if !strings.HasSuffix(stripped, "│") && !strings.HasSuffix(stripped, "╮") && !strings.HasSuffix(stripped, "╯") {
			t.Fatalf("line %d does not end with border char: %q", i, stripped)
		}
	}
}

func TestQuitModalPreservesSideBorders(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain
	m.width = 100
	m.height = 26
	m.confirmQuit = true
	m.state.IsConnected = true

	rendered := m.View()
	lines := strings.Split(rendered, "\n")
	if len(lines) != m.height {
		t.Fatalf("expected %d lines, got %d", m.height, len(lines))
	}

	// Modal starts around line 7 to line 17
	foundModal := false
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w != m.width {
			t.Fatalf("line %d width %d != %d", i, w, m.width)
		}
		stripped := ansi.Strip(line)
		if strings.Contains(stripped, "Confirm Exit") {
			foundModal = true
		}
		// On lines containing modal options, verify that the left and right outer borders still exist
		if strings.Contains(stripped, "Keep VPN in background") || strings.Contains(stripped, "Disconnect VPN") {
			if !strings.HasPrefix(stripped, "│") && !strings.HasPrefix(stripped, "╭") {
				t.Fatalf("line %d lost left outer border: %q", i, stripped)
			}
			if !strings.HasSuffix(stripped, "│") && !strings.HasSuffix(stripped, "╮") && !strings.HasSuffix(stripped, "╯") {
				t.Fatalf("line %d lost right outer border: %q", i, stripped)
			}
		}
	}
	if !foundModal {
		t.Fatalf("did not find Confirm Exit in rendered view")
	}
}

func TestLogsViewTabCyclingAndNavigation(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain

	// 1. Press "L" to open ViewLogs
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m = newModel.(*Model)
	if m.view != ViewLogs {
		t.Fatalf("expected view to be ViewLogs after pressing 'L', got %v", m.view)
	}
	if m.logActiveTab != LogTabMain {
		t.Fatalf("expected initial tab to be LogTabMain (0), got %v", m.logActiveTab)
	}

	// 2. Press Tab to cycle through tabs
	expectedTabs := []LogTab{LogTabCore, LogTabTunnel, LogTabSub, LogTabDaemon, LogTabMain}
	for i, exp := range expectedTabs {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = newModel.(*Model)
		if m.logActiveTab != exp {
			t.Fatalf("step %d: expected active tab %v after Tab, got %v", i+1, exp, m.logActiveTab)
		}
	}

	// 3. Jump directly with number keys
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = newModel.(*Model)
	if m.logActiveTab != LogTabTunnel {
		t.Fatalf("expected LogTabTunnel after pressing '3', got %v", m.logActiveTab)
	}

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m = newModel.(*Model)
	if m.logActiveTab != LogTabSub {
		t.Fatalf("expected LogTabSub after pressing '4', got %v", m.logActiveTab)
	}

	// 4. Press Shift+Tab to cycle backwards
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newModel.(*Model)
	if m.logActiveTab != LogTabTunnel {
		t.Fatalf("expected LogTabTunnel after Shift+Tab from Sub, got %v", m.logActiveTab)
	}

	// 5. Press Esc to return to ViewMain
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(*Model)
	if m.view != ViewMain {
		t.Fatalf("expected view to be ViewMain after Esc, got %v", m.view)
	}

	// 6. Test Cyrillic 'Д' ('L') opens logs
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Д'}})
	m = newModel.(*Model)
	if m.view != ViewLogs {
		t.Fatalf("expected view to be ViewLogs after pressing Cyrillic 'Д', got %v", m.view)
	}

	// 7. Press 'q' to return to ViewMain
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = newModel.(*Model)
	if m.view != ViewMain {
		t.Fatalf("expected view to be ViewMain after 'q', got %v", m.view)
	}
}

func TestSubscriptionProfileGeoRules(t *testing.T) {
	m := createTestModel()

	// Setup: Global has 1 rule (steam -> DIRECT)
	m.state.GeoRules = []*model.GeoRule{
		{Type: model.GeoTypeSite, Name: "steam", Action: model.GeoActionDirect},
	}

	// Setup: Subscription 1 has custom rules (youtube -> PROXY, ru -> DIRECT)
	subNode1 := &model.ProxyNode{ID: "sub1-node1", Name: "Sub1 Server", Address: "1.1.1.1", Port: 443}
	sub1 := &model.Subscription{
		ID:   "sub1",
		Name: "Panda VPN",
		Nodes: []*model.ProxyNode{subNode1},
		GeoRules: []*model.GeoRule{
			{Type: model.GeoTypeSite, Name: "youtube", Action: model.GeoActionProxy},
			{Type: model.GeoTypeSite, Name: "ru", Action: model.GeoActionDirect},
		},
	}

	// Setup: Subscription 2 has NO custom rules (empty)
	subNode2 := &model.ProxyNode{ID: "sub2-node1", Name: "EmptySub Server", Address: "2.2.2.2", Port: 443}
	sub2 := &model.Subscription{
		ID:    "sub2",
		Name:  "EmptySub",
		Nodes: []*model.ProxyNode{subNode2},
	}

	m.state.Subscriptions = []*model.Subscription{sub1, sub2}

	// 1. Verify model.ResolveGeoRules behavior:
	// - Node from sub1 uses sub1's rules
	rulesForSub1 := model.ResolveGeoRules(subNode1, m.state.Subscriptions, m.state.GeoRules)
	if len(rulesForSub1) != 2 {
		t.Fatalf("expected 2 rules for subNode1, got %d", len(rulesForSub1))
	}
	if rulesForSub1[0].Name != "youtube" {
		t.Fatalf("expected youtube rule, got %s", rulesForSub1[0].Name)
	}

	// - Node from sub2 (empty rules) falls back to global rules
	rulesForSub2 := model.ResolveGeoRules(subNode2, m.state.Subscriptions, m.state.GeoRules)
	if len(rulesForSub2) != 1 || rulesForSub2[0].Name != "steam" {
		t.Fatalf("expected fallback to global steam rule for subNode2, got %v", rulesForSub2)
	}

	// 2. Open Geo Rules UI -> starts in Level 0
	m.view = ViewGeoRules
	m.geoInProfileView = false
	m.geoInActionView = false
	m.geoProfileIdx = 0

	// Level 0: Cursor moves down to select [1] Panda VPN
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.geoProfileIdx != 1 {
		t.Fatalf("expected geoProfileIdx to be 1 (Panda VPN), got %d", m.geoProfileIdx)
	}
	if m.getActiveProfileName() != "Panda VPN" {
		t.Fatalf("expected active profile 'Panda VPN', got %q", m.getActiveProfileName())
	}

	// Press Enter to enter Level 1 (Action Menu) for Panda VPN
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.geoInProfileView || m.geoInActionView {
		t.Fatalf("expected Level 1 Action Menu for Panda VPN")
	}

	// Enter DIRECT group (key '1') -> enters Level 2 Action View
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if !m.geoInActionView || m.geoActionIdx != 0 {
		t.Fatalf("expected Level 2 Action View for DIRECT")
	}

	// Verify the rules displayed are from sub1 (ru rule), not global (steam)
	activeRules := m.getActiveProfileRules()
	if len(activeRules) != 2 {
		t.Fatalf("expected 2 rules in sub1, got %d", len(activeRules))
	}

	// Delete 'ru' rule from Panda VPN using DEL key
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDelete})
	if len(sub1.GeoRules) != 1 {
		t.Fatalf("expected 1 rule in sub1 after deletion, got %d", len(sub1.GeoRules))
	}
	// Verify Global rules were not touched!
	if len(m.state.GeoRules) != 1 || m.state.GeoRules[0].Name != "steam" {
		t.Fatalf("global rules were unexpectedly modified: %v", m.state.GeoRules)
	}

	// Press 'q' to go back to Level 1
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.geoInActionView || !m.geoInProfileView {
		t.Fatalf("expected back in Level 1")
	}

	// Press 'q' to go back to Level 0
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.geoInProfileView {
		t.Fatalf("expected back in Level 0")
	}

	// Render Level 0 and verify solid black background
	viewOutput := m.RenderGeoView()
	if !strings.Contains(viewOutput, "Panda VPN") {
		t.Fatalf("expected viewOutput to contain 'Panda VPN'")
	}
	if !strings.Contains(viewOutput, "Global / Default Rules") {
		t.Fatalf("expected viewOutput to contain 'Global / Default Rules'")
	}
}

func TestSubscriptionMoveAndDeletion(t *testing.T) {
	m := createTestModel()
	m.view = ViewMain

	// Setup: 3 subscriptions
	sub1Node := &model.ProxyNode{ID: "sub1-n1", Name: "Node 1", Address: "1.1.1.1", Port: 443}
	sub1 := &model.Subscription{ID: "sub1", Name: "Alpha Sub", Nodes: []*model.ProxyNode{sub1Node}}

	sub2Node := &model.ProxyNode{ID: "sub2-n1", Name: "Node 2", Address: "2.2.2.2", Port: 443}
	sub2 := &model.Subscription{ID: "sub2", Name: "Beta Sub", Nodes: []*model.ProxyNode{sub2Node}}

	sub3Node := &model.ProxyNode{ID: "sub3-n1", Name: "Node 3", Address: "3.3.3.3", Port: 443}
	sub3 := &model.Subscription{ID: "sub3", Name: "Gamma Sub", Nodes: []*model.ProxyNode{sub3Node}}

	m.state.Subscriptions = []*model.Subscription{sub1, sub2, sub3}
	m.state.ActiveNode = sub2Node
	m.state.IsConnected = true
	m.clampNodeState()

	// Initial list:
	// row 0: Header Alpha Sub
	// row 1: Node 1
	// row 2: Header Beta Sub
	// row 3: Node 2
	// row 4: Header Gamma Sub
	// row 5: Node 3

	// Select Header Beta Sub (row 2)
	m.selectedNodeIdx = 2

	// 1. Move Beta Sub UP using key 'K' (Shift+k)
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if m.state.Subscriptions[0].Name != "Beta Sub" || m.state.Subscriptions[1].Name != "Alpha Sub" {
		t.Fatalf("expected Beta Sub to move up to index 0, got %s, %s",
			m.state.Subscriptions[0].Name, m.state.Subscriptions[1].Name)
	}

	// 2. Move Beta Sub DOWN using key 'J' (Shift+j)
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if m.state.Subscriptions[0].Name != "Alpha Sub" || m.state.Subscriptions[1].Name != "Beta Sub" {
		t.Fatalf("expected Beta Sub to move down back to index 1, got %s, %s",
			m.state.Subscriptions[0].Name, m.state.Subscriptions[1].Name)
	}

	// 3. Move Beta Sub DOWN again using Cyrillic 'О' ('J')
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'О'}})
	if m.state.Subscriptions[2].Name != "Beta Sub" {
		t.Fatalf("expected Beta Sub to move to index 2 with Cyrillic 'О', got %s",
			m.state.Subscriptions[2].Name)
	}

	// 4. Test deleting Beta Sub using key 'd' on header
	// Since ActiveNode was sub2Node (from Beta Sub) and connected, it should disconnect
	m.selectSubscriptionHeader("sub2")
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if len(m.state.Subscriptions) != 2 {
		t.Fatalf("expected 2 subscriptions remaining, got %d", len(m.state.Subscriptions))
	}
	if m.state.Subscriptions[0].Name != "Alpha Sub" || m.state.Subscriptions[1].Name != "Gamma Sub" {
		t.Fatalf("unexpected subscriptions after deletion: %s, %s",
			m.state.Subscriptions[0].Name, m.state.Subscriptions[1].Name)
	}
	if m.state.IsConnected {
		t.Fatalf("expected VPN to be disconnected when active subscription is deleted")
	}

	// 5. Test safe tip when pressing lowercase 'd' on a node (row 1 is Node 1)
	m.selectedNodeIdx = 1
	items := m.getServerListItems()
	if items[1].IsHeader {
		t.Fatalf("expected row 1 to be a node")
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	// Should NOT delete Alpha Sub on casual 'd' on a node
	if len(m.state.Subscriptions) != 2 {
		t.Fatalf("casual 'd' on a node should not delete subscription, got %d", len(m.state.Subscriptions))
	}
	if !strings.Contains(m.statusMessage, "Tip:") {
		t.Fatalf("expected status tip, got %q", m.statusMessage)
	}

	// 6. Test Shift+D on a node DOES delete the subscription
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if len(m.state.Subscriptions) != 1 || m.state.Subscriptions[0].Name != "Gamma Sub" {
		t.Fatalf("expected Shift+D on node to delete Alpha Sub, remaining: %d", len(m.state.Subscriptions))
	}
}

func TestSettingsScreenSectionsRendering(t *testing.T) {
	m := createTestModel()
	m.view = ViewSettings

	rendered := m.RenderSettingsView()
	if !strings.Contains(rendered, "[1] APP SETTINGS") {
		t.Errorf("expected rendered settings to contain '[1] APP SETTINGS'")
	}
	if !strings.Contains(rendered, "[2] TUNNEL SETTINGS") {
		t.Errorf("expected rendered settings to contain '[2] TUNNEL SETTINGS'")
	}
	if !strings.Contains(rendered, "Theme & Color Studio") {
		t.Errorf("expected rendered settings to contain 'Theme & Color Studio'")
	}
	if !strings.Contains(rendered, "Preferred IP Type") {
		t.Errorf("expected rendered settings to contain 'Preferred IP Type'")
	}
	if !strings.Contains(rendered, "TLS Packet Fragmentation") {
		t.Errorf("expected rendered settings to contain 'TLS Packet Fragmentation'")
	}
	if !strings.Contains(rendered, "TCP/UDP Multiplexing (Mux)") {
		t.Errorf("expected rendered settings to contain 'TCP/UDP Multiplexing (Mux)'")
	}
	if !strings.Contains(rendered, "Advanced Settings") {
		t.Errorf("expected rendered settings to contain 'Advanced Settings'")
	}
}

func TestAdvancedSettingsTogglesAndEditing(t *testing.T) {
	m := createTestModel()
	m.view = ViewSettings
	m.settingsCursor = 8

	// 1. Enter ViewAdvancedSettings by pressing Enter
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.view != ViewAdvancedSettings {
		t.Fatalf("expected ViewAdvancedSettings, got %v", m.view)
	}

	// 2. Initial state: UseLocalDNS is false -> press Enter to toggle to true
	if m.state.Advanced.UseLocalDNS {
		t.Fatalf("expected UseLocalDNS to be false initially")
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.state.Advanced.UseLocalDNS {
		t.Fatalf("expected UseLocalDNS to be true after Enter")
	}

	// 3. Move to cursor 1 (UseDNSFromJSON) -> press Space to toggle
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.advCursor != 1 {
		t.Fatalf("expected advCursor 1, got %d", m.advCursor)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if !m.state.Advanced.UseDNSFromJSON {
		t.Fatalf("expected UseDNSFromJSON to be true after Space")
	}

	// 4. Move to cursor 2 (EnableResolveServer) -> toggle off
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.advCursor != 2 {
		t.Fatalf("expected advCursor 2, got %d", m.advCursor)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Advanced.EnableResolveServer {
		t.Fatalf("expected EnableResolveServer to be false after toggle")
	}

	// 5. Move to cursor 3 (ResolveServerIP) -> press 'e' to edit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.advCursor != 3 {
		t.Fatalf("expected advCursor 3, got %d", m.advCursor)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !m.advInputActive {
		t.Fatalf("expected advInputActive to be true after 'e'")
	}
	m.advTextInput.SetValue("1.0.0.1")
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.advInputActive {
		t.Fatalf("expected advInputActive to be false after Enter")
	}
	if m.state.Advanced.ResolveServerIP != "1.0.0.1" {
		t.Fatalf("expected ResolveServerIP '1.0.0.1', got %q", m.state.Advanced.ResolveServerIP)
	}

	// 6. Move to cursor 4 (ResolveServerDomain) -> press Enter to edit
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.advCursor != 4 {
		t.Fatalf("expected advCursor 4, got %d", m.advCursor)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.advInputActive {
		t.Fatalf("expected advInputActive to be true after Enter")
	}
	m.advTextInput.SetValue("https://cloudflare-dns.com/dns-query")
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state.Advanced.ResolveServerDomain != "https://cloudflare-dns.com/dns-query" {
		t.Fatalf("expected ResolveServerDomain to be updated, got %q", m.state.Advanced.ResolveServerDomain)
	}

	// 7. Move to cursor 5 (EnableSniffing) -> toggle
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.advCursor != 5 {
		t.Fatalf("expected advCursor 5, got %d", m.advCursor)
	}
	initSniff := m.state.Advanced.EnableSniffing
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Advanced.EnableSniffing == initSniff {
		t.Fatalf("expected EnableSniffing to toggle")
	}

	// 8. Press 'q' to return to ViewSettings
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.view != ViewSettings {
		t.Fatalf("expected ViewSettings after 'q', got %v", m.view)
	}
	if m.settingsCursor != 8 {
		t.Fatalf("expected settingsCursor to be 8, got %d", m.settingsCursor)
	}
}

func TestFragmentationStudioControls(t *testing.T) {
	m := createTestModel()
	m.view = ViewSettings
	m.settingsCursor = 6

	// 1. Test quick Space toggle from ViewSettings
	if m.state.Fragmentation.Enabled {
		t.Fatalf("expected Fragmentation to be disabled initially")
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if !m.state.Fragmentation.Enabled {
		t.Fatalf("expected Fragmentation to be enabled after Space")
	}

	// 2. Press Enter to enter ViewFragmentation
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.view != ViewFragmentation {
		t.Fatalf("expected ViewFragmentation, got %v", m.view)
	}

	// 3. Cursor 0: Toggle with Space
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Fragmentation.Enabled {
		t.Fatalf("expected Fragmentation to be disabled after Space inside ViewFragmentation")
	}

	// 4. Cursor 1: Cycle packets preset
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.fragCursor != 1 {
		t.Fatalf("expected fragCursor 1, got %d", m.fragCursor)
	}
	// Initial packets is tlshello -> cycle to 1-3
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Fragmentation.Packets != "1-3" {
		t.Fatalf("expected packets '1-3', got %q", m.state.Fragmentation.Packets)
	}
	// Cycle to 1-5
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Fragmentation.Packets != "1-5" {
		t.Fatalf("expected packets '1-5', got %q", m.state.Fragmentation.Packets)
	}

	// 5. Cursor 2: Cycle length preset
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.fragCursor != 2 {
		t.Fatalf("expected fragCursor 2, got %d", m.fragCursor)
	}
	// Initial length 100-200 -> cycle to 50-100
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Fragmentation.Length != "50-100" {
		t.Fatalf("expected length '50-100', got %q", m.state.Fragmentation.Length)
	}

	// 6. Cursor 3: Cycle interval preset
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.fragCursor != 3 {
		t.Fatalf("expected fragCursor 3, got %d", m.fragCursor)
	}
	// Initial interval 10-20 -> cycle to 5-10
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Fragmentation.Interval != "5-10" {
		t.Fatalf("expected interval '5-10', got %q", m.state.Fragmentation.Interval)
	}

	// 7. Reset defaults with 'r'
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m.state.Fragmentation.Packets != "tlshello" || m.state.Fragmentation.Length != "100-200" || m.state.Fragmentation.Interval != "10-20" {
		t.Fatalf("expected default fragmentation after 'r', got %+v", m.state.Fragmentation)
	}

	// 8. Press Esc to return to ViewSettings
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if m.view != ViewSettings {
		t.Fatalf("expected ViewSettings after Esc, got %v", m.view)
	}
}

func TestMuxStudioControls(t *testing.T) {
	m := createTestModel()
	m.view = ViewSettings
	m.settingsCursor = 7

	// 1. Quick Space toggle from ViewSettings
	if m.state.Mux.Enabled {
		t.Fatalf("expected Mux to be disabled initially")
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if !m.state.Mux.Enabled {
		t.Fatalf("expected Mux to be enabled after Space")
	}

	// 2. Press Enter to enter ViewMux
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.view != ViewMux {
		t.Fatalf("expected ViewMux, got %v", m.view)
	}

	// 3. Move to cursor 1 (Concurrency): test left/right and H/L
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.muxCursor != 1 {
		t.Fatalf("expected muxCursor 1, got %d", m.muxCursor)
	}
	initConc := m.state.Mux.Concurrency
	// Right (+1)
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRight})
	if m.state.Mux.Concurrency != initConc+1 {
		t.Fatalf("expected concurrency %d, got %d", initConc+1, m.state.Mux.Concurrency)
	}
	// 'L' (+8)
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if m.state.Mux.Concurrency != initConc+9 {
		t.Fatalf("expected concurrency %d, got %d", initConc+9, m.state.Mux.Concurrency)
	}

	// 4. Move to cursor 2 (XUDP Concurrency)
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.muxCursor != 2 {
		t.Fatalf("expected muxCursor 2, got %d", m.muxCursor)
	}
	initXUDP := m.state.Mux.XUDPConcurrency
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRight})
	if m.state.Mux.XUDPConcurrency != initXUDP+1 {
		t.Fatalf("expected xudp %d, got %d", initXUDP+1, m.state.Mux.XUDPConcurrency)
	}

	// 5. Move to cursor 3 (UDP 443 Policy): cycle reject -> allow -> skip
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if m.muxCursor != 3 {
		t.Fatalf("expected muxCursor 3, got %d", m.muxCursor)
	}
	if m.state.Mux.XUDPProxyUDP443 != "reject" {
		t.Fatalf("expected default 'reject', got %q", m.state.Mux.XUDPProxyUDP443)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Mux.XUDPProxyUDP443 != "allow" {
		t.Fatalf("expected 'allow', got %q", m.state.Mux.XUDPProxyUDP443)
	}
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.Mux.XUDPProxyUDP443 != "skip" {
		t.Fatalf("expected 'skip', got %q", m.state.Mux.XUDPProxyUDP443)
	}

	// 6. Reset with 'r'
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m.state.Mux.Concurrency != 8 || m.state.Mux.XUDPConcurrency != 16 || m.state.Mux.XUDPProxyUDP443 != "reject" {
		t.Fatalf("expected defaults after 'r', got %+v", m.state.Mux)
	}

	// 7. Exit with 'q'
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.view != ViewSettings {
		t.Fatalf("expected ViewSettings after 'q', got %v", m.view)
	}
}

func TestIPTypePreferenceCycling(t *testing.T) {
	m := createTestModel()
	m.view = ViewSettings
	m.settingsCursor = 4

	// Starts at prefer_ipv4
	if m.state.IPType != model.IPTypePreferIPv4 && m.state.IPType != "" {
		t.Fatalf("expected initial prefer_ipv4, got %v", m.state.IPType)
	}

	// Press Enter: prefer_ipv4 -> prefer_ipv6
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state.IPType != model.IPTypePreferIPv6 {
		t.Fatalf("expected prefer_ipv6, got %v", m.state.IPType)
	}

	// Press Space: prefer_ipv6 -> ipv4_only
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.IPType != model.IPTypeIPv4Only {
		t.Fatalf("expected ipv4_only, got %v", m.state.IPType)
	}

	// Press Space: ipv4_only -> ipv6_only
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.IPType != model.IPTypeIPv6Only {
		t.Fatalf("expected ipv6_only, got %v", m.state.IPType)
	}

	// Press Space: ipv6_only -> prefer_ipv4
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if m.state.IPType != model.IPTypePreferIPv4 {
		t.Fatalf("expected prefer_ipv4, got %v", m.state.IPType)
	}
}

func TestThemeStudioReworkSectionsAndControls(t *testing.T) {
	m := createTestModel()
	m.view = ViewTheme
	m.themeSection = 0
	m.themeCursor = 0
	m.themeSliderActive = false

	// 1. Verify 3 sections cycling via Tab and 1..3
	if m.themeSection != 0 {
		t.Fatalf("expected initial section 0, got %d", m.themeSection)
	}
	_, _ = m.handleThemeKeys("tab")
	if m.themeSection != 1 {
		t.Fatalf("expected section 1 after Tab, got %d", m.themeSection)
	}
	_, _ = m.handleThemeKeys("tab")
	if m.themeSection != 2 {
		t.Fatalf("expected section 2 after Tab, got %d", m.themeSection)
	}
	_, _ = m.handleThemeKeys("tab")
	if m.themeSection != 0 {
		t.Fatalf("expected section 0 after 3rd Tab, got %d", m.themeSection)
	}
	_, _ = m.handleThemeKeys("2")
	if m.themeSection != 1 {
		t.Fatalf("expected section 1 after '2', got %d", m.themeSection)
	}
	_, _ = m.handleThemeKeys("3")
	if m.themeSection != 2 {
		t.Fatalf("expected section 2 after '3', got %d", m.themeSection)
	}
	_, _ = m.handleThemeKeys("1")
	if m.themeSection != 0 {
		t.Fatalf("expected section 0 after '1', got %d", m.themeSection)
	}

	// 2. Section 0: Edit Gradient Base Color (index 7)
	m.themeCursor = 7
	elems := m.getActiveThemeElements()
	if elems[m.themeCursor].ID != "grad_color" {
		t.Fatalf("expected index 7 to be grad_color, got %s", elems[m.themeCursor].ID)
	}
	_, _ = m.handleThemeKeys("enter")
	if !m.themeSliderActive {
		t.Fatalf("expected slider to be active")
	}
	// Adjust Red slider (+10)
	initR := m.themeR
	m.handleThemeKeys("]")
	if m.themeR != initR+10 {
		t.Fatalf("expected themeR %d, got %d", initR+10, m.themeR)
	}
	expectedGradHex := rgbToHex(m.themeR, m.themeG, m.themeB)
	if string(ColorGradBase) != expectedGradHex {
		t.Fatalf("expected ColorGradBase %s, got %s", expectedGradHex, string(ColorGradBase))
	}
	// Exit slider mode
	_, _ = m.handleThemeKeys("enter")
	if m.themeSliderActive {
		t.Fatalf("expected slider to be inactive after Enter")
	}

	// 3. Section 0: Edit Gradient Intensity (index 8)
	m.themeCursor = 8
	elems = m.getActiveThemeElements()
	if elems[m.themeCursor].ID != "grad_intensity" || !elems[m.themeCursor].IsIntensity {
		t.Fatalf("expected index 8 to be grad_intensity, got %v", elems[m.themeCursor])
	}
	_, _ = m.handleThemeKeys("enter")
	if !m.themeSliderActive {
		t.Fatalf("expected slider active for intensity")
	}
	initInt := ThemeGradIntensity
	m.handleThemeKeys("right")
	if ThemeGradIntensity != initInt+1 {
		t.Fatalf("expected intensity %d, got %d", initInt+1, ThemeGradIntensity)
	}
	m.handleThemeKeys("r")
	if ThemeGradIntensity != 5 {
		t.Fatalf("expected intensity reset to 5, got %d", ThemeGradIntensity)
	}
	_, _ = m.handleThemeKeys("enter")

	// 4. Section 1 (Interface Elements): Edit border_active (index 2)
	m.themeSection = 1
	m.themeCursor = 2
	elems = m.getActiveThemeElements()
	if elems[m.themeCursor].ID != "border_active" {
		t.Fatalf("expected border_active at index 2, got %s", elems[m.themeCursor].ID)
	}
	_, _ = m.handleThemeKeys("enter")
	m.handleThemeKeys("right") // +1
	expectedBorderHex := rgbToHex(m.themeR, m.themeG, m.themeB)
	if string(ColorBorderActive) != expectedBorderHex {
		t.Fatalf("expected ColorBorderActive %s, got %s", expectedBorderHex, string(ColorBorderActive))
	}
	_, _ = m.handleThemeKeys("esc")

	// 5. Section 2 (Logs Viewer): Edit log_error (index 4)
	m.themeSection = 2
	m.themeCursor = 4
	elems = m.getActiveThemeElements()
	if elems[m.themeCursor].ID != "log_error" {
		t.Fatalf("expected log_error at index 4, got %s", elems[m.themeCursor].ID)
	}
	_, _ = m.handleThemeKeys("enter")
	m.handleThemeKeys("left") // -1
	expectedLogErrHex := rgbToHex(m.themeR, m.themeG, m.themeB)
	if string(ColorLogError) != expectedLogErrHex {
		t.Fatalf("expected ColorLogError %s, got %s", expectedLogErrHex, string(ColorLogError))
	}
	_, _ = m.handleThemeKeys("esc")

	// 6. Reset item to default with 'r'
	m.handleThemeKeys("r")
	if string(ColorLogError) != DefaultThemeColors["log_error"] {
		t.Fatalf("expected ColorLogError reset to %s, got %s", DefaultThemeColors["log_error"], string(ColorLogError))
	}

	// 7. Factory Reset ALL with 'R'
	m.handleThemeKeys("R")
	if string(ColorGradBase) != DefaultThemeColors["grad_color"] {
		t.Fatalf("expected ColorGradBase factory reset to %s, got %s", DefaultThemeColors["grad_color"], string(ColorGradBase))
	}
	if ThemeGradIntensity != 5 {
		t.Fatalf("expected ThemeGradIntensity 5 after factory reset, got %d", ThemeGradIntensity)
	}
	if string(ColorBorderActive) != DefaultThemeColors["border_active"] {
		t.Fatalf("expected ColorBorderActive reset to %s, got %s", DefaultThemeColors["border_active"], string(ColorBorderActive))
	}

	// 8. Render view in all 3 sections to ensure no panic and live previews are present
	for sec := 0; sec < 3; sec++ {
		m.themeSection = sec
		m.themeCursor = 0
		rendered := m.RenderThemeView()
		if !strings.Contains(rendered, "Theme Studio") {
			t.Fatalf("expected section %d to contain 'Theme Studio'", sec)
		}
		if !strings.Contains(rendered, "Live Preview") && !strings.Contains(rendered, "Current Palette Preview") {
			t.Fatalf("expected section %d to contain preview section", sec)
		}
	}
}



