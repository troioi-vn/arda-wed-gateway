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
	Host               string
	Port               string
	BuildVersion       string
	BuildCommit        string
	QueueSendInterval  time.Duration
	QueueMaxDepth      int
	SQLitePath         string
	SuggestDebounce    time.Duration
	SuggestRecentLines int
	OpenRouterBaseURL  string
	OpenRouterModel    string
	OpenRouterAPIKey   string
	OpenRouterTimeout  time.Duration
}

func Load() Config {
	return Config{
		Host:               getenv("APP_HOST", "0.0.0.0"),
		Port:               getenv("APP_PORT", "8080"),
		BuildVersion:       getenv("APP_BUILD_VERSION", "dev"),
		BuildCommit:        getenv("APP_BUILD_COMMIT", "local"),
		QueueSendInterval:  getenvDuration("APP_QUEUE_SEND_INTERVAL", 500*time.Millisecond),
		QueueMaxDepth:      getenvInt("APP_QUEUE_MAX_DEPTH", 20),
		SQLitePath:         getenv("APP_SQLITE_PATH", "tmp/gateway.sqlite"),
		SuggestDebounce:    getenvDuration("APP_SUGGEST_DEBOUNCE", 700*time.Millisecond),
		SuggestRecentLines: getenvInt("APP_SUGGEST_RECENT_LINES", 80),
		OpenRouterBaseURL:  getenv("APP_OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		OpenRouterModel:    getenv("APP_OPENROUTER_MODEL", "openai/gpt-4o-mini"),
		OpenRouterAPIKey:   os.Getenv("APP_OPENROUTER_API_KEY"),
		OpenRouterTimeout:  getenvDuration("APP_OPENROUTER_TIMEOUT", 120*time.Second),
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
