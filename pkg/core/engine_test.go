package core

import (
	"encoding/json"
	"testing"

	"signbox-cli/pkg/model"
)

func TestGenerateSingboxConfig(t *testing.T) {
	e := NewEngine()
	node := &model.ProxyNode{
		ID:       "test-node",
		Name:     "Test Node",
		Protocol: "vless",
		Address:  "198.51.100.1",
		Port:     443,
	}

	bypassList := []string{
		"FSD-Win64-Shipping.exe",
		"wine64-preloader",
		"wineserver",
		"steam",
	}

	geoRules := []*model.GeoRule{
		{Type: model.GeoTypeSite, Name: "category-ads-all", Action: model.GeoActionBlock},
		{Type: model.GeoTypeSite, Name: "ru", Action: model.GeoActionDirect},
		{Type: model.GeoTypeSite, Name: "youtube", Action: model.GeoActionProxy},
	}

	cfgBytes, err := e.generateSingboxConfig(node, bypassList, geoRules)
	if err != nil {
		t.Fatalf("generateSingboxConfig failed: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		t.Fatalf("failed to parse generated sing-box config JSON: %v", err)
	}

	// 1. Verify route block contains find_process: true and auto_detect_interface: true
	route, ok := cfg["route"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing route block in config")
	}

	if findProc, ok := route["find_process"].(bool); !ok || !findProc {
		t.Errorf("expected find_process: true in route block, got %v", route["find_process"])
	}

	if autoDetect, ok := route["auto_detect_interface"].(bool); !ok || !autoDetect {
		t.Errorf("expected auto_detect_interface: true in route block, got %v", route["auto_detect_interface"])
	}

	// 2. Verify rules
	rules, ok := route["rules"].([]interface{})
	if !ok || len(rules) < 4 {
		t.Fatalf("expected rules in route block, got %v", route["rules"])
	}

	// Rule 0 MUST be process_name -> direct for bypass list
	rule0, ok := rules[0].(map[string]interface{})
	if !ok {
		t.Fatalf("rule 0 is not a valid map")
	}

	if rule0["outbound"] != "direct" {
		t.Errorf("expected rule 0 outbound to be 'direct', got %v", rule0["outbound"])
	}

	procNames, ok := rule0["process_name"].([]interface{})
	if !ok || len(procNames) == 0 {
		t.Errorf("expected process_name list in rule 0, got %v", rule0["process_name"])
	}

	// Verify DNS hijack is present
	foundHijack := false
	for _, r := range rules {
		if rMap, ok := r.(map[string]interface{}); ok {
			if rMap["action"] == "hijack-dns" {
				foundHijack = true
				break
			}
		}
	}
	if !foundHijack {
		t.Errorf("expected hijack-dns rule in rules, got %v", rules)
	}

	// 3. Verify inbounds exclude 127.0.0.0/8 and node IP
	inbounds, ok := cfg["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		t.Fatalf("missing inbounds in config")
	}
	tunIn, ok := inbounds[0].(map[string]interface{})
	if !ok {
		t.Fatalf("inbound 0 is not a map")
	}
	excludes, ok := tunIn["route_exclude_address"].([]interface{})
	if !ok {
		t.Fatalf("missing route_exclude_address in tun inbound")
	}

	foundLoopback := false
	foundNode := false
	for _, ex := range excludes {
		s, _ := ex.(string)
		if s == "127.0.0.0/8" {
			foundLoopback = true
		}
		if s == "198.51.100.1/32" {
			foundNode = true
		}
	}
	if !foundLoopback {
		t.Errorf("expected 127.0.0.0/8 in route_exclude_address, got %v", excludes)
	}
	if !foundNode {
		t.Errorf("expected 198.51.100.1/32 in route_exclude_address, got %v", excludes)
	}

	// 4. Verify outbounds direct and ensure no bind_interface pointing to tun
	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok || len(outbounds) < 2 {
		t.Fatalf("expected at least 2 outbounds, got %v", cfg["outbounds"])
	}

	for _, ob := range outbounds {
		obMap, ok := ob.(map[string]interface{})
		if !ok {
			continue
		}
		if obMap["tag"] == "direct" {
			if bindIface, ok := obMap["bind_interface"].(string); ok && isVirtualInterface(bindIface) {
				t.Errorf("direct outbound must NOT bind to virtual interface %q", bindIface)
			}
		}
	}
}

func TestVirtualInterfaceDetection(t *testing.T) {
	virtuals := []string{"tun0", "tun1", "tap0", "lo", "docker0", "virbr0", "vmnet1", "wg0", "tailscale0"}
	for _, v := range virtuals {
		if !isVirtualInterface(v) {
			t.Errorf("expected %q to be identified as virtual interface", v)
		}
	}

	physicals := []string{"enp2s0", "eth0", "wlan0", "wlp3s0", "enx001122334455"}
	for _, p := range physicals {
		if isVirtualInterface(p) {
			t.Errorf("expected %q to NOT be identified as virtual interface", p)
		}
	}
}
