package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the staking service.
type Config struct {
	// Server
	GRPCPort int `envconfig:"GRPC_PORT" default:"9090"`
	HTTPPort int `envconfig:"HTTP_PORT" default:"8080"`

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

	// Staking Provider
	StakingProvider string `envconfig:"STAKING_PROVIDER" default:"mock"` // "bitgo" or "mock"

	// BitGo
	BitGoBaseURL     string `envconfig:"BITGO_BASE_URL" default:"https://app.bitgo-test.com"`
	BitGoAccessToken string `envconfig:"BITGO_ACCESS_TOKEN"`
	BitGoWalletID    string `envconfig:"BITGO_WALLET_ID"`
	BitGoCoin        string `envconfig:"BITGO_COIN" default:"hteth"`

	// Reward Poller
	RewardPollInterval time.Duration `envconfig:"REWARD_POLL_INTERVAL" default:"5m"`

	// Authentication
	JWTSecret string `envconfig:"JWT_SECRET" default:"dev-secret-do-not-use-in-production"`

	// Logging
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return &cfg, nil
}
