package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "copilot-proxy",
	Short: "A fast proxy server that bridges local LLM tools to Z.AI GLM Coding PaaS",
	Long: `Copilot Proxy is a single-binary proxy server that mimics Ollama/OpenAI endpoints
locally and forwards requests to Z.AI Coding PaaS with minimal overhead.

It acts as a drop-in replacement for Ollama, listening on port 11434 by default.`,
	Version: "0.5.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Add global flags here if needed
}
