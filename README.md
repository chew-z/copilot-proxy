# Copilot Proxy

A fast, single-binary proxy server that bridges local LLM tools (expecting Ollama/OpenAI APIs) to Z.AI GLM Coding PaaS.

**Version**: 0.4.0 (CLI) / 0.6.4 (API compatibility)

## Table of Contents

-   [Features](#features)
-   [Quick Start](#quick-start)
-   [Configuration](#configuration)
-   [Running as a Service](#running-as-a-service)
-   [Supported Models](#supported-models)
-   [Capabilities](#capabilities)
-   [API Endpoints](#api-endpoints)
-   [Development](#development)
-   [Technical Details](#technical-details)
-   [License](#license)

## Features

-   **Drop-in Ollama replacement** - Listens on port 11434 by default
-   **Single binary** - No external dependencies, easy to distribute
-   **SSE streaming support** - Real-time responses for chat completions
-   **Optimized HTTP client** - Connection pooling and keep-alive
-   **Graceful shutdown** - No dropped requests on restart
-   **Simple configuration** - Config file + environment variables

## Quick Start

### Build

```bash
make build
```

### Configure

```bash
# Set your API key
./bin/copilot-proxy config set api_key YOUR_API_KEY_HERE

# Optional: Set custom base URL
./bin/copilot-proxy config set base_url https://api.z.ai/api/coding/paas/v4
```

### Run

```bash
# Default: quiet mode (logs to $TMPDIR/copilot-proxy.log)
./bin/copilot-proxy serve

# Verbose: see output in terminal
./bin/copilot-proxy serve --verbose

# Debug: detailed logging
./bin/copilot-proxy serve --debug --verbose
```

### Use with your IDE/tool

Configure your tool to use:

-   **API**: `http://127.0.0.1:11434`
-   **Model**: `GLM-4.6` (or any supported model from the catalog)

## Configuration

### Configuration Precedence

1. **Environment variables** (highest priority)
2. **Config file** (`~/.config/copilot-proxy/config.json`)
3. **Defaults** (lowest priority)

### Environment Variables

-   `ZAI_API_KEY`, `ZAI_CODING_API_KEY`, or `GLM_API_KEY` - Your API key
-   `ZAI_BASE_URL` - Base URL for Z.AI API (default: `https://api.z.ai/api/coding/paas/v4`)
-   `ZAI_HOST` - Host to bind server to (default: `127.0.0.1`)
-   `ZAI_PORT` - Port to listen on (default: `11434`)
-   `ZAI_DEBUG` - Enable debug mode (default: `false`)

### CLI Commands

```bash
# Start the server (quiet by default)
copilot-proxy serve

# Start with terminal output
copilot-proxy serve --verbose

# Start with custom host/port and debug logging
copilot-proxy serve --host 0.0.0.0 --port 8080 --debug --verbose

# Set configuration
copilot-proxy config set api_key YOUR_KEY
copilot-proxy config set base_url https://api.z.ai/api/coding/paas/v4
copilot-proxy config set host 127.0.0.1
copilot-proxy config set port 11434
copilot-proxy config set debug true

# Get configuration
copilot-proxy config get api_key
copilot-proxy config get base_url
```

## Running as a Service

The proxy includes launchd integration for macOS. The install script automatically detects your `$GOBIN` path.

### Quick Start

```bash
make install                              # Install binary to $GOBIN
./scripts/copilot-proxy-ctl.sh install    # Install launchd service
./scripts/copilot-proxy-ctl.sh start      # Start the service
```

### Control Commands

```bash
./scripts/copilot-proxy-ctl.sh install   # Install service (uses your $GOBIN)
./scripts/copilot-proxy-ctl.sh start     # Start the service
./scripts/copilot-proxy-ctl.sh status    # Check status + health
./scripts/copilot-proxy-ctl.sh stop      # Stop the service
./scripts/copilot-proxy-ctl.sh restart   # Restart
./scripts/copilot-proxy-ctl.sh logs      # Tail log file
./scripts/copilot-proxy-ctl.sh uninstall # Remove service
```

### Service Configuration

The service starts automatically at login and restarts on crash. To customize environment variables, edit `~/Library/LaunchAgents/pl.rrj.copilot-proxy.plist` after installation.

## Supported Models

The proxy returns a static catalog of supported Z.AI models:

-   GLM-4.6
-   GLM-4.5
-   GLM-4.5-Air

## Capabilities

The proxy fully supports and advertises the advanced capabilities of Z.AI GLM models:

-   **Extended Context**:
    -   `GLM-4.6`: **200k** token context window.
    -   `GLM-4.5`: **128k** token context window.
-   **Reasoning ("Thinking")**: Automatically enabled (`type: enabled`) for all chat completion requests, unlocking deep reasoning capabilities.
-   **Vision**: All models advertise vision support for multimodal tasks.

## API Endpoints

### Model Discovery

To satisfy Ollama-compatible clients (like Copilot and various WebUIs), the proxy implements the full discovery API:

-   `GET /api/tags` - Returns the complete model catalog with capabilities.
-   `GET /api/list` - Alias for `/api/tags`.
-   `GET /api/version` - Returns the API version (mimics Ollama versioning, currently 0.6.4).
-   `GET /api/ps` - Returns list of running models (empty for this proxy).
-   `POST /api/show` - Returns detailed model metadata, including context length, parameters, and advertised capabilities (Tools, Vision). Accepts both `name` and `model` parameters.

### Chat Completions

-   `POST /v1/chat/completions` - Standard OpenAI-compatible format, proxied to Z.AI Coding PaaS.
-   `POST /api/chat` - Ollama-style chat endpoint (internally aliased to `v1/chat/completions` logic).

### Health Check

-   `GET /healthz` - Simple health check endpoint returning `{"status": "ok"}`.

> **Note**: The proxy automatically intercepts chat requests to inject `thinking: { "type": "enabled" }`, ensuring the model's reasoning capabilities are active.

## Development

### Build

```bash
make build        # Build binary with green tea GC experiment
make install      # Install to $GOPATH/bin
make test         # Run tests
make lint         # Run linter
make format       # Format code
make clean        # Clean build artifacts
make dev          # Build and run in development mode
make all          # Format, lint, test, and build
```

> **Note**: The build uses Go's green tea GC experiment (`GOEXPERIMENT=greenteagc`) for improved performance in production environments.

### Project Structure

```
copilot-proxy/
├── main.go                    # Entry point with godotenv autoload
├── cmd/                       # CLI commands
│   ├── root.go               # Cobra root command
│   ├── serve.go              # Serve command with graceful shutdown
│   └── config.go             # Config management commands
├── internal/
│   ├── config/               # Configuration management
│   │   └── config.go         # Viper-based config with multiple sources
│   ├── server/               # HTTP server
│   │   ├── server.go         # Server setup with optimized client
│   │   └── handlers.go       # Route handlers for all endpoints
│   └── models/               # Data models
│       └── catalog.go        # Static model catalog with capabilities
├── go.mod                     # Go module definition
├── go.sum                     # Go module checksums
├── Makefile                   # Build automation with green tea GC
├── run_format.sh              # Code formatting script
├── run_lint.sh                # Linting script
├── run_test.sh                # Test running script
└── docs/                      # Documentation
    └── architecture_overview.md # Detailed architecture documentation
```

## Technical Details

### HTTP Client Optimization

The proxy uses an optimized HTTP client with:

-   `MaxIdleConnsPerHost: 50` (vs default 2) for concurrent requests
-   `IdleConnTimeout: 90s` for connection reuse
-   `Timeout: 120s` for long-running streaming requests

### Streaming Strategy

Responses are streamed with a 32KB buffer and explicit flushes for SSE support, ensuring real-time delivery without buffering delays.

### Graceful Shutdown

The server handles SIGINT/SIGTERM signals and waits up to 30 seconds for in-flight requests to complete before shutting down.

### Logging

-   **Default (quiet)**: Logs to `$TMPDIR/copilot-proxy.log` only
-   **Verbose mode** (`-v`): Also outputs to terminal
-   **Debug mode** (`-d`): Sets log level to DEBUG for detailed information

## License

MIT
