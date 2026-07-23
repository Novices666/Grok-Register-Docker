package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesCurrentFlowControlDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OutputSSOEnabled {
		t.Fatalf("OutputSSOEnabled = false, want true")
	}
	if !cfg.OutputGrok2APISSO {
		t.Fatalf("OutputGrok2APISSO = false, want true")
	}
	if !cfg.OutputCPAEnabled {
		t.Fatalf("OutputCPAEnabled = false, want true")
	}
	if cfg.HTTPPoolSize != 8 {
		t.Fatalf("HTTPPoolSize = %d, want 8", cfg.HTTPPoolSize)
	}
	if cfg.PhysicalCap != 0 {
		t.Fatalf("PhysicalCap = %d, want 0", cfg.PhysicalCap)
	}
	if cfg.TempmailLOLRetries != 30 {
		t.Fatalf("TempmailLOLRetries = %d, want 30", cfg.TempmailLOLRetries)
	}
	if cfg.TempmailLOLIntervalMS != 1500 {
		t.Fatalf("TempmailLOLIntervalMS = %d, want 1500", cfg.TempmailLOLIntervalMS)
	}
	if cfg.OAuthMinIntervalSec != 4 {
		t.Fatalf("OAuthMinIntervalSec = %v, want 4", cfg.OAuthMinIntervalSec)
	}
	if cfg.OAuthRetrySec != 45 {
		t.Fatalf("OAuthRetrySec = %v, want 45", cfg.OAuthRetrySec)
	}
	if !cfg.ProbeEnabled {
		t.Fatalf("ProbeEnabled = false, want true")
	}
	if cfg.ProbeWarmupSec != 1.5 {
		t.Fatalf("ProbeWarmupSec = %v, want 1.5", cfg.ProbeWarmupSec)
	}
}

func TestLoadParsesOutputAndDelayControls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")
	body := "OUTPUT_SSO_ENABLED=0\nOUTPUT_GROK2API_SSO_ENABLED=0\nOUTPUT_CPA_ENABLED=0\nOAUTH_MIN_INTERVAL_SEC=2.5\nOAUTH_RETRY_SEC=12.5\nPROBE_ENABLED=0\nPROBE_WARMUP_SEC=2\nOAUTH_WORKERS=1\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OutputSSOEnabled {
		t.Fatalf("OutputSSOEnabled = true, want false")
	}
	if cfg.OutputGrok2APISSO {
		t.Fatalf("OutputGrok2APISSO = true, want false")
	}
	if cfg.OutputCPAEnabled {
		t.Fatalf("OutputCPAEnabled = true, want false")
	}
	if cfg.OAuthMinIntervalSec != 2.5 {
		t.Fatalf("OAuthMinIntervalSec = %v, want 2.5", cfg.OAuthMinIntervalSec)
	}
	if cfg.OAuthRetrySec != 12.5 {
		t.Fatalf("OAuthRetrySec = %v, want 12.5", cfg.OAuthRetrySec)
	}
	if cfg.ProbeEnabled {
		t.Fatalf("ProbeEnabled = true, want false")
	}
	if cfg.ProbeWarmupSec != 2 {
		t.Fatalf("ProbeWarmupSec = %v, want 2", cfg.ProbeWarmupSec)
	}
	if cfg.OAuthWorkers != 1 {
		t.Fatalf("OAuthWorkers = %v, want 1", cfg.OAuthWorkers)
	}
}
