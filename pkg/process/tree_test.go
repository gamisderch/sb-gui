package process

import (
	"testing"
)

func TestExtractCleanName(t *testing.T) {
	pm := NewProcessManager()

	name := pm.extractCleanName("/proc/nonexistent", "bash")
	if name != "bash" {
		t.Errorf("expected bash, got %s", name)
	}

	if !isKnownSystemDaemon("systemd") {
		t.Errorf("expected systemd to be recognized as system daemon")
	}
	if isKnownSystemDaemon("cyberpunk2077.exe") {
		t.Errorf("cyberpunk2077.exe should not be recognized as system daemon")
	}
}

func TestProcessTreeRefresh(t *testing.T) {
	pm := NewProcessManager()
	vpn, bypass, err := pm.Refresh()
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(vpn) == 0 && len(bypass) == 0 {
		t.Errorf("expected at least some processes on Linux system")
	}
}

func TestToggleBypassMovesBetweenTrees(t *testing.T) {
	pm := NewProcessManager()
	vpn, _, err := pm.Refresh()
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(vpn) == 0 {
		t.Skip("no vpn processes found to test")
	}

	// Pick a process from VPN
	targetNode := vpn[0]
	targetPID := targetNode.PID
	targetName := targetNode.CleanName

	// Toggle it to Bypass
	pm.ToggleBypass(targetNode, false)

	// Refresh
	_, bypassAfter, err := pm.Refresh()
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify target is now in Bypass
	foundInBypass := false
	for _, node := range bypassAfter {
		if node.PID == targetPID || node.CleanName == targetName {
			foundInBypass = true
			break
		}
	}
	if !foundInBypass {
		t.Fatalf("expected node %s (PID %d) to be in bypass tree after toggle, but was not found", targetName, targetPID)
	}

	// Toggle it back to VPN
	// Find the node in bypass
	for _, node := range bypassAfter {
		if node.PID == targetPID || node.CleanName == targetName {
			pm.ToggleBypass(node, false)
			break
		}
	}

	vpnBack, bypassBack, _ := pm.Refresh()
	foundInBypassAgain := false
	for _, node := range bypassBack {
		if node.PID == targetPID || node.CleanName == targetName {
			foundInBypassAgain = true
			break
		}
	}
	if foundInBypassAgain {
		t.Fatalf("expected node %s to be removed from bypass tree after toggling back", targetName)
	}
	if len(vpnBack) == 0 {
		t.Fatalf("expected vpn tree to have nodes")
	}
}

func TestWineBypassAliases(t *testing.T) {
	pm := NewProcessManager()
	pm.BypassNames["FSD-Win64-Shipping.exe"] = true

	list := pm.GetBypassNamesList()
	expected := map[string]bool{
		"FSD-Win64-Shipping.exe": true,
		"FSD-Win64-Shipping":     true,
		"FSD-Win64-Shipp":        true, // truncated to 15 chars (Linux comm)
		"wine64-preloader":       true,
		"wineserver":             true,
	}

	for exp := range expected {
		found := false
		for _, item := range list {
			if item == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in bypass list for Wine game, but not found in %v", exp, list)
		}
	}
}

func TestOfflineBypassProcessesRemainVisible(t *testing.T) {
	pm := NewProcessManager()
	// Add nonexistent / closed processes to BypassNames
	pm.BypassNames["cs2.exe"] = true
	pm.BypassNames["nonexistent_game_12345.exe"] = true

	_, bypassRoots, err := pm.Refresh()
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	foundCS2 := false
	foundNonexistent := false
	for _, node := range bypassRoots {
		if node.CleanName == "cs2.exe" {
			foundCS2 = true
		}
		if node.CleanName == "nonexistent_game_12345.exe" {
			foundNonexistent = true
			if node.PID > 0 {
				t.Errorf("expected offline node to have PID <= 0, got %d", node.PID)
			}
			if !node.IsBypass {
				t.Errorf("expected offline node to have IsBypass = true")
			}
		}
	}

	if !foundCS2 {
		t.Errorf("expected cs2.exe to be present in bypassRoots even if offline")
	}
	if !foundNonexistent {
		t.Errorf("expected nonexistent_game_12345.exe to be present in bypassRoots")
	}
}

func TestTreeCollapseCopies(t *testing.T) {
	pm := NewProcessManager()
	pm.Collapse = true

	// Build a deep linear chain like the user's cs2.exe (7 instances)
	root := &ProcessNode{PID: 284279, CleanName: "cs2.exe", Sockets: 31}
	ch1 := &ProcessNode{PID: 284315, CleanName: "cs2.exe"}
	ch2 := &ProcessNode{PID: 284317, CleanName: "cs2.exe"}
	ch3 := &ProcessNode{PID: 284318, CleanName: "cs2.exe"}
	ch4 := &ProcessNode{PID: 284372, CleanName: "cs2.exe"}
	ch5 := &ProcessNode{PID: 284413, CleanName: "cs2.exe"}
	ch6 := &ProcessNode{PID: 284508, CleanName: "cs2.exe", Sockets: 55}

	ch4.Children = []*ProcessNode{ch5, ch6}
	ch3.Children = []*ProcessNode{ch4}
	ch2.Children = []*ProcessNode{ch3}
	ch1.Children = []*ProcessNode{ch2}
	root.Children = []*ProcessNode{ch1}

	filtered := pm.filterTree([]*ProcessNode{root})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 root, got %d", len(filtered))
	}
	resRoot := filtered[0]
	if resRoot.ChildCount != 7 {
		t.Fatalf("expected ChildCount 7, got %d", resRoot.ChildCount)
	}
	if !resRoot.IsCollapsed {
		t.Fatalf("expected IsCollapsed true")
	}
	if len(resRoot.Children) != 0 {
		t.Fatalf("expected 0 children under collapsed same-name root, got %d", len(resRoot.Children))
	}
	if resRoot.Sockets != 86 {
		t.Fatalf("expected sum of sockets 86, got %d", resRoot.Sockets)
	}

	// Test uncollapse
	pm.Collapse = false
	uncollapsed := pm.filterTree([]*ProcessNode{root})
	if len(uncollapsed) != 1 {
		t.Fatalf("expected 1 root")
	}
	if uncollapsed[0].ChildCount != 0 {
		t.Fatalf("expected ChildCount 0 when uncollapsed, got %d", uncollapsed[0].ChildCount)
	}
	if uncollapsed[0].IsCollapsed {
		t.Fatalf("expected IsCollapsed false when uncollapsed")
	}
}
