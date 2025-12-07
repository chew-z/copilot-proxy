package cmd

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chew-z/copilot-proxy/internal/config"
	"github.com/chew-z/copilot-proxy/internal/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the proxy server",
	Long: `Start the proxy server that listens for incoming requests
and forwards them to Z.AI Coding PaaS.`,
	Run: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Add flags for serve command
	serveCmd.Flags().StringP("host", "H", "127.0.0.1", "Host to bind the server to")
	serveCmd.Flags().IntP("port", "p", 11434, "Port to listen on")
	serveCmd.Flags().BoolP("debug", "d", false, "Enable debug mode (verbose logging)")
	serveCmd.Flags().BoolP("verbose", "v", false, "Enable terminal output (default: quiet, logs to file only)")
}

func runServe(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if API key is configured
	if cfg.APIKey == "" {
		log.Fatal("API key is not configured. Please run 'copilot-proxy config set api_key YOUR_API_KEY' or set ZAI_API_KEY environment variable.")
	}

	// Get host and port from flags (highest precedence)
	host, err := cmd.Flags().GetString("host")
	if err != nil {
		log.Fatalf("Failed to get host flag: %v", err)
	}
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		log.Fatalf("Failed to get port flag: %v", err)
	}

	// Use config values only if flags weren't provided (use default flag values to check)
	defaultHost := "127.0.0.1"
	defaultPort := 11434

	// If host flag is still at default value, check config
	if host == defaultHost && cfg.Host != "" {
		host = cfg.Host
	}

	// If port flag is still at default value, check config
	if port == defaultPort && cfg.Port != 0 {
		port = cfg.Port
	}

	// Get debug flag (CLI flag overrides config)
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		log.Fatalf("Failed to get debug flag: %v", err)
	}
	if debug {
		cfg.Debug = true
	}

	// Get verbose flag (CLI flag overrides config)
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		log.Fatalf("Failed to get verbose flag: %v", err)
	}
	if verbose {
		cfg.Verbose = true
	}

	// Create and start server
	srv := server.NewServer(cfg, host, port)

	// Start server in a goroutine
	go func() {
		if cfg.Verbose {
			log.Printf("Starting server on %s:%d", host, port)
			log.Printf("Base URL: %s", cfg.BaseURL)
		}
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline for graceful shutdown
	ctx, cancel := server.CreateShutdownContext(30 * time.Second)
	defer cancel()

	// Gracefully shutdown the server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
