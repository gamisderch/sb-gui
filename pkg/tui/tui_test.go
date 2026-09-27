package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"signbox-cli/pkg/core"
	"signbox-cli/pkg/model"
	"signbox-cli/pkg/process"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)


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
	views := []ViewType{ViewMain, ViewProc, ViewSettings, ViewTheme, ViewGeoRules}
	viewNames := map[ViewType]string{
		ViewMain:     "ViewMain",
		ViewProc:     "ViewProc",
		ViewSettings: "ViewSettings",
		ViewTheme:    "ViewTheme",
		ViewGeoRules: "ViewGeoRules",
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
	m.settingsCursor = 3

	// Press "enter" to open Geo Rules -> starts at Level 1 Menu
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if m.view != ViewGeoRules {
		t.Fatalf("expected ViewGeoRules, got %v", m.view)
	}
	if m.geoInActionView {
		t.Fatalf("expected to start in Level 1 menu, not action view")
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

	// Delete rule
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if len(m.state.GeoRules) != 0 {
		t.Fatalf("expected 0 geo rules after delete, got %d", len(m.state.GeoRules))
	}

	// Press "q" to return to Level 1 Menu
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.geoInActionView {
		t.Fatalf("expected geoInActionView false")
	}

	// Press "q" on Level 1 Menu to return to ViewSettings
	_, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.view != ViewSettings {
		t.Fatalf("expected ViewSettings after 'q' on menu, got %v", m.view)
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





