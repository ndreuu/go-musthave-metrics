// Package server loads the server configuration from CLI, environment and JSON.
package server

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"go-musthave-metrics/internal/config"
)

type Config struct {
	Address       string
	GRPCAddress   string
	GRPCCertFile  string
	GRPCKeyFile   string
	LogLevel      string
	StoreInterval int
	FilePath      string
	Restore       bool
	DatabaseDSN   string
	Key           string
	CryptoKey     string
	AuditFile     string
	AuditURL      string
	TrustedSubnet string
}

type configSource struct {
	Address       *string `json:"address" flag:"a" env:"ADDRESS"`
	GRPCAddress   *string `json:"grpc_address" flag:"grpc-address" env:"GRPC_ADDRESS"`
	GRPCCertFile  *string `json:"grpc_cert_file" flag:"grpc-cert-file" env:"GRPC_CERT_FILE"`
	GRPCKeyFile   *string `json:"grpc_key_file" flag:"grpc-key-file" env:"GRPC_KEY_FILE"`
	LogLevel      *string `json:"log_level" flag:"l" env:"LOG_LEVEL"`
	StoreInterval *string `json:"store_interval" flag:"i" env:"STORE_INTERVAL"`
	FilePath      *string `json:"store_file" flag:"f" env:"FILE_STORAGE_PATH"`
	Restore       *bool   `json:"restore" flag:"r" env:"RESTORE"`
	DatabaseDSN   *string `json:"database_dsn" flag:"d" env:"DATABASE_DSN"`
	Key           *string `json:"key" flag:"k" env:"KEY"`
	CryptoKey     *string `json:"crypto_key" flag:"crypto-key" env:"CRYPTO_KEY"`
	AuditFile     *string `json:"audit_file" flag:"audit-file" env:"AUDIT_FILE"`
	AuditURL      *string `json:"audit_url" flag:"audit-url" env:"AUDIT_URL"`
	TrustedSubnet *string `json:"trusted_subnet" flag:"t" env:"TRUSTED_SUBNET"`
}

// GetConfig loads defaults, JSON, CLI and environment, in increasing priority.
// FILE_STORAGE_PATH retains its existing exception: explicit -f wins over env.
func GetConfig() (*Config, error) {
	return loadConfig(flag.NewFlagSet("server", flag.ExitOnError), os.Args[1:], os.LookupEnv)
}

func loadConfig(flags *flag.FlagSet, args []string, lookup func(string) (string, bool)) (*Config, error) {
	cli := configSource{
		Address:       flags.String("a", ":8080", "address and port to run server"),
		GRPCAddress:   flags.String("grpc-address", "", "gRPC listen address; empty disables gRPC"),
		GRPCCertFile:  flags.String("grpc-cert-file", "", "path to gRPC TLS certificate file"),
		GRPCKeyFile:   flags.String("grpc-key-file", "", "path to gRPC TLS private key file"),
		LogLevel:      flags.String("l", "info", "log level"),
		StoreInterval: flags.String("i", "300", "store interval (seconds or Go duration)"),
		FilePath:      flags.String("f", "", "path to metrics file"),
		Restore:       flags.Bool("r", false, "restore metrics from file"),
		DatabaseDSN:   flags.String("d", "", "database DSN"),
		Key:           flags.String("k", "", "key for signing data"),
		CryptoKey:     flags.String("crypto-key", "", "path to private key file for decryption"),
		AuditFile:     flags.String("audit-file", "", "path to audit log file"),
		AuditURL:      flags.String("audit-url", "", "URL to send audit logs"),
		TrustedSubnet: flags.String("t", "", "trusted agent subnet in CIDR notation"),
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
		TrustedSubnet: config.Pointer(""), GRPCAddress: config.Pointer(""), GRPCCertFile: config.Pointer(""), GRPCKeyFile: config.Pointer(""),
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
	if *merged.TrustedSubnet != "" {
		if _, _, err := net.ParseCIDR(*merged.TrustedSubnet); err != nil {
			return nil, fmt.Errorf("trusted subnet: %w", err)
		}
	}
	return &Config{
		Address: *merged.Address, GRPCAddress: *merged.GRPCAddress, LogLevel: *merged.LogLevel, StoreInterval: int(interval / time.Second),
		GRPCCertFile: *merged.GRPCCertFile, GRPCKeyFile: *merged.GRPCKeyFile,
		FilePath: *merged.FilePath, Restore: *merged.Restore, DatabaseDSN: *merged.DatabaseDSN,
		Key: *merged.Key, CryptoKey: *merged.CryptoKey, AuditFile: *merged.AuditFile, AuditURL: *merged.AuditURL,
		TrustedSubnet: *merged.TrustedSubnet,
	}, nil
}
