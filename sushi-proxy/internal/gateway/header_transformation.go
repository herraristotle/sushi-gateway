package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
)

// HeaderTransformationPlugin modifies request headers before proxying to upstream.
// Supports: add, remove, rename, replace, and append operations.
type HeaderTransformationPlugin struct {
	config map[string]interface{}
}

func NewHeaderTransformationPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_HEADER_TRANSFORMATION,
		Priority: 300,
		Handler: &HeaderTransformationPlugin{
			config: config,
		},
		Validator: HeaderTransformationPlugin{
			config: config,
		},
	}
}

func (plugin HeaderTransformationPlugin) Validate() error {
	// At least one operation should be configured
	hasAdd := plugin.config["add"] != nil
	hasRemove := plugin.config["remove"] != nil
	hasRename := plugin.config["rename"] != nil
	hasReplace := plugin.config["replace"] != nil
	hasAppend := plugin.config["append"] != nil

	if !hasAdd && !hasRemove && !hasRename && !hasReplace && !hasAppend {
		return fmt.Errorf("at least one header transformation operation (add, remove, rename, replace, append) must be configured")
	}

	// Validate 'add' format: map of header name -> value
	if hasAdd {
		if _, ok := plugin.config["add"].(map[string]interface{}); !ok {
			return fmt.Errorf("'add' must be a map of header names to values")
		}
	}

	// Validate 'remove' format: array of header names
	if hasRemove {
		if _, ok := plugin.config["remove"].([]interface{}); !ok {
			return fmt.Errorf("'remove' must be an array of header names")
		}
	}

	// Validate 'rename' format: map of old name -> new name
	if hasRename {
		if _, ok := plugin.config["rename"].(map[string]interface{}); !ok {
			return fmt.Errorf("'rename' must be a map of old header names to new names")
		}
	}

	// Validate 'replace' format: map of header name -> new value
	if hasReplace {
		if _, ok := plugin.config["replace"].(map[string]interface{}); !ok {
			return fmt.Errorf("'replace' must be a map of header names to replacement values")
		}
	}

	// Validate 'append' format: map of header name -> value to append
	if hasAppend {
		if _, ok := plugin.config["append"].(map[string]interface{}); !ok {
			return fmt.Errorf("'append' must be a map of header names to values to append")
		}
	}

	return nil
}

func (plugin HeaderTransformationPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Executing header transformation plugin...")

		// Process operations in a specific order for predictable behavior
		// Order: remove -> rename -> replace -> append -> add

		// 1. Remove headers
		if removeHeaders, ok := plugin.config["remove"].([]interface{}); ok {
			for _, headerName := range removeHeaders {
				if name, ok := headerName.(string); ok {
					slog.Debug("Removing header", "header", name)
					r.Header.Del(name)
				}
			}
		}

		// 2. Rename headers
		if renameHeaders, ok := plugin.config["rename"].(map[string]interface{}); ok {
			for oldName, newName := range renameHeaders {
				if newNameStr, ok := newName.(string); ok {
					if value := r.Header.Get(oldName); value != "" {
						slog.Debug("Renaming header", "from", oldName, "to", newNameStr)
						r.Header.Del(oldName)
						r.Header.Set(newNameStr, value)
					}
				}
			}
		}

		// 3. Replace headers (only if header exists)
		if replaceHeaders, ok := plugin.config["replace"].(map[string]interface{}); ok {
			for headerName, newValue := range replaceHeaders {
				if existingValue := r.Header.Get(headerName); existingValue != "" {
					if valueStr, ok := newValue.(string); ok {
						slog.Debug("Replacing header value", "header", headerName, "old", existingValue, "new", valueStr)
						r.Header.Set(headerName, valueStr)
					}
				}
			}
		}

		// 4. Append to headers
		if appendHeaders, ok := plugin.config["append"].(map[string]interface{}); ok {
			for headerName, appendValue := range appendHeaders {
				if existingValue := r.Header.Get(headerName); existingValue != "" {
					if valueStr, ok := appendValue.(string); ok {
						newValue := existingValue + ", " + valueStr
						slog.Debug("Appending to header", "header", headerName, "append", valueStr)
						r.Header.Set(headerName, newValue)
					}
				} else {
					// If header doesn't exist, just set it
					if valueStr, ok := appendValue.(string); ok {
						r.Header.Set(headerName, valueStr)
					}
				}
			}
		}

		// 5. Add headers (overwrite if exists)
		if addHeaders, ok := plugin.config["add"].(map[string]interface{}); ok {
			for headerName, headerValue := range addHeaders {
				if valueStr, ok := headerValue.(string); ok {
					slog.Debug("Adding header", "header", headerName, "value", valueStr)
					r.Header.Set(headerName, valueStr)
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// Helper function to check if a string slice contains a value (case-insensitive for headers)
func containsHeaderName(slice []string, name string) bool {
	nameLower := strings.ToLower(name)
	for _, s := range slice {
		if strings.ToLower(s) == nameLower {
			return true
		}
	}
	return false
}
