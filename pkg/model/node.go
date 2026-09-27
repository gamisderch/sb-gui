package model

import (
	"fmt"
	"time"
)

// ProxyNode represents a single proxy server endpoint
type ProxyNode struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Group     string            `json:"group"`
	Protocol  string            `json:"protocol"` // VLESS, VMess, Shadowsocks, Trojan, etc.
	Transport string            `json:"transport"` // tcp, grpc, ws, etc.
	Security  string            `json:"security"`  // reality, tls, none
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Ping      int               `json:"ping"` // in milliseconds; -1 = untested, -2 = error/timeout
	RawURI    string            `json:"raw_uri"`
	Details   map[string]string `json:"details"`
}

// FormatPing returns a formatted string for latency
func (n *ProxyNode) FormatPing() string {
	if n.Ping == -1 {
		return "---"
	}
	if n.Ping < 0 {
		return "err"
	}
	return fmt.Sprintf("%dms", n.Ping)
}

// Subscription represents a collection of proxy nodes fetched from a URL or file
type Subscription struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	URL       string       `json:"url"`
	Nodes     []*ProxyNode `json:"nodes"`
	Upload    int64        `json:"upload"`
	Download  int64        `json:"download"`
	Total     int64        `json:"total"`
	Expire    int64        `json:"expire"`
	UpdatedAt time.Time    `json:"updated_at"`
	GeoRules  []*GeoRule   `json:"geo_rules,omitempty"`
}

// FormatTraffic returns human readable traffic usage e.g. "195 GB / ∞"
func (s *Subscription) FormatTraffic() string {
	used := s.Upload + s.Download
	usedStr := formatBytes(used)
	if s.Total <= 0 {
		return fmt.Sprintf("%s / ∞", usedStr)
	}
	return fmt.Sprintf("%s / %s", usedStr, formatBytes(s.Total))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// TunnelMode defines routing mode
type TunnelMode string

const (
	ModeTUN         TunnelMode = "TUN - Sing-box TUN"
	ModeSystemProxy TunnelMode = "System Proxy"
	ModeDirect      TunnelMode = "Direct"
)

// GeoRuleType defines geosite or geoip
type GeoRuleType string

const (
	GeoTypeSite GeoRuleType = "geosite"
	GeoTypeIP   GeoRuleType = "geoip"
)

// GeoRuleAction defines direct, proxy, or block
type GeoRuleAction string

const (
	GeoActionDirect GeoRuleAction = "direct"
	GeoActionProxy  GeoRuleAction = "proxy"
	GeoActionBlock  GeoRuleAction = "block"
)

// GeoRule represents a routing rule for domain category or IP location
type GeoRule struct {
	Type   GeoRuleType   `json:"type"`   // "geosite" or "geoip"
	Name   string        `json:"name"`   // e.g. "ru", "category-ads-all", "youtube"
	Action GeoRuleAction `json:"action"` // "direct", "proxy", "block"
}

// IPTypePreference defines IP version preference for DNS and dialing
type IPTypePreference string

const (
	IPTypePreferIPv4 IPTypePreference = "prefer_ipv4"
	IPTypePreferIPv6 IPTypePreference = "prefer_ipv6"
	IPTypeIPv4Only   IPTypePreference = "ipv4_only"
	IPTypeIPv6Only   IPTypePreference = "ipv6_only"
)

// FragmentationConfig stores TLS packet fragmentation settings for DPI evasion
type FragmentationConfig struct {
	Enabled  bool   `json:"enabled"`
	Packets  string `json:"packets"`  // "tlshello", "1-3", "1-5", "all"
	Length   string `json:"length"`   // e.g. "100-200", "50-100", "10-30"
	Interval string `json:"interval"` // e.g. "10-20", "5-10", "1-5" (ms)
}

// DefaultFragmentationConfig returns recommended defaults for DPI bypass
func DefaultFragmentationConfig() FragmentationConfig {
	return FragmentationConfig{
		Enabled:  false,
		Packets:  "tlshello",
		Length:   "100-200",
		Interval: "10-20",
	}
}

// MuxConfig stores TCP/UDP multiplexing settings
type MuxConfig struct {
	Enabled         bool   `json:"enabled"`
	Concurrency     int    `json:"concurrency"`       // default 8 (1-128)
	XUDPConcurrency int    `json:"xudp_concurrency"`  // default 16 (1-128)
	XUDPProxyUDP443 string `json:"xudp_proxy_udp443"` // "reject", "allow", "skip"
}

// DefaultMuxConfig returns recommended defaults for multiplexing
func DefaultMuxConfig() MuxConfig {
	return MuxConfig{
		Enabled:         false,
		Concurrency:     8,
		XUDPConcurrency: 16,
		XUDPProxyUDP443: "reject",
	}
}

// AdvancedTunnelConfig stores DNS and packet sniffing preferences (from UI screen)
type AdvancedTunnelConfig struct {
	UseLocalDNS         bool   `json:"use_local_dns"`           // default false
	UseDNSFromJSON      bool   `json:"use_dns_from_json"`       // default false
	EnableResolveServer bool   `json:"enable_resolve_server"`   // default true
	ResolveServerIP     string `json:"resolve_server_ip"`       // default "1.1.1.1"
	ResolveServerDomain string `json:"resolve_server_domain"`   // default "https://1.1.1.1/dns-query"
	EnableSniffing      bool   `json:"enable_sniffing"`         // default true
}

// DefaultAdvancedTunnelConfig returns defaults matching the UI screenshot
func DefaultAdvancedTunnelConfig() AdvancedTunnelConfig {
	return AdvancedTunnelConfig{
		UseLocalDNS:         false,
		UseDNSFromJSON:      false,
		EnableResolveServer: true,
		ResolveServerIP:     "1.1.1.1",
		ResolveServerDomain: "https://1.1.1.1/dns-query",
		EnableSniffing:      true,
	}
}

// AppState holds the global state of the application
type AppState struct {
	ActiveNode    *ProxyNode
	IsConnected   bool
	ConnectedAt   time.Time
	Mode          TunnelMode
	Subscriptions []*Subscription
	UserAgent     string
	BypassList    []string          // List of process names/paths to bypass VPN
	ThemeColors   map[string]string // Custom theme color hex overrides
	GeoRules      []*GeoRule        // Custom GeoSite and GeoIP routing rules
	IPType        IPTypePreference     `json:"ip_type"`
	Fragmentation FragmentationConfig  `json:"fragmentation"`
	Mux           MuxConfig            `json:"mux"`
	Advanced      AdvancedTunnelConfig `json:"advanced"`
}

// ResolveGeoRules returns the rules for the given node's subscription, or falls back to globalRules
func ResolveGeoRules(node *ProxyNode, subs []*Subscription, globalRules []*GeoRule) []*GeoRule {
	if node != nil {
		for _, sub := range subs {
			for _, n := range sub.Nodes {
				if n == node || n.ID == node.ID || (n.Name == node.Name && n.Address == node.Address && n.Port == node.Port) {
					if len(sub.GeoRules) > 0 {
						return sub.GeoRules
					}
					return globalRules
				}
			}
		}
	}
	return globalRules
}

