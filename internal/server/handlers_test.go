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
			wantError:  "Invalid request",
		},
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
