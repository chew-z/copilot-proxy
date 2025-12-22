package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chew-z/copilot-proxy/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestServer() *Server {
	// Use test mode
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Host: "localhost",
		Port: 0,
	}
	return NewServer(cfg, "localhost", 0)
}

func TestHandleVersion(t *testing.T) {
	s := setupTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/version", nil)
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "version") {
		t.Error("Response body should contain version")
	}
}

func TestHandleShow_DefaultModel(t *testing.T) {
	s := setupTestServer()
	w := httptest.NewRecorder()
	// Empty body should default to GLM-4.6 or fallback
	req, _ := http.NewRequest("POST", "/api/show", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	// Check deep nested fields
	details, ok := resp["details"].(map[string]interface{})
	if !ok {
		t.Error("Response missing details")
	}
	if details["family"] != "glm" {
		t.Errorf("Expected family glm, got %v", details["family"])
	}
}

func TestChatCompletions_Validation(t *testing.T) {
	s := setupTestServer()

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "Empty Body",
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid JSON: EOF",
		},
		{
			name:       "Missing Model",
			body:       `{"messages":[{"role":"user","content":"hi"}]}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "model is required",
		},
		{
			name:       "Invalid Role",
			body:       `{"model":"GLM-4.6", "messages":[{"role":"invalid","content":"hi"}]}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "message 0 has invalid role: invalid",
		},
		{
			name:       "Unknown Model",
			body:       `{"model":"UNKNOWN-MODEL", "messages":[{"role":"user","content":"hi"}]}`,
			wantStatus: http.StatusNotFound,
			wantError:  "model 'UNKNOWN-MODEL' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			s.router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantError != "" && !strings.Contains(w.Body.String(), tt.wantError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.wantError, w.Body.String())
			}
		})
	}
}

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

func TestChatCompletions_SuccessfulNonStreaming(t *testing.T) {
	// Create mock upstream that sends JSON response
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		// Verify thinking was injected
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["thinking"])

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "GLM-4.6",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello World"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 9,
				"completion_tokens": 12,
				"total_tokens": 21
			}
		}`))
	}))
	defer mockUpstream.Close()

	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: mockUpstream.URL,
	}
	s := NewServer(cfg, "127.0.0.1", 0)

	reqBody := `{"model": "GLM-4.6", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello World")
	assert.Contains(t, w.Body.String(), "chatcmpl-123")
}

func TestChatCompletions_UpstreamError(t *testing.T) {
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "upstream error"}`))
	}))
	defer mockUpstream.Close()

	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: mockUpstream.URL,
	}
	s := NewServer(cfg, "127.0.0.1", 0)

	reqBody := `{"model": "GLM-4.6", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	// The upstream error should be forwarded as-is (500)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "upstream error")
}

func TestChatCompletions_ConnectionError(t *testing.T) {
	// Use a non-existent URL to simulate connection error
	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:99999", // Non-existent port
	}
	s := NewServer(cfg, "127.0.0.1", 0)

	reqBody := `{"model": "GLM-4.6", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	// Connection errors should return 502
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to connect to upstream server")
}

func TestToolStreamAutoEnable(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com",
		Debug:   true,
	}

	// Create server
	s := NewServer(cfg, "127.0.0.1", 11434)

	// Test cases
	tests := []struct {
		name             string
		requestBody      string
		expectToolStream bool
	}{
		{
			name: "GLM-4.6 with tools and stream should enable tool_stream",
			requestBody: `{
				"model": "GLM-4.6",
				"messages": [{"role": "user", "content": "test"}],
				"stream": true,
				"tools": [{"type": "function", "function": {"name": "test"}}]
			}`,
			expectToolStream: true,
		},
		{
			name: "GLM-4.7 with tools and stream should enable tool_stream",
			requestBody: `{
				"model": "GLM-4.7",
				"messages": [{"role": "user", "content": "test"}],
				"stream": true,
				"tools": [{"type": "function", "function": {"name": "test"}}]
			}`,
			expectToolStream: true,
		},
		{
			name: "GLM-4.6 with tools but no stream should not enable tool_stream",
			requestBody: `{
				"model": "GLM-4.6",
				"messages": [{"role": "user", "content": "test"}],
				"tools": [{"type": "function", "function": {"name": "test"}}]
			}`,
			expectToolStream: false,
		},
		{
			name: "GLM-4.7 with tools but no stream should not enable tool_stream",
			requestBody: `{
				"model": "GLM-4.7",
				"messages": [{"role": "user", "content": "test"}],
				"tools": [{"type": "function", "function": {"name": "test"}}]
			}`,
			expectToolStream: false,
		},
		{
			name: "GLM-4.6 with stream but no tools should not enable tool_stream",
			requestBody: `{
				"model": "GLM-4.6",
				"messages": [{"role": "user", "content": "test"}],
				"stream": true
			}`,
			expectToolStream: false,
		},
		{
			name: "GLM-4.7 with stream but no tools should not enable tool_stream",
			requestBody: `{
				"model": "GLM-4.7",
				"messages": [{"role": "user", "content": "test"}],
				"stream": true
			}`,
			expectToolStream: false,
		},
		{
			name: "GLM-4.5 should not enable tool_stream even with tools and stream",
			requestBody: `{
				"model": "GLM-4.5",
				"messages": [{"role": "user", "content": "test"}],
				"stream": true,
				"tools": [{"type": "function", "function": {"name": "test"}}]
			}`,
			expectToolStream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP server to capture upstream request
			capturedBody := make(chan map[string]any, 1)
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var bodyMap map[string]any
				json.NewDecoder(r.Body).Decode(&bodyMap)
				capturedBody <- bodyMap
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"choices": [{"delta": {"content": "test"}}]}`))
			}))
			defer mockServer.Close()

			// Update server config to use mock server
			s.config.BaseURL = mockServer.URL

			// Create request
			req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Set up gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			s.handleChatCompletions(c)

			// Check captured request
			select {
			case bodyMap := <-capturedBody:
				toolStream, exists := bodyMap["tool_stream"]
				if tt.expectToolStream {
					assert.True(t, exists, "tool_stream should exist")
					assert.Equal(t, true, toolStream, "tool_stream should be true")
				} else {
					assert.False(t, exists, "tool_stream should not exist")
				}
			default:
				t.Error("No request captured")
			}
		})
	}
}
