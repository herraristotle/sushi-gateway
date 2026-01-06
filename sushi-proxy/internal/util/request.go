package util

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func GetServiceAndRouteFromRequest(proxyConfig *model.ProxyConfig, req *http.Request) (*model.Service, *model.Route, *model.HttpError) {
	path := req.URL.Path
	slog.Debug("Routing request", "path", path)

	for _, service := range proxyConfig.Services {
		slog.Debug("Checking service", "service", service.Name, "base_path", service.BasePath)
		// Check if request path starts with service base path
		if strings.HasPrefix(path, service.BasePath) {
			slog.Debug("Matched service base path", "service", service.Name)
			// Extract the remaining path after service base path
			routePath := strings.TrimPrefix(path, service.BasePath)
			if routePath != "" && !strings.HasPrefix(routePath, "/") {
				// If service base path matched but didn't end with / and next char isn't /
				// e.g., base=/api, path=/apis -> this shouldn't match
				continue
			}
			if routePath == "" {
				routePath = "/"
			}

			// Find a matching route within the service
			for _, route := range service.Routes {
				// If methods not specified, allow all (smart default like Kong)
				allowAllMethods := len(route.Methods) == 0
				routeContainsMethod := allowAllMethods || SliceContainsString(route.Methods, req.Method)

				// A route might match if its path is "/" (meaning it covers the base path)
				// or if it matches the remaining routePath.
				if (MatchRoute(&route, routePath, req) || (routePath == "/" && isRootRoute(&route))) && routeContainsMethod {
					return &service, &route, nil
				}
			}

			// If we matched the service but no route, return ROUTE_NOT_FOUND
			return nil, nil, &model.HttpError{
				Code:     "ROUTE_NOT_FOUND",
				Message:  fmt.Sprintf("Route not found for path: %s. Check your HTTP Method and Route path", routePath),
				HttpCode: http.StatusNotFound,
			}
		}
	}

	return nil, nil, &model.HttpError{
		Code:     "SERVICE_NOT_FOUND",
		Message:  "Service not found",
		HttpCode: http.StatusNotFound,
	}
}

// Gets the IP address from a remote address
func GetHostIp(remoteAddress string) (string, *model.HttpError) {

	ipAddr, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		slog.Error("unable to get the host from ip address: " + err.Error())
		return "", model.NewHttpError(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "something went wrong in the server.")
	}
	return ipAddr, nil
}

// GetHostAndPortFromURL parses a URL string (with scheme, example: http://127.0.0.1:8080) and returns the host and port
// If no port is specified in the URL:
// - HTTP defaults to 80
// - HTTPS defaults to 443
func GetHostAndPortFromURL(urlStr string) (string, int, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Get host and port from URL
	host := parsedURL.Hostname()
	port := parsedURL.Port()

	// If port is not specified, use default ports based on scheme
	if port == "" {
		switch parsedURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", 0, fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
		}
	}

	// Convert port string to integer
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port number: %w", err)
	}

	return host, portNum, nil
}

// Check whether the route exists in the service, match either static or dynamic routes
func MatchRoute(route *model.Route, requestPath string, req *http.Request) bool {

	// Check headers match
	if len(route.Headers) > 0 {
		for key, value := range route.Headers {
			if req.Header.Get(key) != value {
				return false
			}
		}
	}

	// Build list of paths to check (Paths array takes precedence over Path)
	pathsToCheck := route.Paths
	if len(pathsToCheck) == 0 && route.Path != "" {
		// Backward compatibility: if Paths not specified, use Path
		pathsToCheck = []string{route.Path}
	}

	// Check if request path matches any of the configured paths
	for _, routePath := range pathsToCheck {
		// if not contains any {param} in route path, do a simple static match
		isStaticRoute := !strings.Contains(routePath, "{") && !strings.Contains(routePath, "}")
		if isStaticRoute {
			if routePath == requestPath {
				return true
			}
		} else {
			// if contains {param} in route path, then do a dynamic match
			if matchDynamicRoute(routePath, requestPath) {
				return true
			}
		}
	}

	return false
}

func matchDynamicRoute(routePath string, requestPath string) bool {
	routeParts := strings.Split(routePath, "/")
	requestPathParts := strings.Split(requestPath, "/")
	if len(routeParts) != len(requestPathParts) {
		return false
	}

	// Compare each segment
	for i := range routeParts {
		if strings.HasPrefix(routeParts[i], "{") && strings.HasSuffix(routeParts[i], "}") {
			// This is a dynamic segment, so continue without checking
			continue
		}

		// For static segments, they must match exactly
		if routeParts[i] != requestPathParts[i] {
			return false
		}
	}

	// All segments match for dynamic values
	return true
}

func GetContentLength(input string) int64 {
	if input == "" {
		return 0
	}
	conv, _ := strconv.ParseInt(input, 10, 64)
	return conv
}

// GetPathParam extracts a path parameter from the request URL
// This is a simple implementation that extracts the last segment if paramName is "id"
// or looks for the segment after a matching prefix
func GetPathParam(req *http.Request, paramName string) string {
	path := req.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Common pattern: /resource/{id} - return last segment for "id"
	if paramName == "id" && len(parts) > 0 {
		return parts[len(parts)-1]
	}

	// Look for segment after a matching prefix
	// e.g., /users/123 with paramName "users" returns nothing
	// e.g., /users/123/orders with paramName "users" returns 123
	for i, part := range parts {
		if part == paramName && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

func isRootRoute(route *model.Route) bool {
	// Build list of paths to check (Paths array takes precedence over Path)
	pathsToCheck := route.Paths
	if len(pathsToCheck) == 0 && route.Path != "" {
		pathsToCheck = []string{route.Path}
	}
	for _, p := range pathsToCheck {
		if p == "/" || p == "" {
			return true
		}
	}
	return false
}
