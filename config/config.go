package config

import (
	"errors"
	"os"
	"strconv"
)

// Config struct holds all of the runtime configuration for the application
type Config struct {
	APIBaseURL               string
	OAuthToken               string
	ListenPort               string
	CacheInvalidationSeconds int
	MetricsPath              string
}

func getEnv(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		val = defaultValue
	}
	return val
}

// NewConfig creates a new config
func NewConfig() (*Config, error) {
	token := os.Getenv("OAUTH_TOKEN")
	if len(token) == 0 {
		return nil, errors.New("OAUTH_TOKEN not set. Be sure to set the Remo oauth token to the OAUTH_TOKEN environment variable")
	}

	metricsPath := getEnv("METRICS_PATH", "/metrics")
	baseURL := getEnv("API_BASE_URL", "https://api.nature.global")
	listenPort := getEnv("PORT", "9352")
	cacheInvalidationSeconds, err := strconv.Atoi(getEnv("CACHE_INVALIDATION_SECONDS", "60"))
	if err != nil {
		return nil, err
	}

	config := &Config{
		MetricsPath:              metricsPath,
		APIBaseURL:               baseURL,
		OAuthToken:               token,
		ListenPort:               listenPort,
		CacheInvalidationSeconds: cacheInvalidationSeconds,
	}

	return config, nil
}
