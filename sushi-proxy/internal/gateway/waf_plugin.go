package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// SQLi patterns (simplified)
var sqliPattern = regexp.MustCompile(`(?i)(union\s+select|select\s+.*\s+from|insert\s+into|delete\s+from|update\s+.*\s+set|drop\s+table|exec(\s|\+)+(s|x)p\w+|--|1=1)`)

// XSS patterns (simplified)
var xssPattern = regexp.MustCompile(`(?i)(<script|javascript:|on\w+\s*=)`)

type WAFConfig struct {
	ObservationMode bool `json:"observation_mode"` // Log only, don't block
	BlockSQLi       bool `json:"block_sqli"`
	BlockXSS        bool `json:"block_xss"`
}

type WAFPlugin struct {
	Config WAFConfig
}

func NewWAFPlugin(config map[string]interface{}) *Plugin {
	// Parse config
	wafConfig := WAFConfig{
		ObservationMode: false,
		BlockSQLi:       true, // Default to true for security
		BlockXSS:        true, // Default to true
	}

	if v, ok := config["observation_mode"].(bool); ok {
		wafConfig.ObservationMode = v
	}
	if v, ok := config["block_sqli"].(bool); ok {
		wafConfig.BlockSQLi = v
	}
	if v, ok := config["block_xss"].(bool); ok {
		wafConfig.BlockXSS = v
	}

	return &Plugin{
		Name:     constant.PLUGIN_WAF,
		Priority: 1000,
		Handler: &WAFPlugin{
			Config: wafConfig,
		},
		Validator: &WAFPlugin{},
	}
}

func (p *WAFPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Executing WAF plugin...", "config", p.Config)

		blocked := false
		reason := ""

		// Check Query Params
		queryParams := r.URL.Query()
		for key, values := range queryParams {
			for _, v := range values {
				if p.Config.BlockSQLi && sqliPattern.MatchString(v) {
					blocked, reason = true, fmt.Sprintf("SQLi in query param '%s'", key)
					break
				}
				if p.Config.BlockXSS && xssPattern.MatchString(v) {
					blocked, reason = true, fmt.Sprintf("XSS in query param '%s'", key)
					break
				}
			}
			if blocked {
				break
			}
		}

		// Check Headers
		if !blocked {
			for key, values := range r.Header {
				for _, v := range values {
					if p.Config.BlockSQLi && sqliPattern.MatchString(v) {
						blocked, reason = true, fmt.Sprintf("SQLi in header '%s'", key)
						break
					}
					if p.Config.BlockXSS && xssPattern.MatchString(v) {
						blocked, reason = true, fmt.Sprintf("XSS in header '%s'", key)
						break
					}
				}
				if blocked {
					break
				}
			}
		}

		if blocked {
			slog.Warn("WAF detected malicious payload", "reason", reason, "observation_mode", p.Config.ObservationMode)
			if !p.Config.ObservationMode {
				WriteErrorResponse(w, http.StatusBadRequest, "MALICIOUS_PAYLOAD", "Request blocked by WAF: "+reason)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (p *WAFPlugin) Validate() error {
	return nil
}

func WriteErrorResponse(w http.ResponseWriter, status int, code, message string) {
	err := &model.HttpError{
		Code:     code,
		Message:  message,
		HttpCode: status,
	}
	err.WriteJSONResponse(w)
}
