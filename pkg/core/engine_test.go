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

	// Verify strict_route is false and stack is mixed for P2P gaming support
	if strictRoute, ok := tunIn["strict_route"].(bool); !ok || strictRoute {
		t.Errorf("expected strict_route: false in tunInbound for P2P games, got %v", tunIn["strict_route"])
	}
	if stack, ok := tunIn["stack"].(string); !ok || stack != "mixed" {
		t.Errorf("expected stack: 'mixed' in tunInbound, got %v", tunIn["stack"])
	}

	// Verify process_path_regex is present for bypassed Wine/Proton games
	foundPathRegex := false
	foundSteamUDP := false
	for _, r := range rules {
		if rMap, ok := r.(map[string]interface{}); ok {
			if _, ok := rMap["process_path_regex"]; ok && rMap["outbound"] == "direct" {
				foundPathRegex = true
			}
			if rMap["outbound"] == "direct" {
				if portRanges, ok := rMap["port_range"].([]interface{}); ok {
					for _, pr := range portRanges {
						if prStr, ok := pr.(string); ok && prStr == "27000:27100" {
							foundSteamUDP = true
						}
					}
				}
			}
		}
	}
	if !foundPathRegex {
		t.Errorf("expected process_path_regex rule for bypassed Wine/Proton game")
	}
	if !foundSteamUDP {
		t.Errorf("expected Steam UDP ports direct rule in rules")
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

func TestXrayFragmentationAndMux(t *testing.T) {
	e := NewEngine()
	e.fragConfig = model.FragmentationConfig{
		Enabled:  true,
		Packets:  "tlshello",
		Length:   "100-200",
		Interval: "10-20",
	}
	e.muxConfig = model.MuxConfig{
		Enabled:         true,
		Concurrency:     16,
		XUDPConcurrency: 32,
		XUDPProxyUDP443: "reject",
	}
	e.ipType = model.IPTypeIPv4Only
	e.advConfig.EnableSniffing = true

	node := &model.ProxyNode{
		ID:        "node-trojan",
		Name:      "Trojan Node",
		Protocol:  "trojan",
		Address:   "1.2.3.4",
		Port:      443,
		Security:  "tls",
		Transport: "tcp",
		Details: map[string]string{
			"password": "secretpassword",
			"sni":      "example.com",
		},
	}

	cfgBytes, err := e.generateXrayConfig(node)
	if err != nil {
		t.Fatalf("generateXrayConfig failed: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		t.Fatalf("failed to parse xray json: %v", err)
	}

	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok {
		t.Fatalf("missing outbounds in xray config")
	}

	// 1. Verify fragment outbound exists
	foundFragment := false
	for _, ob := range outbounds {
		obMap, ok := ob.(map[string]interface{})
		if !ok {
			continue
		}
		if obMap["tag"] == "fragment" {
			foundFragment = true
			settings, ok := obMap["settings"].(map[string]interface{})
			if !ok {
				t.Fatalf("fragment outbound missing settings")
			}
			frag, ok := settings["fragment"].(map[string]interface{})
			if !ok {
				t.Fatalf("fragment settings missing fragment block")
			}
			if frag["packets"] != "tlshello" || frag["length"] != "100-200" || frag["interval"] != "10-20" {
				t.Errorf("unexpected fragment values: %v", frag)
			}
		}
	}
	if !foundFragment {
		t.Errorf("expected fragment outbound to be present")
	}

	// 2. Verify proxy outbound has dialerProxy and mux
	proxyOb := outbounds[0].(map[string]interface{})
	streamSettings, ok := proxyOb["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("proxy outbound missing streamSettings")
	}
	sockopt, ok := streamSettings["sockopt"].(map[string]interface{})
	if !ok {
		t.Fatalf("streamSettings missing sockopt")
	}
	if sockopt["dialerProxy"] != "fragment" {
		t.Errorf("expected dialerProxy to be 'fragment', got %v", sockopt["dialerProxy"])
	}

	mux, ok := proxyOb["mux"].(map[string]interface{})
	if !ok {
		t.Fatalf("proxy outbound missing mux block")
	}
	if mux["enabled"] != true || mux["concurrency"] != float64(16) || mux["xudpConcurrency"] != float64(32) || mux["xudpProxyUDP443"] != "reject" {
		t.Errorf("unexpected mux settings: %v", mux)
	}

	// 3. Verify sniffing on inbound socks 2080
	inbounds, ok := cfg["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		t.Fatalf("missing inbounds")
	}
	socksIn := inbounds[0].(map[string]interface{})
	sniffing, ok := socksIn["sniffing"].(map[string]interface{})
	if !ok || sniffing["enabled"] != true {
		t.Errorf("expected sniffing enabled on socks inbound")
	}
}

func TestSingboxAdvancedSettings(t *testing.T) {
	e := NewEngine()
	e.advConfig = model.AdvancedTunnelConfig{
		UseLocalDNS:         true,
		UseDNSFromJSON:      false,
		EnableResolveServer: true,
		ResolveServerIP:     "9.9.9.9",
		ResolveServerDomain: "https://dns.quad9.net/dns-query",
		EnableSniffing:      true,
	}
	e.ipType = model.IPTypeIPv6Only

	node := &model.ProxyNode{
		ID:       "test-node",
		Name:     "Test Node",
		Protocol: "vless",
		Address:  "198.51.100.1",
		Port:     443,
	}

	cfgBytes, err := e.generateSingboxConfig(node, []string{"mock.exe"}, nil)
	if err != nil {
		t.Fatalf("generateSingboxConfig failed: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		t.Fatalf("failed to parse sing-box json: %v", err)
	}

	dns, ok := cfg["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing dns block")
	}
	if dns["strategy"] != "ipv6_only" {
		t.Errorf("expected dns strategy to be ipv6_only, got %v", dns["strategy"])
	}
	if dns["final"] != "remote-doh" {
		t.Errorf("expected final dns resolver to be remote-doh, got %v", dns["final"])
	}

	servers, ok := dns["servers"].([]interface{})
	if !ok {
		t.Fatalf("missing dns.servers")
	}

	foundLocal := false
	foundCustomDoH := false
	for _, s := range servers {
		sMap := s.(map[string]interface{})
		if sMap["tag"] == "local-dns" && sMap["type"] == "local" {
			foundLocal = true
		}
		if sMap["tag"] == "remote-doh" && sMap["server"] == "9.9.9.9" {
			foundCustomDoH = true
		}
	}
	if !foundLocal {
		t.Errorf("expected local-dns server in dns.servers when UseLocalDNS is true")
	}
	if !foundCustomDoH {
		t.Errorf("expected remote-doh with IP 9.9.9.9 in dns.servers")
	}

	// Verify route rules contain sniffing
	route := cfg["route"].(map[string]interface{})
	rules := route["rules"].([]interface{})
	foundSniff := false
	for _, r := range rules {
		rMap := r.(map[string]interface{})
		if rMap["action"] == "sniff" {
			foundSniff = true
			break
		}
	}
	if !foundSniff {
		t.Errorf("expected sniff rule in route.rules when EnableSniffing is true")
	}
}
