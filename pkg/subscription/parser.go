package subscription

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"signbox-cli/pkg/model"
)

const (
	DefaultUserAgent = "Happ/1.8.0 (Linux; x86_64)"
	ClashUserAgent   = "clash-meta/1.18.0"
	V2RayNUserAgent  = "v2rayN/6.23"
)

// GetDeviceHWID retrieves or generates a persistent Hardware ID (HWID).
// Many subscription backends (e.g. Remnawave) require an X-HWID header.
func GetDeviceHWID() string {
	// 1. Check /etc/machine-id
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	// 2. Check /var/lib/dbus/machine-id
	if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	// 3. Check persistent device_id in ~/.config/sb-cli/device_id
	home, err := os.UserHomeDir()
	if err == nil {
		idFile := filepath.Join(home, ".config", "sb-cli", "device_id")
		if data, err := os.ReadFile(idFile); err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id
			}
		}

		// Generate random 16-byte (32 hex) ID
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		newID := hex.EncodeToString(b)
		_ = os.MkdirAll(filepath.Dir(idFile), 0755)
		_ = os.WriteFile(idFile, []byte(newID), 0644)
		return newID
	}

	return "10c94e2292214ad1a49c5a760f27cffc"
}

// Fetcher handles downloading and parsing subscriptions
type Fetcher struct {
	UserAgent string
	HWID      string
	Client    *http.Client
}

// NewFetcher creates a Fetcher with custom User-Agent and automatic HWID injection
func NewFetcher(userAgent string) *Fetcher {
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	return &Fetcher{
		UserAgent: userAgent,
		HWID:      GetDeviceHWID(),
		Client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// FetchSubscription downloads and parses a subscription from a URL
func (f *Fetcher) FetchSubscription(subURL, subName string) (*model.Subscription, error) {
	req, err := http.NewRequest("GET", subURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription URL: %w", err)
	}

	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "*/*")
	if f.HWID != "" {
		req.Header.Set("X-HWID", f.HWID)
		req.Header.Set("HWID", f.HWID)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %s", resp.Status)
	}

	sub := &model.Subscription{
		ID:        subURL,
		Name:      subName,
		URL:       subURL,
		UpdatedAt: time.Now(),
	}

	if sub.Name == "" || sub.Name == "Subscription" {
		if parsedU, err := url.Parse(subURL); err == nil && parsedU.Host != "" {
			sub.Name = parsedU.Host
		} else {
			sub.Name = "Subscription"
		}
	}

	// Parse subscription-userinfo header if present
	if userInfo := resp.Header.Get("subscription-userinfo"); userInfo != "" {
		parseUserInfo(userInfo, sub)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription response: %w", err)
	}

	nodes, rules, err := ParseContentWithRules(string(bodyBytes), sub.Name)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid proxy nodes found in subscription")
	}

	sub.Nodes = nodes
	sub.GeoRules = rules
	return sub, nil
}

func parseUserInfo(header string, sub *model.Subscription) {
	parts := strings.Split(header, ";")
	for _, p := range parts {
		kv := strings.Split(strings.TrimSpace(p), "=")
		if len(kv) == 2 {
			k := strings.ToLower(kv[0])
			val, _ := strconv.ParseInt(kv[1], 10, 64)
			switch k {
			case "upload":
				sub.Upload = val
			case "download":
				sub.Download = val
			case "total":
				sub.Total = val
			case "expire":
				sub.Expire = val
			}
		}
	}
}

// ParseContent parses raw subscription content supporting JSON, Clash YAML, Base64, and URI lines.
func ParseContent(content, groupName string) ([]*model.ProxyNode, error) {
	nodes, _, err := ParseContentWithRules(content, groupName)
	return nodes, err
}

// ParseContentWithRules parses raw subscription content and extracts both proxy nodes and GeoSite/GeoIP routing rules.
func ParseContentWithRules(content, groupName string) ([]*model.ProxyNode, []*model.GeoRule, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil, fmt.Errorf("empty content")
	}

	// 1. Try base64 decoding
	if decoded, err := tryBase64Decode(content); err == nil {
		trimmedDecoded := strings.TrimSpace(decoded)
		if trimmedDecoded != "" {
			// Check if base64 decoded content is JSON or YAML or URI lines
			if strings.HasPrefix(trimmedDecoded, "[") || strings.HasPrefix(trimmedDecoded, "{") {
				if nodes, jsonErr := parseJSONContent(trimmedDecoded, groupName); jsonErr == nil && len(nodes) > 0 {
					rules := ParseJSONRules(trimmedDecoded)
					return nodes, rules, nil
				} else if jsonErr != nil && strings.Contains(jsonErr.Error(), "provider rejected") {
					return nil, nil, jsonErr
				}
			}
			if strings.Contains(trimmedDecoded, "proxies:") {
				if nodes, rules, yamlErr := parseClashYAML(trimmedDecoded, groupName); yamlErr == nil && len(nodes) > 0 {
					return nodes, rules, nil
				}
			}
			content = trimmedDecoded
		}
	}

	// 2. Check if content is JSON directly
	if strings.HasPrefix(content, "[") || strings.HasPrefix(content, "{") {
		nodes, err := parseJSONContent(content, groupName)
		if err != nil {
			return nil, nil, err
		}
		if len(nodes) > 0 {
			rules := ParseJSONRules(content)
			return nodes, rules, nil
		}
	}

	// 3. Check if content is Clash YAML
	if strings.Contains(content, "proxies:") {
		nodes, rules, err := parseClashYAML(content, groupName)
		if err == nil && len(nodes) > 0 {
			return nodes, rules, nil
		}
	}

	// 4. Line-by-line URI parsing (vless://, ss://, trojan://, vmess://, hysteria2://)
	lines := strings.Split(content, "\n")
	var nodes []*model.ProxyNode

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		node, err := ParseNodeURI(line, groupName)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}

	if len(nodes) == 0 {
		return nil, nil, fmt.Errorf("no valid proxy nodes found")
	}

	return nodes, nil, nil
}

// parseJSONContent parses Remnawave, Sing-box, or Xray JSON configurations
func parseJSONContent(content, groupName string) ([]*model.ProxyNode, error) {
	// A. Try JSON Array of configs (Remnawave profile array or outbounds array)
	var rawArray []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &rawArray); err == nil && len(rawArray) > 0 {
		var nodes []*model.ProxyNode
		hasDummyStubs := false
		dummyMessage := ""

		for i, item := range rawArray {
			remarks, _ := item["remarks"].(string)

			// Check if this item is a Remnawave dummy error stub (e.g. 0.0.0.0:1)
			if isDummyRemnawaveItem(item) {
				hasDummyStubs = true
				if remarks != "" {
					dummyMessage = remarks
				}
				continue
			}

			// Case 1: Remnawave / Xray full profile (has "outbounds" array)
			if outbounds, ok := item["outbounds"].([]interface{}); ok && len(outbounds) > 0 {
				node := extractNodeFromProfile(item, remarks, groupName, i)
				if node != nil {
					nodes = append(nodes, node)
					continue
				}
			}

			// Case 2: Array of sing-box or Xray outbounds directly
			if node := parseOutboundObject(item, groupName, remarks, i); node != nil {
				nodes = append(nodes, node)
			}
		}

		if len(nodes) == 0 && hasDummyStubs {
			if dummyMessage == "" {
				dummyMessage = "application not supported / HWID required"
			}
			return nil, fmt.Errorf("provider rejected subscription: %s", dummyMessage)
		}

		if len(nodes) > 0 {
			return nodes, nil
		}
	}

	// B. Try Single JSON Object (e.g. Sing-box or Xray config with "outbounds")
	var singleObj map[string]interface{}
	if err := json.Unmarshal([]byte(content), &singleObj); err == nil {
		if outbounds, ok := singleObj["outbounds"].([]interface{}); ok && len(outbounds) > 0 {
			var nodes []*model.ProxyNode
			remarks, _ := singleObj["remarks"].(string)
			if remarks == "" {
				remarks = groupName
			}

			for i, out := range outbounds {
				if outMap, ok := out.(map[string]interface{}); ok {
					tag, _ := outMap["tag"].(string)
					proto, _ := outMap["type"].(string)
					if proto == "" {
						proto, _ = outMap["protocol"].(string)
					}
					proto = strings.ToLower(proto)
					if proto == "freedom" || proto == "blackhole" || proto == "direct" || proto == "block" || proto == "dns" {
						continue
					}

					node := parseOutboundObject(outMap, groupName, tag, i)
					if node != nil {
						nodes = append(nodes, node)
					}
				}
			}

			if len(nodes) > 0 {
				return nodes, nil
			}
		}

		// Single outbound object directly
		if node := parseOutboundObject(singleObj, groupName, "", 0); node != nil {
			return []*model.ProxyNode{node}, nil
		}
	}

	return nil, fmt.Errorf("unrecognized JSON structure")
}

// isDummyRemnawaveItem checks if a config item is an error dummy stub (e.g. 0.0.0.0:1)
func isDummyRemnawaveItem(item map[string]interface{}) bool {
	remarks, _ := item["remarks"].(string)
	if strings.Contains(strings.ToLower(remarks), "не поддерживается") ||
		strings.Contains(strings.ToLower(remarks), "not supported") {
		return true
	}

	if outbounds, ok := item["outbounds"].([]interface{}); ok {
		for _, o := range outbounds {
			if outMap, ok := o.(map[string]interface{}); ok {
				if settings, ok := outMap["settings"].(map[string]interface{}); ok {
					if vnext, ok := settings["vnext"].([]interface{}); ok && len(vnext) > 0 {
						if vMap, ok := vnext[0].(map[string]interface{}); ok {
							addr, _ := vMap["address"].(string)
							port, _ := vMap["port"].(float64)
							if addr == "0.0.0.0" && int(port) <= 1 {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// extractNodeFromProfile creates a ProxyNode from an Xray/Remnawave profile item
func extractNodeFromProfile(item map[string]interface{}, remarks, groupName string, idx int) *model.ProxyNode {
	outbounds, ok := item["outbounds"].([]interface{})
	if !ok || len(outbounds) == 0 {
		return nil
	}

	// Find the first proxy outbound (non-direct, non-block)
	var primaryOutbound map[string]interface{}
	for _, o := range outbounds {
		if outMap, ok := o.(map[string]interface{}); ok {
			proto, _ := outMap["protocol"].(string)
			if proto == "" {
				proto, _ = outMap["type"].(string)
			}
			proto = strings.ToLower(proto)
			if proto != "freedom" && proto != "blackhole" && proto != "direct" && proto != "block" && proto != "dns" && proto != "" {
				primaryOutbound = outMap
				break
			}
		}
	}

	if primaryOutbound == nil {
		return nil
	}

	itemJSON, _ := json.Marshal(item)
	node := parseOutboundObject(primaryOutbound, groupName, remarks, idx)
	if node != nil {
		if remarks != "" {
			node.Name = remarks
		}
		// Store the full raw JSON profile so Xray can execute its load balancer directly!
		node.Details["raw_json"] = string(itemJSON)
	}
	return node
}

// parseOutboundObject extracts a ProxyNode from an outbound map (sing-box or xray)
func parseOutboundObject(outMap map[string]interface{}, groupName, fallbackName string, idx int) *model.ProxyNode {
	proto, _ := outMap["protocol"].(string)
	if proto == "" {
		proto, _ = outMap["type"].(string)
	}
	proto = strings.ToLower(proto)
	if proto == "freedom" || proto == "blackhole" || proto == "direct" || proto == "block" || proto == "dns" || proto == "" {
		return nil
	}

	tag, _ := outMap["tag"].(string)
	name := fallbackName
	if name == "" {
		name = tag
	}
	if name == "" {
		name = fmt.Sprintf("Node-%d", idx+1)
	}

	node := &model.ProxyNode{
		ID:        fmt.Sprintf("%s-%d", name, idx),
		Name:      name,
		Group:     groupName,
		Ping:      -1,
		Details:   make(map[string]string),
		Transport: "tcp",
		Security:  "none",
	}

	// 1. Check Sing-box outbound format ("server", "server_port")
	if server, ok := outMap["server"].(string); ok && server != "" {
		node.Address = server
		if port, ok := outMap["server_port"].(float64); ok {
			node.Port = int(port)
		}
		if uuid, ok := outMap["uuid"].(string); ok {
			node.Details["uuid"] = uuid
		}
		if method, ok := outMap["method"].(string); ok {
			node.Details["method"] = method
		}
		if pwd, ok := outMap["password"].(string); ok {
			node.Details["password"] = pwd
		}
		if flow, ok := outMap["flow"].(string); ok {
			node.Details["flow"] = flow
		}
		if tlsMap, ok := outMap["tls"].(map[string]interface{}); ok {
			node.Security = "tls"
			if sni, ok := tlsMap["server_name"].(string); ok {
				node.Details["sni"] = sni
			}
			if reality, ok := tlsMap["reality"].(map[string]interface{}); ok {
				node.Security = "reality"
				if pbk, ok := reality["public_key"].(string); ok {
					node.Details["pbk"] = pbk
				}
				if sid, ok := reality["short_id"].(string); ok {
					node.Details["sid"] = sid
				}
			}
		}
		node.Protocol = strings.ToUpper(proto)
		return node
	}

	// 2. Check Xray / V2Ray outbound format ("settings.vnext" or "settings.servers")
	settings, _ := outMap["settings"].(map[string]interface{})
	if settings != nil {
		// VLESS / VMess (vnext)
		if vnext, ok := settings["vnext"].([]interface{}); ok && len(vnext) > 0 {
			if vMap, ok := vnext[0].(map[string]interface{}); ok {
				node.Address, _ = vMap["address"].(string)
				if p, ok := vMap["port"].(float64); ok {
					node.Port = int(p)
				}
				if users, ok := vMap["users"].([]interface{}); ok && len(users) > 0 {
					if uMap, ok := users[0].(map[string]interface{}); ok {
						if id, ok := uMap["id"].(string); ok {
							node.Details["uuid"] = id
						}
						if flow, ok := uMap["flow"].(string); ok {
							node.Details["flow"] = flow
						}
					}
				}
			}
		}

		// Shadowsocks / Trojan (servers)
		if servers, ok := settings["servers"].([]interface{}); ok && len(servers) > 0 {
			if sMap, ok := servers[0].(map[string]interface{}); ok {
				node.Address, _ = sMap["address"].(string)
				if p, ok := sMap["port"].(float64); ok {
					node.Port = int(p)
				}
				if pwd, ok := sMap["password"].(string); ok {
					node.Details["password"] = pwd
				}
				if method, ok := sMap["method"].(string); ok {
					node.Details["method"] = method
				}
			}
		}
	}

	// Stream settings (network, security, realitySettings, tlsSettings)
	if stream, ok := outMap["streamSettings"].(map[string]interface{}); ok {
		if netw, ok := stream["network"].(string); ok && netw != "" {
			node.Transport = netw
		}
		if sec, ok := stream["security"].(string); ok && sec != "" {
			node.Security = sec
		}
		if reality, ok := stream["realitySettings"].(map[string]interface{}); ok {
			node.Security = "reality"
			if sn, ok := reality["serverName"].(string); ok {
				node.Details["sni"] = sn
			}
			if pk, ok := reality["publicKey"].(string); ok {
				node.Details["pbk"] = pk
			}
			if sid, ok := reality["shortId"].(string); ok {
				node.Details["sid"] = sid
			}
			if fp, ok := reality["fingerprint"].(string); ok {
				node.Details["fp"] = fp
			}
			if spx, ok := reality["spiderX"].(string); ok {
				node.Details["spx"] = spx
			}
		}
		if tlsMap, ok := stream["tlsSettings"].(map[string]interface{}); ok {
			if sn, ok := tlsMap["serverName"].(string); ok {
				node.Details["sni"] = sn
			}
		}
	}

	switch proto {
	case "vless":
		node.Protocol = "VLESS"
	case "shadowsocks", "ss":
		node.Protocol = "Shadowsocks"
	case "trojan":
		node.Protocol = "Trojan"
	case "vmess":
		node.Protocol = "VMess"
	case "hysteria2", "hy2":
		node.Protocol = "Hysteria2"
	default:
		node.Protocol = strings.ToUpper(proto)
	}

	if node.Address == "" {
		return nil
	}
	return node
}

// parseClashYAML parses Clash Meta / Mihomo configuration format
func parseClashYAML(content, groupName string) ([]*model.ProxyNode, []*model.GeoRule, error) {
	var cfg struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
		Rules   []string                  `yaml:"rules"`
	}

	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, nil, err
	}

	if len(cfg.Proxies) == 0 {
		return nil, nil, fmt.Errorf("no proxies found in clash config")
	}

	var nodes []*model.ProxyNode
	for i, p := range cfg.Proxies {
		name, _ := p["name"].(string)
		proto, _ := p["type"].(string)
		server, _ := p["server"].(string)
		if server == "" {
			continue
		}

		port := 0
		switch pt := p["port"].(type) {
		case int:
			port = pt
		case float64:
			port = int(pt)
		case string:
			port, _ = strconv.Atoi(pt)
		}

		if name == "" {
			name = fmt.Sprintf("Clash-%d", i+1)
		}

		node := &model.ProxyNode{
			ID:        fmt.Sprintf("%s:%d#%s", server, port, name),
			Name:      name,
			Group:     groupName,
			Address:   server,
			Port:      port,
			Ping:      -1,
			Details:   make(map[string]string),
			Transport: "tcp",
			Security:  "none",
		}

		if netw, ok := p["network"].(string); ok && netw != "" {
			node.Transport = netw
		}

		if tlsVal, ok := p["tls"].(bool); ok && tlsVal {
			node.Security = "tls"
		}

		if sn, ok := p["servername"].(string); ok && sn != "" {
			node.Details["sni"] = sn
		} else if sn, ok := p["sni"].(string); ok && sn != "" {
			node.Details["sni"] = sn
		}

		if realityOpts, ok := p["reality-opts"].(map[string]interface{}); ok {
			node.Security = "reality"
			if pk, ok := realityOpts["public-key"].(string); ok {
				node.Details["pbk"] = pk
			}
			if sid, ok := realityOpts["short-id"].(string); ok {
				node.Details["sid"] = sid
			}
		}

		if fp, ok := p["client-fingerprint"].(string); ok {
			node.Details["fp"] = fp
		}

		if flow, ok := p["flow"].(string); ok {
			node.Details["flow"] = flow
		}

		if uuid, ok := p["uuid"].(string); ok {
			node.Details["uuid"] = uuid
		}

		if pwd, ok := p["password"].(string); ok {
			node.Details["password"] = pwd
		}

		if cipher, ok := p["cipher"].(string); ok {
			node.Details["method"] = cipher
		}

		switch strings.ToLower(proto) {
		case "vless":
			node.Protocol = "VLESS"
		case "ss", "shadowsocks":
			node.Protocol = "Shadowsocks"
		case "trojan":
			node.Protocol = "Trojan"
		case "vmess":
			node.Protocol = "VMess"
		case "hysteria2", "hy2":
			node.Protocol = "Hysteria2"
		default:
			node.Protocol = strings.ToUpper(proto)
		}

		nodes = append(nodes, node)
	}

	rules := parseClashRules(cfg.Rules)
	return nodes, rules, nil
}

// parseClashRules extracts GeoSite and GeoIP routing rules from Clash configuration
func parseClashRules(rules []string) []*model.GeoRule {
	if len(rules) == 0 {
		return nil
	}

	var parsed []*model.GeoRule
	seen := make(map[string]bool)

	for _, line := range rules {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}

		ruleType := strings.ToUpper(strings.TrimSpace(parts[0]))
		payload := strings.ToLower(strings.TrimSpace(parts[1]))
		target := strings.ToLower(strings.TrimSpace(parts[2]))

		var gType model.GeoRuleType
		switch ruleType {
		case "GEOSITE":
			gType = model.GeoTypeSite
		case "GEOIP":
			gType = model.GeoTypeIP
		default:
			continue
		}

		if payload == "" || payload == "lan" || payload == "private" {
			continue
		}

		action := model.GeoActionProxy
		if target == "direct" || target == "bypass" {
			action = model.GeoActionDirect
		} else if target == "reject" || target == "drop" || target == "block" {
			action = model.GeoActionBlock
		}

		key := fmt.Sprintf("%s:%s", gType, payload)
		if seen[key] {
			continue
		}
		seen[key] = true

		parsed = append(parsed, &model.GeoRule{
			Type:   gType,
			Name:   payload,
			Action: action,
		})
	}

	return parsed
}

// ParseJSONRules extracts GeoSite, GeoIP, domain, and IP routing rules from Sing-box, Xray, or Remnawave JSON configs
func ParseJSONRules(content string) []*model.GeoRule {
	var root interface{}
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil
	}

	var rulesSlice []map[string]interface{}

	if rootMap, ok := root.(map[string]interface{}); ok {
		// 1. route.rules
		if route, ok := rootMap["route"].(map[string]interface{}); ok {
			if rList, ok := route["rules"].([]interface{}); ok {
				for _, r := range rList {
					if rm, ok := r.(map[string]interface{}); ok {
						rulesSlice = append(rulesSlice, rm)
					}
				}
			}
		}
		// 2. routing.rules
		if routing, ok := rootMap["routing"].(map[string]interface{}); ok {
			if rList, ok := routing["rules"].([]interface{}); ok {
				for _, r := range rList {
					if rm, ok := r.(map[string]interface{}); ok {
						rulesSlice = append(rulesSlice, rm)
					}
				}
			}
		}
		// 3. top-level rules
		if rList, ok := rootMap["rules"].([]interface{}); ok {
			for _, r := range rList {
				if rm, ok := r.(map[string]interface{}); ok {
					rulesSlice = append(rulesSlice, rm)
				}
			}
		}
	} else if rootArr, ok := root.([]interface{}); ok {
		// Array of configs (like Remnawave profiles)
		for _, item := range rootArr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if route, ok := itemMap["route"].(map[string]interface{}); ok {
					if rList, ok := route["rules"].([]interface{}); ok {
						for _, r := range rList {
							if rm, ok := r.(map[string]interface{}); ok {
								rulesSlice = append(rulesSlice, rm)
							}
						}
					}
				}
				if routing, ok := itemMap["routing"].(map[string]interface{}); ok {
					if rList, ok := routing["rules"].([]interface{}); ok {
						for _, r := range rList {
							if rm, ok := r.(map[string]interface{}); ok {
								rulesSlice = append(rulesSlice, rm)
							}
						}
					}
				}
			}
		}
	}

	if len(rulesSlice) == 0 {
		return nil
	}

	var parsed []*model.GeoRule
	seen := make(map[string]bool)

	for _, rm := range rulesSlice {
		outbound, _ := rm["outbound"].(string)
		if outbound == "" {
			outbound, _ = rm["outboundTag"].(string)
		}
		if outbound == "" {
			outbound, _ = rm["action"].(string)
		}
		outboundLower := strings.ToLower(strings.TrimSpace(outbound))

		action := model.GeoActionProxy
		if outboundLower == "direct" || outboundLower == "freedom" || outboundLower == "bypass" {
			action = model.GeoActionDirect
		} else if outboundLower == "block" || outboundLower == "blackhole" || outboundLower == "reject" {
			action = model.GeoActionBlock
		}

		addRule := func(rType model.GeoRuleType, name string) {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				return
			}
			key := fmt.Sprintf("%s:%s", rType, name)
			if seen[key] {
				return
			}
			seen[key] = true
			parsed = append(parsed, &model.GeoRule{
				Type:   rType,
				Name:   name,
				Action: action,
			})
		}

		extractSite := func(val string) {
			val = strings.TrimSpace(val)
			if val == "" {
				return
			}
			lower := strings.ToLower(val)
			if strings.HasPrefix(lower, "geosite:") {
				addRule(model.GeoTypeSite, strings.TrimPrefix(val, val[:8]))
			} else if strings.HasPrefix(lower, "domain:") {
				addRule(model.GeoTypeSite, strings.TrimPrefix(val, val[:7]))
			} else if strings.HasPrefix(lower, "full:") {
				addRule(model.GeoTypeSite, strings.TrimPrefix(val, val[:5]))
			} else if strings.HasPrefix(lower, "keyword:") {
				addRule(model.GeoTypeSite, strings.TrimPrefix(val, val[:8]))
			} else {
				addRule(model.GeoTypeSite, val)
			}
		}

		extractIP := func(val string) {
			val = strings.TrimSpace(val)
			if val == "" {
				return
			}
			lower := strings.ToLower(val)
			if strings.HasPrefix(lower, "geoip:") {
				addRule(model.GeoTypeIP, strings.TrimPrefix(val, val[:6]))
			} else {
				addRule(model.GeoTypeIP, val)
			}
		}

		// 1. Check geosite
		if gs, ok := rm["geosite"].([]interface{}); ok {
			for _, v := range gs {
				if s, ok := v.(string); ok {
					extractSite(s)
				}
			}
		} else if gs, ok := rm["geosite"].(string); ok {
			extractSite(gs)
		}

		// 2. Check domain, domains, domain_suffix, domain_keyword
		for _, key := range []string{"domain", "domains", "domain_suffix", "domain_keyword"} {
			if dList, ok := rm[key].([]interface{}); ok {
				for _, v := range dList {
					if s, ok := v.(string); ok {
						extractSite(s)
					}
				}
			} else if dStr, ok := rm[key].(string); ok {
				extractSite(dStr)
			}
		}

		// 3. Check geoip
		if gi, ok := rm["geoip"].([]interface{}); ok {
			for _, v := range gi {
				if s, ok := v.(string); ok {
					extractIP(s)
				}
			}
		} else if gi, ok := rm["geoip"].(string); ok {
			extractIP(gi)
		}

		// 4. Check ip, ips, ip_cidr
		for _, key := range []string{"ip", "ips", "ip_cidr"} {
			if ipList, ok := rm[key].([]interface{}); ok {
				for _, v := range ipList {
					if s, ok := v.(string); ok {
						extractIP(s)
					}
				}
			} else if ipStr, ok := rm[key].(string); ok {
				extractIP(ipStr)
			}
		}

		// 5. Check rule_set
		if rs, ok := rm["rule_set"].([]interface{}); ok {
			for _, v := range rs {
				if s, ok := v.(string); ok {
					sLower := strings.ToLower(s)
					if strings.HasPrefix(sLower, "geosite-") {
						addRule(model.GeoTypeSite, strings.TrimPrefix(sLower, "geosite-"))
					} else if strings.HasPrefix(sLower, "geoip-") {
						addRule(model.GeoTypeIP, strings.TrimPrefix(sLower, "geoip-"))
					} else if strings.HasPrefix(sLower, "site-") {
						addRule(model.GeoTypeSite, strings.TrimPrefix(sLower, "site-"))
					} else if strings.HasPrefix(sLower, "ip-") {
						addRule(model.GeoTypeIP, strings.TrimPrefix(sLower, "ip-"))
					}
				}
			}
		} else if rs, ok := rm["rule_set"].(string); ok {
			sLower := strings.ToLower(rs)
			if strings.HasPrefix(sLower, "geosite-") {
				addRule(model.GeoTypeSite, strings.TrimPrefix(sLower, "geosite-"))
			} else if strings.HasPrefix(sLower, "geoip-") {
				addRule(model.GeoTypeIP, strings.TrimPrefix(sLower, "geoip-"))
			}
		}
	}

	return parsed
}

func tryBase64Decode(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")

	// Try StdEncoding
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(data), nil
	}

	// Try URLEncoding
	if data, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(data), nil
	}

	// Try RawStdEncoding
	if data, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(data), nil
	}

	// Try RawURLEncoding
	if data, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(data), nil
	}

	return "", fmt.Errorf("failed to decode base64")
}

// ParseNodeURI parses a single proxy URI link (vless, vmess, ss, trojan, hysteria2)
func ParseNodeURI(rawURI, groupName string) (*model.ProxyNode, error) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return nil, err
	}

	node := &model.ProxyNode{
		ID:      rawURI,
		Group:   groupName,
		Ping:    -1,
		RawURI:  rawURI,
		Details: make(map[string]string),
	}

	scheme := strings.ToLower(u.Scheme)
	name := u.Fragment
	if decodedName, err := url.QueryUnescape(name); err == nil && decodedName != "" {
		name = decodedName
	}
	if name == "" {
		name = u.Host
	}
	node.Name = name

	switch scheme {
	case "vless":
		node.Protocol = "VLESS"
		node.Address = u.Hostname()
		port, _ := strconv.Atoi(u.Port())
		node.Port = port
		if u.User != nil {
			node.Details["uuid"] = u.User.Username()
		}

		q := u.Query()
		node.Transport = q.Get("type")
		if node.Transport == "" {
			node.Transport = "tcp"
		}
		node.Security = q.Get("security")
		if node.Security == "" {
			node.Security = "none"
		}

		for k, v := range q {
			if len(v) > 0 {
				node.Details[k] = v[0]
			}
		}

	case "trojan":
		node.Protocol = "Trojan"
		node.Address = u.Hostname()
		port, _ := strconv.Atoi(u.Port())
		node.Port = port
		if u.User != nil {
			node.Details["password"] = u.User.Username()
		}

		q := u.Query()
		node.Transport = q.Get("type")
		if node.Transport == "" {
			node.Transport = "tcp"
		}
		node.Security = "tls"

		for k, v := range q {
			if len(v) > 0 {
				node.Details[k] = v[0]
			}
		}

	case "ss":
		node.Protocol = "Shadowsocks"
		if u.User != nil {
			node.Address = u.Hostname()
			port, _ := strconv.Atoi(u.Port())
			node.Port = port
			userStr := u.User.Username()
			if decoded, err := tryBase64Decode(userStr); err == nil {
				parts := strings.SplitN(decoded, ":", 2)
				if len(parts) == 2 {
					node.Details["method"] = parts[0]
					node.Details["password"] = parts[1]
				}
			}
		} else {
			if decoded, err := tryBase64Decode(u.Host); err == nil {
				if parsedSub, err := url.Parse("ss://" + decoded); err == nil {
					node.Address = parsedSub.Hostname()
					port, _ := strconv.Atoi(parsedSub.Port())
					node.Port = port
					if parsedSub.User != nil {
						node.Details["method"] = parsedSub.User.Username()
						pwd, _ := parsedSub.User.Password()
						node.Details["password"] = pwd
					}
				}
			}
		}
		node.Transport = "tcp"
		node.Security = "none"

	case "vmess":
		node.Protocol = "VMess"
		if decoded, err := tryBase64Decode(u.Host); err == nil {
			var vmessData struct {
				PS   string      `json:"ps"`
				Add  string      `json:"add"`
				Port interface{} `json:"port"`
				ID   string      `json:"id"`
				Net  string      `json:"net"`
				TLS  string      `json:"tls"`
			}
			if err := json.Unmarshal([]byte(decoded), &vmessData); err == nil {
				if vmessData.PS != "" {
					node.Name = vmessData.PS
				}
				node.Address = vmessData.Add
				switch p := vmessData.Port.(type) {
				case float64:
					node.Port = int(p)
				case string:
					node.Port, _ = strconv.Atoi(p)
				}
				node.Transport = vmessData.Net
				if node.Transport == "" {
					node.Transport = "tcp"
				}
				node.Security = vmessData.TLS
				if node.Security == "" {
					node.Security = "none"
				}
			}
		}

	case "hysteria2", "hy2":
		node.Protocol = "Hysteria2"
		node.Address = u.Hostname()
		port, _ := strconv.Atoi(u.Port())
		node.Port = port
		if u.User != nil {
			node.Details["password"] = u.User.Username()
		}
		node.Security = "tls"
		q := u.Query()
		if sni := q.Get("sni"); sni != "" {
			node.Details["sni"] = sni
		}

	default:
		return nil, fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return node, nil
}

// PingNode tests TCP handshake latency to the node in milliseconds
func PingNode(node *model.ProxyNode, timeout time.Duration) int {
	if node.Address == "" || node.Port == 0 {
		node.Ping = -2
		return -2
	}

	addr := net.JoinHostPort(node.Address, strconv.Itoa(node.Port))
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		node.Ping = -2
		return -2
	}
	defer conn.Close()

	elapsed := time.Since(start)
	ms := int(elapsed.Milliseconds())
	if ms == 0 {
		ms = 1
	}
	node.Ping = ms
	return ms
}
