package core

import (
	"encoding/json"
	"os"
	"path/filepath"

	"signbox-cli/pkg/model"
)

// ConfigData stores saved user preferences, subscriptions and bypass list
type ConfigData struct {
	UserAgent     string                `json:"user_agent"`
	Mode          model.TunnelMode      `json:"mode"`
	ActiveNodeID  string                `json:"active_node_id"`
	Subscriptions []*model.Subscription `json:"subscriptions"`
	BypassNames   []string              `json:"bypass_names"`
	ThemeColors   map[string]string     `json:"theme_colors,omitempty"`
	GeoRules      []*model.GeoRule      `json:"geo_rules,omitempty"`
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

	return &cfg, nil
}
