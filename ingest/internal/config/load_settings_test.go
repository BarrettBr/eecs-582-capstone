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

func TestLoadSettingsDefaultsBufferChunkSize(t *testing.T) {
	t.Setenv("BUFFER_CHUNK_SIZE", "")

	cfg, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings() error = %v", err)
	}
	if cfg.Ingest.BufferChunkSize != 32 {
		t.Fatalf("cfg.Ingest.BufferChunkSize = %d, want 32", cfg.Ingest.BufferChunkSize)
	}
}

func TestLoadSettingsRejectsInvalidBufferChunkSize(t *testing.T) {
	t.Setenv("BUFFER_CHUNK_SIZE", "24")

	_, err := loadSettings()
	if err == nil {
		t.Fatalf("loadSettings() error = nil, want non-nil")
	}
}
