# Copilot Proxy - Architecture Overview

## Executive Summary

Copilot Proxy is a lightweight, high-performance HTTP proxy server that acts as a bridge between local LLM tools expecting Ollama/OpenAI APIs and Z.AI's GLM Coding PaaS. The application is designed as a single binary with minimal dependencies, focusing on transparency and performance while maintaining compatibility with existing tooling.

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Client Applications                         │
│  (IDEs, CLI tools, WebUIs expecting Ollama/OpenAI APIs)      │
└─────────────────────┬───────────────────────────────────────────┘
                      │ HTTP Requests
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                 Copilot Proxy Server                           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   HTTP Router   │  │   Request       │  │   Response      │ │
│  │   (Gin)         │  │   Interceptor   │  │   Streamer     │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────┬───────────────────────────────────────────┘
                      │ HTTP Requests (with modifications)
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Z.AI GLM Coding PaaS                          │
│              (Cloud-based LLM Service)                         │
└─────────────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. Application Entry Point (`main.go`)
- Minimal entry point that delegates to Cobra CLI
- Auto-loads environment variables using `godotenv/autoload`
- Provides clean separation between bootstrap and application logic

#### 2. CLI Layer (`cmd/`)
- **Cobra-based command structure** with root command and subcommands
- **Serve Command** (`serve.go`): Main server lifecycle management
  - Configuration loading and validation
  - Server initialization with graceful shutdown
  - Signal handling for SIGINT/SIGTERM
- **Config Command** (`config.go`): Configuration management
  - Get/set configuration values
  - Persistent storage to `~/.copilot-proxy/config.json`
  - API key masking for security

#### 3. Configuration System (`internal/config/`)
- **Hierarchical configuration** with precedence:
  1. Environment variables (highest priority)
  2. Configuration file
  3. Default values (lowest priority)
- **Multi-source API key support**:
  - `ZAI_API_KEY`, `ZAI_CODING_API_KEY`, `GLM_API_KEY`
- **Viper-based** configuration management
- **Persistent configuration** in JSON format

#### 4. HTTP Server (`internal/server/`)
- **Gin-based HTTP router** with middleware support
- **CORS middleware** with configurable origins, methods, and headers
- **Optimized HTTP client** with connection pooling:
  - `MaxIdleConnsPerHost: 50` (vs default 2)
  - `IdleConnTimeout: 90s`
  - `Timeout: 120s` for streaming requests
- **Graceful shutdown** with 30-second timeout
- **Debug mode** with file-based logging to `$TMPDIR/copilot-proxy.log`
- **Context-aware request handling** with proper cancellation propagation

#### 5. Request Handlers (`internal/server/handlers.go`)
- **Model Discovery Endpoints**:
  - `/api/tags`, `/api/list`: Static model catalog
  - `/api/version`: API version information (currently 0.6.4)
  - `/api/ps`: Empty running models list
  - `/api/show`: Model metadata with context lengths (200k for GLM-4.6, 128k for GLM-4.5)
- **Chat Completion Proxy**:
  - `/v1/chat/completions`, `/api/chat`: Request forwarding with model validation
  - **Automatic "thinking" injection** for enhanced reasoning
  - **Context-aware streaming** with 32KB buffer and SSE flushing
  - **Client disconnection detection** with proper context cancellation handling
- **Health Check**: `/healthz` endpoint
- **Error Handling**: Standardized error responses with context-aware cancellation detection (HTTP 499 for client disconnections)

#### 6. Model Catalog (`internal/models/`)
- **Static model definitions** for GLM-4.6, GLM-4.5, GLM-4.5-Air
- **Capability advertising**: Tools, Vision, extended context
- **Context length metadata**: 200k for GLM-4.6, 128k for GLM-4.5

## Data Flow

### Request Processing Flow

```
1. Client Request → CORS Middleware → HTTP Router
2. Router → Appropriate Handler
3. Handler → Model Validation (for chat completions)
4. Handler → Request Modification (thinking injection)
5. Handler → Context-Aware Upstream Request
6. Upstream Response → Context-Aware Response Streaming
7. Response → Client (with proper disconnection handling)
```

### Configuration Loading Flow

```
1. Application Start → Default Config
2. Config File Read → Merge (if exists)
3. Environment Variables → Merge (if set)
4. Final Config → Application Components
```

### Error Handling Flow

```
1. Error Detection → Context Cancellation Check
2. Standardized Error Response → HTTP Status Code Mapping
3. Client Disconnection → HTTP 499 Response
4. Upstream Errors → Proper Error Propagation
```

## Key Design Patterns

### 1. Proxy Pattern
- **Transparent forwarding** of requests to Z.AI API
- **Request modification** for capability enhancement
- **Response streaming** without buffering delays

### 2. Configuration Hierarchy
- **Precedence-based** configuration system
- **Multiple sources** with clear priority rules
- **Environment variable aliases** for flexibility

### 3. Context-Aware Request Handling
- **Context propagation** throughout the request lifecycle
- **Client disconnection detection** with HTTP 499 responses
- **Graceful cancellation** of upstream requests on client disconnect
- **Non-blocking I/O** with proper context checking

### 4. Graceful Degradation
- **Fail-safe operation** when config file is missing
- **Fallback to defaults** for all configuration options
- **Error handling** without service interruption

### 4. Single Responsibility
- **Clear separation** between CLI, server, and configuration
- **Focused handlers** for specific endpoint types
- **Modular design** for maintainability

## Technology Stack

### Core Dependencies
- **Gin**: HTTP web framework (routing, middleware, CORS)
- **Cobra**: CLI framework for command structure
- **Viper**: Configuration management with multiple sources
- **godotenv**: Environment variable loading from .env files
- **gin-contrib/cors**: CORS middleware for cross-origin requests

### Go Version
- **Go 1.25.5** with modern language features
- **Standard library** for HTTP client optimization
- **Context-based** request lifecycle management

## Security Considerations

### Request Validation
- **Multiple environment variable names** for flexibility
- **Masked display** in configuration commands
- **No logging** of sensitive information
- **Bearer token** authentication for upstream requests
- **Model validation** against static catalog before forwarding
- **Context-aware error handling** with proper HTTP status codes

## Performance Optimizations

### HTTP Client Optimization
- **Connection pooling** with high MaxIdleConnsPerHost
- **Keep-alive connections** for reduced latency
- **Appropriate timeouts** for streaming scenarios

### Streaming Strategy
- **32KB buffer** for memory efficiency
- **Explicit flushing** for real-time SSE delivery
- **Non-blocking I/O** for concurrent request handling
- **Context-aware streaming** with client disconnection detection
- **Proper resource cleanup** on cancellation

### Memory Management
- **Request body buffering** only when necessary
- **Streaming responses** without full buffering
- **Efficient JSON handling** with minimal allocations

## Deployment Architecture

### Single Binary Deployment
- **Self-contained executable** with no external dependencies
- **Configuration file** in user home directory
- **Environment variable** support for containerized deployments

### Operational Considerations
- **Graceful shutdown** for zero-downtime deployments
- **Health check endpoint** for load balancer integration
- **Configurable binding** addresses and ports
- **Debug mode** for troubleshooting

## Extensibility Points

### Model Catalog Extension
- **Static model definitions** easily extensible
- **Capability metadata** for new model features
- **Context length** configuration per model

### Handler Extension
- **Modular handler structure** for new endpoints
- **Middleware support** for cross-cutting concerns (CORS, logging)
- **Request/response transformation** hooks
- **Context-aware error handling** with standardized responses
- **Model validation framework** for extensibility

### Configuration Extension
- **Viper-based** system supports new configuration keys
- **Environment variable binding** for container deployments
- **Validation framework** for configuration integrity

## Monitoring and Observability

### Logging Strategy
- **Debug mode** with file-based logging to `$TMPDIR/copilot-proxy.log`
- **Structured logging** through Gin framework and slog
- **Request/response logging** in debug mode
- **Error logging** with context information
- **Client disconnection logging** for debugging

### Health Monitoring
- **Health check endpoint** for service monitoring
- **Graceful shutdown** signals for lifecycle management
- **Error response standardization** for client debugging

## Conclusion

Copilot Proxy demonstrates a well-architected Go application that balances simplicity with functionality. The clean separation of concerns, optimized HTTP handling, thoughtful configuration management, and context-aware request processing make it a robust bridge between local development tools and cloud-based AI services. The architecture supports both development flexibility and production reliability through its modular design, performance optimizations, and comprehensive error handling with proper client disconnection detection.