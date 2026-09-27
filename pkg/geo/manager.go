package geo

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"signbox-cli/pkg/model"
)

const (
	GeoIPURL   = "https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
	GeoSiteURL = "https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"
)

// DBStat holds metadata about a geo database file
type DBStat struct {
	Exists     bool
	Size       int64
	ModTime    time.Time
	TotalCount int
}

// RuleSetMeta represents a compiled or exported rule-set ready for sing-box
type RuleSetMeta struct {
	Tag      string
	Path     string
	IsBinary bool
	Action   model.GeoRuleAction
}

// Manager manages GeoIP and GeoSite databases and compiled rule-sets
type Manager struct {
	mu            sync.RWMutex
	baseDir       string
	cachedSites   []string
	cachedIPs     []string
	cachedSiteMap map[string]bool
	cachedIPMap   map[string]bool
}

// NewManager creates a new Geo Manager
func NewManager() *Manager {
	home, err := os.UserHomeDir()
	var baseDir string
	if err == nil {
		baseDir = filepath.Join(home, ".config", "sb-cli", "geo")
	} else {
		baseDir = filepath.Join(os.TempDir(), "sb-cli", "geo")
	}
	_ = os.MkdirAll(filepath.Join(baseDir, "rules"), 0755)

	return &Manager{
		baseDir:       baseDir,
		cachedSiteMap: make(map[string]bool),
		cachedIPMap:   make(map[string]bool),
	}
}

// GetBaseDir returns the base geo directory
func (m *Manager) GetBaseDir() string {
	return m.baseDir
}

// GetGeoSitePath returns path to geosite.db
func (m *Manager) GetGeoSitePath() string {
	return filepath.Join(m.baseDir, "geosite.db")
}

// GetGeoIPPath returns path to geoip.db
func (m *Manager) GetGeoIPPath() string {
	return filepath.Join(m.baseDir, "geoip.db")
}

// GetRulesDir returns path to compiled rules directory
func (m *Manager) GetRulesDir() string {
	return filepath.Join(m.baseDir, "rules")
}

// GetDBStats returns information about local geoip.db and geosite.db
func (m *Manager) GetDBStats(singboxBin string) (siteStat DBStat, ipStat DBStat) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sitePath := m.GetGeoSitePath()
	if fi, err := os.Stat(sitePath); err == nil {
		siteStat.Exists = true
		siteStat.Size = fi.Size()
		siteStat.ModTime = fi.ModTime()
		if len(m.cachedSites) > 0 {
			siteStat.TotalCount = len(m.cachedSites)
		}
	}

	ipPath := m.GetGeoIPPath()
	if fi, err := os.Stat(ipPath); err == nil {
		ipStat.Exists = true
		ipStat.Size = fi.Size()
		ipStat.ModTime = fi.ModTime()
		if len(m.cachedIPs) > 0 {
			ipStat.TotalCount = len(m.cachedIPs)
		}
	}

	return siteStat, ipStat
}

// SyncDatabases downloads fresh geosite.db and geoip.db from GitHub releases
func (m *Manager) SyncDatabases(client *http.Client, progress func(file string, current, total int64)) error {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	_ = os.MkdirAll(m.baseDir, 0755)

	// 1. Download geosite.db
	if err := downloadFileWithProgress(client, GeoSiteURL, m.GetGeoSitePath()+".tmp", func(cur, tot int64) {
		if progress != nil {
			progress("geosite.db", cur, tot)
		}
	}); err != nil {
		return fmt.Errorf("failed to download geosite.db: %w", err)
	}

	// 2. Download geoip.db
	if err := downloadFileWithProgress(client, GeoIPURL, m.GetGeoIPPath()+".tmp", func(cur, tot int64) {
		if progress != nil {
			progress("geoip.db", cur, tot)
		}
	}); err != nil {
		_ = os.Remove(m.GetGeoSitePath() + ".tmp")
		return fmt.Errorf("failed to download geoip.db: %w", err)
	}

	// Atomic replace
	if err := os.Rename(m.GetGeoSitePath()+".tmp", m.GetGeoSitePath()); err != nil {
		return fmt.Errorf("failed to save geosite.db: %w", err)
	}
	if err := os.Rename(m.GetGeoIPPath()+".tmp", m.GetGeoIPPath()); err != nil {
		return fmt.Errorf("failed to save geoip.db: %w", err)
	}

	// Invalidate cache
	m.mu.Lock()
	m.cachedSites = nil
	m.cachedIPs = nil
	m.cachedSiteMap = make(map[string]bool)
	m.cachedIPMap = make(map[string]bool)
	m.mu.Unlock()

	return nil
}

func downloadFileWithProgress(client *http.Client, url, targetPath string, onProgress func(cur, tot int64)) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad HTTP status: %s", resp.Status)
	}

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var current int64

	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if ew != nil {
				return ew
			}
			current += int64(nw)
			if onProgress != nil && time.Since(lastReport) > 100*time.Millisecond {
				lastReport = time.Now()
				onProgress(current, total)
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}
	}

	if onProgress != nil {
		onProgress(current, total)
	}

	return nil
}

// GetGeoSiteCategories returns all categories available in geosite.db
func (m *Manager) GetGeoSiteCategories(singboxBin string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cachedSites) > 0 {
		return m.cachedSites, nil
	}

	sitePath := m.GetGeoSitePath()
	if _, err := os.Stat(sitePath); err != nil {
		// Return common top categories if database is not downloaded yet
		return getBuiltinTopGeoSites(), nil
	}

	cmd := exec.Command(singboxBin, "geosite", "list", "-f", sitePath)
	out, err := cmd.Output()
	if err != nil {
		return getBuiltinTopGeoSites(), nil
	}

	seen := make(map[string]bool)
	var list []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: "category (count)" e.g. "youtube (354)" or "ruanmei (18)"
		cat := line
		if idx := strings.Index(line, " ("); idx != -1 {
			cat = strings.TrimSpace(line[:idx])
		}
		if cat != "" && !seen[cat] {
			seen[cat] = true
			list = append(list, cat)
		}
	}

	sort.Strings(list)
	m.cachedSites = list
	m.cachedSiteMap = seen
	return list, nil
}

// GetGeoIPCodes returns all 2-letter country codes available in geoip.db
func (m *Manager) GetGeoIPCodes(singboxBin string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.cachedIPs) > 0 {
		return m.cachedIPs, nil
	}

	ipPath := m.GetGeoIPPath()
	if _, err := os.Stat(ipPath); err != nil {
		return getBuiltinTopGeoIPs(), nil
	}

	cmd := exec.Command(singboxBin, "geoip", "list", "-f", ipPath)
	out, err := cmd.Output()
	if err != nil {
		return getBuiltinTopGeoIPs(), nil
	}

	seen := make(map[string]bool)
	var list []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		code := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if code != "" && !seen[code] {
			seen[code] = true
			list = append(list, code)
		}
	}

	sort.Strings(list)
	m.cachedIPs = list
	m.cachedIPMap = seen
	return list, nil
}

// CompileRuleSet exports and compiles a rule-set from geo databases into binary (.srs)
func (m *Manager) CompileRuleSet(singboxBin string, rule *model.GeoRule) (srsPath string, isBinary bool, err error) {
	rulesDir := m.GetRulesDir()
	_ = os.MkdirAll(rulesDir, 0755)

	nameSanitized := strings.ReplaceAll(rule.Name, "@", "_")
	nameSanitized = strings.ReplaceAll(nameSanitized, "!", "not_")

	prefix := string(rule.Type)
	binSRS := filepath.Join(rulesDir, fmt.Sprintf("%s-%s.srs", prefix, nameSanitized))
	srcJSON := filepath.Join(rulesDir, fmt.Sprintf("%s-%s.json", prefix, nameSanitized))

	var dbPath string
	if rule.Type == model.GeoTypeSite {
		dbPath = m.GetGeoSitePath()
	} else {
		dbPath = m.GetGeoIPPath()
	}

	dbInfo, dbErr := os.Stat(dbPath)

	// If binary SRS exists and is newer than database, reuse it
	if srsInfo, err := os.Stat(binSRS); err == nil && dbErr == nil {
		if srsInfo.ModTime().After(dbInfo.ModTime()) {
			return binSRS, true, nil
		}
	}

	if dbErr != nil {
		return "", false, fmt.Errorf("database %s not found (run sync first): %w", dbPath, dbErr)
	}

	// 1. Export from database
	var exportCmd *exec.Cmd
	if rule.Type == model.GeoTypeSite {
		exportCmd = exec.Command(singboxBin, "geosite", "export", rule.Name, "-f", dbPath, "-o", srcJSON)
	} else {
		exportCmd = exec.Command(singboxBin, "geoip", "export", rule.Name, "-f", dbPath, "-o", srcJSON)
	}

	if out, err := exportCmd.CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("failed to export %s %s: %s (%w)", rule.Type, rule.Name, strings.TrimSpace(string(out)), err)
	}

	// 2. Compile to binary .srs
	compileCmd := exec.Command(singboxBin, "rule-set", "compile", srcJSON, "-o", binSRS)
	if out, err := compileCmd.CombinedOutput(); err != nil {
		// If compile command failed, we can still fall back to source JSON!
		_ = out
		return srcJSON, false, nil
	}

	// Remove intermediary JSON to save disk space
	_ = os.Remove(srcJSON)
	return binSRS, true, nil
}

// EnsureRuleSets compiles all active geo rules and returns metadata for sing-box configuration
func (m *Manager) EnsureRuleSets(singboxBin string, rules []*model.GeoRule) ([]RuleSetMeta, error) {
	var metas []RuleSetMeta
	for _, r := range rules {
		if r == nil || strings.TrimSpace(r.Name) == "" {
			continue
		}
		path, isBin, err := m.CompileRuleSet(singboxBin, r)
		if err != nil {
			// Non-fatal: log and skip rule that cannot be compiled
			continue
		}
		tag := fmt.Sprintf("%s-%s", r.Type, strings.ReplaceAll(r.Name, "@", "_"))
		metas = append(metas, RuleSetMeta{
			Tag:      tag,
			Path:     path,
			IsBinary: isBin,
			Action:   r.Action,
		})
	}
	return metas, nil
}

func getBuiltinTopGeoSites() []string {
	return []string{
		"category-ads-all",
		"category-ads",
		"ru",
		"youtube",
		"google",
		"telegram",
		"discord",
		"openai",
		"steam",
		"github",
		"vk",
		"yandex",
		"mailru",
		"rutracker",
		"kinopoisk",
		"netflix",
		"spotify",
		"twitch",
		"twitter",
		"meta",
		"instagram",
		"facebook",
		"tiktok",
		"apple",
		"microsoft",
		"cloudflare",
	}
}

func getBuiltinTopGeoIPs() []string {
	return []string{
		"ru", "cn", "us", "nl", "de", "gb", "ua", "by", "kz", "fr", "ca", "jp", "sg", "fi", "se", "ch", "pl",
	}
}
