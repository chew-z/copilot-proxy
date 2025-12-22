package models

import "strings"

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
			Name:         "GLM-4.7",
			Model:        "glm-4.7",
			ModifiedAt:   "2025-01-01T00:00:00Z",
			Size:         0,
			Digest:       "glm-4.7",
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
			Name:         "GLM-4.6",
			Model:        "glm-4.6",
			ModifiedAt:   "2024-01-01T00:00:00Z",
			Size:         0,
			Digest:       "glm-4.6",
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
			Model:        "glm-4.5",
			ModifiedAt:   "2024-01-01T00:00:00Z",
			Size:         0,
			Digest:       "glm-4.5",
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
			Model:        "glm-4.5-air",
			ModifiedAt:   "2024-01-01T00:00:00Z",
			Size:         0,
			Digest:       "glm-4.5-air",
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

// IsValidModel checks if a model name exists in the catalog (case-insensitive)
func IsValidModel(name string) bool {
	for _, m := range Catalog.Models {
		if strings.EqualFold(m.Name, name) || strings.EqualFold(m.Model, name) {
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

// GetCanonicalModelName returns the canonical (lowercase) model name for any input
// This ensures the proxy sends the correct lowercase model name to the upstream API
func GetCanonicalModelName(name string) string {
	for _, m := range Catalog.Models {
		if strings.EqualFold(m.Name, name) || strings.EqualFold(m.Model, name) {
			return m.Model // Return the lowercase Model field
		}
	}
	return name // Return original if not found (shouldn't happen after validation)
}
