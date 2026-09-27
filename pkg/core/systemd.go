package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const ServiceName = "signbox-vpn.service"

// GetServiceFilePath returns ~/.config/systemd/user/signbox-vpn.service
func GetServiceFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, ServiceName), nil
}

// InstallSystemdService creates and enables systemd --user service for auto-start
func InstallSystemdService() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	servicePath, err := GetServiceFilePath()
	if err != nil {
		return "", err
	}

	workDir := filepath.Dir(filepath.Dir(execPath))
	if _, err := os.Stat(filepath.Join(workDir, "bin")); err != nil {
		workDir = filepath.Dir(execPath)
	}

	content := fmt.Sprintf(`[Unit]
Description=signbox-cli VPN background service
After=network-online.target default.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=%s
ExecStart=%s --start
ExecStop=%s --stop
Restart=on-failure
RestartSec=5
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/bin:/bin:%s

[Install]
WantedBy=default.target
`, workDir, execPath, execPath, filepath.Dir(execPath))

	if err := os.WriteFile(servicePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write service file: %w", err)
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if err := exec.Command("systemctl", "--user", "enable", ServiceName).Run(); err != nil {
		return servicePath, fmt.Errorf("failed to enable service: %w", err)
	}

	return servicePath, nil
}

// UninstallSystemdService disables and removes the systemd user service
func UninstallSystemdService() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", ServiceName).Run()

	servicePath, err := GetServiceFilePath()
	if err == nil {
		_ = os.Remove(servicePath)
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

// IsSystemdServiceEnabled checks if the systemd user service is enabled
func IsSystemdServiceEnabled() bool {
	out, err := exec.Command("systemctl", "--user", "is-enabled", ServiceName).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "enabled"
}

// IsSystemdServiceRunning checks if the systemd user service is active
func IsSystemdServiceRunning() bool {
	out, err := exec.Command("systemctl", "--user", "is-active", ServiceName).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}
