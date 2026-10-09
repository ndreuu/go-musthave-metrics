package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func lookupEnvironment(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) { value, ok := values[name]; return value, ok }
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGetConfigDefaults(t *testing.T) {
	got, err := getConfig(nil, lookupEnvironment(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := &Config{ServerAddress: "http://localhost:8080", PollInterval: 2 * time.Second, ReportInterval: 10 * time.Second, RateLimit: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestGetConfigSourcePriority(t *testing.T) {
	path := writeConfig(t, `{"address":"file:8080","key":"file-key","crypto_key":"file.pem","poll_interval":"1s","report_interval":"1m","rate_limit":3}`)
	file := &Config{ServerAddress: "http://file:8080", Key: "file-key", CryptoKey: "file.pem", PollInterval: time.Second, ReportInterval: time.Minute, RateLimit: 3}
	environment := map[string]string{
		"ADDRESS": "https://env:8080", "KEY": "env-key", "CRYPTO_KEY": "env.pem",
		"POLL_INTERVAL": "4", "REPORT_INTERVAL": "5s", "RATE_LIMIT": "6",
	}
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want *Config
	}{
		{"file beats defaults", nil, nil, file},
		{"env beats file", nil, environment, &Config{ServerAddress: "https://env:8080", Key: "env-key", CryptoKey: "env.pem", PollInterval: 4 * time.Second, ReportInterval: 5 * time.Second, RateLimit: 6}},
		{"explicit flags beat env", []string{"-a=flag:8080", "-k=flag-key", "-crypto-key=flag.pem", "-p=7s", "-r=8", "-l=9"}, environment, &Config{ServerAddress: "http://flag:8080", Key: "flag-key", CryptoKey: "flag.pem", PollInterval: 7 * time.Second, ReportInterval: 8 * time.Second, RateLimit: 9}},
		{"empty env is absent", nil, map[string]string{"ADDRESS": "", "KEY": "", "RATE_LIMIT": ""}, file},
		{"explicit zero rate and empty strings", []string{"-a=", "-k=", "-crypto-key=", "-l=0"}, environment, &Config{RateLimit: 1, PollInterval: 4 * time.Second, ReportInterval: 5 * time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-c", path}, tt.args...)
			got, err := getConfig(args, lookupEnvironment(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("config = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGetConfigFileSelection(t *testing.T) {
	envPath := writeConfig(t, `{"key":"env-file"}`)
	flagPath := writeConfig(t, `{"key":"flag-file"}`)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"CONFIG env", nil, "env-file"},
		{"short flag", []string{"-c", flagPath}, "flag-file"},
		{"long flag", []string{"-config", flagPath}, "flag-file"},
		{"last alias wins", []string{"-c", envPath, "-config", flagPath}, "flag-file"},
		{"empty short flag disables CONFIG", []string{"-c="}, ""},
		{"empty long flag disables CONFIG", []string{"-config="}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getConfig(tt.args, lookupEnvironment(map[string]string{"CONFIG": envPath}))
			if err != nil {
				t.Fatal(err)
			}
			if got.Key != tt.want {
				t.Fatalf("key = %q, want %q", got.Key, tt.want)
			}
		})
	}
}

func TestGetConfigErrors(t *testing.T) {
	malformed := writeConfig(t, `{`)
	wrongType := writeConfig(t, `{"rate_limit":"three"}`)
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"missing file", []string{"-c", filepath.Join(t.TempDir(), "missing")}, nil},
		{"malformed JSON", []string{"-c", malformed}, nil},
		{"wrong JSON type", []string{"-c", wrongType}, nil},
		{"unknown flag", []string{"-unknown"}, nil},
		{"invalid rate flag", []string{"-l=abc"}, nil},
		{"invalid rate env", nil, map[string]string{"RATE_LIMIT": "abc"}},
		{"invalid poll interval", []string{"-p=abc"}, nil},
		{"invalid report interval", nil, map[string]string{"REPORT_INTERVAL": "abc"}},
		{"zero poll interval", []string{"-p=0"}, nil},
		{"negative poll interval", []string{"-p=-2s"}, nil},
		{"zero report interval", nil, map[string]string{"REPORT_INTERVAL": "0"}},
		{"negative report interval", nil, map[string]string{"REPORT_INTERVAL": "-2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := getConfig(tt.args, lookupEnvironment(tt.env)); err == nil || got != nil {
				t.Fatalf("config = %#v, error = %v; want nil config and error", got, err)
			}
		})
	}
}

func TestGetConfigIgnoresOverriddenInvalidEnvironment(t *testing.T) {
	got, err := getConfig([]string{"-l=4", "-p=3s"}, lookupEnvironment(map[string]string{"RATE_LIMIT": "invalid", "POLL_INTERVAL": "invalid"}))
	if err != nil || got.RateLimit != 4 || got.PollInterval != 3*time.Second {
		t.Fatalf("config = %#v, error = %v", got, err)
	}
}

func TestGetConfigExplicitEmptyFileValues(t *testing.T) {
	path := writeConfig(t, `{"address":"","key":"","crypto_key":"","rate_limit":0}`)
	got, err := getConfig([]string{"-c", path}, lookupEnvironment(nil))
	if err != nil || !reflect.DeepEqual(got, &Config{RateLimit: 1, PollInterval: 2 * time.Second, ReportInterval: 10 * time.Second}) {
		t.Fatalf("config = %#v, error = %v", got, err)
	}
}

func TestGetConfigExplicitZeroIntervalOverridesPositiveValues(t *testing.T) {
	path := writeConfig(t, `{"poll_interval":"3s"}`)
	if got, err := getConfig([]string{"-c", path, "-p=0"}, lookupEnvironment(map[string]string{"POLL_INTERVAL": "4s"})); err == nil || got != nil {
		t.Fatalf("config = %#v, error = %v; expected zero flag to override positive file/env and fail validation", got, err)
	}
}
