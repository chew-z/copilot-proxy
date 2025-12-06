# Architecture Analysis & Go Rewrite Plan

## 1. Project Overview

**Project Name:** `copilot-proxy`
**Current Version:** 0.1.2
**Goal:** A middleware proxy server that bridges local development tools (expecting local LLM APIs like Ollama) to the Z.AI GLM Coding PaaS API.

The proxy effectively "spoofs" a local Ollama instance, intercepting requests, injecting authentication for the upstream Z.AI service, and streaming the responses back to the client.

## 2. Current Architecture (Python)

### Tech Stack

-   **Language:** Python 3.10+
-   **Web Framework:** FastAPI
-   **Server:** Uvicorn (ASGI)
-   **HTTP Client:** HTTPX (Async)
-   **CLI:** `argparse`

### Key Components

#### A. Entry Point (`cli.py` & `proxy_server.py`)

-   **CLI Driven:** The application is controlled via command-line arguments.
-   **Commands:**
    -   `serve`: Launches the Uvicorn server (Default port: `11434`, Host: `127.0.0.1`).
    -   `config`: Manages configuration (set/get API keys and Base URLs).
    -   `setup`: Interactive wizard for first-time users.
-   **Compatibility:** The default port `11434` is chosen specifically to be a drop-in replacement for Ollama.

#### B. Application Logic (`app.py`)

-   **Static Model Catalog:** The proxy does not host models. It returns a hardcoded list of supported Z.AI models (e.g., `GLM-4.6`, `GLM-4.5`) to satisfy client discovery requests.
-   **Endpoints:**
    -   `GET /api/tags` & `GET /api/list`: Returns the static catalog.
    -   `POST /api/show`: Returns dummy metadata for a specific model.
    -   `POST /v1/chat/completions`: The core proxy endpoint.
        -   **Input:** Standard OpenAI/Ollama chat completion JSON body.
        -   **Processing:**
            1.  Extracts `stream` flag.
            2.  Retrieves API Key from config or environment variables (`ZAI_API_KEY`, `ZAI_CODING_API_KEY`, `GLM_API_KEY`).
            3.  Injects `Authorization: Bearer <KEY>` header.
            4.  Forwards request to `https://api.z.ai/api/coding/paas/v4/chat/completions`.
        -   **Output:** Streams the upstream response bytes directly to the client (SSE support).

#### C. Configuration (`config.py`)

-   **Storage:** JSON file located at `~/.copilot-proxy/config.json`.
-   **Precedence:** Config File > Environment Variables > Hardcoded Defaults.

## 3. Go Rewrite Plan

Rewriting in Go will produce a single, static binary that is easier to distribute and consumes fewer resources.

### Recommended Stack

-   **Language:** Go (Golang)
-   **CLI Framework:** `github.com/spf13/cobra` (Industry standard for Go CLIs).
-   **Configuration:** `github.com/spf13/viper` (Handles config files + env vars seamlessly).
-   **Web Server:** `net/http` (Standard library is sufficient and robust).

### Proposed Project Structure

```text
copilot-proxy-go/
├── cmd/
│   └── root.go          # Entry point, CLI command definitions (serve, config)
├── internal/
│   ├── config/          # Viper setup, loading/saving JSON config
│   ├── server/          # HTTP server logic
│   │   ├── server.go    # Server setup and routing
│   │   ├── handlers.go  # Handlers for /api/tags, /api/show
│   │   └── proxy.go     # The reverse proxy logic for /chat/completions
│   └── models/          # Structs for JSON request/response bodies
├── main.go              # Calls cmd.Execute()
└── go.mod
```

### Implementation Details

#### 1. The Proxy Handler

Instead of a full reverse proxy struct, a custom handler using `http.Client` is recommended to allow precise header manipulation.

**Logic Flow:**

1.  **Parse Request:** Read the incoming JSON body.
2.  **Prepare Upstream Request:**
    -   Create a new `http.Request` to `api.z.ai`.
    -   Copy `Content-Type` header.
    -   Inject `Authorization: Bearer <API_KEY>`.
3.  **Execute:** Send request via `http.Client`.
4.  **Stream Response:**
    -   Copy upstream status code and headers to the downstream `ResponseWriter`.
    -   Use `io.Copy(w, upstreamResp.Body)` to stream the body efficiently.

#### 2. Configuration Management

Use `viper` to manage the configuration file at `$HOME/.copilot-proxy/config.json`.

**Config Struct:**

```go
type Config struct {
    APIKey  string `mapstructure:"api_key"`
    BaseURL string `mapstructure:"base_url"`
}
```

#### 3. Static Responses

The Python `MODEL_CATALOG` should be converted to a Go struct slice and served as JSON on `/api/tags`.

### Benefits of Go Version

-   **Single Binary:** Zero dependencies for the end-user (no Python/Pip required).
-   **Performance:** Significantly lower memory footprint and faster startup time.
-   **Concurrency:** Go's goroutines are ideal for handling multiple concurrent proxy connections efficiently.
