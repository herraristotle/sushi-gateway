package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/container"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/gateway"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

type GatewayController struct {
}

func NewGatewayController() *GatewayController {
	return &GatewayController{}
}

func (c *GatewayController) RegisterRoutes(router chi.Router) {
	router.Get("/gateway/config", ProtectRouteUsingJWT(c.GetGatewayConfig()).ServeHTTP)
	router.Get("/gateway", ProtectRouteUsingJWT(c.GetGatewayInformation()).ServeHTTP)

	// Services CRUD
	router.Get("/services", ProtectRouteUsingJWT(c.ListServices()).ServeHTTP)
	router.Post("/services", ProtectRouteUsingJWT(c.UpsertService()).ServeHTTP)
	router.Delete("/services/{name}", ProtectRouteUsingJWT(c.DeleteService()).ServeHTTP)

	// Routes CRUD
	router.Get("/services/{serviceName}/routes", ProtectRouteUsingJWT(c.ListRoutes()).ServeHTTP)
	router.Post("/services/{serviceName}/routes", ProtectRouteUsingJWT(c.UpsertRoute()).ServeHTTP)
	router.Delete("/services/{serviceName}/routes/{routeName}", ProtectRouteUsingJWT(c.DeleteRoute()).ServeHTTP)

	// Upstreams CRUD
	router.Get("/upstreams", ProtectRouteUsingJWT(c.ListUpstreams()).ServeHTTP)
	router.Post("/upstreams", ProtectRouteUsingJWT(c.UpsertUpstream()).ServeHTTP)
	router.Delete("/upstreams/{name}", ProtectRouteUsingJWT(c.DeleteUpstream()).ServeHTTP)
}

func (c *GatewayController) GetGatewayInformation() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		slog.Info("GatewayController:: Admin API - Getting gateway information")
		gatewayConfig := gateway.GetGlobalProxyConfig()
		payload, _ := json.Marshal(gatewayConfig)
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}
}

func (c *GatewayController) GetGatewayConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		slog.Info("GatewayController:: Admin API - Getting gateway configuration")
		appConfig := container.Global.AppConfig
		payload, _ := json.Marshal(appConfig)
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}
}

// Services

func (c *GatewayController) ListServices() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		services, err := gateway.GlobalConfigStore.ListServices(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(services)
	}
}

func (c *GatewayController) UpsertService() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var svc model.Service
		if err := json.NewDecoder(req.Body).Decode(&svc); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := gateway.GlobalConfigStore.UpsertService(req.Context(), svc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gateway.ReloadConfigFromStore()
		w.WriteHeader(http.StatusOK)
	}
}

func (c *GatewayController) DeleteService() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		if err := gateway.GlobalConfigStore.DeleteService(req.Context(), name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gateway.ReloadConfigFromStore()
		w.WriteHeader(http.StatusNoContent)
	}
}

// Routes

func (c *GatewayController) ListRoutes() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		serviceName := chi.URLParam(req, "serviceName")
		routes, err := gateway.GlobalConfigStore.ListRoutes(req.Context(), serviceName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(routes)
	}
}

func (c *GatewayController) UpsertRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		serviceName := chi.URLParam(req, "serviceName")
		var route model.Route
		if err := json.NewDecoder(req.Body).Decode(&route); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := gateway.GlobalConfigStore.UpsertRoute(req.Context(), serviceName, route); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gateway.ReloadConfigFromStore()
		w.WriteHeader(http.StatusOK)
	}
}

func (c *GatewayController) DeleteRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		serviceName := chi.URLParam(req, "serviceName")
		routeName := chi.URLParam(req, "routeName")
		if err := gateway.GlobalConfigStore.DeleteRoute(req.Context(), serviceName, routeName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gateway.ReloadConfigFromStore()
		w.WriteHeader(http.StatusNoContent)
	}
}

// Upstreams

func (c *GatewayController) ListUpstreams() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		upstreams, err := gateway.GlobalConfigStore.ListUpstreams(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(upstreams)
	}
}

func (c *GatewayController) UpsertUpstream() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var upstream model.UpstreamConfig
		if err := json.NewDecoder(req.Body).Decode(&upstream); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := gateway.GlobalConfigStore.UpsertUpstream(req.Context(), upstream); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gateway.ReloadConfigFromStore()
		w.WriteHeader(http.StatusOK)
	}
}

func (c *GatewayController) DeleteUpstream() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		if err := gateway.GlobalConfigStore.DeleteUpstream(req.Context(), name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gateway.ReloadConfigFromStore()
		w.WriteHeader(http.StatusNoContent)
	}
}
