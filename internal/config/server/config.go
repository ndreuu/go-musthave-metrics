// Package server loads the server configuration from CLI, environment and JSON.
package server

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"go-musthave-metrics/internal/config"
)

type Config struct {
	Address       string
	LogLevel      string
	StoreInterval int
	FilePath      string
	Restore       bool
	DatabaseDSN   string
	Key           string
	CryptoKey     string
	AuditFile     string
	AuditURL      string
}

type configSource struct {
	Address       *string `json:"address" flag:"a" env:"ADDRESS"`
	LogLevel      *string `json:"log_level" flag:"l" env:"LOG_LEVEL"`
	StoreInterval *string `json:"store_interval" flag:"i" env:"STORE_INTERVAL"`
	FilePath      *string `json:"store_file" flag:"f" env:"FILE_STORAGE_PATH"`
	Restore       *bool   `json:"restore" flag:"r" env:"RESTORE"`
	DatabaseDSN   *string `json:"database_dsn" flag:"d" env:"DATABASE_DSN"`
	Key           *string `json:"key" flag:"k" env:"KEY"`
	CryptoKey     *string `json:"crypto_key" flag:"crypto-key" env:"CRYPTO_KEY"`
	AuditFile     *string `json:"audit_file" flag:"audit-file" env:"AUDIT_FILE"`
	AuditURL      *string `json:"audit_url" flag:"audit-url" env:"AUDIT_URL"`
}

// GetConfig loads defaults, JSON, CLI and environment, in increasing priority.
// FILE_STORAGE_PATH retains its existing exception: explicit -f wins over env.
func GetConfig() (*Config, error) {
	return loadConfig(flag.NewFlagSet("server", flag.ExitOnError), os.Args[1:], os.LookupEnv)
}

func getConfig(args []string, lookup func(string) (string, bool)) (*Config, error) {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return loadConfig(flags, args, lookup)
}

func loadConfig(flags *flag.FlagSet, args []string, lookup func(string) (string, bool)) (*Config, error) {
	cli := configSource{
		Address:       flags.String("a", ":8080", "address and port to run server"),
		LogLevel:      flags.String("l", "info", "log level"),
		StoreInterval: flags.String("i", "300", "store interval (seconds or Go duration)"),
		FilePath:      flags.String("f", "", "path to metrics file"),
		Restore:       flags.Bool("r", false, "restore metrics from file"),
		DatabaseDSN:   flags.String("d", "", "database DSN"),
		Key:           flags.String("k", "", "key for signing data"),
		CryptoKey:     flags.String("crypto-key", "", "path to private key file for decryption"),
		AuditFile:     flags.String("audit-file", "", "path to audit log file"),
		AuditURL:      flags.String("audit-url", "", "URL to send audit logs"),
	}
	var path string
	flags.StringVar(&path, "c", "", "path to JSON config file")
	flags.StringVar(&path, "config", "", "path to JSON config file")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	config.ExplicitFlags(&cli, flags)
	var file configSource
	if err := config.ReadFile(flags, path, lookup, &file); err != nil {
		return nil, err
	}
	env, err := config.Environment(lookup, configSource{FilePath: cli.FilePath})
	if err != nil {
		return nil, err
	}
	defaults := configSource{
		Address: config.Pointer(":8080"), LogLevel: config.Pointer("info"), StoreInterval: config.Pointer("300"),
		FilePath: config.Pointer(""), Restore: config.Pointer(false), DatabaseDSN: config.Pointer(""),
		Key: config.Pointer(""), CryptoKey: config.Pointer(""), AuditFile: config.Pointer(""), AuditURL: config.Pointer(""),
	}
	merged, err := config.Merge(defaults, file, cli, env)
	if err != nil {
		return nil, err
	}
	interval, err := config.Duration(*merged.StoreInterval)
	if err != nil {
		return nil, fmt.Errorf("store interval: %w", err)
	}
	if interval%time.Second != 0 {
		return nil, fmt.Errorf("store interval must be a whole number of seconds: %q", *merged.StoreInterval)
	}
	return &Config{
		Address: *merged.Address, LogLevel: *merged.LogLevel, StoreInterval: int(interval / time.Second),
		FilePath: *merged.FilePath, Restore: *merged.Restore, DatabaseDSN: *merged.DatabaseDSN,
		Key: *merged.Key, CryptoKey: *merged.CryptoKey, AuditFile: *merged.AuditFile, AuditURL: *merged.AuditURL,
	}, nil
}
