package geo

import (
	"os"
	"path/filepath"
	"testing"

	"signbox-cli/pkg/model"
)

func TestGeoManagerPaths(t *testing.T) {
	m := NewManager()
	if m.GetBaseDir() == "" {
		t.Errorf("expected non-empty base directory")
	}
	if filepath.Base(m.GetGeoSitePath()) != "geosite.db" {
		t.Errorf("expected geosite.db, got %s", m.GetGeoSitePath())
	}
	if filepath.Base(m.GetGeoIPPath()) != "geoip.db" {
		t.Errorf("expected geoip.db, got %s", m.GetGeoIPPath())
	}
	if filepath.Base(m.GetRulesDir()) != "rules" {
		t.Errorf("expected rules dir, got %s", m.GetRulesDir())
	}
}

func TestBuiltinFallbacks(t *testing.T) {
	m := NewManager()
	// Temporary non-existent binary to force fallback
	sites, err := m.GetGeoSiteCategories("/non-existent/sing-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sites) == 0 {
		t.Errorf("expected fallback geosite categories, got empty list")
	}

	ips, err := m.GetGeoIPCodes("/non-existent/sing-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) == 0 {
		t.Errorf("expected fallback geoip codes, got empty list")
	}
}

func TestExportAndCompileWithSampleDB(t *testing.T) {
	// If /tmp/geo-test has the databases we downloaded during test, verify real export
	if _, err := os.Stat("/tmp/geo-test/geosite.db"); err != nil {
		t.Skip("sample geosite.db not available in /tmp/geo-test, skipping real export test")
	}

	tmpDir, err := os.MkdirTemp("", "geo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := &Manager{
		baseDir:       tmpDir,
		cachedSiteMap: make(map[string]bool),
		cachedIPMap:   make(map[string]bool),
	}
	_ = os.MkdirAll(filepath.Join(tmpDir, "rules"), 0755)

	// Copy geosite.db from /tmp/geo-test
	data, err := os.ReadFile("/tmp/geo-test/geosite.db")
	if err != nil {
		t.Fatalf("failed to read test geosite.db: %v", err)
	}
	if err := os.WriteFile(m.GetGeoSitePath(), data, 0644); err != nil {
		t.Fatalf("failed to write geosite.db: %v", err)
	}

	singboxBin, _ := filepath.Abs("../../bin/sing-box")
	if _, err := os.Stat(singboxBin); err != nil {
		singboxBin, _ = filepath.Abs("./bin/sing-box")
		if _, err := os.Stat(singboxBin); err != nil {
			t.Skip("sing-box binary not found, skipping compile test")
		}
	}

	rule := &model.GeoRule{
		Type:   model.GeoTypeSite,
		Name:   "youtube",
		Action: model.GeoActionProxy,
	}

	srsPath, isBin, err := m.CompileRuleSet(singboxBin, rule)
	if err != nil {
		t.Fatalf("CompileRuleSet failed: %v", err)
	}
	if !isBin {
		t.Errorf("expected compiled binary SRS")
	}
	if _, err := os.Stat(srsPath); err != nil {
		t.Errorf("compiled SRS file does not exist: %v", err)
	}
}

func TestEnsureRuleSetsWithCustomRules(t *testing.T) {
	singboxBin, _ := filepath.Abs("../../bin/sing-box")
	if _, err := os.Stat(singboxBin); err != nil {
		singboxBin, _ = filepath.Abs("./bin/sing-box")
		if _, err := os.Stat(singboxBin); err != nil {
			t.Skip("sing-box binary not found, skipping compile test")
		}
	}

	tmpDir := t.TempDir()
	m := &Manager{
		baseDir:       tmpDir,
		cachedSiteMap: make(map[string]bool),
		cachedIPMap:   make(map[string]bool),
	}
	_ = os.MkdirAll(filepath.Join(tmpDir, "rules"), 0755)

	rules := []*model.GeoRule{
		{Type: model.GeoTypeSite, Name: "vk.com", Action: model.GeoActionDirect},
		{Type: model.GeoTypeSite, Name: "yandex.ru", Action: model.GeoActionDirect},
		{Type: model.GeoTypeIP, Name: "0.0.0.0/8", Action: model.GeoActionDirect},
		{Type: model.GeoTypeIP, Name: "5.45.192.0/18", Action: model.GeoActionDirect},
		{Type: model.GeoTypeSite, Name: "blocked-ads.com", Action: model.GeoActionBlock},
	}

	metas, err := m.EnsureRuleSets(singboxBin, rules)
	if err != nil {
		t.Fatalf("EnsureRuleSets failed: %v", err)
	}

	if len(metas) != 2 { // custom-direct and custom-block
		t.Fatalf("expected 2 metas, got %d", len(metas))
	}

	for _, meta := range metas {
		if !meta.IsBinary {
			t.Errorf("expected binary rule set for %s", meta.Tag)
		}
		if _, err := os.Stat(meta.Path); err != nil {
			t.Errorf("rule set file not found: %s", meta.Path)
		}
	}
}

