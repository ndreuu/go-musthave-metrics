package agent

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultServerAddress  = "localhost:8080"
	defaultPollInterval   = 2
	defaultReportInterval = 10
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

type fileConfig struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
	Key            string `json:"key"`
	RateLimit      *int   `json:"rate_limit"`
}

func NewConfig() *Config {
	var (
		serverAddress   string
		reportIntervalS string
		pollIntervalS   string
		key             string
		cryptoKey       string
		rateLimit       int
		configPath      string
	)

	flag.StringVar(&serverAddress, "a", defaultServerAddress, "address of the server")
	flag.StringVar(&reportIntervalS, "r", "", "report interval (e.g. 10s)")
	flag.StringVar(&pollIntervalS, "p", "", "poll interval (e.g. 2s)")
	flag.StringVar(&key, "k", "", "key for signing data")
	flag.StringVar(&cryptoKey, "crypto-key", "", "path to public key file for encryption")
	flag.IntVar(&rateLimit, "l", defaultRateLimit, "rate limit (max concurrent requests)")
	flag.StringVar(&configPath, "c", "", "path to JSON config file")
	flag.StringVar(&configPath, "config", "", "path to JSON config file")
	flag.Parse()

	setFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	cfgPath := configPath
	if !setFlags["c"] && !setFlags["config"] {
		if env, ok := os.LookupEnv("CONFIG"); ok && env != "" {
			cfgPath = env
		}
	}

	var fc fileConfig
	if cfgPath != "" {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read config file %s: %v\n", cfgPath, err)
		} else if err := json.Unmarshal(data, &fc); err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse config file %s: %v\n", cfgPath, err)
		}
	}

	serverAddress = resolveString(setFlags["a"], serverAddress, "ADDRESS", fc.Address, defaultServerAddress)
	key = resolveString(setFlags["k"], key, "KEY", fc.Key, "")
	cryptoKey = resolveString(setFlags["crypto-key"], cryptoKey, "CRYPTO_KEY", fc.CryptoKey, "")

	reportVal := resolveString(setFlags["r"], reportIntervalS, "REPORT_INTERVAL", fc.ReportInterval, strconv.Itoa(defaultReportInterval))
	pollVal := resolveString(setFlags["p"], pollIntervalS, "POLL_INTERVAL", fc.PollInterval, strconv.Itoa(defaultPollInterval))

	reportInterval := parseInterval(reportVal, defaultReportInterval)
	pollInterval := parseInterval(pollVal, defaultPollInterval)

	rateLimit = resolveInt(setFlags["l"], rateLimit, "RATE_LIMIT", fc.RateLimit, defaultRateLimit)
	if rateLimit <= 0 {
		rateLimit = 1
	}

	if serverAddress != "" && !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	return &Config{
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
		ServerAddress:  serverAddress,
		Key:            key,
		CryptoKey:      cryptoKey,
		RateLimit:      rateLimit,
	}
}

func resolveString(flagSet bool, flagVal, envName, fileVal, def string) string {
	if flagSet {
		return flagVal
	}
	if v, ok := os.LookupEnv(envName); ok && v != "" {
		return v
	}
	if fileVal != "" {
		return fileVal
	}
	return def
}

func resolveInt(flagSet bool, flagVal int, envName string, fileVal *int, def int) int {
	if flagSet {
		return flagVal
	}
	if v, ok := os.LookupEnv(envName); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	if fileVal != nil {
		return *fileVal
	}
	return def
}

func parseInterval(s string, defSeconds int) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	if n, err := strconv.Atoi(s); err == nil {
		return time.Duration(n) * time.Second
	}
	return time.Duration(defSeconds) * time.Second
}