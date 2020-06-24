package scdt

import "time"

type Config struct {
	HearBeatCheck bool
	Timeout       time.Duration
}

type ConfigFunc func(c *Config)

func defaultConfig() *Config {
	return &Config{
		Timeout: 0,
	}
}
