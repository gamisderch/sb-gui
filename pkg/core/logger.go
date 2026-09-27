package core

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	logMu       sync.Mutex
	logDirOnce  sync.Once
	logDirPath  string
)

// EnsureLogDir ensures /tmp/sb-cli exists
func EnsureLogDir() string {
	logDirOnce.Do(func() {
		logDirPath = filepath.Join(os.TempDir(), "sb-cli")
		_ = os.MkdirAll(logDirPath, 0755)
	})
	return logDirPath
}

func getLogFilePath(name string) string {
	return filepath.Join(EnsureLogDir(), name)
}

func appendToLog(filename string, line string) {
	logMu.Lock()
	defer logMu.Unlock()

	p := getLogFilePath(filename)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, _ = fmt.Fprintf(f, "%s %s\n", timestamp, line)
}

// LogInfo writes an informational message to main.log
func LogInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	appendToLog("main.log", "[INFO] "+msg)
}

// LogWarn writes a warning message to main.log
func LogWarn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	appendToLog("main.log", "[WARN] "+msg)
}

// LogError writes an error message to main.log
func LogError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	appendToLog("main.log", "[ERROR] "+msg)
}

// LogSub writes a message to subscription.log
func LogSub(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	appendToLog("subscription.log", msg)
}

// LogDaemon writes a message to daemon.log
func LogDaemon(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	appendToLog("daemon.log", msg)
}

// ReadLastLines reads the last maxLines from a file
func ReadLastLines(filePath string, maxLines int) []string {
	if maxLines <= 0 {
		maxLines = 500
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	// Allow large lines
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

// GetMainLogs returns recent lines from main.log
func GetMainLogs(maxLines int) []string {
	return ReadLastLines(getLogFilePath("main.log"), maxLines)
}

// GetCoreLogs returns recent lines from xray.log
func GetCoreLogs(maxLines int) []string {
	return ReadLastLines(getLogFilePath("xray.log"), maxLines)
}

// GetTunnelLogs returns recent lines from sing-box.log
func GetTunnelLogs(maxLines int) []string {
	return ReadLastLines(getLogFilePath("sing-box.log"), maxLines)
}

// GetSubLogs returns recent lines from subscription.log
func GetSubLogs(maxLines int) []string {
	return ReadLastLines(getLogFilePath("subscription.log"), maxLines)
}

// GetDaemonLogs returns recent daemon logs from journalctl or daemon.log
func GetDaemonLogs(maxLines int) []string {
	if maxLines <= 0 {
		maxLines = 200
	}

	// First try journalctl --user
	out, err := exec.Command("journalctl", "--user", "-u", ServiceName, "-n", fmt.Sprintf("%d", maxLines), "--no-pager").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		rawLines := strings.Split(strings.TrimSpace(string(out)), "\n")
		// Filter out useless '-- Logs begin at ... --' line if present
		var res []string
		for _, l := range rawLines {
			if strings.HasPrefix(l, "-- Logs begin") {
				continue
			}
			res = append(res, l)
		}
		if len(res) > 0 {
			return res
		}
	}

	// Fallback to daemon.log
	return ReadLastLines(getLogFilePath("daemon.log"), maxLines)
}

// ClearLogFile truncates the specified log file
func ClearLogFile(filename string) error {
	logMu.Lock()
	defer logMu.Unlock()
	p := getLogFilePath(filename)
	return os.WriteFile(p, []byte{}, 0644)
}
