package models

import "testing"

// TestIsValidModel tests the IsValidModel function
func TestIsValidModel(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"GLM-4.6", true},
		{"GLM-4.5", true},
		{"GLM-4.5-Air", true},
		{"glm-4.6", true}, // case insensitive
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
		{"GLM-4.6", 200000},
		{"GLM-4.5", 128000},
		{"GLM-4.5-Air", 128000},
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
	model, found := GetModel("GLM-4.6")
	if !found {
		t.Error("Expected to find GLM-4.6 model")
	}
	if model.Name != "GLM-4.6" {
		t.Errorf("Expected model name GLM-4.6, got %s", model.Name)
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
