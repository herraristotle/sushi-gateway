package gateway

import (
	"log/slog"
	"net/http"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
)

// RequestTerminationPlugin terminates a request with a specific status code and message.
// Useful for:
// - Blocking specific routes
// - Maintenance mode responses
// - Health check endpoints that don't need to hit backend
type RequestTerminationPlugin struct {
	config map[string]interface{}
}

func NewRequestTerminationPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_REQUEST_TERMINATION,
		Priority: 2, // Very high priority - runs early to terminate quickly
		Handler: &RequestTerminationPlugin{
			config: config,
		},
		Validator: RequestTerminationPlugin{
			config: config,
		},
	}
}

func (plugin RequestTerminationPlugin) Validate() error {
	// status_code is optional, defaults to 200
	// message is optional
	// content_type is optional, defaults to application/json
	return nil
}

func (plugin RequestTerminationPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Executing request-termination plugin")

		// Get status code (default 200)
		statusCode := http.StatusOK
		if code, ok := plugin.config["status_code"].(float64); ok {
			statusCode = int(code)
		} else if code, ok := plugin.config["status_code"].(int); ok {
			statusCode = code
		}

		// Get message (default empty)
		message := ""
		if msg, ok := plugin.config["message"].(string); ok {
			message = msg
		}

		// Get content type (default text/plain for simple messages)
		contentType := "text/plain; charset=utf-8"
		if ct, ok := plugin.config["content_type"].(string); ok {
			contentType = ct
		}

		// Get body (can be used instead of message for JSON, etc.)
		body := message
		if b, ok := plugin.config["body"].(string); ok {
			body = b
			// If body looks like JSON, set content type appropriately
			if contentType == "text/plain; charset=utf-8" && len(body) > 0 && (body[0] == '{' || body[0] == '[') {
				contentType = "application/json"
			}
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(statusCode)
		if body != "" {
			w.Write([]byte(body))
		}

		// Do NOT call next.ServeHTTP - request is terminated here
	})
}

func init() {
	// Ensure the plugin is registered
}
