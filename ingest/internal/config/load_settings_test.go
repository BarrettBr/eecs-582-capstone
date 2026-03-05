package config

import "testing"

func TestLoadSettingsDefaultsAPIBasePath(t *testing.T) {
	t.Setenv("API_BASE_PATH", "")

	cfg, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings() error = %v", err)
	}
	if cfg.Stream.APIBasePath != "/api/v1" {
		t.Fatalf("cfg.Stream.APIBasePath = %q, want %q", cfg.Stream.APIBasePath, "/api/v1")
	}
}

func TestLoadSettingsNormalizesAPIBasePath(t *testing.T) {
	t.Setenv("API_BASE_PATH", "api/v1/")

	cfg, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings() error = %v", err)
	}
	if cfg.Stream.APIBasePath != "/api/v1" {
		t.Fatalf("cfg.Stream.APIBasePath = %q, want %q", cfg.Stream.APIBasePath, "/api/v1")
	}
}
