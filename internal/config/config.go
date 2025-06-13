package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	API   APIConfig   `mapstructure:"api"`
	Mount MountConfig `mapstructure:"mount"`
	Cache CacheConfig `mapstructure:"cache"`
}

type APIConfig struct {
	BaseURL      string        `mapstructure:"base_url"`
	ClientID     string        `mapstructure:"client_id"`
	ClientSecret string        `mapstructure:"client_secret"`
	DirectoryID  string        `mapstructure:"directory_id"`
	Timeout      time.Duration `mapstructure:"timeout"`
	RetryCount   int           `mapstructure:"retry_count"`
}

type MountConfig struct {
	ReadOnly   bool   `mapstructure:"readonly"`
	AllowOther bool   `mapstructure:"allow_other"`
	UID        uint32 `mapstructure:"uid"`
	GID        uint32 `mapstructure:"gid"`
	Umask      uint32 `mapstructure:"umask"`
}

type CacheConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Directory string        `mapstructure:"directory"`
	TTL       time.Duration `mapstructure:"ttl"`
	MaxSize   int64         `mapstructure:"max_size"`
}

func Load() (*Config, error) {
	var cfg Config

	// Set defaults
	viper.SetDefault("api.timeout", "30s")
	viper.SetDefault("api.retry_count", 3)
	viper.SetDefault("mount.umask", 0022)
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.ttl", "5m")
	viper.SetDefault("cache.max_size", 1<<30) // 1GB

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if cfg.API.BaseURL == "" {
		return nil, fmt.Errorf("api.base_url is required")
	}
	if cfg.API.ClientID == "" {
		return nil, fmt.Errorf("api.client_id is required")
	}
	if cfg.API.ClientSecret == "" {
		return nil, fmt.Errorf("api.client_secret is required")
	}
	if cfg.API.DirectoryID == "" {
		return nil, fmt.Errorf("api.directory_id is required")
	}

	return &cfg, nil
}