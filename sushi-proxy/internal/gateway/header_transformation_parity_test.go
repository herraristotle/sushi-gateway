package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeaderTransformationPlugin_KongParity(t *testing.T) {
	// CONFIG PARITY TEST
	// Mimic Kong's request-transformer config structure

	config := map[string]interface{}{
		"add": map[string]interface{}{
			"headers": []interface{}{
				"X-Added-Header: added-value",
				"X-Template-Test: $(headers['x-consumer-id'] or '')",
			},
		},
		"remove": map[string]interface{}{
			"headers": []interface{}{
				"X-Remove-Me",
			},
		},
		"replace": map[string]interface{}{
			"headers": []interface{}{
				"X-Replace-Me: new-value",
			},
		},
		"append": map[string]interface{}{
			"headers": []interface{}{
				"X-Append-Me: appended-value",
			},
		},
		"rename": map[string]interface{}{
			"headers": []interface{}{
				"X-Old-Name: X-New-Name",
			},
		},
	}

	plugin := NewHeaderTransformationPlugin(config)

	// Validate configuration
	if err := plugin.Validator.Validate(); err != nil {
		t.Fatalf("Validation failed for Kong-style config: %v", err)
	}

	// Setup Request
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Remove-Me", "should-be-gone")
	req.Header.Set("X-Replace-Me", "old-value")
	req.Header.Set("X-Append-Me", "initial-value")
	req.Header.Set("X-Old-Name", "content")
	req.Header.Set("X-Consumer-ID", "123456") // For template test

	rec := httptest.NewRecorder()

	// Execute Plugin
	handler := plugin.Handler.Execute(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Add
		if val := r.Header.Get("X-Added-Header"); val != "added-value" {
			t.Errorf("Add failed: expected 'added-value', got '%s'", val)
		}

		// Verify Template Substitution
		if val := r.Header.Get("X-Template-Test"); val != "123456" {
			t.Errorf("Template substitution failed: expected '123456', got '%s'", val)
		}

		// Verify Remove
		if val := r.Header.Get("X-Remove-Me"); val != "" {
			t.Errorf("Remove failed: header still exists with value '%s'", val)
		}

		// Verify Replace
		if val := r.Header.Get("X-Replace-Me"); val != "new-value" {
			t.Errorf("Replace failed: expected 'new-value', got '%s'", val)
		}

		// Verify Append
		// Note: H.Set appends depending on client? No, Set overwrites.
		// Sushi Header Transformation Append logic implemented as: existing + ", " + new
		// Let's check logic: newValue := existingValue + ", " + valueStr
		expectedAppend := "initial-value, appended-value"
		if val := r.Header.Get("X-Append-Me"); val != expectedAppend {
			t.Errorf("Append failed: expected '%s', got '%s'", expectedAppend, val)
		}

		// Verify Rename
		if val := r.Header.Get("X-Old-Name"); val != "" {
			t.Errorf("Rename failed: old header still exists")
		}
		if val := r.Header.Get("X-New-Name"); val != "content" {
			t.Errorf("Rename failed: new header not set or wrong value")
		}
	}))

	handler.ServeHTTP(rec, req)
}

func TestHeaderTransformationPlugin_ValidationMixed(t *testing.T) {
	// Verify that mix of Kong style and Sushi style is valid or handled
	// The Validator update was permissive.

	config := map[string]interface{}{
		"remove": []interface{}{"X-Sushi-Style"},
	}
	plugin := NewHeaderTransformationPlugin(config)
	if err := plugin.Validator.Validate(); err != nil {
		t.Errorf("Validation failed for Sushi-style list config: %v", err)
	}
}
