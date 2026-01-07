package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// Known malicious bot patterns (production list)
var defaultMaliciousBots = []string{
	// Vulnerability scanners
	"sqlmap", "nikto", "nessus", "openvas", "acunetix", "netsparker",
	"burpsuite", "owasp", "skipfish", "wpscan", "nuclei", "gobuster",
	"dirbuster", "sqlninja", "havij", "pangolin", "nmap",

	// Scrapers and crawlers
	"scrapy", "python-requests", "python-urllib", "curl", "wget",
	"httpclient", "java", "libwww", "lwp", "mechanize", "phantom",
	"selenium", "headless", "puppeteer", "playwright",

	// Bad bots
	"ahrefsbot", "semrushbot", "dotbot", "rogerbot", "exabot",
	"mj12bot", "blexbot", "linkdexbot", "megaindex", "majestic",
	"seokicks", "siteexplorer", "aspiegelbot", "backlinkcrawler",

	// DDoS / Attack tools
	"slowloris", "siege", "ab", "wrk", "vegeta", "locust", "jmeter",
	"loadrunner", "gatling", "artillery", "k6", "hey",
}

// Known good bots (to allow through)
var defaultGoodBots = []string{
	"googlebot", "bingbot", "slurp", "duckduckbot", "baiduspider",
	"yandexbot", "facebookexternalhit", "twitterbot", "linkedinbot",
	"whatsapp", "telegrambot", "discordbot", "slackbot",
	"applebot", "pinterest", "uptimerobot", "pingdom", "statuscake",
}

// BotProtectionConfig contains bot protection settings
type BotProtectionConfig struct {
	// Mode: "block" (default) or "allow"
	// - block: Block bots in blacklist
	// - allow: Only allow bots in whitelist
	Mode string `json:"mode"`

	// Blacklist patterns (regex supported)
	Blacklist []string `json:"blacklist"`

	// Whitelist patterns (regex supported) - override blacklist
	Whitelist []string `json:"whitelist"`

	// Block empty User-Agent
	BlockEmptyUA bool `json:"block_empty_ua"`

	// Use default malicious bot list
	UseDefaultBlacklist bool `json:"use_default_blacklist"`

	// Allow known good bots (search engines, social media)
	AllowGoodBots bool `json:"allow_good_bots"`

	// Block requests without common browser headers
	ValidateBrowserHeaders bool `json:"validate_browser_headers"`

	// Custom message for blocked bots
	BlockMessage string `json:"block_message"`
}

type BotProtectionPlugin struct {
	config         BotProtectionConfig
	blacklistRegex []*regexp.Regexp
	whitelistRegex []*regexp.Regexp
	goodBotsRegex  []*regexp.Regexp
}

func NewBotProtectionPlugin(config map[string]interface{}) *Plugin {
	cfg := BotProtectionConfig{
		Mode:                   "block",
		BlockEmptyUA:           false,
		UseDefaultBlacklist:    true,
		AllowGoodBots:          true,
		ValidateBrowserHeaders: false,
		BlockMessage:           "Access denied",
	}

	// Parse config
	if v, ok := config["mode"].(string); ok {
		cfg.Mode = v
	}
	if v, ok := config["block_empty_ua"].(bool); ok {
		cfg.BlockEmptyUA = v
	}
	if v, ok := config["use_default_blacklist"].(bool); ok {
		cfg.UseDefaultBlacklist = v
	}
	if v, ok := config["allow_good_bots"].(bool); ok {
		cfg.AllowGoodBots = v
	}
	if v, ok := config["validate_browser_headers"].(bool); ok {
		cfg.ValidateBrowserHeaders = v
	}
	if v, ok := config["block_message"].(string); ok {
		cfg.BlockMessage = v
	}

	// Parse blacklist
	cfg.Blacklist = parseStringList(config, "blacklist")

	// Parse whitelist
	cfg.Whitelist = parseStringList(config, "whitelist")

	// Build regex patterns
	plugin := &BotProtectionPlugin{config: cfg}
	plugin.compilePatterns()

	return &Plugin{
		Name:      constant.PLUGIN_BOT_PROTECTION,
		Priority:  900, // Run after auth but before most plugins
		Handler:   plugin,
		Validator: plugin,
	}
}

func parseStringList(config map[string]interface{}, key string) []string {
	var result []string
	if v, ok := config[key]; ok {
		switch list := v.(type) {
		case []string:
			result = list
		case []interface{}:
			for _, item := range list {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
		}
	}
	return result
}

func (p *BotProtectionPlugin) compilePatterns() {
	// Compile blacklist patterns
	var blacklist []string
	if p.config.UseDefaultBlacklist {
		blacklist = append(blacklist, defaultMaliciousBots...)
	}
	blacklist = append(blacklist, p.config.Blacklist...)

	for _, pattern := range blacklist {
		if regex, err := regexp.Compile("(?i)" + regexp.QuoteMeta(pattern)); err == nil {
			p.blacklistRegex = append(p.blacklistRegex, regex)
		}
	}

	// Compile whitelist patterns
	for _, pattern := range p.config.Whitelist {
		if regex, err := regexp.Compile("(?i)" + regexp.QuoteMeta(pattern)); err == nil {
			p.whitelistRegex = append(p.whitelistRegex, regex)
		}
	}

	// Compile good bots patterns
	if p.config.AllowGoodBots {
		for _, pattern := range defaultGoodBots {
			if regex, err := regexp.Compile("(?i)" + regexp.QuoteMeta(pattern)); err == nil {
				p.goodBotsRegex = append(p.goodBotsRegex, regex)
			}
		}
	}
}

func (p *BotProtectionPlugin) Validate() error {
	if p.config.Mode != "block" && p.config.Mode != "allow" {
		return fmt.Errorf("mode must be 'block' or 'allow'")
	}

	// In allow mode, whitelist is required
	if p.config.Mode == "allow" && len(p.config.Whitelist) == 0 {
		return fmt.Errorf("whitelist is required in 'allow' mode")
	}

	// In block mode, some form of protection should be enabled
	if p.config.Mode == "block" && !p.config.UseDefaultBlacklist && len(p.config.Blacklist) == 0 && !p.config.BlockEmptyUA {
		return fmt.Errorf("blacklist or use_default_blacklist must be provided in 'block' mode")
	}

	return nil
}

func (p *BotProtectionPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		userAgentLower := strings.ToLower(userAgent)

		// Check empty User-Agent
		if userAgent == "" && p.config.BlockEmptyUA {
			p.blockRequest(w, r, "empty_user_agent")
			return
		}

		// Check whitelist first (always allow if matched)
		for _, regex := range p.whitelistRegex {
			if regex.MatchString(userAgentLower) {
				slog.Debug("Bot protection: Whitelisted", "user_agent", userAgent)
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check good bots (search engines, etc.)
		if p.config.AllowGoodBots {
			for _, regex := range p.goodBotsRegex {
				if regex.MatchString(userAgentLower) {
					slog.Debug("Bot protection: Good bot allowed", "user_agent", userAgent)
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// Check blacklist
		for _, regex := range p.blacklistRegex {
			if regex.MatchString(userAgentLower) {
				p.blockRequest(w, r, "blacklisted_bot")
				return
			}
		}

		// Validate browser headers if enabled
		if p.config.ValidateBrowserHeaders && !p.isLikelyBrowser(r) {
			p.blockRequest(w, r, "invalid_browser_headers")
			return
		}

		// In allow mode, block if not in whitelist
		if p.config.Mode == "allow" {
			p.blockRequest(w, r, "not_whitelisted")
			return
		}

		// Request passed bot protection
		next.ServeHTTP(w, r)
	})
}

// isLikelyBrowser checks if request has typical browser headers
func (p *BotProtectionPlugin) isLikelyBrowser(r *http.Request) bool {
	// Real browsers typically send these headers
	hasAccept := r.Header.Get("Accept") != ""
	hasAcceptLang := r.Header.Get("Accept-Language") != ""
	hasAcceptEnc := r.Header.Get("Accept-Encoding") != ""

	// At least 2 of 3 should be present
	count := 0
	if hasAccept {
		count++
	}
	if hasAcceptLang {
		count++
	}
	if hasAcceptEnc {
		count++
	}

	return count >= 2
}

func (p *BotProtectionPlugin) blockRequest(w http.ResponseWriter, r *http.Request, reason string) {
	slog.Warn("Bot protection: Request blocked",
		"reason", reason,
		"user_agent", r.Header.Get("User-Agent"),
		"client_ip", getClientIP(r),
		"path", r.URL.Path)

	err := &model.HttpError{
		Code:     "BOT_DETECTED",
		Message:  p.config.BlockMessage,
		HttpCode: http.StatusForbidden,
	}
	err.WriteJSONResponse(w)
}
