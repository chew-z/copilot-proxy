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

	// Normalize model name to lowercase for upstream API (Z.AI expects lowercase)
	canonicalModel := models.GetCanonicalModelName(model)
	bodyMap["model"] = canonicalModel

	// Auto-enable tool_stream for GLM-4.6 and GLM-4.7 when tools are present and streaming is enabled
	// This enables real-time streaming of tool call parameters
	if canonicalModel == "glm-4.6" || canonicalModel == "glm-4.7" {
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
