package agent

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go-musthave-metrics/internal/config"
)

const (
	defaultServerAddress  = "localhost:8080"
	defaultPollInterval   = "2"
	defaultReportInterval = "10"
	defaultRateLimit      = 2
)

// generate:reset
type Config struct {
	ServerAddress  string
	Key            string
	CryptoKey      string
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      int
}

type configSource struct {
	Address        *string `json:"address" flag:"a" env:"ADDRESS"`
	ReportInterval *string `json:"report_interval" flag:"r" env:"REPORT_INTERVAL"`
	PollInterval   *string `json:"poll_interval" flag:"p" env:"POLL_INTERVAL"`
	CryptoKey      *string `json:"crypto_key" flag:"crypto-key" env:"CRYPTO_KEY"`
	Key            *string `json:"key" flag:"k" env:"KEY"`
	RateLimit      *int    `json:"rate_limit" flag:"l" env:"RATE_LIMIT"`
}

// GetConfig loads defaults, a JSON file, environment and explicit CLI flags,
// in that order of increasing priority.
func GetConfig() (*Config, error) {
	return loadConfig(flag.NewFlagSet("agent", flag.ExitOnError), os.Args[1:], os.LookupEnv)
}

// NewConfig is retained for callers of the previous API.
// Deprecated: use GetConfig to handle configuration errors without a panic.
func NewConfig() *Config {
	cfg, err := GetConfig()
	if err != nil {
		panic(err)
	}
	return cfg
}

func getConfig(args []string, lookup func(string) (string, bool)) (*Config, error) {
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return loadConfig(flags, args, lookup)
}

func loadConfig(flags *flag.FlagSet, args []string, lookup func(string) (string, bool)) (*Config, error) {
	cli := configSource{
		Address:        flags.String("a", defaultServerAddress, "address of the server"),
		ReportInterval: flags.String("r", defaultReportInterval, "report interval (seconds or Go duration)"),
		PollInterval:   flags.String("p", defaultPollInterval, "poll interval (seconds or Go duration)"),
		Key:            flags.String("k", "", "key for signing data"),
		CryptoKey:      flags.String("crypto-key", "", "path to public key file for encryption"),
		RateLimit:      flags.Int("l", defaultRateLimit, "rate limit (max concurrent requests)"),
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
	env, err := config.Environment(lookup, cli)
	if err != nil {
		return nil, err
	}
	defaults := configSource{
		Address: config.Pointer(defaultServerAddress), ReportInterval: config.Pointer(defaultReportInterval),
		PollInterval: config.Pointer(defaultPollInterval), RateLimit: config.Pointer(defaultRateLimit),
		Key: config.Pointer(""), CryptoKey: config.Pointer(""),
	}
	merged, err := config.Merge(defaults, file, env, cli)
	if err != nil {
		return nil, err
	}
	report, err := config.Duration(*merged.ReportInterval)
	if err != nil {
		return nil, fmt.Errorf("report interval: %w", err)
	}
	if report <= 0 {
		return nil, fmt.Errorf("report interval must be positive: %q", *merged.ReportInterval)
	}
	poll, err := config.Duration(*merged.PollInterval)
	if err != nil {
		return nil, fmt.Errorf("poll interval: %w", err)
	}
	if poll <= 0 {
		return nil, fmt.Errorf("poll interval must be positive: %q", *merged.PollInterval)
	}
	address := *merged.Address
	if address != "" && !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	rate := *merged.RateLimit
	if rate <= 0 {
		rate = 1
	}
	return &Config{
		ServerAddress: address, Key: *merged.Key, CryptoKey: *merged.CryptoKey,
		PollInterval: poll, ReportInterval: report, RateLimit: rate,
	}, nil
}
