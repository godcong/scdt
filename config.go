package scdt

import "time"
import "github.com/google/uuid"

type CustomIDerFunc func() string

type Config struct {
	HearBeatCheck bool
	CustomIDer    CustomIDerFunc
	Timeout       time.Duration
}

type ConfigFunc func(c *Config)

func defaultConfig() *Config {
	return &Config{
		Timeout: 0,
	}
}

// UUID ...
func UUID() string {
	return uuid.Must(uuid.NewUUID()).String()
}
