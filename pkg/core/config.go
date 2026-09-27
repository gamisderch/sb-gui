package core

import (
	"encoding/json"
	"os"
	"path/filepath"

	"signbox-cli/pkg/model"
)

// ConfigData stores saved user preferences, subscriptions and bypass list
type ConfigData struct {
	UserAgent     string                     `json:"user_agent"`
	Mode          model.TunnelMode           `json:"mode"`
	ActiveNodeID  string                     `json:"active_node_id"`
	Subscriptions []*model.Subscription      `json:"subscriptions"`
	BypassNames   []string                   `json:"bypass_names"`
	ThemeColors   map[string]string          `json:"theme_colors,omitempty"`
	GeoRules      []*model.GeoRule           `json:"geo_rules,omitempty"`
	IPType        model.IPTypePreference     `json:"ip_type,omitempty"`
	Fragmentation model.FragmentationConfig  `json:"fragmentation,omitempty"`
	Mux           model.MuxConfig            `json:"mux,omitempty"`
	Advanced      model.AdvancedTunnelConfig `json:"advanced,omitempty"`
}

// GetConfigPath returns ~/.config/sb-cli/config.json
func GetConfigPath() (string, error) {
	if custom := os.Getenv("SB_CONFIG_PATH"); custom != "" {
		return custom, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "sb-cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// SaveConfig saves application state to disk
func SaveConfig(state *model.AppState, bypassNames map[string]bool) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	var names []string
	for name := range bypassNames {
		names = append(names, name)
	}

	activeID := ""
	if state.ActiveNode != nil {
		activeID = state.ActiveNode.ID
	}

	cfg := ConfigData{
		UserAgent:     state.UserAgent,
		Mode:          state.Mode,
		ActiveNodeID:  activeID,
		Subscriptions: state.Subscriptions,
		BypassNames:   names,
		ThemeColors:   state.ThemeColors,
		GeoRules:      state.GeoRules,
		IPType:        state.IPType,
		Fragmentation: state.Fragmentation,
		Mux:           state.Mux,
		Advanced:      state.Advanced,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadConfig loads application state from disk
func LoadConfig() (*ConfigData, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ConfigData
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults for any missing configuration fields
	if cfg.IPType == "" {
		cfg.IPType = model.IPTypePreferIPv4
	}
	if cfg.Fragmentation.Packets == "" {
		cfg.Fragmentation = model.DefaultFragmentationConfig()
	}
	if cfg.Mux.Concurrency == 0 {
		cfg.Mux = model.DefaultMuxConfig()
	}
	if cfg.Advanced.ResolveServerIP == "" {
		cfg.Advanced = model.DefaultAdvancedTunnelConfig()
	}

	return &cfg, nil
}
