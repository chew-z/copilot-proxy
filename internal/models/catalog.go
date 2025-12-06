package models

// Catalog is the static model catalog returned by the proxy
var Catalog = map[string]interface{}{
	"models": []map[string]interface{}{
		{
			"name":         "GLM-4.6",
			"model":        "GLM-4.6",
			"modified_at":  "2024-01-01T00:00:00Z",
			"size":         0,
			"digest":       "GLM-4.6",
			"capabilities": []string{"tools", "vision"},
			"details": map[string]interface{}{
				"format":             "glm",
				"family":             "glm",
				"families":           []string{"glm"},
				"parameter_size":     "cloud",
				"quantization_level": "cloud",
			},
		},
		{
			"name":         "GLM-4.5",
			"model":        "GLM-4.5",
			"modified_at":  "2024-01-01T00:00:00Z",
			"size":         0,
			"digest":       "GLM-4.5",
			"capabilities": []string{"tools", "vision"},
			"details": map[string]interface{}{
				"format":             "glm",
				"family":             "glm",
				"families":           []string{"glm"},
				"parameter_size":     "cloud",
				"quantization_level": "cloud",
			},
		},
		{
			"name":         "GLM-4.5-Air",
			"model":        "GLM-4.5-Air",
			"modified_at":  "2024-01-01T00:00:00Z",
			"size":         0,
			"digest":       "GLM-4.5-Air",
			"capabilities": []string{"tools", "vision"},
			"details": map[string]interface{}{
				"format":             "glm",
				"family":             "glm",
				"families":           []string{"glm"},
				"parameter_size":     "cloud",
				"quantization_level": "cloud",
			},
		},
	},
}
