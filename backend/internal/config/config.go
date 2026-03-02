package config

import (
	"context"
	"fmt"
	"os"
	"time"
)

const defaultShutdownTimeout = 10 * time.Second

type Config struct {
	Host         string
	Port         string
	BuildVersion string
	BuildCommit  string
}

func Load() Config {
	return Config{
		Host:         getenv("APP_HOST", "0.0.0.0"),
		Port:         getenv("APP_PORT", "8080"),
		BuildVersion: getenv("APP_BUILD_VERSION", "dev"),
		BuildCommit:  getenv("APP_BUILD_COMMIT", "local"),
	}
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

func ShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultShutdownTimeout)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
