# Go Copilot Proxy Concept

## Goal
A fast, single-binary proxy that mimics Ollama/OpenAI endpoints locally and forwards requests to Z.AI Coding PaaS with minimal overhead and strong concurrency handling.

## Stack
- Go (std `net/http` + `github.com/gin-gonic/gin` for routing).
- CLI via `github.com/spf13/cobra`; config via `github.com/spf13/viper`.
- Env autoload in entrypoint: `import _ "github.com/joho/godotenv/autoload"`. 
- HTTP client: tuned `http.Transport` with keep-alives, timeouts; no extra proxy layer.

## Key Endpoints
- `GET /api/tags` and `GET /api/list`: static model catalog.
- `POST /api/show`: static metadata stub.
- `POST /v1/chat/completions`: forward to `https://api.z.ai/api/coding/paas/v4/chat/completions`, inject `Authorization: Bearer <key>`, stream upstream bytes to client (SSE compatible), abort on client disconnect.

## Config & Auth
- Config file at `$HOME/.copilot-proxy/config.json` managed by viper.
- Precedence: CLI flag > env (`ZAI_API_KEY`/`ZAI_CODING_API_KEY`/`GLM_API_KEY`, `ZAI_BASE_URL`) > config file defaults.
- `setup`/`config` commands for initial API key and base URL.

## Performance & Reliability
- Reuse a single `http.Client` with custom transport (HTTP/2, `MaxIdleConnsPerHost`, timeouts).
- Stream bodies with `io.CopyBuffer`; avoid re-marshalling payloads.
- Minimal middleware in gin; optional debug logging toggle.
- Graceful shutdown on SIGINT/SIGTERM; optional `/healthz`.

## Project Layout
```
copilot-proxy/
├── main.go                # cobra root, env autoload
├── cmd/                   # CLI commands: serve, config, setup
├── internal/config        # viper setup, config file read/write
├── internal/server        # gin engine, routes, shutdown
├── internal/proxy         # upstream request builder + streaming
├── internal/models        # structs and static catalog
└── bin/                   # build outputs (go build -o bin/copilot-proxy .)
```

## Testing
- Handler tests for catalog endpoints.
- Proxy integration test with httptest upstream to assert auth header injection and streaming behavior.
- Config precedence tests.
