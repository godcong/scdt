package scdt

import "time"
import "github.com/google/uuid"

// CustomIDerFunc ...
type CustomIDerFunc func() string

// Config ...
type Config struct {
	CustomIDer CustomIDerFunc
	Timeout    time.Duration
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
