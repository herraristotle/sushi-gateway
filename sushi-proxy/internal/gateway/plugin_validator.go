package gateway

import (
	"fmt"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
)

type PluginValidator struct {
}

func NewPluginValidator() *PluginValidator {
	return &PluginValidator{}
}

func (pv *PluginValidator) ValidatePlugin(plugin model.PluginConfig) error {

	if err := pv.validateNameExists(plugin); err != nil {
		return err
	}
	if err := pv.validateAvailablePlugin(plugin); err != nil {
		return err
	}

	// Validate individual plugin configurations.
	individualPlugin := pv.createPluginFromConfig(plugin)

	// Only check if the plugin has a validator installed.
	if individualPlugin.Validator != nil {
		if err := individualPlugin.Validator.Validate(); err != nil {
			return err
		}
	}

	// Pass validation
	return nil
}

func (pv *PluginValidator) createPluginFromConfig(plugin model.PluginConfig) *Plugin {
	switch plugin.Name {
	case constant.PLUGIN_BASIC_AUTH:
		return NewBasicAuthPlugin(plugin.Config)
	case constant.PLUGIN_ACL:
		return NewAclPlugin(plugin.Config)
	case constant.PLUGIN_BOT_PROTECTION:
		return NewBotProtectionPlugin(plugin.Config)
	case constant.PLUGIN_RATE_LIMIT:
		return NewRateLimitPlugin(plugin.Config, nil)
	case constant.PLUGIN_REQUEST_SIZE_LIMIT:
		return NewRequestSizeLimitPlugin(plugin.Config)
	case constant.PLUGIN_JWT:
		return NewJwtPlugin(plugin.Config)
	case constant.PLUGIN_KEY_AUTH:
		return NewKeyAuthPlugin(plugin.Config)
	case constant.PLUGIN_MTLS:
		return NewMtlsPlugin(plugin.Config)
	case constant.PLUGIN_HTTP_LOG:
		return NewHttpLogPlugin(plugin.Config)
	case constant.PLUGIN_CORS:
		return NewCorsPlugin(plugin.Config)
	case constant.PLUGIN_HEADER_TRANSFORMATION:
		return NewHeaderTransformationPlugin(plugin.Config)
	case constant.PLUGIN_CIRCUIT_BREAKER:
		return NewCircuitBreakerPlugin(plugin.Config)
	case constant.PLUGIN_RBAC:
		return NewRBACPlugin(plugin.Config)
	case constant.PLUGIN_WAF, constant.PLUGIN_SANITIZATION:
		return NewWAFPlugin(plugin.Config)
	case constant.PLUGIN_CACHE:
		return NewCachePlugin(plugin.Config)
	case constant.PLUGIN_SHADOW_TRAFFIC:
		return NewShadowTrafficPlugin(plugin.Config)
	case constant.PLUGIN_MULTI_AUTH:
		return NewMultiAuthPlugin(plugin.Config)
	case constant.PLUGIN_REQUEST_TERMINATION:
		return NewRequestTerminationPlugin(plugin.Config)
	case constant.PLUGIN_PROMETHEUS, constant.PLUGIN_OPENTELEMETRY:
		// Observability plugins are validated differently or have simple configs
		return &Plugin{Name: plugin.Name}
	default:
		return &Plugin{Name: plugin.Name}
	}
}

func (pv *PluginValidator) validateNameExists(plugin model.PluginConfig) error {
	if plugin.Name == "" {
		return fmt.Errorf("plugin name is required")
	}

	return nil
}

func (pv *PluginValidator) validateAvailablePlugin(plugin model.PluginConfig) error {
	if !util.SliceContainsString(constant.AVAILABLE_PLUGINS, plugin.Name) {
		return fmt.Errorf("plugin name is invalid. "+
			"Available plugins: %v", constant.AVAILABLE_PLUGINS)
	}
	return nil
}
