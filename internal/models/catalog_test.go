package models

import "testing"

// TestIsValidModel tests the IsValidModel function
func TestIsValidModel(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"GLM-4.7", true},
		{"GLM-4.7-Flash", true},
		{"GLM-4.7-FlashX", true},
		{"glm-4.7", true},        // case insensitive
		{"glm-4.7-flash", true},  // case insensitive
		{"glm-4.7-flashx", true}, // case insensitive
		{"unknown-model", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidModel(tt.name)
			if result != tt.expected {
				t.Errorf("IsValidModel(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

// TestGetModelContextLength tests the GetModelContextLength function
func TestGetModelContextLength(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{"GLM-4.7", 200000},
		{"GLM-4.7-Flash", 200000},
		{"GLM-4.7-FlashX", 200000},
		{"unknown-model", 128000}, // default
		{"", 128000},              // default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelContextLength(tt.name)
			if result != tt.expected {
				t.Errorf("GetModelContextLength(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

// TestGetModel tests the GetModel function
func TestGetModel(t *testing.T) {
	// Test existing model
	model, found := GetModel("GLM-4.7-Flash")
	if !found {
		t.Error("Expected to find GLM-4.7-Flash model")
	}
	if model.Name != "GLM-4.7-Flash" {
		t.Errorf("Expected model name GLM-4.7-Flash, got %s", model.Name)
	}
	if model.ContextLen != 200000 {
		t.Errorf("Expected context length 200000, got %d", model.ContextLen)
	}

	// Test non-existing model
	_, found = GetModel("unknown-model")
	if found {
		t.Error("Expected not to find unknown-model")
	}
}

// TestGetCanonicalModelName tests the GetCanonicalModelName function
func TestGetCanonicalModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Uppercase GLM-4.7", "GLM-4.7", "glm-4.7"},
		{"Lowercase glm-4.7", "glm-4.7", "glm-4.7"},
		{"Mixed case GLm-4.7", "GLm-4.7", "glm-4.7"},
		{"Uppercase GLM-4.7-Flash", "GLM-4.7-Flash", "glm-4.7-flash"},
		{"Lowercase glm-4.7-flash", "glm-4.7-flash", "glm-4.7-flash"},
		{"Uppercase GLM-4.7-FlashX", "GLM-4.7-FlashX", "glm-4.7-flashx"},
		{"Lowercase glm-4.7-flashx", "glm-4.7-flashx", "glm-4.7-flashx"},
		{"Unknown model (returns as-is)", "unknown-model", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCanonicalModelName(tt.input)
			if result != tt.expected {
				t.Errorf("GetCanonicalModelName(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
