package gateway

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
)

// Multi-Auth Plugin - Tries multiple authentication strategies in order
// If one succeeds, the request proceeds. If all fail, returns the last error.
//
// Configuration:
//   strategies: ["jwt", "key_auth", "basic_auth"]
//   jwt: { alg: "HS256", iss: "my-issuer", secret: "..." }
//   key_auth: { key: "my-api-key" }
//   basic_auth: { username: "admin", password: "secret" }

var (
	MultiAuthAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sushi_gateway_multi_auth_attempts_total",
			Help: "Total multi-auth attempts by strategy and result",
		},
		[]string{"strategy", "result"}, // result: "success", "failure", "skipped"
	)
)

type MultiAuthPlugin struct {
	config     map[string]interface{}
	strategies []string
}

func NewMultiAuthPlugin(config map[string]interface{}) *Plugin {
	plugin := &MultiAuthPlugin{
		config:     config,
		strategies: []string{"jwt", "key_auth", "basic_auth"}, // Default order
	}

	// Parse strategies from config
	if strategiesRaw, ok := config["strategies"].([]interface{}); ok {
		plugin.strategies = make([]string, 0, len(strategiesRaw))
		for _, s := range strategiesRaw {
			if str, ok := s.(string); ok {
				plugin.strategies = append(plugin.strategies, str)
			}
		}
	}

	return &Plugin{
		Name:      constant.PLUGIN_MULTI_AUTH,
		Priority:  199, // Run just before individual auth plugins
		Handler:   plugin,
		Validator: plugin,
	}
}

func (p *MultiAuthPlugin) Validate() error {
	// At least one strategy must be configured
	if len(p.strategies) == 0 {
		return nil // Will use defaults
	}
	return nil
}

// authProbeWriter captures whether the handler wrote an error response
type authProbeWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	written    bool
}

func (w *authProbeWriter) WriteHeader(code int) {
	w.statusCode = code
	w.written = true
}

func (w *authProbeWriter) Write(data []byte) (int, error) {
	w.written = true
	return w.body.Write(data)
}

func (w *authProbeWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (p *MultiAuthPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Executing multi_auth plugin...", "strategies", p.strategies)

		var lastProbe *authProbeWriter

		for _, strategy := range p.strategies {
			// Create the auth plugin for this strategy
			authPlugin := p.createAuthPlugin(strategy)
			if authPlugin == nil {
				slog.Warn("Unknown auth strategy in multi_auth", "strategy", strategy)
				MultiAuthAttemptsTotal.WithLabelValues(strategy, "skipped").Inc()
				continue
			}

			// Create a probe writer to capture if auth writes an error
			probe := &authProbeWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Create a "success detector" next handler
			// If auth succeeds, it will call this handler
			authSucceeded := false
			successDetector := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authSucceeded = true
			})

			// Execute the auth plugin
			authHandler := authPlugin.Handler.Execute(successDetector)
			authHandler.ServeHTTP(probe, r)

			if authSucceeded {
				// Auth passed! Record success and continue to real next handler
				slog.Info("Multi-auth succeeded", "strategy", strategy)
				MultiAuthAttemptsTotal.WithLabelValues(strategy, "success").Inc()
				next.ServeHTTP(w, r)
				return
			}

			// Auth failed, record and try next
			slog.Debug("Multi-auth strategy failed, trying next", "strategy", strategy)
			MultiAuthAttemptsTotal.WithLabelValues(strategy, "failure").Inc()
			lastProbe = probe
		}

		// All strategies failed - write the last error response
		slog.Info("All multi-auth strategies failed")
		if lastProbe != nil && lastProbe.written {
			if lastProbe.statusCode != 0 {
				w.WriteHeader(lastProbe.statusCode)
			}
			w.Write(lastProbe.body.Bytes())
		} else {
			// Fallback error
			http.Error(w, `{"code":"AUTHENTICATION_FAILED","message":"All authentication strategies failed"}`, http.StatusUnauthorized)
		}
	})
}

func (p *MultiAuthPlugin) createAuthPlugin(strategy string) *Plugin {
	// Get strategy-specific config
	strategyConfig, ok := p.config[strategy].(map[string]interface{})
	if !ok {
		strategyConfig = make(map[string]interface{})
	}

	switch strategy {
	case "jwt":
		return NewJwtPlugin(strategyConfig)
	case "key_auth":
		return NewKeyAuthPlugin(strategyConfig)
	case "basic_auth":
		return NewBasicAuthPlugin(strategyConfig)
	default:
		return nil
	}
}
