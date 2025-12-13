# Medium-Priority Issues Solutions

**Date:** December 13, 2025  
**Status:** Proposed solutions for all confirmed medium-priority issues

---

## Issue Analysis & Solutions

### 1. ✅ Untyped Model Catalog

**Current State:** Uses `map[string]interface{}` throughout with type assertions
**Impact:** No compile-time type safety, runtime errors, difficult maintenance

#### Proposed Solution: Typed Model Catalog

```go
// internal/models/catalog.go
package models

// Model represents a single model in the catalog
type Model struct {
    Name         string       `json:"name"`
    Model        string       `json:"model"`
    ModifiedAt   string       `json:"modified_at"`
    Size         int          `json:"size"`
    Digest       string       `json:"digest"`
    Capabilities []string     `json:"capabilities"`
    Details      ModelDetails `json:"details"`
    ContextLen   int          `json:"-"` // Internal use, not serialized
}

// ModelDetails contains model metadata
type ModelDetails struct {
    Format            string   `json:"format"`
    Family            string   `json:"family"`
    Families          []string `json:"families"`
    ParameterSize     string   `json:"parameter_size"`
    QuantizationLevel string   `json:"quantization_level"`
}

// ModelCatalog represents the complete model catalog
type ModelCatalog struct {
    Models []Model `json:"models"`
}

// Typed catalog with context lengths included
var Catalog = ModelCatalog{
    Models: []Model{
        {
            Name:         "GLM-4.6",
            Model:        "GLM-4.6",
            ModifiedAt:   "2024-01-01T00:00:00Z",
            Size:         0,
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
        {
            Name:         "GLM-4.5",
            Model:        "GLM-4.5",
            ModifiedAt:   "2024-01-01T00:00:00Z",
            Size:         0,
            Digest:       "GLM-4.5",
            Capabilities: []string{"tools", "vision"},
            ContextLen:   128000,
            Details: ModelDetails{
                Format:            "glm",
                Family:            "glm",
                Families:          []string{"glm"},
                ParameterSize:     "cloud",
                QuantizationLevel: "cloud",
            },
        },
        {
            Name:         "GLM-4.5-Air",
            Model:        "GLM-4.5-Air",
            ModifiedAt:   "2024-01-01T00:00:00Z",
            Size:         0,
            Digest:       "GLM-4.5-Air",
            Capabilities: []string{"tools", "vision"},
            ContextLen:   128000,
            Details: ModelDetails{
                Format:            "glm",
                Family:            "glm",
                Families:          []string{"glm"},
                ParameterSize:     "cloud",
                QuantizationLevel: "cloud",
            },
        },
    },
}

// IsValidModel checks if a model name exists in the catalog
func IsValidModel(name string) bool {
    for _, m := range Catalog.Models {
        if m.Name == name || m.Model == name {
            return true
        }
    }
    return false
}

// GetModelContextLength returns the context length for a model
func GetModelContextLength(name string) int {
    for _, m := range Catalog.Models {
        if m.Name == name {
            return m.ContextLen
        }
    }
    return 128000 // Default for older models
}

// GetModel returns the full model struct if found
func GetModel(name string) (*Model, bool) {
    for _, m := range Catalog.Models {
        if m.Name == name || m.Model == name {
            return &m, true
        }
    }
    return nil, false
}
```

**Benefits:**
- Compile-time type safety
- No more type assertions
- Context length is data-driven
- Easy to add new models
- Better IDE support

---

### 2. ✅ Buggy Configuration Logic

**Current State:** Multiple `BindEnv` calls overwrite each other
**Impact:** Only last binding (`GLM_API_KEY`) works via Viper

#### Proposed Solution: Single BindEnv with Manual Fallback

```go
// internal/config/config.go
// In Load() function, replace lines 56-62:

// Set environment variable prefix and bind them
v.SetEnvPrefix("ZAI")
v.AutomaticEnv()

// Bind specific environment variables (no duplicates!)
_ = v.BindEnv("api_key", "ZAI_API_KEY")  // Primary only
_ = v.BindEnv("base_url", "ZAI_BASE_URL")
_ = v.BindEnv("host", "ZAI_HOST")
_ = v.BindEnv("port", "ZAI_PORT")
_ = v.BindEnv("debug", "ZAI_DEBUG")

// Remove these duplicate lines:
// _ = v.BindEnv("api_key", "ZAI_CODING_API_KEY")  // DELETE
// _ = v.BindEnv("api_key", "GLM_API_KEY")         // DELETE

// Keep the getAPIKeyFromEnv() function as-is for fallbacks
```

**Benefits:**
- Clear precedence: ZAI_API_KEY > config file > defaults
- Manual fallback handles ZAI_CODING_API_KEY and GLM_API_KEY
- No confusing overwrite behavior

---

### 3. ✅ Inefficient Request Handling

**Current State:** Body is read twice and parsed twice
**Impact:** Memory waste, code duplication

#### Proposed Solution: Single Parse with Map Validation

```go
// internal/server/handlers.go
// Replace handleChatCompletions function:

func (s *Server) handleChatCompletions(c *gin.Context) {
    // Parse once into map
    var bodyMap map[string]any
    if err := c.ShouldBindJSON(&bodyMap); err != nil {
        handleError(c, api.ErrBadRequest("Invalid JSON: "+err.Error()))
        return
    }
    
    // Validate required fields
    model, ok := bodyMap["model"].(string)
    if !ok || model == "" {
        handleError(c, api.ErrBadRequest("model is required"))
        return
    }
    
    messages, ok := bodyMap["messages"].([]any)
    if !ok || len(messages) == 0 {
        handleError(c, api.ErrBadRequest("messages is required and must be non-empty"))
        return
    }
    
    // Validate message structure
    for i, msg := range messages {
        msgMap, ok := msg.(map[string]any)
        if !ok {
            handleError(c, api.ErrBadRequest(fmt.Sprintf("message %d must be an object", i)))
            return
        }
        
        role, ok := msgMap["role"].(string)
        if !ok || role == "" {
            handleError(c, api.ErrBadRequest(fmt.Sprintf("message %d requires a role", i)))
            return
        }
        
        validRoles := map[string]bool{"system": true, "user": true, "assistant": true, "tool": true}
        if !validRoles[role] {
            handleError(c, api.ErrBadRequest(fmt.Sprintf("message %d has invalid role: %s", i, role)))
            return
        }
    }
    
    // Validate model exists
    if !models.IsValidModel(model) {
        handleError(c, api.ErrNotFound(fmt.Sprintf("model '%s' not found", model)))
        return
    }
    
    // Enable deep thinking for GLM models
    bodyMap["thinking"] = map[string]string{
        "type": "enabled",
    }
    
    // Auto-enable tool_stream for GLM-4.6 when tools are present and streaming is enabled
    if model == "GLM-4.6" {
        _, hasTools := bodyMap["tools"]
        stream, _ := bodyMap["stream"].(bool)
        if hasTools && stream {
            bodyMap["tool_stream"] = true
        }
    }
    
    // Create upstream request
    newBodyBytes, err := json.Marshal(bodyMap)
    if err != nil {
        handleError(c, api.ErrInternalServer("Failed to prepare upstream request"))
        return
    }
    
    // ... rest of function remains the same
}
```

**Benefits:**
- Single parse, less memory usage
- Cleaner validation logic
- Uses Gin's binding properly
- Preserves all fields for forwarding

---

### 4. ✅ Inconsistent/Disorganized Tests

**Current State:** Duplicate tests, mixed testing approaches
**Impact:** Maintenance overhead, potential for inconsistent behavior

#### Proposed Solution: Consolidate and Standardize Tests

```go
// internal/server/handlers_test.go
// Add these new tests at the end of the file:

// TestChatCompletions_VisionContent tests vision model multipart content
func TestChatCompletions_VisionContent(t *testing.T) {
    mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var body map[string]any
        json.NewDecoder(r.Body).Decode(&body)
        
        // Verify vision content structure
        messages, ok := body["messages"].([]any)
        assert.True(t, ok)
        
        firstMsg := messages[0].(map[string]any)
        content := firstMsg["content"]
        
        // Should be an array for vision content
        contentArray, ok := content.([]any)
        assert.True(t, ok)
        assert.Len(t, contentArray, 2)
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"choices": [{"message": {"content": "I see the image"}}]}`))
    }))
    defer mockUpstream.Close()
    
    cfg := &config.Config{
        APIKey:  "test-key",
        BaseURL: mockUpstream.URL,
    }
    s := NewServer(cfg, "127.0.0.1", 0)
    
    // Vision content with text and image
    reqBody := `{
        "model": "GLM-4.6",
        "messages": [{
            "role": "user",
            "content": [
                {"type": "text", "text": "What do you see?"},
                {"type": "image_url", "image_url": {"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="}}
            ]
        }]
    }`
    
    req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    
    s.router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}

// TestChatCompletions_ContextCancellation tests client disconnect handling
func TestChatCompletions_ContextCancellation(t *testing.T) {
    mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Simulate slow response
        time.Sleep(100 * time.Millisecond)
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("data: {\"choices\": [{\"delta\": {\"content\": \"Hi\"}}]}\n\n"))
        w.(http.Flusher).Flush()
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
    
    // Create context with cancellation
    ctx, cancel := context.WithCancel(req.Context())
    cancel() // Cancel immediately
    req = req.WithContext(ctx)
    
    w := httptest.NewRecorder()
    
    s.router.ServeHTTP(w, req)
    
    // Should return 499 for client cancellation
    assert.Equal(t, 499, w.Code)
}

// Remove TestValidationStillWorks from handlers_extended_test.go as it duplicates tests here
```

**Benefits:**
- All tests in one place
- Consistent router-based testing
- Better coverage of edge cases
- Tests for new vision content feature

---

## Implementation Order

### Phase 1: Foundation (Model Catalog)
1. Refactor `models/catalog.go` to use typed structs
2. Update any code that uses the old map structure
3. Run tests to ensure compatibility

### Phase 2: Configuration Cleanup
1. Fix `config/config.go` BindEnv redundancy
2. Test with different environment variables
3. Verify fallback behavior works

### Phase 3: Request Optimization
1. Refactor `handlers.go` to single parse
2. Add comprehensive validation
3. Test with various request formats

### Phase 4: Test Consolidation
1. Move all tests to `handlers_test.go`
2. Delete `handlers_extended_test.go`
3. Add new tests for vision and cancellation

---

## Testing Strategy

After each phase:
1. Run `go test ./internal/server -v`
2. Run `go test ./internal/models -v` (add tests if needed)
3. Run `go test ./internal/config -v` (add tests if needed)
4. Build project: `go build -o bin/copilot-proxy .`
5. Manual smoke test with actual API

---

## Migration Notes

### Breaking Changes
- Model catalog now uses structs - may affect code that accesses raw map
- Request validation is stricter - may reject previously accepted malformed requests

### Compatibility
- API response format unchanged
- Environment variable handling preserved
- All existing endpoints work the same

---

## Future Enhancements

These changes enable:
1. Easy addition of new models to catalog
2. Better error messages with validation details
3. Potential for model-specific configuration
4. More comprehensive test coverage