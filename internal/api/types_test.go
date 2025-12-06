package api

import (
	"encoding/json"
	"testing"
)

func TestStatusError_Error(t *testing.T) {
	err := &StatusError{
		StatusCode:   404,
		ErrorMessage: "not found",
	}
	if err.Error() != "not found" {
		t.Errorf("Expected 'not found', got '%s'", err.Error())
	}
}

// Note: Test for validation tags would typically require running the validator,
// which is usually done by Gin binding buffer. Since we can't easily import validation engine here
// without Gin context or explicit validator, we'll rely on handler tests for validation logic.

func TestShowResponse_JSON(t *testing.T) {
	resp := ShowResponse{
		Template: "test",
		Details: ModelDetails{
			Family: "glm",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	// Basic sanity check
	if string(data) == "" {
		t.Error("Empty JSON output")
	}
}
