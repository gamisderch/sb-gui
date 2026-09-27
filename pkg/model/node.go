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
}

