package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	ListenAddr string `mapstructure:"listen_addr"`

	// Immich connection
	ImmichURL    string `mapstructure:"immich_url"`
	ImmichAPIKey string `mapstructure:"immich_api_key"`

	// Authentication
	AuthMode string       `mapstructure:"auth_mode"` // "none", "api_key", "oauth", "both"
	APIKeys  []string     `mapstructure:"api_keys"`
	OAuth    *OAuthConfig `mapstructure:"oauth"`

	// Cache settings
	CacheTTL     time.Duration `mapstructure:"cache_ttl"`
	CacheMaxSize int           `mapstructure:"cache_max_size"`

	// Rate limiting
	RateLimitPerSecond int `mapstructure:"rate_limit_per_second"`
	RateLimitBurst     int `mapstructure:"rate_limit_burst"`

	// Timeouts
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	ImmichTimeout  time.Duration `mapstructure:"immich_timeout"`

	// Logging
	LogLevel string `mapstructure:"log_level"`
	LogJSON  bool   `mapstructure:"log_json"`

	// Metrics
	EnableMetrics bool   `mapstructure:"enable_metrics"`
	MetricsPort   string `mapstructure:"metrics_port"`
}

// OAuthConfig holds OAuth configuration
type OAuthConfig struct {
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	RedirectURL  string   `mapstructure:"redirect_url"`
	AuthURL      string   `mapstructure:"auth_url"`
	TokenURL     string   `mapstructure:"token_url"`
	Scopes       []string `mapstructure:"scopes"`
}

// Load loads configuration from file and environment
func Load(configFile string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file
	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			// Config file is optional
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to read config: %w", err)
			}
		}
	}

	// Read environment variables
	v.SetEnvPrefix("MCP")
	v.AutomaticEnv()

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applyDerivedDefaults(&cfg, v)

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("listen_addr", ":8080")

	// Auth defaults
	v.SetDefault("auth_mode", "none")
	v.SetDefault("api_keys", []string{})

	// Cache defaults
	v.SetDefault("cache_ttl", 5*time.Minute)
	v.SetDefault("cache_max_size", 1000)

	// Rate limiting defaults
	v.SetDefault("rate_limit_per_second", 100)
	v.SetDefault("rate_limit_burst", 200)

	// Timeout defaults
	v.SetDefault("request_timeout", 30*time.Second)
	v.SetDefault("immich_timeout", 30*time.Second)

	// Logging defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("log_json", false)

	// Metrics defaults
	v.SetDefault("enable_metrics", false)
	v.SetDefault("metrics_port", ":9090")
}

func applyDerivedDefaults(cfg *Config, v *viper.Viper) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = v.GetString("listen_addr")
		if cfg.ListenAddr == "" {
			cfg.ListenAddr = ":8080"
		}
	}

	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = v.GetDuration("cache_ttl")
		if cfg.CacheTTL <= 0 {
			cfg.CacheTTL = 5 * time.Minute
		}
	}

	if cfg.CacheMaxSize <= 0 {
		cfg.CacheMaxSize = v.GetInt("cache_max_size")
		if cfg.CacheMaxSize <= 0 {
			cfg.CacheMaxSize = 1000
		}
	}

	if cfg.RateLimitPerSecond <= 0 {
		cfg.RateLimitPerSecond = v.GetInt("rate_limit_per_second")
		if cfg.RateLimitPerSecond <= 0 {
			cfg.RateLimitPerSecond = 100
		}
	}

	if cfg.RateLimitBurst <= 0 {
		cfg.RateLimitBurst = v.GetInt("rate_limit_burst")
		if cfg.RateLimitBurst <= 0 {
			cfg.RateLimitBurst = 200
		}
	}

	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = v.GetDuration("request_timeout")
		if cfg.RequestTimeout <= 0 {
			cfg.RequestTimeout = 30 * time.Second
		}
	}

	if cfg.ImmichTimeout <= 0 {
		cfg.ImmichTimeout = v.GetDuration("immich_timeout")
		if cfg.ImmichTimeout <= 0 {
			cfg.ImmichTimeout = 30 * time.Second
		}
	}

	if cfg.MetricsPort == "" {
		cfg.MetricsPort = v.GetString("metrics_port")
		if cfg.MetricsPort == "" {
			cfg.MetricsPort = ":9090"
		}
	}

	// Ensure auth mode is set even if empty string was provided
	if cfg.AuthMode == "" {
		cfg.AuthMode = v.GetString("auth_mode")
		if cfg.AuthMode == "" {
			cfg.AuthMode = "none"
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ImmichURL == "" {
		return fmt.Errorf("immich_url is required")
	}

	if c.ImmichAPIKey == "" {
		return fmt.Errorf("immich_api_key is required")
	}

	// Validate auth mode
	validAuthModes := map[string]bool{
		"none":    true,
		"api_key": true,
		"oauth":   true,
		"both":    true,
	}
	if !validAuthModes[c.AuthMode] {
		return fmt.Errorf("invalid auth_mode: %s", c.AuthMode)
	}

	// If auth mode requires API keys, ensure they exist
	if (c.AuthMode == "api_key" || c.AuthMode == "both") && len(c.APIKeys) == 0 {
		return fmt.Errorf("api_keys required when auth_mode is %s", c.AuthMode)
	}

	// If auth mode requires OAuth, ensure config exists
	if (c.AuthMode == "oauth" || c.AuthMode == "both") && c.OAuth == nil {
		return fmt.Errorf("oauth configuration required when auth_mode is %s", c.AuthMode)
	}

	return nil
}
