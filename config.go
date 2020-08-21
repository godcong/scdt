package scdt

import "time"
import "github.com/google/uuid"

// Config ...
type Config struct {
	Timeout time.Duration
}

// ConfigFunc ...
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
