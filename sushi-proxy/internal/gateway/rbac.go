package gateway

import (
	"log/slog"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

type RBACPlugin struct {
	config   map[string]interface{}
	enforcer *casbin.Enforcer
}

func NewRBACPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_RBAC,
		Priority: 900, // Execute after Authentication (JWT/BasicAuth) but before RateLimit/CircuitBreaker
		Handler: RBACPlugin{
			config: config,
		},
		Validator: RBACPlugin{
			config: config,
		},
	}
}

func (plugin RBACPlugin) Validate() error {
	// We can validate config here if needed, e.g. custom policy paths
	return nil
}

func (plugin RBACPlugin) Execute(next http.Handler) http.Handler {
	// Initialize Enforcer lazily or on first request if not already done?
	// Ideally, it should be initialized once.
	// Since Plugin.Handler is an interface, we can't easily store state in 'plugin' if it's passed by value receiver.
	// However, the PluginManager stores *Plugin, and Handler is an interface. Use a pointer receiver for 'Handler' if we want state.
	// But the existing design uses value receivers for Execute in other plugins (e.g. AclPlugin).
	// Let's check how other stateful plugins (like CircuitBreaker) work.
	// CircuitBreaker uses a global map.
	// RateLimit uses a global map (store).

	// For Casbin, initializing the enforcer is expensive (file IO).
	// We should use a singleton or global enforcer per service/config.
	// Or we can rely on `getOrCreateEnforcer`.

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enforcer, err := getOrCreateEnforcer(plugin.config)
		if err != nil {
			slog.Error("Failed to initialize RBAC enforcer", "error", err)
			model.NewHttpError(http.StatusInternalServerError, "RBAC_INIT_ERROR", "Internal Server Error").WriteJSONResponse(w)
			return
		}

		// 1. Get Subject (User/Role)
		// Assuming previous Auth plugins (JWT, BasicAuth) have put the user/role in the context.
		// If not, default to "anonymous".
		sub := "anonymous"

		// Check for JWT claims first (common pattern)
		if claims, ok := r.Context().Value("jwt_claims").(map[string]interface{}); ok {
			// Try to get 'sub' or 'role' or 'username'
			if s, ok := claims["sub"].(string); ok {
				sub = s
			} else if u, ok := claims["username"].(string); ok {
				sub = u
			} else if r, ok := claims["role"].(string); ok {
				sub = r
			}
		} else if username, ok := r.Context().Value("username").(string); ok {
			// Basic Auth
			sub = username
		}

		// 2. Get Object (Resource path)
		obj := r.URL.Path

		// 3. Get Action (HTTP Method)
		act := r.Method

		slog.Debug("Enforcing RBAC", "sub", sub, "obj", obj, "act", act)

		// Enforce
		allowed, err := enforcer.Enforce(sub, obj, act)
		if err != nil {
			slog.Error("RBAC enforcement error", "error", err)
			model.NewHttpError(http.StatusInternalServerError, "RBAC_ERROR", "Internal Server Error").WriteJSONResponse(w)
			return
		}

		if allowed {
			next.ServeHTTP(w, r)
		} else {
			slog.Warn("RBAC access denied", "sub", sub, "obj", obj, "act", act)
			model.NewHttpError(http.StatusForbidden, "ACCESS_DENIED", "Access Denied").WriteJSONResponse(w)
		}
	})
}

// Global enforcer cache to avoid reloading files on every request
// Keyed by config hash or just a single global one if config is static per instance
// Since config can vary per route/service, we might need a map.
// For simplicity, let's assume standard paths or config-provided paths.
var enforcerCache = make(map[string]*casbin.Enforcer)

func getOrCreateEnforcer(config map[string]interface{}) (*casbin.Enforcer, error) {
	// Determine paths
	modelPath := "config/rbac/model.conf"
	policyPath := "config/rbac/policy.csv"

	if m, ok := config["model_path"].(string); ok {
		modelPath = m
	}
	if p, ok := config["policy_path"].(string); ok {
		policyPath = p
	}

	key := modelPath + "|" + policyPath
	if e, ok := enforcerCache[key]; ok {
		return e, nil
	}

	// Create new enforcer
	e, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		return nil, err
	}

	enforcerCache[key] = e
	return e, nil
}
