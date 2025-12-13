package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Debug   bool   `mapstructure:"debug"`
	Verbose bool   `mapstructure:"verbose"` // Enable terminal output (default: quiet, logs to file only)
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		APIKey:  "",
		BaseURL: "https://api.z.ai/api/coding/paas/v4",
		Host:    "127.0.0.1",
		Port:    11434,
	}
}

// Load loads configuration with precedence: ENV vars > config file > defaults
func Load() (*Config, error) {
	// Initialize viper
	v := viper.New()

	// Set defaults
	defaultCfg := DefaultConfig()
	v.SetDefault("api_key", defaultCfg.APIKey)
	v.SetDefault("base_url", defaultCfg.BaseURL)
	v.SetDefault("host", defaultCfg.Host)
	v.SetDefault("port", defaultCfg.Port)
	v.SetDefault("debug", defaultCfg.Debug)

	// Set config file name and paths
	v.SetConfigName("config")
	v.SetConfigType("json")

	// Add config paths
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}
	v.AddConfigPath(configDir)

	// Set environment variable prefix and bind them
	v.SetEnvPrefix("ZAI")
	v.AutomaticEnv()

	// Bind specific environment variables (no duplicates!)
	_ = v.BindEnv("api_key", "ZAI_API_KEY")
	_ = v.BindEnv("base_url", "ZAI_BASE_URL")
	_ = v.BindEnv("host", "ZAI_HOST")
	_ = v.BindEnv("port", "ZAI_PORT")
	_ = v.BindEnv("debug", "ZAI_DEBUG")

	// Try to read config file (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, that's ok - we'll use defaults/env vars
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Get API key from environment variables (highest precedence)
	if apiKey := getAPIKeyFromEnv(); apiKey != "" {
		cfg.APIKey = apiKey
	}

	return &cfg, nil
}

// Save saves the configuration to file
func Save(cfg *Config) error {
	// Create config directory if it doesn't exist
	configDir, err := getConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Initialize viper for writing
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("json")
	v.AddConfigPath(configDir)

	// Set values
	v.Set("api_key", cfg.APIKey)
	v.Set("base_url", cfg.BaseURL)
	v.Set("host", cfg.Host)
	v.Set("port", cfg.Port)
	v.Set("debug", cfg.Debug)

	// Write config file
	configPath := filepath.Join(configDir, "config.json")
	if err := v.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigDir returns the configuration directory path (XDG-compliant)
func getConfigDir() (string, error) {
	// Check XDG_CONFIG_HOME first
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "copilot-proxy"), nil
	}
	// Default to ~/.config/copilot-proxy
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "copilot-proxy"), nil
}

// getAPIKeyFromEnv checks multiple environment variable names for API key
func getAPIKeyFromEnv() string {
	// Check multiple environment variable names in order of preference
	envVars := []string{
		"ZAI_API_KEY",
		"ZAI_CODING_API_KEY",
		"GLM_API_KEY",
	}

	for _, envVar := range envVars {
		if value := os.Getenv(envVar); value != "" {
			return value
		}
	}

	return ""
}
