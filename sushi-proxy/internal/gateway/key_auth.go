package gateway

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

type KeyAuthPlugin struct {
	config map[string]interface{}
}

func NewKeyAuthPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_KEY_AUTH,
		Priority: 200,
		Handler: &KeyAuthPlugin{
			config: config,
		},
		Validator: KeyAuthPlugin{
			config: config,
		},
	}
}

func (plugin KeyAuthPlugin) Validate() error {
	// Key can now come from consumers, so only validate if specified in config
	if key, ok := plugin.config["key"].(string); ok && key != "" {
		return nil
	}
	// Check if we have consumers configured (will be validated at runtime)
	return nil
}

func (plugin KeyAuthPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Executing key_auth function...")

		apiKey, err := extractAPIKey(r, plugin.config)
		if err != nil {
			err.WriteJSONResponse(w)
			return
		}

		consumerName, err := plugin.validateAPIKey(apiKey)
		if err != nil {
			err.WriteJSONResponse(w)
			return
		}

		// Strip header based on config
		if hideCredentials, ok := plugin.config["hide_credentials"].(bool); !ok || hideCredentials {
			r.Header.Del("X-API-Key")
			r.Header.Del("apiKey")
		}

		// Set consumer ID in context
		ctx := context.WithValue(r.Context(), constant.CONTEXT_CONSUMER_ID, consumerName)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (plugin KeyAuthPlugin) validateAPIKey(apiKey string) (string, *model.HttpError) {
	// First, try to find the key in consumers section (Kong-style)
	proxyConfig := GetGlobalProxyConfig()
	if proxyConfig != nil {
		for _, consumer := range proxyConfig.Consumers {
			for _, cred := range consumer.KeyAuthCredentials {
				if cred.Key == apiKey {
					slog.Info("API key validated from consumer", "consumer", consumer.Username)
					return consumer.Username, nil
				}
			}
		}
	}

	// Fallback to plugin config (backwards compatibility)
	config := plugin.config
	if key, ok := config["key"].(string); ok && key == apiKey {
		return "config-key", nil // Generic identity for config-based auth
	}

	return "", model.NewHttpError(http.StatusUnauthorized,
		"INVALID_CREDENTIALS", "Invalid credentials.")
}

func extractAPIKey(r *http.Request, config map[string]interface{}) (string, *model.HttpError) {
	// Get key names from config, default to X-API-Key and apiKey
	keyNames := []string{"X-API-Key", "apiKey"}
	if names, ok := config["key_names"].([]interface{}); ok {
		keyNames = make([]string, 0, len(names))
		for _, name := range names {
			if s, ok := name.(string); ok {
				keyNames = append(keyNames, s)
			}
		}
	}

	// Check query params unless disabled
	keyInQuery := true
	if v, ok := config["key_in_query"].(bool); ok {
		keyInQuery = v
	}
	if keyInQuery {
		for _, keyName := range keyNames {
			if apiKey := r.URL.Query().Get(keyName); apiKey != "" {
				return apiKey, nil
			}
		}
	}

	// Check headers unless disabled
	keyInHeader := true
	if v, ok := config["key_in_header"].(bool); ok {
		keyInHeader = v
	}
	if keyInHeader {
		for _, keyName := range keyNames {
			if apiKey := r.Header.Get(keyName); apiKey != "" {
				return apiKey, nil
			}
		}
	}

	return "", model.NewHttpError(http.StatusUnauthorized,
		"MISSING_API_KEY", "API key is missing.")
}
