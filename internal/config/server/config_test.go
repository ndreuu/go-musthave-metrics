package server

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func lookupEnvironment(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) { value, ok := values[name]; return value, ok }
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.json")
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
	want := &Config{Address: ":8080", LogLevel: "info", StoreInterval: 300}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestGetConfigSourcePriority(t *testing.T) {
	path := writeConfig(t, `{"address":"file:8080","log_level":"debug","store_interval":"10","store_file":"file.json","restore":true,"database_dsn":"file-dsn","key":"file-key","crypto_key":"file.pem","audit_file":"file.audit","audit_url":"https://file"}`)
	environment := map[string]string{
		"ADDRESS": "env:8080", "LOG_LEVEL": "warn", "STORE_INTERVAL": "20", "FILE_STORAGE_PATH": "env.json",
		"RESTORE": "false", "DATABASE_DSN": "env-dsn", "KEY": "env-key", "CRYPTO_KEY": "env.pem",
		"AUDIT_FILE": "env.audit", "AUDIT_URL": "https://env",
	}
	cli := []string{"-a=flag:8080", "-l=error", "-i=30", "-f=flag.json", "-r=false", "-d=flag-dsn", "-k=flag-key", "-crypto-key=flag.pem", "-audit-file=flag.audit", "-audit-url=https://flag"}
	file := &Config{Address: "file:8080", LogLevel: "debug", StoreInterval: 10, FilePath: "file.json", Restore: true, DatabaseDSN: "file-dsn", Key: "file-key", CryptoKey: "file.pem", AuditFile: "file.audit", AuditURL: "https://file"}
	flag := &Config{Address: "flag:8080", LogLevel: "error", StoreInterval: 30, FilePath: "flag.json", DatabaseDSN: "flag-dsn", Key: "flag-key", CryptoKey: "flag.pem", AuditFile: "flag.audit", AuditURL: "https://flag"}
	env := &Config{Address: "env:8080", LogLevel: "warn", StoreInterval: 20, FilePath: "env.json", DatabaseDSN: "env-dsn", Key: "env-key", CryptoKey: "env.pem", AuditFile: "env.audit", AuditURL: "https://env"}
	envWithExplicitFile := *env
	envWithExplicitFile.FilePath = "flag.json"
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want *Config
	}{
		{"file beats defaults", nil, nil, file},
		{"flags beat file", cli, nil, flag},
		{"env beats file", nil, environment, env},
		{"env beats flags except file path", cli, environment, &envWithExplicitFile},
		{"empty env is absent", nil, map[string]string{"ADDRESS": "", "STORE_INTERVAL": "", "RESTORE": ""}, file},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getConfig(append([]string{"-c", path}, tt.args...), lookupEnvironment(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("config = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGetConfigExplicitZeroFalseAndEmpty(t *testing.T) {
	path := writeConfig(t, `{"address":"file","log_level":"debug","store_interval":"10","store_file":"file.json","restore":true,"key":"file-key"}`)
	got, err := getConfig([]string{"-c", path, "-a=", "-l=", "-i=0", "-f=", "-r=false", "-k="}, lookupEnvironment(map[string]string{"FILE_STORAGE_PATH": "env.json"}))
	if err != nil || !reflect.DeepEqual(got, &Config{}) {
		t.Fatalf("config = %#v, error = %v", got, err)
	}
	zeroFile := writeConfig(t, `{"address":"","log_level":"","store_interval":"0","store_file":"","restore":false}`)
	got, err = getConfig([]string{"-c", zeroFile}, lookupEnvironment(nil))
	if err != nil || !reflect.DeepEqual(got, &Config{}) {
		t.Fatalf("config = %#v, error = %v", got, err)
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
		{"last alias wins", []string{"-config", envPath, "-c", flagPath}, "flag-file"},
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
	wrongType := writeConfig(t, `{"restore":"yes"}`)
	badInterval := writeConfig(t, `{"store_interval":"bad"}`)
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"missing file", []string{"-c", filepath.Join(t.TempDir(), "missing")}, nil},
		{"malformed JSON", []string{"-c", malformed}, nil},
		{"wrong JSON type", []string{"-c", wrongType}, nil},
		{"bad file interval", []string{"-c", badInterval}, nil},
		{"unknown flag", []string{"-unknown"}, nil},
		{"invalid flag interval", []string{"-i=abc"}, nil},
		{"invalid env interval", nil, map[string]string{"STORE_INTERVAL": "abc"}},
		{"fractional seconds", []string{"-i=1500ms"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := getConfig(tt.args, lookupEnvironment(tt.env)); err == nil || got != nil {
				t.Fatalf("config = %#v, error = %v; want nil config and error", got, err)
			}
		})
	}
}

func TestGetConfigDuration(t *testing.T) {
	path := writeConfig(t, `{"store_interval":"1m"}`)
	got, err := getConfig([]string{"-config", path}, lookupEnvironment(nil))
	if err != nil || got.StoreInterval != 60 {
		t.Fatalf("config = %#v, error = %v", got, err)
	}
}
