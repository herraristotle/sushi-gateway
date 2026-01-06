package gateway

import (
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

type SanitizationPlugin struct {
	Config map[string]interface{}
}

func NewSanitizationPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_SANITIZATION,
		Priority: 1000, // High priority, run early
		Handler: &SanitizationPlugin{
			Config: config,
		},
		Validator: &SanitizationPlugin{},
	}
}

func (p *SanitizationPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Executing sanitization plugin...")

		// Check Query Params
		queryParams := r.URL.Query()
		for key, values := range queryParams {
			for _, v := range values {
				if isMalicious(v) {
					slog.Warn("Malicious payload detected in query param", "key", key, "value", v)
					WriteErrorResponse(w, http.StatusBadRequest, "MALICIOUS_PAYLOAD", "Request blocked due to malicious payload detection")
					return
				}
			}
		}

		// Check Headers (Configurable? For now check all except safe ones)
		for key, values := range r.Header {
			// Skip cookies or auth headers if sensitive? Maybe not, they can carry payloads too.
			// But XSS in Cookie is less likely to be reflected immediately without other vectors.
			// Let's check everything for now.
			for _, v := range values {
				if isMalicious(v) {
					slog.Warn("Malicious payload detected in header", "key", key, "value", v)
					WriteErrorResponse(w, http.StatusBadRequest, "MALICIOUS_PAYLOAD", "Request blocked due to malicious payload detection")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (p *SanitizationPlugin) Validate() error {
	// No config validation needed for now
	return nil
}

func isMalicious(input string) bool {
	return sqliPattern.MatchString(input) || xssPattern.MatchString(input)
}

func WriteErrorResponse(w http.ResponseWriter, status int, code, message string) {
	err := &model.HttpError{
		Code:     code,
		Message:  message,
		HttpCode: status,
	}
	err.WriteJSONResponse(w)
}
