package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/chew-z/copilot-proxy/internal/api"
	"github.com/chew-z/copilot-proxy/internal/models"
	"github.com/gin-gonic/gin"
)

// handleError sends a standardized error response with context-aware cancellation handling
func handleError(c *gin.Context, err error) {
	// Check for context cancellation (client disconnected)
	if errors.Is(err, context.Canceled) {
		c.JSON(499, gin.H{"error": "request canceled"})
		return
	}
	if se, ok := err.(*api.StatusError); ok {
		c.JSON(se.StatusCode, se)
		return
	}
	c.JSON(http.StatusInternalServerError, api.StatusError{ErrorMessage: err.Error()})
}

// handleVersion returns the API version
func (s *Server) handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "0.6.4",
	})
}

// handlePs returns running models (empty for proxy)
func (s *Server) handlePs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"models": []interface{}{},
	})
}

// handleTags returns the model catalog
func (s *Server) handleTags(c *gin.Context) {
	c.JSON(http.StatusOK, models.Catalog)
}

// handleShow returns model metadata
func (s *Server) handleShow(c *gin.Context) {
	var req api.ShowRequest
	// We don't strictly require the body to be valid, if it's empty we'll use default
	_ = c.ShouldBindJSON(&req)

	modelName := req.Name
	if modelName == "" {
		modelName = req.Model
	}
	if modelName == "" {
		modelName = "GLM-4.6"
	}

	contextLength := models.GetModelContextLength(modelName)

	response := api.ShowResponse{
		Template:     "{{ .System }}\n{{ .Prompt }}",
		Capabilities: []string{"tools", "vision"},
		Details: api.ModelDetails{
			Family:            "glm",
			Families:          []string{"glm"},
			Format:            "glm",
			ParameterSize:     "cloud",
			QuantizationLevel: "cloud",
		},
		ModelInfo: map[string]any{
			"general.basename":     modelName,
			"general.architecture": "glm",
			"glm.context_length":   contextLength,
		},
	}

	c.JSON(http.StatusOK, response)
}

// handleChatCompletions proxies requests to Z.AI API
func (s *Server) handleChatCompletions(c *gin.Context) {
	// Read the raw request body first
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		handleError(c, api.ErrBadRequest("Failed to read request body"))
		return
	}

	// First, validate with the strict struct to ensure required fields are present
	var req api.ChatRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		handleError(c, api.ErrBadRequest("Invalid request: "+err.Error()))
		return
	}

	// Perform manual validation since json.Unmarshal doesn't check binding tags
	if req.Model == "" {
		handleError(c, api.ErrBadRequest("Error:Field validation for 'Model' failed on the 'required' tag"))
		return
	}
	if len(req.Messages) == 0 {
		handleError(c, api.ErrBadRequest("Error:Field validation for 'Messages' failed on the 'required' tag"))
		return
	}
	for i, msg := range req.Messages {
		if msg.Role == "" {
			handleError(c, api.ErrBadRequest(fmt.Sprintf("Error:Field validation for 'Role' failed on the 'required' tag at message %d", i)))
			return
		}
		validRoles := map[string]bool{"system": true, "user": true, "assistant": true, "tool": true}
		if !validRoles[msg.Role] {
			handleError(c, api.ErrBadRequest(fmt.Sprintf("Error:Field validation for 'Role' failed on the 'oneof' tag at message %d", i)))
			return
		}
	}

	// Validate model exists in catalog
	if !models.IsValidModel(req.Model) {
		handleError(c, api.ErrNotFound(fmt.Sprintf("model '%s' not found", req.Model)))
		return
	}

	// Now parse as map to preserve all fields (e.g., tools, tool_choice, etc.)
	var bodyMap map[string]any
	if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
		handleError(c, api.ErrBadRequest("Invalid request: "+err.Error()))
		return
	}

	// Use the validated model name
	modelName := req.Model

	// Enable deep thinking for GLM models
	bodyMap["thinking"] = map[string]string{
		"type": "enabled",
	}

	// Auto-enable tool_stream for GLM-4.6 when tools are present and streaming is enabled
	// This enables real-time streaming of tool call parameters (GLM-4.6 exclusive feature)
	if modelName == "GLM-4.6" {
		_, hasTools := bodyMap["tools"]
		stream, _ := bodyMap["stream"].(bool)
		if hasTools && stream {
			bodyMap["tool_stream"] = true
		}
	}

	newBodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		handleError(c, api.ErrInternalServer("Failed to prepare upstream request"))
		return
	}

	// Create upstream request with context for cancellation handling
	ctx := c.Request.Context()
	upstreamURL := s.config.BaseURL + "/chat/completions"
	upstreamReq, err := http.NewRequestWithContext(ctx, "POST", upstreamURL, bytes.NewReader(newBodyBytes))
	if err != nil {
		handleError(c, api.ErrInternalServer("Failed to create upstream request"))
		return
	}

	// Set Content-Type for upstream
	upstreamReq.Header.Set("Content-Type", "application/json")

	// Add Authorization header
	if s.config.APIKey != "" {
		upstreamReq.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	// Execute request
	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		// Check for context cancellation (client disconnected)
		if errors.Is(err, context.Canceled) {
			slog.Debug("Client disconnected during upstream request")
			c.JSON(499, gin.H{"error": "request canceled"})
			return
		}
		handleError(c, api.ErrBadGateway("Failed to connect to upstream server"))
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	// Set status code
	c.Writer.WriteHeader(resp.StatusCode)

	// Stream response body with context awareness
	if err := streamResponse(ctx, c, resp.Body); err != nil {
		// Check if client disconnected
		if errors.Is(err, context.Canceled) {
			slog.Debug("Client disconnected during streaming")
		}
		return
	}
}

// streamResponse streams the response body with SSE support and context awareness
func streamResponse(ctx context.Context, c *gin.Context, body io.ReadCloser) error {
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		// Check if context is canceled before reading
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := body.Read(buf)
		if n > 0 {
			// Write chunk
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			// Flush for SSE support
			c.Writer.Flush()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}
