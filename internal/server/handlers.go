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
	var req api.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, api.ErrBadRequest("Invalid request: "+err.Error()))
		return
	}

	// Validate model exists in catalog
	if !models.IsValidModel(req.Model) {
		handleError(c, api.ErrNotFound(fmt.Sprintf("model '%s' not found", req.Model)))
		return
	}

	// Set Content-Type based on streaming mode (Ollama pattern)
	// Default is streaming (stream=true or stream=nil)
	if req.Stream == nil || *req.Stream {
		c.Header("Content-Type", "application/x-ndjson")
	} else {
		c.Header("Content-Type", "application/json; charset=utf-8")
	}

	// Inject thinking parameter
	if req.Options == nil {
		req.Options = make(map[string]any)
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		handleError(c, api.ErrInternalServer("Failed to marshal request"))
		return
	}

	var bodyMap map[string]interface{}
	if unmarshalErr := json.Unmarshal(bodyBytes, &bodyMap); unmarshalErr != nil {
		handleError(c, api.ErrInternalServer("Failed to process request body"))
		return
	}

	bodyMap["thinking"] = map[string]string{
		"type": "enabled",
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

	// Copy response headers (except Content-Type which we already set)
	for key, values := range resp.Header {
		if key != "Content-Type" {
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
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
