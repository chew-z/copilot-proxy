package cmd

import (
	"fmt"
	"log"

	"github.com/chew-z/copilot-proxy/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Manage configuration settings for the copilot-proxy.`,
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Supported keys:
- api_key: Your Z.AI API key
- base_url: Base URL for Z.AI API (default: https://api.z.ai)
- host: Host to bind server to (default: 127.0.0.1)
- port: Port to listen on (default: 11434)`,
	Args: cobra.ExactArgs(2),
	Run:  runConfigSet,
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Long: `Get a configuration value. Supported keys:
- api_key: Your Z.AI API key (masked)
- base_url: Base URL for Z.AI API
- host: Host to bind server to
- port: Port to listen on`,
	Args: cobra.ExactArgs(1),
	Run:  runConfigGet,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) {
	key := args[0]
	value := args[1]

	// Validate key
	validKeys := map[string]bool{
		"api_key":  true,
		"base_url": true,
		"host":     true,
		"port":     true,
	}

	if !validKeys[key] {
		log.Fatalf("Invalid key: %s. Valid keys are: api_key, base_url, host, port", key)
	}

	// Load existing config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Update the config value
	switch key {
	case "api_key":
		cfg.APIKey = value
	case "base_url":
		cfg.BaseURL = value
	case "host":
		cfg.Host = value
	case "port":
		// Try to parse port as integer
		var portInt int
		if _, err := fmt.Sscanf(value, "%d", &portInt); err != nil {
			log.Fatalf("Invalid port value: %s. Must be an integer.", value)
		}
		cfg.Port = portInt
	}

	// Save the updated config
	if err := config.Save(cfg); err != nil {
		log.Fatalf("Failed to save configuration: %v", err)
	}

	fmt.Printf("Configuration updated: %s = %s\n", key, maskIfAPIKey(key, value))
}

func runConfigGet(cmd *cobra.Command, args []string) {
	key := args[0]

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Get the value
	var value string
	switch key {
	case "api_key":
		if cfg.APIKey != "" {
			value = "********" // Mask API key
		}
	case "base_url":
		value = cfg.BaseURL
	case "host":
		value = cfg.Host
	case "port":
		if cfg.Port != 0 {
			value = fmt.Sprintf("%d", cfg.Port)
		}
	default:
		log.Fatalf("Invalid key: %s. Valid keys are: api_key, base_url, host, port", key)
	}

	if value == "" {
		fmt.Printf("%s is not set\n", key)
	} else {
		fmt.Printf("%s = %s\n", key, value)
	}
}

func maskIfAPIKey(key, value string) string {
	if key == "api_key" && value != "" {
		return "********"
	}
	return value
}
