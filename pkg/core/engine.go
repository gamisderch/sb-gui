package core

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"signbox-cli/pkg/geo"
	"signbox-cli/pkg/model"
)

// Engine manages the proxy and TUN cores (Xray + Sing-box)
type Engine struct {
	mu          sync.Mutex
	cmdXray     *exec.Cmd
	cmdSingBox  *exec.Cmd
	xrayBin     string
	singboxBin  string
	xrayCfgPath string
	sbCfgPath   string
	xrayLogPath string
	sbLogPath   string
	xrayPidPath string
	sbPidPath       string
	connectedAtPath string
	isRunning       bool
	HasTunCap       bool
	activeNode      *model.ProxyNode
	bypassList      []string
	geoRules        []*model.GeoRule
	geoMgr          *geo.Manager
}

// NewEngine creates an Engine instance
func NewEngine() *Engine {
	xrayPath := "./bin/xray"
	if _, err := os.Stat(xrayPath); err != nil {
		if path, err := exec.LookPath("xray"); err == nil {
			xrayPath = path
		} else if _, err := os.Stat("/opt/happ/bin/core/xray"); err == nil {
			xrayPath = "/opt/happ/bin/core/xray"
		}
	}

	sbPath := "./bin/sing-box"
	if _, err := os.Stat(sbPath); err != nil {
		if path, err := exec.LookPath("sing-box"); err == nil {
			sbPath = path
		}
	}

	tmpDir := filepath.Join(os.TempDir(), "sb-cli")
	_ = os.MkdirAll(tmpDir, 0755)

	return &Engine{
		xrayBin:         xrayPath,
		singboxBin:      sbPath,
		xrayCfgPath:     filepath.Join(tmpDir, "xray.json"),
		sbCfgPath:       filepath.Join(tmpDir, "sing-box.json"),
		xrayLogPath:     filepath.Join(tmpDir, "xray.log"),
		sbLogPath:       filepath.Join(tmpDir, "sing-box.log"),
		xrayPidPath:     filepath.Join(tmpDir, "xray.pid"),
		sbPidPath:       filepath.Join(tmpDir, "singbox.pid"),
		connectedAtPath: filepath.Join(tmpDir, "connected_at"),
		isRunning:       false,
		geoMgr:          geo.NewManager(),
	}
}

// GetGeoManager returns the Geo database manager
func (e *Engine) GetGeoManager() *geo.Manager {
	return e.geoMgr
}

// GetSingboxBin returns path to sing-box binary
func (e *Engine) GetSingboxBin() string {
	return e.singboxBin
}

// IsRunning returns whether the core is running (foreground or detached background)
func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmdSingBox != nil && e.cmdSingBox.Process != nil {
		if err := e.cmdSingBox.Process.Signal(syscall.Signal(0)); err == nil {
			return true
		}
	}

	// Check if background sing-box is running from PID file
	if pidData, err := os.ReadFile(e.sbPidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(pidData))); err == nil && pid > 0 {
			if proc, err := os.FindProcess(pid); err == nil {
				if err := proc.Signal(syscall.Signal(0)); err == nil {
					return true
				}
			}
		}
	}

	// Check if tun0 interface is active
	if iface, err := net.InterfaceByName("tun0"); err == nil && iface != nil {
		return true
	}

	return false
}

// GetConnectedAt returns the timestamp when the current VPN session started
func (e *Engine) GetConnectedAt() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Try reading from connected_at file
	if data, err := os.ReadFile(e.connectedAtPath); err == nil {
		if sec, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil && sec > 1577836800 {
			return time.Unix(sec, 0)
		}
	}

	// 2. Try file modification time of singbox.pid
	if fi, err := os.Stat(e.sbPidPath); err == nil {
		if fi.ModTime().Year() > 2020 {
			return fi.ModTime()
		}
	}

	return time.Now()
}

// Start connects to the target node
func (e *Engine) Start(node *model.ProxyNode, bypassList []string, geoRules []*model.GeoRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.activeNode = node
	e.bypassList = bypassList
	e.geoRules = geoRules

	// Stop existing processes and kill any orphans holding tun0 / ports
	e.stopLocked()

	// 1. Start Xray (handles VLESS Reality with SpiderX, Shadowsocks, Trojan)
	if err := e.startXray(node); err != nil {
		return fmt.Errorf("failed to start proxy core (xray): %w", err)
	}

	// 2. Start Sing-box for TUN interface
	if err := e.startSingboxTUN(node, bypassList, geoRules); err != nil {
		if strings.Contains(err.Error(), "TUNSETIFF: operation not permitted") {
			e.HasTunCap = false
			e.isRunning = true
			return fmt.Errorf("tun_perm_error")
		}

		// If device or resource busy, an orphan may be holding tun0; force kill and retry once
		if strings.Contains(err.Error(), "device or resource busy") {
			killExistingCores()
			waitForTunReleased(1 * time.Second)
			if retryErr := e.startSingboxTUN(node, bypassList, geoRules); retryErr == nil {
				e.HasTunCap = true
				e.isRunning = true
				return nil
			}
		}

		e.HasTunCap = false
		e.stopLocked()
		return fmt.Errorf("failed to start sing-box TUN: %w", err)
	} else {
		e.HasTunCap = true
	}

	e.isRunning = true
	return nil
}

// UpdateBypass updates split tunneling rules while engine is running
func (e *Engine) UpdateBypass(bypassList []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.bypassList = bypassList
	if !e.isRunning || e.activeNode == nil {
		return nil
	}

	// Gracefully restart sing-box with the updated bypass list
	e.stopSingBoxLocked()
	if err := e.startSingboxTUN(e.activeNode, bypassList, e.geoRules); err != nil {
		if strings.Contains(err.Error(), "device or resource busy") {
			killExistingCores()
			waitForTunReleased(1 * time.Second)
			err = e.startSingboxTUN(e.activeNode, bypassList, e.geoRules)
		}
		if err != nil {
			return fmt.Errorf("failed to restart sing-box with updated bypass rules: %w", err)
		}
	}
	return nil
}

// UpdateGeoRules updates geo routing rules while engine is running
func (e *Engine) UpdateGeoRules(geoRules []*model.GeoRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.geoRules = geoRules
	if !e.isRunning || e.activeNode == nil {
		return nil
	}

	// Gracefully restart sing-box with the updated geo rules
	e.stopSingBoxLocked()
	if err := e.startSingboxTUN(e.activeNode, e.bypassList, geoRules); err != nil {
		if strings.Contains(err.Error(), "device or resource busy") {
			killExistingCores()
			waitForTunReleased(1 * time.Second)
			err = e.startSingboxTUN(e.activeNode, e.bypassList, geoRules)
		}
		if err != nil {
			return fmt.Errorf("failed to restart sing-box with updated geo rules: %w", err)
		}
	}
	return nil
}

// Stop cleanly terminates all running cores
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()
	return nil
}

func (e *Engine) stopSingBoxLocked() {
	if e.cmdSingBox != nil && e.cmdSingBox.Process != nil {
		proc := e.cmdSingBox.Process
		_ = proc.Signal(syscall.SIGTERM)

		// Wait for process to cleanly terminate and release TUN interface
		for i := 0; i < 25; i++ {
			time.Sleep(100 * time.Millisecond)
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				// Process has exited
				break
			}
		}

		if err := proc.Signal(syscall.Signal(0)); err == nil {
			_ = proc.Kill()
			time.Sleep(100 * time.Millisecond)
		}
		e.cmdSingBox = nil
	}

	// Terminate any stray sing-box instances holding tun0 and wait for tun0 to drop
	killOrphanedSingBox()
	waitForTunReleased(1 * time.Second)
}

func (e *Engine) stopLocked() {
	e.stopSingBoxLocked()
	if e.cmdXray != nil && e.cmdXray.Process != nil {
		proc := e.cmdXray.Process
		_ = proc.Signal(syscall.SIGTERM)
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				break
			}
		}
		if err := proc.Signal(syscall.Signal(0)); err == nil {
			_ = proc.Kill()
			time.Sleep(100 * time.Millisecond)
		}
		e.cmdXray = nil
	}
	killExistingCores()
	waitForTunReleased(1 * time.Second)
	_ = os.Remove(e.xrayPidPath)
	_ = os.Remove(e.sbPidPath)
	_ = os.Remove(e.connectedAtPath)
	e.isRunning = false
}

func (e *Engine) startXray(node *model.ProxyNode) error {
	cfgJSON, err := e.generateXrayConfig(node)
	if err != nil {
		return err
	}

	if err := os.WriteFile(e.xrayCfgPath, cfgJSON, 0644); err != nil {
		return err
	}

	logFile, err := os.OpenFile(e.xrayLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(e.xrayBin, "run", "-c", e.xrayCfgPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = os.WriteFile(e.xrayPidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	e.cmdXray = cmd

	go func() {
		_ = cmd.Wait()
		e.mu.Lock()
		if e.cmdSingBox == nil {
			e.isRunning = false
		}
		e.mu.Unlock()
		_ = logFile.Close()
	}()

	time.Sleep(200 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		logContent, _ := os.ReadFile(e.xrayLogPath)
		return fmt.Errorf("xray exited immediately:\n%s", string(logContent))
	}

	return nil
}

func (e *Engine) startSingboxTUN(node *model.ProxyNode, bypassList []string, geoRules []*model.GeoRule) error {
	cfgJSON, err := e.generateSingboxConfig(node, bypassList, geoRules)
	if err != nil {
		return err
	}

	if err := os.WriteFile(e.sbCfgPath, cfgJSON, 0644); err != nil {
		return err
	}

	logFile, err := os.OpenFile(e.sbLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(e.singboxBin, "run", "-c", e.sbCfgPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = os.WriteFile(e.sbPidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	e.cmdSingBox = cmd

	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()

	time.Sleep(250 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		e.cmdSingBox = nil
		logContent, _ := os.ReadFile(e.sbLogPath)
		return fmt.Errorf("sing-box tun exited: %s", strings.TrimSpace(string(logContent)))
	}

	_ = os.WriteFile(e.connectedAtPath, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0644)
	return nil
}

func (e *Engine) generateXrayConfig(node *model.ProxyNode) ([]byte, error) {
	// If node contains a full pre-configured raw JSON profile (e.g. from Remnawave, V2Ray or custom JSON)
	if rawJSON, ok := node.Details["raw_json"]; ok && strings.TrimSpace(rawJSON) != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(rawJSON), &cfg); err == nil {
			// 1. Ensure SOCKS inbound on 127.0.0.1:2080 exists for sing-box TUN detour
			hasSocks2080 := false
			if inbounds, ok := cfg["inbounds"].([]interface{}); ok {
				for _, in := range inbounds {
					if inMap, ok := in.(map[string]interface{}); ok {
						proto, _ := inMap["protocol"].(string)
						if strings.ToLower(proto) == "socks" {
							inMap["port"] = 2080
							inMap["listen"] = "127.0.0.1"
							hasSocks2080 = true
							break
						}
					}
				}
			}
			if !hasSocks2080 {
				newInbounds := []map[string]interface{}{
					{
						"tag":      "socks-in",
						"port":     2080,
						"listen":   "127.0.0.1",
						"protocol": "socks",
						"settings": map[string]interface{}{
							"udp":  true,
							"auth": "noauth",
						},
					},
				}
				if inbounds, ok := cfg["inbounds"].([]interface{}); ok {
					for _, in := range inbounds {
						if inMap, ok := in.(map[string]interface{}); ok {
							newInbounds = append(newInbounds, inMap)
						}
					}
				}
				cfg["inbounds"] = newInbounds
			}

			// 2. Ensure all proxy outbounds have sockopt mark 255 to bypass routing loops
			if outbounds, ok := cfg["outbounds"].([]interface{}); ok {
				for _, out := range outbounds {
					if outMap, ok := out.(map[string]interface{}); ok {
						proto, _ := outMap["protocol"].(string)
						proto = strings.ToLower(proto)
						if proto != "freedom" && proto != "blackhole" && proto != "direct" && proto != "block" {
							streamSettings, _ := outMap["streamSettings"].(map[string]interface{})
							if streamSettings == nil {
								streamSettings = make(map[string]interface{})
							}
							streamSettings["sockopt"] = map[string]interface{}{
								"mark": 255,
							}
							outMap["streamSettings"] = streamSettings
						}
					}
				}
			}

			return json.MarshalIndent(cfg, "", "  ")
		}
	}

	outbound := make(map[string]interface{})
	proto := strings.ToLower(node.Protocol)

	switch proto {
	case "vless":
		uuid := node.Details["uuid"]
		if uuid == "" {
			uuid = "516f8a46-297f-433c-a514-2887cff0a4f0"
		}
		flow := node.Details["flow"]

		user := map[string]interface{}{
			"id":         uuid,
			"encryption": "none",
		}
		if flow != "" {
			user["flow"] = flow
		}

		outbound["protocol"] = "vless"
		outbound["settings"] = map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": node.Address,
					"port":    node.Port,
					"users":   []map[string]interface{}{user},
				},
			},
		}

		streamSettings := map[string]interface{}{
			"network": node.Transport,
			// Mark outbound packets to bypass any potential routing loops
			"sockopt": map[string]interface{}{
				"mark": 255,
			},
		}
		if streamSettings["network"] == "" {
			streamSettings["network"] = "tcp"
		}

		sec := strings.ToLower(node.Security)
		if sec == "reality" {
			streamSettings["security"] = "reality"
			fp := node.Details["fp"]
			if fp == "" {
				fp = "firefox"
			}
			sni := node.Details["sni"]
			if sni == "" {
				sni = node.Address
			}

			realityMap := map[string]interface{}{
				"show":        false,
				"fingerprint": fp,
				"serverName":  sni,
				"publicKey":   node.Details["pbk"],
				"shortId":     node.Details["sid"],
			}
			if spx, ok := node.Details["spx"]; ok && spx != "" {
				realityMap["spiderX"] = spx
			}
			streamSettings["realitySettings"] = realityMap
		} else if sec == "tls" {
			streamSettings["security"] = "tls"
			tlsMap := map[string]interface{}{
				"serverName": node.Details["sni"],
			}
			streamSettings["tlsSettings"] = tlsMap
		}

		outbound["streamSettings"] = streamSettings

	case "shadowsocks":
		outbound["protocol"] = "shadowsocks"
		outbound["settings"] = map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  node.Address,
					"port":     node.Port,
					"method":   node.Details["method"],
					"password": node.Details["password"],
				},
			},
		}
		outbound["streamSettings"] = map[string]interface{}{
			"sockopt": map[string]interface{}{
				"mark": 255,
			},
		}

	case "trojan":
		outbound["protocol"] = "trojan"
		outbound["settings"] = map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  node.Address,
					"port":     node.Port,
					"password": node.Details["password"],
				},
			},
		}
		outbound["streamSettings"] = map[string]interface{}{
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName": node.Details["sni"],
			},
			"sockopt": map[string]interface{}{
				"mark": 255,
			},
		}

	default:
		outbound["protocol"] = "freedom"
	}

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds": []map[string]interface{}{
			{
				"port":     2080,
				"listen":   "127.0.0.1",
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp":  true,
					"auth": "noauth",
				},
			},
			{
				"port":     2081,
				"listen":   "127.0.0.1",
				"protocol": "http",
				"settings": map[string]interface{}{
					"allowTransparent": false,
				},
			},
		},
		"outbounds": []map[string]interface{}{
			outbound,
			{
				"protocol": "freedom",
				"tag":      "direct",
			},
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

func (e *Engine) generateSingboxConfig(node *model.ProxyNode, bypassList []string, geoRules []*model.GeoRule) ([]byte, error) {
	// Collect IP CIDRs to exclude from TUN routing to prevent loops
	var excludeAddrs []string
	addExclude := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if ip := net.ParseIP(addr); ip != nil {
			excludeAddrs = append(excludeAddrs, fmt.Sprintf("%s/32", addr))
		} else {
			if ips, err := net.LookupIP(addr); err == nil {
				for _, ip := range ips {
					if ipv4 := ip.To4(); ipv4 != nil {
						excludeAddrs = append(excludeAddrs, fmt.Sprintf("%s/32", ipv4.String()))
					}
				}
			}
		}
	}

	addExclude(node.Address)

	// If node contains raw_json with multiple outbound servers, exclude all of them
	if rawJSON, ok := node.Details["raw_json"]; ok && strings.TrimSpace(rawJSON) != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(rawJSON), &cfg); err == nil {
			if outbounds, ok := cfg["outbounds"].([]interface{}); ok {
				for _, out := range outbounds {
					if outMap, ok := out.(map[string]interface{}); ok {
						if settings, ok := outMap["settings"].(map[string]interface{}); ok {
							if vnext, ok := settings["vnext"].([]interface{}); ok {
								for _, v := range vnext {
									if vMap, ok := v.(map[string]interface{}); ok {
										if a, ok := vMap["address"].(string); ok {
											addExclude(a)
										}
									}
								}
							}
							if servers, ok := settings["servers"].([]interface{}); ok {
								for _, s := range servers {
									if sMap, ok := s.(map[string]interface{}); ok {
										if a, ok := sMap["address"].(string); ok {
											addExclude(a)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Exclude loopback from TUN
	excludeAddrs = append(excludeAddrs, "127.0.0.0/8")

	// 1. Process and compile GeoRules into local rule-sets
	var ruleSetBlocks []map[string]interface{}
	var blockRuleSets []string
	var directRuleSets []string
	var proxyRuleSets []string

	if e.geoMgr != nil && len(geoRules) > 0 {
		metas, _ := e.geoMgr.EnsureRuleSets(e.singboxBin, geoRules)
		for _, m := range metas {
			format := "binary"
			if !m.IsBinary {
				format = "source"
			}
			ruleSetBlocks = append(ruleSetBlocks, map[string]interface{}{
				"tag":    m.Tag,
				"type":   "local",
				"format": format,
				"path":   m.Path,
			})
			switch m.Action {
			case model.GeoActionBlock:
				blockRuleSets = append(blockRuleSets, m.Tag)
			case model.GeoActionDirect:
				directRuleSets = append(directRuleSets, m.Tag)
			case model.GeoActionProxy:
				proxyRuleSets = append(proxyRuleSets, m.Tag)
			}
		}
	}

	// Prepare route rules in strict priority order
	var rules []map[string]interface{}

	hasWineProcess := false
	for _, bp := range bypassList {
		lower := strings.ToLower(bp)
		if strings.HasSuffix(lower, ".exe") || strings.Contains(lower, "wine") || strings.Contains(lower, "proton") || strings.Contains(lower, "fsd") {
			hasWineProcess = true
			break
		}
	}

	// 1. Selective Split Tunneling: Bypassed processes (Right column) go 100% direct!
	if len(bypassList) > 0 {
		rules = append(rules, map[string]interface{}{
			"process_name": bypassList,
			"outbound":     "direct",
		})

		// If Wine/Proton/Steam games are in bypass, add process_path_regex to match
		// any socket path created by Wine prefixes, Proton, Steam Linux Runtime, or game folders
		if hasWineProcess {
			var wineRegexes []string
			wineRegexes = append(wineRegexes,
				`.*[/\\].*wine.*`,
				`.*[/\\].*proton.*`,
				`.*[/\\].*pressure-vessel.*`,
				`.*[/\\].*bwrap.*`,
			)
			for _, bp := range bypassList {
				lower := strings.ToLower(bp)
				if strings.HasSuffix(lower, ".exe") || strings.Contains(lower, "fsd") {
					base := strings.TrimSuffix(bp, filepath.Ext(bp))
					if len(base) > 0 {
						wineRegexes = append(wineRegexes, fmt.Sprintf(`.*[/\\].*%s.*`, regexp.QuoteMeta(base)))
					}
				}
			}
			rules = append(rules, map[string]interface{}{
				"process_path_regex": wineRegexes,
				"outbound":           "direct",
			})
		}
	}

	// 2. Steam P2P, Voice, SDR relay, and Game UDP ports go DIRECT ONLY if Wine/Proton games are bypassed!
	// This fixes P2P multiplayer games (like Deep Rock Galactic, Risk of Rain, etc.)
	// whose unconnected UDP sockets (bound to 0.0.0.0:*) cannot be matched by Netlink socket-diag.
	// If the user leaves the game in the VPN column, these ports are NOT forced to direct,
	// allowing the game's P2P and relay traffic to flow through the VPN tunnel.
	if hasWineProcess {
		rules = append(rules, map[string]interface{}{
			"port": []int{3478},
			"port_range": []string{
				"4379:4380",
				"27000:27100",
			},
			"network":  []string{"udp"},
			"outbound": "direct",
		})
		rules = append(rules, map[string]interface{}{
			"source_port": []int{3478},
			"source_port_range": []string{
				"4379:4380",
				"27000:27100",
			},
			"network":  []string{"udp"},
			"outbound": "direct",
		})
	}

	// 3. Sing-box, Xray, and sb-cli self-traffic safety direct rule
	rules = append(rules, map[string]interface{}{
		"process_name": []string{"sing-box", "xray", "sb-cli"},
		"outbound":     "direct",
	})

	// 4. Direct route for the proxy server itself to prevent routing loops
	if len(excludeAddrs) > 0 {
		rules = append(rules, map[string]interface{}{
			"ip_cidr":  excludeAddrs,
			"outbound": "direct",
		})
	}

	// 5. Direct route for private IP ranges (LAN, localhost)
	rules = append(rules, map[string]interface{}{
		"ip_is_private": true,
		"outbound":      "direct",
	})

	// 6. Geo Rules: Block rules (highest priority among geo rules, e.g. ads/trackers/malware)
	if len(blockRuleSets) > 0 {
		rules = append(rules, map[string]interface{}{
			"rule_set": blockRuleSets,
			"outbound": "block",
		})
	}

	// 7. Geo Rules: Direct rules (e.g. ru domains, ru geoip)
	if len(directRuleSets) > 0 {
		rules = append(rules, map[string]interface{}{
			"rule_set": directRuleSets,
			"outbound": "direct",
		})
	}

	// 8. Geo Rules: Proxy rules (e.g. youtube, telegram, discord)
	if len(proxyRuleSets) > 0 {
		rules = append(rules, map[string]interface{}{
			"rule_set": proxyRuleSets,
			"outbound": "proxy",
		})
	}

	// 9. Hijack port 53 DNS queries to internal DoH for all remaining VPN traffic
	rules = append(rules, map[string]interface{}{
		"port":   53,
		"action": "hijack-dns",
	})

	// 10. Default rule: all other TUN traffic goes through the VPN proxy (Left column)
	rules = append(rules, map[string]interface{}{
		"inbound":  []string{"tun-in"},
		"outbound": "proxy",
	})

	tunInbound := map[string]interface{}{
		"type":                     "tun",
		"tag":                      "tun-in",
		"interface_name":           "tun0",
		"address":                  []string{"172.16.250.1/30"},
		"mtu":                      1360,
		"auto_route":               true,
		"strict_route":             false,
		"endpoint_independent_nat": true,
		"stack":                    "mixed",
	}
	if len(excludeAddrs) > 0 {
		tunInbound["route_exclude_address"] = excludeAddrs
	}

	defaultIface := getDefaultInterface()

	directOutbound := map[string]interface{}{
		"type": "direct",
		"tag":  "direct",
	}

	routeBlock := map[string]interface{}{
		"find_process":            true,
		"auto_detect_interface":   true,
		"default_domain_resolver": "remote-doh-cf",
		"rules":                   rules,
	}
	if len(ruleSetBlocks) > 0 {
		routeBlock["rule_set"] = ruleSetBlocks
	}
	if defaultIface != "" && !isVirtualInterface(defaultIface) {
		routeBlock["default_interface"] = defaultIface
	}

	// Sing-box config with DoH DNS and split tunneling
	sbConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
		},
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"tag":    "remote-doh-cf",
					"type":   "https",
					"server": "1.1.1.1",
					"detour": "proxy",
				},
				{
					"tag":    "remote-doh-google",
					"type":   "https",
					"server": "8.8.8.8",
					"detour": "proxy",
				},
			},
			"final":    "remote-doh-cf",
			"strategy": "prefer_ipv4",
		},
		"inbounds": []map[string]interface{}{tunInbound},
		"outbounds": []map[string]interface{}{
			{
				"type":        "socks",
				"tag":         "proxy",
				"server":      "127.0.0.1",
				"server_port": 2080,
			},
			directOutbound,
			{
				"type": "block",
				"tag":  "block",
			},
		},
		"route": routeBlock,
	}

	return json.MarshalIndent(sbConfig, "", "  ")
}

// isVirtualInterface returns true for loopback, tunnel, bridge and container virtual interfaces
func isVirtualInterface(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return true
	}
	prefixes := []string{
		"tun", "tap", "lo", "docker", "veth", "virbr", "vmnet", "wg", "tailscale", "zerotier", "dummy",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// getDefaultInterface detects the active physical network interface (e.g. enp2s0, wlan0).
// It strictly ignores virtual and tunnel interfaces.
func getDefaultInterface() string {
	// 1. Primary: Parse Linux kernel routing table /proc/net/route
	if data, err := os.ReadFile("/proc/net/route"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				iface := fields[0]
				dest := fields[1]
				flags := fields[3]
				// Destination 00000000 is the default route (0.0.0.0)
				// Flag 0x2 is RTF_GATEWAY
				if dest == "00000000" && !isVirtualInterface(iface) {
					if n, err := strconv.ParseInt(flags, 16, 64); err == nil {
						if n&0x2 != 0 {
							return iface
						}
					}
				}
			}
		}
	}

	// 2. Fallback: Find first UP, non-loopback, non-virtual interface with IPv4
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
				continue
			}
			if isVirtualInterface(iface.Name) {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil || len(addrs) == 0 {
				continue
			}
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					if ipNet.IP.To4() != nil {
						return iface.Name
					}
				}
			}
		}
	}

	return ""
}

// waitForTunReleased waits until the tun0 interface is completely removed by kernel
func waitForTunReleased(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := net.InterfaceByName("tun0"); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// killOrphanedSingBox forcefully terminates any stray sing-box processes holding tun0
func killOrphanedSingBox() {
	killMatchingProcesses(func(cmdline string) bool {
		return strings.Contains(cmdline, "sb-cli/sing-box.json") ||
			(strings.Contains(cmdline, "sing-box") && strings.Contains(cmdline, "sing-box.json"))
	})
}

// killExistingCores forcefully terminates any stray sing-box or xray processes associated with sb-cli
func killExistingCores() {
	killMatchingProcesses(func(cmdline string) bool {
		return strings.Contains(cmdline, "sb-cli/sing-box.json") ||
			strings.Contains(cmdline, "sb-cli/xray.json") ||
			(strings.Contains(cmdline, "sing-box") && strings.Contains(cmdline, "sing-box.json")) ||
			(strings.Contains(cmdline, "xray") && strings.Contains(cmdline, "xray.json"))
	})
}

func killMatchingProcesses(match func(string) bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	myPID := os.Getpid()
	var pidsToKill []int

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == myPID || pid <= 1 {
			continue
		}

		cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}
		cmdline := string(cmdlineBytes)
		if match(cmdline) {
			pidsToKill = append(pidsToKill, pid)
		}
	}

	for _, pid := range pidsToKill {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}

	if len(pidsToKill) > 0 {
		time.Sleep(200 * time.Millisecond)
		for _, pid := range pidsToKill {
			if proc, err := os.FindProcess(pid); err == nil {
				if err := proc.Signal(syscall.Signal(0)); err == nil {
					_ = proc.Kill()
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
