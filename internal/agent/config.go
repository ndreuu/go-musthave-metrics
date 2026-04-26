package agent

import "time"

type Config struct {
	PollInterval   time.Duration
	ReportInterval time.Duration
	ServerAddress  string
}

func NewConfig() *Config {
	return &Config{
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		ServerAddress:  "http://localhost:8080",
	}
}
