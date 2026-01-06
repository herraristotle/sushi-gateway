package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/container"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// getJwtKey returns the JWT signing key from the DI container.
func getJwtKey() []byte {
	return container.Global.AppConfig.JwtSecret
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type AuthController struct {
}

func NewAuthController() *AuthController {
	return &AuthController{}
}

func (c *AuthController) RegisterRoutes(router chi.Router) {
	router.Post("/login", c.Login())
	router.Delete("/logout", c.Logout())
}

func (c *AuthController) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		slog.Info("AuthController:: Admin API - Logging in")
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" {
			model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH",
				"Authorization header missing").WriteJSONResponse(w)
			return
		}

		if !strings.HasPrefix(authHeader, "Basic ") {
			model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH",
				"Invalid Authorization scheme").WriteJSONResponse(w)
			return
		}

		encodedCredentials := strings.TrimPrefix(authHeader, "Basic ")
		decodedBytes, err := base64.StdEncoding.DecodeString(encodedCredentials)
		if err != nil {
			model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH",
				"Invalid Base64 encoding").WriteJSONResponse(w)
			return
		}

		credentials := string(decodedBytes)
		parts := strings.SplitN(credentials, ":", 2)
		if len(parts) != 2 {
			model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH",
				"Invalid credentials format").WriteJSONResponse(w)
			return
		}

		username, password := parts[0], parts[1]
		if !validate(username, password) {
			model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH",
				"Invalid credentials").WriteJSONResponse(w)
			return
		}

		tokenString, err := generateJWT("user")
		if err != nil {
			model.NewHttpError(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR",
				"Error generating JWT token").WriteJSONResponse(w)
			return
		}

		// Set JWT in cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    tokenString,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
		})

		slog.Info("Login success:: " + username)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"token": tokenString,
		})
	}
}

func (c *AuthController) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		slog.Info("AuthController:: Admin API - Logging out")
		// To log out, we invalidate the token by setting a past expiration date
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    "",
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusOK)
	}
}

func validate(username string, password string) bool {
	return username == container.Global.AppConfig.AdminUser &&
		password == container.Global.AppConfig.AdminPassword
}

func generateJWT(username string) (string, error) {
	// Set expiration time to 24 hours
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "sushi-gateway-admin-api",
			Audience:  []string{"sushi-gateway-manager"},
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(getJwtKey())
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func validateJWT(tokenString string) (*Claims, *model.HttpError) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return getJwtKey(), nil
	})
	if err != nil {
		return nil, model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH", "Invalid token")
	}
	if !token.Valid {
		return nil, model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH", "Invalid token")
	}
	return claims, nil
}

func ProtectRouteUsingJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var tokenString string

		// 1. Try to get token from Authorization header (Bearer)
		authHeader := req.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// 2. Fallback to cookie if not in header
		if tokenString == "" {
			cookie, err := req.Cookie("token")
			if err == nil {
				tokenString = cookie.Value
			}
		}

		if tokenString == "" {
			model.NewHttpError(http.StatusUnauthorized, "UNAUTHORIZED_AUTH",
				"Authentication token missing").WriteJSONResponse(w)
			return
		}

		claims, httpErr := validateJWT(tokenString)
		if httpErr != nil {
			httpErr.WriteJSONResponse(w)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(req.Context(), "claims", claims)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}
