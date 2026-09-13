package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defSeconds int
		want       time.Duration
	}{
		{"duration format", "10s", 5, 10 * time.Second},
		{"duration minutes", "1m", 5, time.Minute},
		{"plain seconds", "3", 5, 3 * time.Second},
		{"invalid falls back to default", "abc", 5, 5 * time.Second},
		{"empty falls back to default", "", 5, 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseInterval(tt.input, tt.defSeconds); got != tt.want {
				t.Errorf("parseInterval(%q, %d) = %v, want %v", tt.input, tt.defSeconds, got, tt.want)
			}
		})
	}
}

func TestResolveString(t *testing.T) {
	t.Run("flag wins", func(t *testing.T) {
		got := resolveString(true, "flag", "ENV", "file", "def")
		if got != "flag" {
			t.Errorf("got %q, want flag", got)
		}
	})

	t.Run("file beats default", func(t *testing.T) {
		t.Setenv("UNUSED_ENV", "")
		got := resolveString(false, "", "UNUSED_ENV", "file", "def")
		if got != "file" {
			t.Errorf("got %q, want file", got)
		}
	})
}

func TestResolveInt(t *testing.T) {
	t.Run("flag wins", func(t *testing.T) {
		fileVal := 99
		got := resolveInt(true, 1, "ENV", &fileVal, 5)
		if got != 1 {
			t.Errorf("got %d, want 1", got)
		}
	})

	t.Run("file beats default", func(t *testing.T) {
		t.Setenv("UNUSED_ENV", "")
		fileVal := 99
		got := resolveInt(false, 0, "UNUSED_ENV", &fileVal, 5)
		if got != 99 {
			t.Errorf("got %d, want 99", got)
		}
	})

	t.Run("default when no file", func(t *testing.T) {
		t.Setenv("UNUSED_ENV", "")
		got := resolveInt(false, 0, "UNUSED_ENV", nil, 5)
		if got != 5 {
			t.Errorf("got %d, want 5", got)
		}
	})
}

func TestLoadFileConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")
	content := `{
		"address": "localhost:9999",
		"report_interval": "5s",
		"poll_interval": "1s",
		"crypto_key": "/keys/pub.pem",
		"rate_limit": 7
	}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if fc.Address != "localhost:9999" {
		t.Errorf("address = %q, want localhost:9999", fc.Address)
	}
	if fc.ReportInterval != "5s" {
		t.Errorf("report_interval = %q, want 5s", fc.ReportInterval)
	}
	if fc.PollInterval != "1s" {
		t.Errorf("poll_interval = %q, want 1s", fc.PollInterval)
	}
	if fc.CryptoKey != "/keys/pub.pem" {
		t.Errorf("crypto_key = %q, want /keys/pub.pem", fc.CryptoKey)
	}
	if fc.RateLimit == nil || *fc.RateLimit != 7 {
		t.Errorf("rate_limit = %v, want 7", fc.RateLimit)
	}
}

func TestFileConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if fc.RateLimit != nil {
		t.Errorf("rate_limit should be nil for empty config, got %v", *fc.RateLimit)
	}
}