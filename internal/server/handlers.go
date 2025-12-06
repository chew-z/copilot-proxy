package server

import (
	"bytes"
	"io"
	"net/http"

	"github.com/chew-z/copilot-proxy/internal/models"
	"github.com/gin-gonic/gin"
)

// sendError sends a standardized error response
func (s *Server) sendError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"error": message,
	})
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
	// Parse request body to get model name
	// Ollama standard is "name", but reference implementation uses "model"
	var request struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	}

	// We don't strictly require the body to be valid, if it's empty we'll use default
	_ = c.ShouldBindJSON(&request)

	modelName := request.Name
	if modelName == "" {
		modelName = request.Model
	}
	if modelName == "" {
		modelName = "GLM-4.6"
	}

	// Construct response matching the Python reference implementation
	response := gin.H{
		"template":     "{{ .System }}\n{{ .Prompt }}",
		"capabilities": []string{"tools"},
		"details": gin.H{
			"family":             "glm",
			"families":           []string{"glm"},
			"format":             "glm",
			"parameter_size":     "cloud",
			"quantization_level": "cloud",
		},
		"model_info": gin.H{
			"general.basename":     modelName,
			"general.architecture": "glm",
			"glm.context_length":   32768,
		},
	}

	c.JSON(http.StatusOK, response)
}

// handleChatCompletions proxies requests to Z.AI API
func (s *Server) handleChatCompletions(c *gin.Context) {
	// Read the request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.sendError(c, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer c.Request.Body.Close()

	// Create upstream request
	upstreamURL := s.config.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		s.sendError(c, http.StatusInternalServerError, "Failed to create upstream request")
		return
	}

	// Copy Content-Type header
	if contentType := c.GetHeader("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add Authorization header
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	// Execute request
	resp, err := s.client.Do(req)
	if err != nil {
		s.sendError(c, http.StatusBadGateway, "Failed to connect to upstream server")
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

	// Stream response body
	if err := streamResponse(c, resp.Body); err != nil {
		// Client may have disconnected, just log and return
		return
	}
}

// streamResponse streams the response body with SSE support
func streamResponse(c *gin.Context, body io.ReadCloser) error {
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
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
