package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kenfdev/remo-exporter/log"
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

func getOAuthToken(r Reader) (string, error) {
	token := ""
	tokenPath := getEnv("OAUTH_TOKEN_FILE", "")
	if tokenPath == "" {
		log.Info("No oauth token file found. Falling back to environment variable")
		token = getEnv("OAUTH_TOKEN", "")
	} else {
		key, err := r.ReadFile(tokenPath)
		if err != nil {
			return "", fmt.Errorf("Unable to load oauth token file at: %s. %s", tokenPath, err.Error())
		}
		token = strings.TrimSpace(string(key))
	}

	if token == "" {
		return "", errors.New("OAUTH_TOKEN not set. Be sure to set the Remo oauth token to a secret or environment variable")
	}

	return token, nil
}

// NewConfig creates a new config
func NewConfig(r Reader) (*Config, error) {
	token, err := getOAuthToken(r)
	if err != nil {
		return nil, err
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
