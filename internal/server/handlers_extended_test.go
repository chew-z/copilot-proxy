package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chew-z/copilot-proxy/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

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
			name: "GLM-4.6 with tools but no stream should not enable tool_stream",
			requestBody: `{
				"model": "GLM-4.6",
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
			// Create a mock HTTP server to capture the upstream request
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
			req := httptest.NewRequest("POST", "/api/chat", bytes.NewBufferString(tt.requestBody))
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

func TestValidationStillWorks(t *testing.T) {
	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com",
		Debug:   true,
	}

	s := NewServer(cfg, "127.0.0.1", 11434)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "Missing Model",
			body:       `{"messages":[{"role":"user","content":"hi"}]}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "Error:Field validation for 'Model' failed",
		},
		{
			name:       "Invalid Role",
			body:       `{"model":"GLM-4.6", "messages":[{"role":"invalid","content":"hi"}]}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "Error:Field validation for 'Role' failed",
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
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			s.handleChatCompletions(c)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantError)
		})
	}
}
