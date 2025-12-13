package api

// ChatRequest represents an incoming chat completion request
type ChatRequest struct {
	Model    string         `binding:"required"            json:"model"`
	Messages []Message      `binding:"required,min=1,dive" json:"messages"`
	Stream   *bool          `json:"stream,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
}

// Message represents a single chat message
type Message struct {
	Role    string `binding:"required,oneof=system user assistant tool" json:"role"`
	Content any    `json:"content"` // string or []ContentPart for vision models
}

// ContentPart represents a part of a message content (for vision models)
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL for vision models
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", or "high"
}

// ShowRequest for /api/show endpoint
type ShowRequest struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// ShowResponse for /api/show endpoint
type ShowResponse struct {
	Template     string         `json:"template"`
	Capabilities []string       `json:"capabilities"`
	Details      ModelDetails   `json:"details"`
	ModelInfo    map[string]any `json:"model_info"`
}

// ModelDetails contains model metadata
type ModelDetails struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}
