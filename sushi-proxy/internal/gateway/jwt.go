package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
)

type JwtPlugin struct {
	config map[string]interface{}
}

type JwtCredentials struct {
	alg          string
	iss          string
	secret       string
	publicKey    string
	consumerName string // Name of the consumer that owns these credentials
}

func NewJwtPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_JWT,
		Priority: 1450,
		Handler: JwtPlugin{
			config: config,
		},
		Validator: JwtPlugin{
			config: config,
		},
	}
}

func (plugin JwtPlugin) Validate() error {
	// Relaxed validation: Most settings are optional when using Consumer auth
	// Only validate alg if provided
	if alg, ok := plugin.config["alg"].(string); ok && alg != "" {
		supportedJwtSigningMethods := []string{constant.HS_256, constant.RSA_256}
		if !util.SliceContainsString(supportedJwtSigningMethods, alg) {
			return fmt.Errorf("alg must be one of: HS256, RS256")
		}

		// If alg is HS256, verification secret is required
		if alg == constant.HS_256 {
			secret, ok := plugin.config["secret"].(string)
			if !ok || secret == "" {
				return fmt.Errorf("secret must be a non-empty string when alg is HS256")
			}
		}

		// If alg is RS256, public key is required
		if alg == constant.RSA_256 {
			publicKey, ok := plugin.config["publicKey"].(string)
			if !ok || publicKey == "" {
				return fmt.Errorf("publicKey must be a non-empty string when alg is RS256")
			}
			// Validate the RSA public key format and structure
			_, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKey))
			if err != nil {
				return fmt.Errorf("invalid RSA public key: %v", err)
			}
		}
	}

	// iss is optional (defaults to verifying from consumers matching iss claim)
	if iss, ok := plugin.config["iss"].(string); ok && iss == "" {
		// If explicitly provided as empty string, that's fine, but usually implies intent
	}

	return nil
}

func (plugin JwtPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Executing jwt auth function...")

		tokenString, err := verifyAndParseAuthHeaderJwt(r)
		if err != nil {
			writeWWWAuthenticateHeaderJwt(w)
			err.WriteJSONResponse(w)
			return
		}

		creds, err := plugin.validateToken(tokenString)
		if err != nil {
			writeWWWAuthenticateHeaderJwt(w)
			err.WriteJSONResponse(w)
			return
		}

		// Strip Authorization header
		r.Header.Del("Authorization")

		// Extract subject (sub) from token and set in context
		// If we have a consumer, use that; otherwise fall back to sub claim
		var consumerId string
		if creds != nil && creds.consumerName != "" {
			consumerId = creds.consumerName
		} else {
			consumerId, _ = getSubFromToken(tokenString)
		}
		ctx := context.WithValue(r.Context(), constant.CONTEXT_CONSUMER_ID, consumerId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})

}

func writeWWWAuthenticateHeaderJwt(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate",
		fmt.Sprintf("Bearer realm=\"%s\", "+
			"charset=%s",
			"Access to sushi gateway", constant.UTF_8))
}

func verifyAndParseAuthHeaderJwt(req *http.Request) (string, *model.HttpError) {
	authHeader := req.Header.Get("Authorization")
	bits := strings.Split(authHeader, " ")

	// valid format : Bearer token
	isValidAuthFormat := authHeader != "" && len(bits) == 2
	if !isValidAuthFormat {
		slog.Info("Invalid jwt auth format passed in.")
		return "", model.NewHttpError(http.StatusUnauthorized,
			"MALFORMED_AUTH_HEADER", "Invalid auth format passed in.")
	}

	return bits[1], nil
}

func (plugin JwtPlugin) validateToken(token string) (*JwtCredentials, *model.HttpError) {
	// First, try to get credentials from consumers section (Kong-style)
	creds, consumerErr := plugin.getCredentialsFromConsumers(token)
	if consumerErr == nil && creds != nil {
		slog.Info("Using JWT credentials from consumer", "consumer", creds.consumerName, "iss", creds.iss)
		return creds, plugin.validateWithCredentials(creds, token)
	}

	// Fallback to plugin config (backwards compatibility)
	config := plugin.config
	alg, algOk := config["alg"].(string)
	iss, issOk := config["iss"].(string)
	if !algOk || !issOk {
		return nil, model.NewHttpError(http.StatusUnauthorized,
			"INVALID_CONFIG", "JWT credentials not found in consumers or plugin config")
	}

	creds = &JwtCredentials{
		alg: alg,
		iss: iss,
	}

	if creds.alg == constant.HS_256 {
		creds.secret = config["secret"].(string)
	} else {
		creds.publicKey = config["publicKey"].(string)
	}

	return creds, plugin.validateWithCredentials(creds, token)
}

// getCredentialsFromConsumers looks up JWT credentials from the consumers section
// based on the 'iss' (issuer) claim in the token
func (plugin JwtPlugin) getCredentialsFromConsumers(tokenString string) (*JwtCredentials, error) {
	// Parse token without verification to extract issuer
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	issuer, ok := claims["iss"].(string)
	if !ok || issuer == "" {
		return nil, fmt.Errorf("no issuer in token")
	}

	// Look up in consumers
	proxyConfig := GetGlobalProxyConfig()
	if proxyConfig == nil {
		return nil, fmt.Errorf("no proxy config")
	}

	for _, consumer := range proxyConfig.Consumers {
		for _, jwtSecret := range consumer.JwtSecrets {
			// In Kong, 'key' is the issuer identifier
			if jwtSecret.Key == issuer {
				creds := &JwtCredentials{
					iss:          jwtSecret.Key,
					consumerName: consumer.Username,
				}
				// Determine algorithm from secret config
				if jwtSecret.Algorithm != "" {
					creds.alg = jwtSecret.Algorithm
				} else {
					creds.alg = constant.HS_256 // Default
				}
				if jwtSecret.Secret != "" {
					creds.secret = jwtSecret.Secret
				}
				if jwtSecret.RsaPublicKey != "" {
					creds.publicKey = jwtSecret.RsaPublicKey
				}
				return creds, nil
			}
		}
	}

	return nil, fmt.Errorf("no matching consumer for issuer: %s", issuer)
}

func (plugin JwtPlugin) validateWithCredentials(credentials *JwtCredentials, token string) *model.HttpError {
	if credentials.alg == constant.HS_256 {
		return plugin.validateHS256(*credentials, token)
	} else {
		return plugin.validateRS256(*credentials, token)
	}
}

func (plugin JwtPlugin) validateRS256(credentials JwtCredentials, token string) *model.HttpError {
	tokenInvalidErr := model.NewHttpError(http.StatusUnauthorized, "INVALID_TOKEN", "The token is not valid.")

	// Parse public key
	rsaPublicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(credentials.publicKey))
	if err != nil {
		slog.Info("Failed to parse RSA public key: " + err.Error())
		return model.NewHttpError(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Something went wrong")
	}

	// Parse and validate the token using golang-jwt/jwt/v5
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return rsaPublicKey, nil
	})

	if err != nil {
		slog.Info("Error parsing token: " + err.Error())
		return tokenInvalidErr
	}

	if !jwtToken.Valid {
		return tokenInvalidErr
	}

	// Check claims if iss is valid from the token
	if !plugin.isClaimValid(jwtToken, credentials.iss) {
		return tokenInvalidErr
	}

	return nil
}

func (plugin JwtPlugin) validateHS256(credentials JwtCredentials, token string) *model.HttpError {

	tokenInvalidErr := model.NewHttpError(http.StatusUnauthorized, "INVALID_TOKEN", "The token is not valid.")

	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if credentials.alg == constant.HS_256 {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
		}
		return []byte(credentials.secret), nil
	})

	if err != nil {
		slog.Info("Error parsing token: " + err.Error())
		return tokenInvalidErr
	}

	if !jwtToken.Valid {
		return tokenInvalidErr
	}

	// Check claims if iss is valid from the token
	if !plugin.isClaimValid(jwtToken, credentials.iss) {
		return tokenInvalidErr
	}

	return nil
}

func (plugin JwtPlugin) isClaimValid(token *jwt.Token, issuer string) bool {
	// golang-jwt/jwt/v5 uses MapClaims which implements jwt.Claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok {
		if iss, ok := claims["iss"].(string); ok {
			if iss == issuer {
				return true
			} else {
				slog.Info(fmt.Sprintf("Invalid JWT issuer: %s", iss))
				return false
			}
		}
	}

	slog.Error("Invalid JWT claims")
	return false
}

func getSubFromToken(tokenString string) (string, error) {
	token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return nil, nil // We don't verify here, just parse claims, validation happened before
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if sub, ok := claims["sub"].(string); ok {
			return sub, nil
		}
	}
	return "anonymous", nil
}
