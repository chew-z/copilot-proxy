# FINDINGS.md Verification Report

**Date:** December 13, 2025  
**Reviewer:** GitHub Copilot

This document verifies the issues identified in [FINDINGS.md](FINDINGS.md) against the actual codebase and proposes concrete solutions.

---

## Verification Summary

| Finding | Severity | Status | Priority |
|---------|----------|--------|----------|
| Logger file descriptor leak | 🔴 High | ✅ Confirmed | 1 |
| HTTP client timeout breaks streaming | 🔴 High | ✅ Confirmed | 1 |
| Vision model content type | 🔴 High | ✅ Confirmed | 2 |
| Double-parsing request body | 🔴 High | ✅ Confirmed | 3 |
| CORS `*` policy | 🔴 High | ⚠️ Acceptable for localhost | 5 |
| Missing happy-path tests | 🔴 High | ✅ Confirmed | 2 |
| Untyped model catalog | 🟡 Medium | ✅ Confirmed | 3 |
| Error wrapping loses context | 🟡 Medium | ⚠️ Partial (acceptable for API) | 4 |
| Buggy BindEnv config | 🟡 Medium | ✅ Confirmed | 3 |
| Hardcoded model logic | 🟡 Medium | ✅ Confirmed | 3 |
| Duplicate tests | 🟡 Medium | ✅ Confirmed | 4 |
| Over-engineered Save | 🟡 Medium | ❌ Not confirmed | N/A |
| Version string mismatch | 🟢 Low | ✅ Confirmed | 5 |
| Experimental GC default | 🟢 Low | ⚠️ Acceptable for Go 1.25+ | 5 |
| Script hardcoded port | 🟢 Low | ✅ Confirmed | 5 |
| log.Fatalf overuse | 🟢 Low | ✅ Confirmed | 4 |

---

## 🔴 HIGH-SEVERITY ISSUES

### 1. Critical Resource Leak: Logger File Descriptor ✅ CONFIRMED

**Location:** `internal/server/server.go` lines 36-66

**Issue:** The log file is opened but never closed:
```go
logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
// ...file is used but never closed
```

**Impact:** File descriptor leak on every server restart. In long-running systems or frequent restarts, this can exhaust OS file descriptors.

**Proposed Solution:** Store `logFile` in the `Server` struct and close it in the `Shutdown()` method:
```go
type Server struct {
    config  *config.Config
    router  *gin.Engine
    server  *http.Server
    client  *http.Client
    logFile *os.File  // ADD THIS
}

func (s *Server) Shutdown(ctx context.Context) error {
    if s.logFile != nil {
        s.logFile.Close()
    }
    return s.server.Shutdown(ctx)
}
```

---

### 2. Streaming Bug: Global HTTP Client Timeout ✅ CONFIRMED

**Location:** `internal/server/server.go` lines 88-95

**Issue:** The HTTP client has a 120-second timeout:
```go
client := &http.Client{
    Transport: &http.Transport{...},
    Timeout: 120 * time.Second,  // This kills long streams!
}
```

**Impact:** Long-running streaming responses (e.g., complex code generation) will be terminated after 120 seconds, breaking a core feature of the proxy.

**Proposed Solution:** Remove the global timeout and use context-based request cancellation instead:
```go
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConnsPerHost:   50,
        IdleConnTimeout:       90 * time.Second,
        ResponseHeaderTimeout: 30 * time.Second, // Timeout only for headers
    },
    // No global Timeout - let context handle cancellation
}
```

---

### 3. Vision Model Bug: Message Content Type ✅ CONFIRMED

**Location:** `internal/api/types.go` lines 13-14

**Issue:** `Content` is a plain string:
```go
type Message struct {
    Role    string `binding:"required,oneof=system user assistant tool" json:"role"`
    Content string `json:"content"`
}
```

**Impact:** Vision models (advertised in the catalog with `"capabilities": ["vision"]`) require multipart content like:
```json
{"content": [{"type": "text", "text": "..."}, {"type": "image_url", "image_url": {...}}]}
```

This makes the vision capability non-functional.

**Proposed Solution:** Use `any` or a custom union type:
```go
type Message struct {
    Role    string `binding:"required,oneof=system user assistant tool" json:"role"`
    Content any    `json:"content"` // string or []ContentPart
}

type ContentPart struct {
    Type     string    `json:"type"`
    Text     string    `json:"text,omitempty"`
    ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
    URL    string `json:"url"`
    Detail string `json:"detail,omitempty"`
}
```

---

### 4. Inefficient Request Handling ✅ CONFIRMED

**Location:** `internal/server/handlers.go` lines 87-142

**Issue:** The handler reads the body twice and performs manual validation:
```go
bodyBytes, err := io.ReadAll(c.Request.Body)  // First read
// ...
if err := json.Unmarshal(bodyBytes, &req); err != nil {  // Parse for validation
// ...
var bodyMap map[string]any
if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {  // Parse again for forwarding
```

Also, manual validation duplicates what Gin's `ShouldBindJSON` does with binding tags.

**Impact:** Memory inefficiency (2x parsing), code duplication, and deviation from framework patterns.

**Proposed Solution:** Parse once into a map, then validate:
```go
func (s *Server) handleChatCompletions(c *gin.Context) {
    var bodyMap map[string]any
    if err := c.ShouldBindJSON(&bodyMap); err != nil {
        handleError(c, api.ErrBadRequest("Invalid JSON: "+err.Error()))
        return
    }
    
    // Validate required fields from bodyMap
    model, ok := bodyMap["model"].(string)
    if !ok || model == "" {
        handleError(c, api.ErrBadRequest("model is required"))
        return
    }
    
    messages, ok := bodyMap["messages"].([]any)
    if !ok || len(messages) == 0 {
        handleError(c, api.ErrBadRequest("messages is required"))
        return
    }
    
    // Validate model exists
    if !models.IsValidModel(model) {
        handleError(c, api.ErrNotFound(fmt.Sprintf("model '%s' not found", model)))
        return
    }
    
    // Continue with bodyMap for forwarding...
}
```

---

### 5. Insecure CORS Policy ⚠️ ACCEPTABLE FOR LOCALHOST

**Location:** `internal/server/server.go` lines 80-87

**Issue:** CORS allows all origins:
```go
router.Use(cors.New(cors.Config{
    AllowOrigins: []string{"*"},
    // ...
}))
```

**Assessment:** For a localhost proxy designed to work with various local clients (IDEs, Copilot, WebUIs), this is **acceptable**. However, if the proxy is ever exposed on a network, this becomes a security risk.

**Proposed Solution (optional):** Make CORS configurable:
```go
// In config.go
type Config struct {
    // ...existing fields...
    AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// In server.go
origins := cfg.AllowedOrigins
if len(origins) == 0 {
    origins = []string{"*"}
}
router.Use(cors.New(cors.Config{
    AllowOrigins: origins,
    // ...
}))
```

---

### 6. Incomplete Test Coverage ✅ CONFIRMED

**Location:** `internal/server/handlers_test.go`, `internal/server/handlers_extended_test.go`

**Issue:** Tests only cover validation failures and metadata endpoints. There are **no tests** for:
- Successful streaming responses
- Successful non-streaming responses  
- Proper header forwarding
- Error handling from upstream
- Context cancellation during streaming

**Proposed Solution:** Add integration tests with a mock upstream server:
```go
func TestChatCompletions_SuccessfulStreaming(t *testing.T) {
    // Create mock upstream that sends SSE events
    mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify Authorization header
        assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
        
        // Verify thinking was injected
        var body map[string]any
        json.NewDecoder(r.Body).Decode(&body)
        assert.NotNil(t, body["thinking"])
        
        // Send SSE response
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        flusher := w.(http.Flusher)
        
        w.Write([]byte("data: {\"choices\": [{\"delta\": {\"content\": \"Hello\"}}]}\n\n"))
        flusher.Flush()
        w.Write([]byte("data: {\"choices\": [{\"delta\": {\"content\": \" World\"}}]}\n\n"))
        flusher.Flush()
        w.Write([]byte("data: [DONE]\n\n"))
        flusher.Flush()
    }))
    defer mockUpstream.Close()
    
    cfg := &config.Config{
        APIKey:  "test-key",
        BaseURL: mockUpstream.URL,
    }
    s := NewServer(cfg, "127.0.0.1", 0)
    
    reqBody := `{"model": "GLM-4.6", "messages": [{"role": "user", "content": "hi"}], "stream": true}`
    req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    
    s.router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Body.String(), "Hello")
    assert.Contains(t, w.Body.String(), "World")
}

func TestChatCompletions_UpstreamError(t *testing.T) {
    mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte(`{"error": "upstream error"}`))
    }))
    defer mockUpstream.Close()
    
    // ... test that error is propagated correctly
}

func TestChatCompletions_ClientCancellation(t *testing.T) {
    // Test that context cancellation is handled gracefully
}
```

---

## 🟡 MEDIUM-SEVERITY ISSUES

### 1. Untyped Model Catalog ✅ CONFIRMED

**Location:** `internal/models/catalog.go` lines 4-52

**Issue:** Uses `map[string]interface{}` everywhere with no type safety.

**Proposed Solution:** Define proper structs:
```go
package models

type Model struct {
    Name         string       `json:"name"`
    Model        string       `json:"model"`
    ModifiedAt   string       `json:"modified_at"`
    Size         int          `json:"size"`
    Digest       string       `json:"digest"`
    Capabilities []string     `json:"capabilities"`
    Details      ModelDetails `json:"details"`
    ContextLen   int          `json:"-"` // internal use, not serialized
}

type ModelDetails struct {
    Format            string   `json:"format"`
    Family            string   `json:"family"`
    Families          []string `json:"families"`
    ParameterSize     string   `json:"parameter_size"`
    QuantizationLevel string   `json:"quantization_level"`
}

type ModelCatalog struct {
    Models []Model `json:"models"`
}

var Catalog = ModelCatalog{
    Models: []Model{
        {
            Name:         "GLM-4.6",
            Model:        "GLM-4.6",
            ModifiedAt:   "2024-01-01T00:00:00Z",
            Digest:       "GLM-4.6",
            Capabilities: []string{"tools", "vision"},
            ContextLen:   200000,
            Details: ModelDetails{
                Format:            "glm",
                Family:            "glm",
                Families:          []string{"glm"},
                ParameterSize:     "cloud",
                QuantizationLevel: "cloud",
            },
        },
        // ... other models
    },
}

func IsValidModel(name string) bool {
    for _, m := range Catalog.Models {
        if m.Name == name || m.Model == name {
            return true
        }
    }
    return false
}

func GetModelContextLength(name string) int {
    for _, m := range Catalog.Models {
        if m.Name == name {
            return m.ContextLen
        }
    }
    return 128000 // default
}
```

---

### 2. Improper Error Wrapping ⚠️ ACCEPTABLE FOR API

**Location:** `internal/api/errors.go` lines 49-57

**Issue:** `WrapError` concatenates strings instead of wrapping with `%w`.

**Assessment:** For HTTP API responses, losing the error chain is acceptable (users shouldn't see internal stack traces). However, internal logging should preserve the chain.

**Proposed Solution:** Keep `StatusError` for user-facing responses but add proper wrapping for internal use:
```go
// For internal logging (preserves chain for errors.Is/As)
func WrapWithContext(err error, msg string) error {
    return fmt.Errorf("%s: %w", msg, err)
}

// WrapError remains as-is for API responses (intentionally flattens)
```

---

### 3. Buggy Configuration Logic ✅ CONFIRMED

**Location:** `internal/config/config.go` lines 56-60

**Issue:** Multiple `BindEnv` calls for the same key overwrite each other:
```go
_ = v.BindEnv("api_key", "ZAI_API_KEY")
_ = v.BindEnv("api_key", "ZAI_CODING_API_KEY")  // Overwrites previous!
_ = v.BindEnv("api_key", "GLM_API_KEY")         // Overwrites again!
```

Only `GLM_API_KEY` will actually work via Viper binding.

**Proposed Solution:** Remove redundant BindEnv calls; the manual `getAPIKeyFromEnv()` already handles fallback correctly:
```go
// In Load():
_ = v.BindEnv("api_key", "ZAI_API_KEY")  // Primary binding only
_ = v.BindEnv("base_url", "ZAI_BASE_URL")
_ = v.BindEnv("host", "ZAI_HOST")
_ = v.BindEnv("port", "ZAI_PORT")
_ = v.BindEnv("debug", "ZAI_DEBUG")

// Remove the duplicate BindEnv calls for api_key
// getAPIKeyFromEnv() handles ZAI_CODING_API_KEY and GLM_API_KEY fallbacks
```

---

### 4. Hardcoded Model Logic ✅ CONFIRMED

**Location:** `internal/models/catalog.go` lines 68-72

**Issue:** Context length uses hardcoded if statement instead of data.

**Proposed Solution:** See typed catalog solution above - context length becomes a field on the Model struct.

---

### 5. Inconsistent/Disorganized Tests ✅ CONFIRMED

**Locations:** `internal/server/handlers_test.go`, `internal/server/handlers_extended_test.go`

**Issues:**
1. `TestValidationStillWorks` duplicates tests from `handlers_test.go`
2. Extended tests bypass the router (call handler directly)
3. No tests for `config` or `models` packages

**Proposed Solution:**
- Consolidate duplicate tests into a single file
- Standardize on router-based testing for integration tests
- Add unit tests for config and models packages

---

### 6. Over-engineered Save Function ❌ NOT CONFIRMED

**Location:** `internal/config/config.go` lines 91-117

**Assessment:** The Save function is straightforward Viper usage - creates directory, initializes Viper, sets values, writes file. Not over-engineered.

---

## 🟢 LOW-SEVERITY ISSUES

### 1. Inconsistent Version Strings ✅ CONFIRMED

**Locations:**
- `cmd/root.go` line 17: `Version: "0.4.0"`
- `internal/server/handlers.go` line 36: `"version": "0.6.4"`

**Proposed Solution:** Single source of truth:
```go
// internal/version/version.go
package version

var Version = "0.6.4"

// cmd/root.go
import "github.com/chew-z/copilot-proxy/internal/version"

var rootCmd = &cobra.Command{
    // ...
    Version: version.Version,
}

// internal/server/handlers.go
import "github.com/chew-z/copilot-proxy/internal/version"

func (s *Server) handleVersion(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "version": version.Version,
    })
}
```

---

### 2. Risky Build Defaults ⚠️ ACCEPTABLE

**Location:** `Makefile` lines 5-7

**Issue:** Uses `GOEXPERIMENT=greenteagc`.

**Assessment:** The project uses Go 1.25.5 where `greenteagc` is production-ready. The race detector omission in `test` is intentional for speed.

**Proposed Solution (optional):** Add a separate race test target:
```makefile
# Run tests with race detector
test-race:
	go test -race -v ./...
```

---

### 3. Brittle Helper Script ✅ CONFIRMED

**Location:** `scripts/copilot-proxy-ctl.sh` line 11

**Issue:** Hardcoded port:
```bash
HEALTH_URL="http://127.0.0.1:11434/healthz"
```

**Proposed Solution:** Read port from config or environment:
```bash
# Try to get port from copilot-proxy config, fall back to default
CONFIG_PORT=$(copilot-proxy config get port 2>/dev/null | grep -oE '[0-9]+' || echo "11434")
PROXY_PORT="${COPILOT_PROXY_PORT:-$CONFIG_PORT}"
HEALTH_URL="http://127.0.0.1:${PROXY_PORT}/healthz"
```

---

### 4. Overuse of log.Fatalf ✅ CONFIRMED

**Location:** `cmd/serve.go` - multiple occurrences

**Issue:** `log.Fatalf` calls `os.Exit(1)` which bypasses deferred cleanup and makes testing difficult.

**Proposed Solution:** Return errors from helper functions:
```go
func loadAndValidateConfig(cmd *cobra.Command) (*config.Config, error) {
    cfg, err := config.Load()
    if err != nil {
        return nil, fmt.Errorf("failed to load configuration: %w", err)
    }

    if cfg.APIKey == "" {
        return nil, fmt.Errorf("API key is not configured; " +
            "run 'copilot-proxy config set api_key YOUR_API_KEY' " +
            "or set ZAI_API_KEY environment variable")
    }

    applyCLIOverrides(cmd, cfg)
    return cfg, nil
}

func runServe(cmd *cobra.Command, args []string) {
    cfg, err := loadAndValidateConfig(cmd)
    if err != nil {
        fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
        os.Exit(1)
    }
    // ... rest of function
}
```

---

## Recommended Fix Order

### Phase 1: Critical Bugs (Priority 1)
1. Fix logger file descriptor leak in `server.go`
2. Remove global HTTP client timeout in `server.go`

### Phase 2: Feature Completeness (Priority 2)
3. Fix vision model content type in `types.go`
4. Add happy-path integration tests

### Phase 3: Code Quality (Priority 3)
5. Refactor model catalog to use typed structs
6. Fix config BindEnv redundancy
7. Optimize request handling (single parse)

### Phase 4: Polish (Priority 4)
8. Consolidate duplicate tests
9. Improve error handling patterns
10. Replace log.Fatalf with proper error returns

### Phase 5: Nice-to-have (Priority 5)
11. Unify version strings
12. Make CORS configurable
13. Fix script hardcoded port
14. Add `test-race` Makefile target

---

## Implementation Notes

When implementing these fixes, follow the project's Go instructions in `.github/instructions/Go.instructions.md`:

- Use proper error wrapping with `%w`
- Keep functions small and focused
- Add tests for new code
- Use `gofmt` and `goimports`
- Follow idiomatic Go patterns
