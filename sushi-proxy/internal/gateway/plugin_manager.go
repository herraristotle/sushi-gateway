package gateway

import (
	"net/http"
	"sort"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
)

type PluginManager struct {
	plugins []*Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make([]*Plugin, 0),
	}
}

func NewPluginManagerFromConfig(req *http.Request) (*PluginManager, *model.HttpError) {
	// Load the plugin configuration from the gateway file
	// Based on the plugins loaded from http request
	// Order of precedence: route.plugins > service.plugins > global.plugins
	pm := NewPluginManager()

	globalConfig := GetGlobalProxyConfig()
	globalPlugins := globalConfig.Plugins
	for _, pluginConfig := range globalPlugins {
		err := pm.loadConfig(pluginConfig)
		if err != nil {
			return nil, err
		}
	}

	// Search for service.plugins and route.plugins
	service, route, err := util.GetServiceAndRouteFromRequest(GetGlobalProxyConfig(), req)
	if err != nil {
		return nil, err
	}

	for _, pluginConfig := range route.Plugins {
		err := pm.loadConfig(pluginConfig)
		if err != nil {
			return nil, err
		}
	}

	for _, pluginConfig := range service.Plugins {
		err := pm.loadConfig(pluginConfig)
		if err != nil {
			return nil, err
		}
	}

	return pm, nil
}

// Load the plugin configuration from the gateway file
func (pm *PluginManager) loadConfig(pc model.PluginConfig) *model.HttpError {
	// If enabled is specified and false, skip. If nil, assume enabled (Kong style).
	if pc.Enabled != nil && !*pc.Enabled {
		return nil
	}

	switch pc.Name {
	case constant.PLUGIN_BASIC_AUTH:
		pm.RegisterPlugin(NewBasicAuthPlugin(pc.Config))
	case constant.PLUGIN_ACL:
		pm.RegisterPlugin(NewAclPlugin(pc.Config))
	case constant.PLUGIN_BOT_PROTECTION:
		pm.RegisterPlugin(NewBotProtectionPlugin(pc.Config))
	case constant.PLUGIN_KEY_AUTH:
		pm.RegisterPlugin(NewKeyAuthPlugin(pc.Config))
	case constant.PLUGIN_RATE_LIMIT:
		pm.RegisterPlugin(NewRateLimitPlugin(pc.Config, GetGlobalProxyConfig()))
	case constant.PLUGIN_REQUEST_SIZE_LIMIT:
		pm.RegisterPlugin(NewRequestSizeLimitPlugin(pc.Config))
	case constant.PLUGIN_JWT:
		pm.RegisterPlugin(NewJwtPlugin(pc.Config))
	case constant.PLUGIN_MTLS:
		pm.RegisterPlugin(NewMtlsPlugin(pc.Config))
	case constant.PLUGIN_HTTP_LOG:
		pm.RegisterPlugin(NewHttpLogPlugin(pc.Config))
	case constant.PLUGIN_CORS:
		pm.RegisterPlugin(NewCorsPlugin(pc.Config))
	case constant.PLUGIN_HEADER_TRANSFORMATION:
		pm.RegisterPlugin(NewHeaderTransformationPlugin(pc.Config))
	case constant.PLUGIN_CIRCUIT_BREAKER:
		pm.RegisterPlugin(NewCircuitBreakerPlugin(pc.Config))
	case constant.PLUGIN_RBAC:
		pm.RegisterPlugin(NewRBACPlugin(pc.Config))
	case constant.PLUGIN_SANITIZATION:
		pm.RegisterPlugin(NewSanitizationPlugin(pc.Config))
	case constant.PLUGIN_CACHE:
		pm.RegisterPlugin(NewCachePlugin(pc.Config))
	case constant.PLUGIN_SHADOW_TRAFFIC:
		pm.RegisterPlugin(NewShadowTrafficPlugin(pc.Config))
	case constant.PLUGIN_MULTI_AUTH:
		pm.RegisterPlugin(NewMultiAuthPlugin(pc.Config))
	case constant.PLUGIN_PROMETHEUS:
		EnableMetrics(pc.Config)
	case constant.PLUGIN_OPENTELEMETRY:
		EnableTracing(pc.Config)
	}
	return nil
}

func (pm *PluginManager) RegisterPlugin(plugin *Plugin) {
	// If plugins already exists replace
	exists := false
	for i, p := range pm.plugins {
		if p.Name == plugin.Name {
			pm.plugins[i] = plugin
			exists = true
			break
		}
	}

	if !exists {
		pm.plugins = append(pm.plugins, plugin)
	}

	// Sort the plugins by priority, higher priority executes first
	sort.Slice(pm.plugins, func(i, j int) bool {
		return pm.plugins[i].Priority < pm.plugins[j].Priority
	})
}

// ExecutePlugins chains the plugins and returns a single http.Handler
// finalHandler is the application's main handler that should execute after all plugins
func (pm *PluginManager) ExecutePlugins(finalHandler http.Handler) http.Handler {
	// Apply plugins in reverse order of priority (so high priority wraps first and runs first)
	// wait, if we iterate slice normally, we are wrapping:
	// H' = P1.Execute(H) -> P1 calls H
	// H'' = P2.Execute(H') -> P2 calls P1 calls H
	// So if P2 should run first, it should be applied LAST if we are building the onion out-to-in?
	// The Sort in RegisterPlugin is `plugins[i].Priority < plugins[j].Priority`.
	// Lower value index = Lower priority value? Or implies earlier execution?
	// Usually Priority 0 = High Priority (Runs First).
	// If P(0) should run first, it is the OUTERMOST wrapper.
	// H_final = P(0) -> P(1) -> ... -> MainHandler
	// To build this:
	// current = MainHandler
	// current = P(N).Execute(current)
	// ...
	// current = P(0).Execute(current)

	// So we need to iterate from Last-Priority (Highest numeric value?) to First-Priority (Lowest numeric value 0)?
	// Or rather, we need to apply the LOWEST priority execution (Inner-most) first?
	// No.
	// Let's assume Priority 0 is HIGHEST priority (runs first).
	// We want P0(P1(P2(Handler))).
	// So we start with Handler.
	// Wrap with P2.
	// Wrap with P1.
	// Wrap with P0.
	// The `pm.plugins` are sorted `i.Priority < j.Priority`.
	// So index 0 has Priority 0 (Highest). Index N has Priority 100 (Lowest).
	// We should iterate in REVERSE order of the sorted slice to build the chain correctly.

	// Iterate backwards through the sorted plugins
	currentHandler := finalHandler
	for i := len(pm.plugins) - 1; i >= 0; i-- {
		plugin := pm.plugins[i]
		currentHandler = plugin.Handler.Execute(currentHandler)
	}
	return currentHandler
}

func (pm *PluginManager) GetPlugins() []*Plugin {
	return pm.plugins
}
