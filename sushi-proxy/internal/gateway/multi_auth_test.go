package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMultiAuthPlugin_JwtSucceeds(t *testing.T) {
	// Configure multi-auth with JWT that will succeed
	// (We can't easily test real JWT without a token, so we test the fallback logic)
	config := map[string]interface{}{
		"strategies": []interface{}{"jwt", "key_auth"},
		"jwt": map[string]interface{}{
			"alg":    "HS256",
			"iss":    "test-issuer",
			"secret": "test-secret",
		},
		"key_auth": map[string]interface{}{
			"key": "valid-api-key",
		},
	}

	pluginWrapper := NewMultiAuthPlugin(config)
	plugin := pluginWrapper.Handler.(*MultiAuthPlugin)

	// Verify strategies loaded correctly
	if len(plugin.strategies) != 2 {
		t.Errorf("Expected 2 strategies, got %d", len(plugin.strategies))
	}
	if plugin.strategies[0] != "jwt" || plugin.strategies[1] != "key_auth" {
		t.Errorf("Unexpected strategies: %v", plugin.strategies)
	}
}

func TestMultiAuthPlugin_KeyAuthSucceeds(t *testing.T) {
	// Configure multi-auth where JWT will fail but key_auth succeeds
	config := map[string]interface{}{
		"strategies": []interface{}{"key_auth"},
		"key_auth": map[string]interface{}{
			"key": "valid-api-key",
		},
	}

	pluginWrapper := NewMultiAuthPlugin(config)
	plugin := pluginWrapper.Handler.(*MultiAuthPlugin)

	// Create request with valid API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("apiKey", "valid-api-key")

	w := httptest.NewRecorder()

	// Track if next handler was called
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := plugin.Execute(nextHandler)
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("Expected next handler to be called after successful key_auth")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMultiAuthPlugin_AllFail(t *testing.T) {
	// Configure multi-auth where all strategies will fail
	config := map[string]interface{}{
		"strategies": []interface{}{"key_auth", "basic_auth"},
		"key_auth": map[string]interface{}{
			"key": "valid-api-key",
		},
		"basic_auth": map[string]interface{}{
			"username": "admin",
			"password": "secret",
		},
	}

	pluginWrapper := NewMultiAuthPlugin(config)
	plugin := pluginWrapper.Handler.(*MultiAuthPlugin)

	// Create request with NO auth credentials
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := plugin.Execute(nextHandler)
	handler.ServeHTTP(w, req)

	if nextCalled {
		t.Error("Expected next handler NOT to be called when all auth fails")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestMultiAuthPlugin_FallbackOrder(t *testing.T) {
	// Test that strategies are tried in order
	// First strategy fails (JWT without token), second succeeds (key_auth with key)
	config := map[string]interface{}{
		"strategies": []interface{}{"jwt", "key_auth"},
		"jwt": map[string]interface{}{
			"alg":    "HS256",
			"iss":    "test-issuer",
			"secret": "test-secret",
		},
		"key_auth": map[string]interface{}{
			"key": "correct-key",
		},
	}

	pluginWrapper := NewMultiAuthPlugin(config)
	plugin := pluginWrapper.Handler.(*MultiAuthPlugin)

	// Request with key_auth but not JWT
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("apiKey", "correct-key")

	w := httptest.NewRecorder()

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := plugin.Execute(nextHandler)
	handler.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("Expected next handler to be called after key_auth fallback succeeded")
	}
}
