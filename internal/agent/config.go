package agent

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerAddress  string
	Key            string
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      int
}

func NewConfig() *Config {
	var (
		serverAddress  string
		reportInterval int
		pollInterval   int
		key            string
		rateLimit      int
	)

	flag.StringVar(&serverAddress, "a", "localhost:8080", "address of the server")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.StringVar(&key, "k", "", "key for signing data")
	flag.IntVar(&rateLimit, "l", 2, "rate limit (max concurrent requests)")
	flag.Parse()

	if envAddr, ok := os.LookupEnv("ADDRESS"); ok && envAddr != "" {
		serverAddress = envAddr
	}
	if envReportInterval, ok := os.LookupEnv("REPORT_INTERVAL"); ok && envReportInterval != "" {
		if interval, err := strconv.Atoi(envReportInterval); err == nil {
			reportInterval = interval
		}
	}
	if envPollInterval, ok := os.LookupEnv("POLL_INTERVAL"); ok && envPollInterval != "" {
		if interval, err := strconv.Atoi(envPollInterval); err == nil {
			pollInterval = interval
		}
	}
	if envKey, ok := os.LookupEnv("KEY"); ok && envKey != "" {
		key = envKey
	}
	if envRateLimit, ok := os.LookupEnv("RATE_LIMIT"); ok && envRateLimit != "" {
		if limit, err := strconv.Atoi(envRateLimit); err == nil {
			rateLimit = limit
		}
	}

	if rateLimit <= 0 {
		rateLimit = 1
	}

	if serverAddress != "" && !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	return &Config{
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		ServerAddress:  serverAddress,
		Key:            key,
		RateLimit:      rateLimit,
	}
}
