# Simplified Implementation Plan

## Core Goal
A **fast, single-binary proxy** that works. Period.

---

## Project Structure

```
copilot-proxy/
├── main.go                    # Entry + godotenv autoload
├── cmd/
│   ├── root.go               # Cobra root
│   ├── serve.go              # Server command
│   └── config.go             # Config get/set commands
├── internal/
│   ├── config/
│   │   └── config.go         # Viper + simple struct
│   ├── server/
│   │   ├── server.go         # HTTP server + routes
│   │   └── handlers.go       # Catalog + proxy handler
│   └── models/
│       └── catalog.go        # Static model list
├── go.mod
└── README.md
```

---

## Implementation Timeline - 3 Days Max

### Day 1: Config & CLI Foundation

#### 1.1 Setup Go Module
```bash
go mod init github.com/yourusername/copilot-proxy
go get github.com/spf13/cobra
go get github.com/spf13/viper
go get github.com/gin-gonic/gin
go get github.com/joho/godotenv
```

#### 1.2 Configuration System (`internal/config/config.go`)

**Features:**
- Simple struct with 4 fields: APIKey, BaseURL, Host, Port
- Config file at `~/.copilot-proxy/config.json`
- Precedence: ENV vars > config file > defaults
- Support multiple API key env vars: `ZAI_API_KEY`, `ZAI_CODING_API_KEY`, `GLM_API_KEY`

**Key Functions:**
- `Load()` - Load config with precedence
- `Save()` - Save config to JSON file
- `getAPIKey()` - Check multiple env var names

#### 1.3 CLI Commands (`cmd/`)

**Commands:**
- `copilot-proxy serve` - Start server
- `copilot-proxy config set <key> <value>` - Set config value
- `copilot-proxy config get <key>` - Get config value (optional)

**Main Entry Point (`main.go`):**
```go
package main

import (
    _ "github.com/joho/godotenv/autoload"
    "yourproject/cmd"
)

func main() {
    cmd.Execute()
}
```

---

### Day 2: Core Proxy Logic

#### 2.1 HTTP Server Setup (`internal/server/server.go`)

**Features:**
- Single shared `http.Client` with optimized transport
- Gin router in release mode
- Graceful shutdown (30s timeout)
- Signal handling (SIGINT, SIGTERM)

**HTTP Client Configuration:**
```go
httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConnsPerHost: 50,  // vs default 2
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 120 * time.Second,
}
```

#### 2.2 Endpoints (`internal/server/handlers.go`)

**Static Endpoints:**
- `GET /api/tags` → Return model catalog
- `GET /api/list` → Return model catalog (alias)
- `POST /api/show` → Return dummy model metadata

**Proxy Endpoint:**
- `POST /v1/chat/completions` → Forward to Z.AI with auth injection

**Proxy Handler Logic:**
1. Build upstream request to `{baseURL}/api/coding/paas/v4/chat/completions`
2. Use request context for cancellation support
3. Inject `Authorization: Bearer <api_key>` header
4. Copy `Content-Type` header
5. Execute request with shared client
6. Stream response body to client
7. Flush after each chunk for SSE support

#### 2.3 Model Catalog (`internal/models/catalog.go`)

**Static Response:**
```go
var Catalog = map[string]interface{}{
    "models": []map[string]interface{}{
        {
            "name": "GLM-4-Plus",
            "model": "GLM-4-Plus",
            "modified_at": "2024-01-01T00:00:00Z",
            "size": 0,
        },
        {
            "name": "GLM-4-Air",
            "model": "GLM-4-Air",
            "modified_at": "2024-01-01T00:00:00Z",
            "size": 0,
        },
        // Add other supported models
    },
}
```

---

### Day 3: CLI Polish & Build

#### 3.1 Complete CLI Commands

**Config Command (`cmd/config.go`):**
- Subcommands: `set`, `get`
- Validate keys (api_key, base_url only)
- Error handling

**Serve Command (`cmd/serve.go`):**
- Check API key exists before starting
- Log server address on startup
- Handle errors gracefully

#### 3.2 Build System

**Makefile:**
```makefile
.PHONY: build install test clean

build:
	go build -o bin/copilot-proxy .

install:
	go install .

test:
	go test -v ./...

clean:
	rm -rf bin/
```

#### 3.3 Basic Testing

**Test Coverage:**
- Config load/save
- Handler responses (catalog endpoints)
- Proxy header injection (integration test with httptest)

---

## Technical Details

### HTTP Client Best Practices

**Why these settings matter:**
- `MaxIdleConnsPerHost: 50` - Default is 2, way too low for concurrent requests
- `IdleConnTimeout: 90s` - Keep connections alive for reuse
- `Timeout: 120s` - Overall timeout for long-running streaming requests

### Streaming Strategy

**Efficient streaming:**
```go
buf := make([]byte, 32*1024)  // 32KB buffer
for {
    n, err := resp.Body.Read(buf)
    if n > 0 {
        c.Writer.Write(buf[:n])
        c.Writer.Flush()  // Critical for SSE
    }
    if err == io.EOF {
        break
    }
    if err != nil {
        return
    }
}
```

**Why this works:**
- Buffered reading reduces syscalls
- Explicit flush after each write for SSE
- Context cancellation handled by `NewRequestWithContext`

### Graceful Shutdown

**Shutdown sequence:**
1. Receive OS signal (SIGINT/SIGTERM)
2. Call `srv.Shutdown(ctx)` - stops accepting new connections
3. Wait up to 30s for in-flight requests to complete
4. Force close if timeout exceeded

---

## What You Get

✅ **Fast single binary** - No dependencies for end users  
✅ **Drop-in Ollama replacement** - Default port 11434  
✅ **SSE streaming support** - Real-time responses  
✅ **Config precedence** - ENV > config file > defaults  
✅ **Graceful shutdown** - No dropped requests  
✅ **Optimized HTTP client** - Connection pooling, keep-alive  
✅ **Simple codebase** - ~400 lines total  

---

## What You Don't Get (Intentionally Excluded)

❌ Keyring integration - Config file + env vars are sufficient  
❌ Prometheus metrics - Keep it simple  
❌ Structured logging - `log` package is enough  
❌ OpenTelemetry - Not needed  
❌ Complex middleware - Gin's recovery is sufficient  
❌ Health checks - Not required for local proxy  

---

## Dependencies

**Total: 4 dependencies**

```go
require (
    github.com/spf13/cobra v1.8.0      // CLI framework
    github.com/spf13/viper v1.18.0     // Config management
    github.com/gin-gonic/gin v1.10.0   // HTTP server/routing
    github.com/joho/godotenv v1.5.1    // .env autoload
)
```

---

## Quick Start After Implementation

```bash
# Build
make build

# Setup
./bin/copilot-proxy config set api_key YOUR_API_KEY_HERE

# Optional: Set custom base URL
./bin/copilot-proxy config set base_url https://api.z.ai

# Run
./bin/copilot-proxy serve

# Configure your IDE/tool to use:
# - API: http://127.0.0.1:11434
# - Model: GLM-4-Plus (or any supported model)
```

---

## File Checklist

- [ ] `main.go` - Entry point with godotenv autoload
- [ ] `cmd/root.go` - Cobra root command setup
- [ ] `cmd/serve.go` - Serve command
- [ ] `cmd/config.go` - Config management commands
- [ ] `internal/config/config.go` - Config struct and load/save
- [ ] `internal/server/server.go` - HTTP server and graceful shutdown
- [ ] `internal/server/handlers.go` - Route handlers and proxy logic
- [ ] `internal/models/catalog.go` - Static model catalog
- [ ] `go.mod` - Dependencies
- [ ] `Makefile` - Build automation
- [ ] `README.md` - Usage documentation
- [ ] `.gitignore` - Exclude bin/, .env

---

## Success Criteria

**The implementation is complete when:**

1. `copilot-proxy serve` starts a server on port 11434
2. `curl http://localhost:11434/api/tags` returns model catalog
3. Chat completion requests are proxied to Z.AI with auth
4. SSE streaming works without buffering
5. Ctrl+C performs graceful shutdown
6. Config can be set via CLI commands
7. ENV vars override config file
8. Binary is under 20MB

---

## Notes

- Keep it simple - resist feature creep
- Performance first - profile if needed
- Single binary - no external dependencies
- Good defaults - minimal configuration required
- Fast startup - server ready in <100ms
