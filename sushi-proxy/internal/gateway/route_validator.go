package gateway

import (
	"fmt"
	"strings"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
)

type RouteValidator struct {
}

func NewRouteValidator() *RouteValidator {
	return &RouteValidator{}
}

func (rv *RouteValidator) ValidateRoute(route model.Route) error {
	if err := validatePath(route); err != nil {
		return err
	}
	if err := validateMethod(route); err != nil {
		return err
	}
	return nil
}

func validatePath(route model.Route) error {
	if route.Path == "" && len(route.Paths) == 0 {
		return fmt.Errorf("route %s: must specify at least one path", route.Name)
	}

	if route.Path != "" {
		if !strings.HasPrefix(route.Path, "/") {
			return fmt.Errorf("route path: %s must start with /", route.Path)
		}
		if strings.HasSuffix(route.Path, "/") && len(route.Path) > 1 {
			return fmt.Errorf("route path: %s must not end with /", route.Path)
		}
	}

	for _, p := range route.Paths {
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("route path: %s must start with /", p)
		}
		if strings.HasSuffix(p, "/") && len(p) > 1 {
			return fmt.Errorf("route path: %s must not end with /", p)
		}
	}

	return nil
}

func validateMethod(route model.Route) error {
	// Empty methods means ALL methods are allowed (standard proxy behavior)
	if len(route.Methods) == 0 {
		return nil
	}

	for _, method := range route.Methods {
		if !util.SliceContainsString(constant.VALID_METHODS, method) {
			return fmt.Errorf("route method: %s is invalid", method)
		}
	}

	return nil
}
