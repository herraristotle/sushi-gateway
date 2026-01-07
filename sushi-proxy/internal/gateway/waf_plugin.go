package gateway

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	coreruleset "github.com/corazawaf/coraza-coreruleset"
	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// Maximum body size to process (1MB)
const maxWAFBodySize = 1024 * 1024

// WAFConfig contains WAF configuration options
type WAFConfig struct {
	// Core settings
	ObservationMode bool   `json:"observation_mode"` // Log only, don't block
	RulesFile       string `json:"rules_file"`       // Additional custom rules file
	RulesDir        string `json:"rules_dir"`        // Additional rules directory

	// Paranoia level for OWASP CRS (1-4, default 1)
	// Level 1: Basic protection, low false positives
	// Level 2: Moderate protection
	// Level 3: High protection
	// Level 4: Maximum protection, may have false positives
	ParanoiaLevel int `json:"paranoia_level"`
}

type WAFPlugin struct {
	Config WAFConfig
}

// Global WAF instance (thread-safe singleton)
var (
	globalWAF     coraza.WAF
	globalWAFOnce sync.Once
	globalWAFErr  error
	globalWAFCfg  WAFConfig
)

func NewWAFPlugin(config map[string]interface{}) *Plugin {
	wafConfig := WAFConfig{
		ObservationMode: false,
		ParanoiaLevel:   1, // Default to low false positives
	}

	if v, ok := config["observation_mode"].(bool); ok {
		wafConfig.ObservationMode = v
	}
	if v, ok := config["rules_file"].(string); ok {
		wafConfig.RulesFile = v
	}
	if v, ok := config["rules_dir"].(string); ok {
		wafConfig.RulesDir = v
	}
	if v, ok := config["paranoia_level"].(float64); ok {
		wafConfig.ParanoiaLevel = int(v)
		if wafConfig.ParanoiaLevel < 1 {
			wafConfig.ParanoiaLevel = 1
		}
		if wafConfig.ParanoiaLevel > 4 {
			wafConfig.ParanoiaLevel = 4
		}
	}

	// Store config for WAF creation
	globalWAFCfg = wafConfig

	return &Plugin{
		Name:      constant.PLUGIN_WAF,
		Priority:  1000, // Run early for security
		Handler:   &WAFPlugin{Config: wafConfig},
		Validator: &WAFPlugin{Config: wafConfig},
	}
}

// getWAF returns the singleton Coraza WAF instance
func (p *WAFPlugin) getWAF() (coraza.WAF, error) {
	globalWAFOnce.Do(func() {
		globalWAF, globalWAFErr = createProductionWAF(globalWAFCfg)
	})
	return globalWAF, globalWAFErr
}

// createProductionWAF initializes Coraza with OWASP CRS
func createProductionWAF(cfg WAFConfig) (coraza.WAF, error) {
	wafCfg := coraza.NewWAFConfig()

	// Use embedded OWASP Core Rule Set
	wafCfg = wafCfg.WithRootFS(coreruleset.FS)

	// Build CRS configuration
	crsSetup := buildCRSSetup(cfg)
	wafCfg = wafCfg.WithDirectives(crsSetup)

	// Include OWASP CRS rules
	wafCfg = wafCfg.WithDirectives(`Include @owasp_crs/*.conf`)

	// Load additional custom rules if specified
	if cfg.RulesFile != "" {
		if _, err := os.Stat(cfg.RulesFile); err == nil {
			wafCfg = wafCfg.WithDirectivesFromFile(cfg.RulesFile)
			slog.Info("WAF: Loaded custom rules", "file", cfg.RulesFile)
		}
	}

	if cfg.RulesDir != "" {
		files, err := filepath.Glob(filepath.Join(cfg.RulesDir, "*.conf"))
		if err == nil {
			for _, f := range files {
				wafCfg = wafCfg.WithDirectivesFromFile(f)
			}
			slog.Info("WAF: Loaded rules directory", "dir", cfg.RulesDir, "count", len(files))
		}
	}

	waf, err := coraza.NewWAF(wafCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAF: %w", err)
	}

	slog.Info("WAF: Production engine initialized",
		"engine", "Coraza",
		"ruleset", "OWASP CRS",
		"paranoia_level", cfg.ParanoiaLevel,
		"observation_mode", cfg.ObservationMode)

	return waf, nil
}

// buildCRSSetup generates CRS configuration directives
func buildCRSSetup(cfg WAFConfig) string {
	var sb strings.Builder

	// Core engine settings
	sb.WriteString("SecRuleEngine On\n")
	sb.WriteString("SecRequestBodyAccess On\n")
	sb.WriteString("SecRequestBodyLimit 1048576\n")
	sb.WriteString("SecRequestBodyNoFilesLimit 131072\n")

	// Set paranoia level
	sb.WriteString(fmt.Sprintf("SecAction \"id:900000,phase:1,nolog,pass,t:none,setvar:tx.paranoia_level=%d\"\n", cfg.ParanoiaLevel))

	// Anomaly scoring thresholds (standard CRS values)
	sb.WriteString("SecAction \"id:900110,phase:1,nolog,pass,t:none,setvar:tx.inbound_anomaly_score_threshold=5\"\n")
	sb.WriteString("SecAction \"id:900111,phase:1,nolog,pass,t:none,setvar:tx.outbound_anomaly_score_threshold=4\"\n")

	// Set blocking mode based on observation_mode
	if cfg.ObservationMode {
		sb.WriteString("SecAction \"id:900120,phase:1,nolog,pass,t:none,setvar:tx.blocking_paranoia_level=0\"\n")
		slog.Info("WAF: Running in OBSERVATION mode - attacks will be logged but not blocked")
	} else {
		sb.WriteString(fmt.Sprintf("SecAction \"id:900120,phase:1,nolog,pass,t:none,setvar:tx.blocking_paranoia_level=%d\"\n", cfg.ParanoiaLevel))
	}

	// Include CRS setup
	sb.WriteString("Include @crs-setup.conf.example\n")

	return sb.String()
}

func (p *WAFPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		waf, err := p.getWAF()
		if err != nil {
			slog.Error("WAF: Initialization failed", "error", err)
			// Security: Block requests if WAF fails to initialize
			WriteWAFError(w, http.StatusServiceUnavailable, "WAF_ERROR", "Security service unavailable")
			return
		}

		// Create transaction
		tx := waf.NewTransaction()
		defer func() {
			tx.ProcessLogging()
			tx.Close()
		}()

		// Process connection info
		clientIP := getClientIP(r)
		tx.ProcessConnection(clientIP, 0, "", 0)

		// Process request URI
		tx.ProcessURI(r.URL.String(), r.Method, r.Proto)

		// Process request headers
		for key, values := range r.Header {
			for _, value := range values {
				tx.AddRequestHeader(key, value)
			}
		}
		tx.ProcessRequestHeaders()

		// Check Phase 1 (request headers)
		if it := tx.Interruption(); it != nil {
			p.handleBlock(w, it, r, "phase1")
			return
		}

		// Process request body
		if r.Body != nil && r.ContentLength > 0 {
			scanSize := r.ContentLength
			if scanSize > maxWAFBodySize {
				scanSize = maxWAFBodySize
			}

			bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, scanSize))
			if err == nil && len(bodyBytes) > 0 {
				if it, _, err := tx.WriteRequestBody(bodyBytes); err == nil && it != nil {
					p.handleBlock(w, it, r, "body_write")
					return
				}
				// Restore body for downstream
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Process Phase 2 (request body)
		if it, _ := tx.ProcessRequestBody(); it != nil {
			p.handleBlock(w, it, r, "phase2")
			return
		}

		// Final interruption check
		if it := tx.Interruption(); it != nil {
			p.handleBlock(w, it, r, "final")
			return
		}

		// Request passed WAF - continue to next handler
		next.ServeHTTP(w, r)
	})
}

// handleBlock logs and responds to blocked requests
func (p *WAFPlugin) handleBlock(w http.ResponseWriter, it *types.Interruption, r *http.Request, phase string) {
	slog.Warn("WAF: Attack blocked",
		"rule_id", it.RuleID,
		"action", it.Action,
		"phase", phase,
		"client_ip", getClientIP(r),
		"method", r.Method,
		"path", r.URL.Path,
		"user_agent", r.UserAgent())

	// Always block in production (observation mode handled via CRS config)
	status := it.Status
	if status == 0 {
		status = http.StatusForbidden
	}

	WriteWAFError(w, status, "WAF_BLOCKED",
		fmt.Sprintf("Request blocked by security policy [%d]", it.RuleID))
}

func (p *WAFPlugin) Validate() error {
	return nil
}

// getClientIP extracts the real client IP
func getClientIP(r *http.Request) string {
	// X-Forwarded-For (first IP is original client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// RemoteAddr fallback
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host
}

// WriteWAFError sends a WAF error response
func WriteWAFError(w http.ResponseWriter, status int, code, message string) {
	err := &model.HttpError{
		Code:     code,
		Message:  message,
		HttpCode: status,
	}
	err.WriteJSONResponse(w)
}
