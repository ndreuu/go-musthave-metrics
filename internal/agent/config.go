package agent

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	PollInterval   time.Duration
	ReportInterval time.Duration
	ServerAddress  string
}

func NewConfig() *Config {
	var (
		serverAddress  string
		reportInterval int
		pollInterval   int
	)

	flag.StringVar(&serverAddress, "a", "localhost:8080", "address of the server")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.Parse()

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		serverAddress = envAddr
	}
	if envReportInterval := os.Getenv("REPORT_INTERVAL"); envReportInterval != "" {
		if interval, err := strconv.Atoi(envReportInterval); err == nil {
			reportInterval = interval
		}
	}
	if envPollInterval := os.Getenv("POLL_INTERVAL"); envPollInterval != "" {
		if interval, err := strconv.Atoi(envPollInterval); err == nil {
			pollInterval = interval
		}
	}

	if serverAddress != "" && !strings.HasPrefix(serverAddress, "http://") && !strings.HasPrefix(serverAddress, "https://") {
		serverAddress = "http://" + serverAddress
	}

	return &Config{
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		ServerAddress:  serverAddress,
	}
}
