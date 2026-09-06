package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("LLM_API_KEY")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPath == "" || cfg.LLMModel == "" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoadMissingEnvFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err == nil {
		t.Fatal("expected error for missing env file")
	}
}

func TestValidateMissingKey(t *testing.T) {
	// LLM API Key 缺失不报错（设计上允许），但 base_url/model/db 缺失必须报错
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestEnsureDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	cfg := &Config{DBPath: filepath.Join(dir, "db.sqlite")}
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("dir not created: %v", err)
	}
}
