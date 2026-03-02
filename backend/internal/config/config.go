package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

const defaultShutdownTimeout = 10 * time.Second

type Config struct {
	Host              string
	Port              string
	BuildVersion      string
	BuildCommit       string
	QueueSendInterval time.Duration
	QueueMaxDepth     int
}

func Load() Config {
	return Config{
		Host:              getenv("APP_HOST", "0.0.0.0"),
		Port:              getenv("APP_PORT", "8080"),
		BuildVersion:      getenv("APP_BUILD_VERSION", "dev"),
		BuildCommit:       getenv("APP_BUILD_COMMIT", "local"),
		QueueSendInterval: getenvDuration("APP_QUEUE_SEND_INTERVAL", 500*time.Millisecond),
		QueueMaxDepth:     getenvInt("APP_QUEUE_MAX_DEPTH", 20),
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

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
