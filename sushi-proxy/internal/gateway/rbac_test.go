package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRBACPlugin_Execute(t *testing.T) {
	// Setup temporary RBAC config files
	tempDir, err := os.MkdirTemp("", "rbac_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	modelPath := filepath.Join(tempDir, "model.conf")
	policyPath := filepath.Join(tempDir, "policy.csv")

	modelContent := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)
`
	policyContent := `
p, admin, /*, (GET)|(POST)
p, user, /api/public, GET
g, alice, admin
g, bob, user
`
	err = os.WriteFile(modelPath, []byte(modelContent), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(policyPath, []byte(policyContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"model_path":  modelPath,
		"policy_path": policyPath,
	}

	plugin := NewRBACPlugin(config)

	tests := []struct {
		name           string
		user           string
		path           string
		method         string
		expectedStatus int
	}{
		{
			name:           "Alice (admin) access root",
			user:           "alice",
			path:           "/",
			method:         "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Alice (admin) access any",
			user:           "alice",
			path:           "/anything",
			method:         "POST",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Bob (user) access public",
			user:           "bob",
			path:           "/api/public",
			method:         "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Bob (user) denied private",
			user:           "bob",
			path:           "/api/private",
			method:         "GET",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Bob (user) denied POST",
			user:           "bob",
			path:           "/api/public",
			method:         "POST",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Anonymous denied",
			user:           "",
			path:           "/api/public",
			method:         "GET",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)

			// Inject user into context (mocking Authentication plugin)
			if tt.user != "" {
				ctx := context.WithValue(req.Context(), "username", tt.user)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			plugin.Handler.Execute(successHandler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}
