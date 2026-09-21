package http

import (
	"net/http"
	"time"

	"github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/delivery/http/modules"
	"github.com/brandsrx/supay/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// maxBodyBytes limita el tamaño del cuerpo de las peticiones (10 MB): los
// endpoints masivos aceptan hasta 500 facturas por paquete.
const maxBodyBytes int64 = 10 << 20

type v1Module interface {
	modules.Module
	RegisterV1Routes(chi.Router)
}

type AuthRoutes interface {
	RegisterPublicRoutes(chi.Router)
	RegisterProtectedRoutes(chi.Router)
}

type AuthOptions struct {
	Routes         AuthRoutes
	Tokens         AccessTokenVerifier
	Memberships    CompanyMembershipLookup
	SignedDownload http.HandlerFunc
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","message":"Supay API running"}`))
}

func registerModules(r chi.Router, registered []modules.Module, v1 bool) {
	for _, m := range registered {
		register := m.RegisterRoutes
		if v1 {
			if versioned, ok := m.(v1Module); ok {
				register = versioned.RegisterV1Routes
			}
		}
		if prefix := m.PathPrefix(); prefix != "" {
			r.Route(prefix, register)
		} else {
			register(r)
		}
	}
}

func registerTenantRoutes(r chi.Router, registered []modules.Module, lookup ApiKeyLookup, auth AuthOptions, v1 bool) {
	r.Group(func(protected chi.Router) {
		if auth.Tokens != nil && auth.Memberships != nil {
			protected.Use(TenantAccessMiddleware(lookup, auth.Tokens, auth.Memberships))
		} else if lookup != nil {
			protected.Use(TenantMiddleware(lookup))
		}
		registerModules(protected, registered, v1)
	})
}

func registerAuthRoutes(r chi.Router, auth AuthOptions) {
	if auth.Routes == nil {
		return
	}
	r.Route("/auth", func(routes chi.Router) {
		routes.Group(func(public chi.Router) {
			public.Use(RateLimitIP(1, 10, 5*time.Minute))
			auth.Routes.RegisterPublicRoutes(public)
		})
		if auth.Tokens != nil {
			routes.Group(func(protected chi.Router) {
				protected.Use(UserMiddleware(auth.Tokens))
				auth.Routes.RegisterProtectedRoutes(protected)
			})
		}
	})
}

// NewRouter construye el router de Chi con todas las rutas de la API.
// lookup resuelve API keys a tenants. Si lookup es nil la API queda abierta
// (útil para tests de handlers aislados).
// companyCreateHandler es el handler interno para POST /internal/companies
// (bootstrap protegido por X-Backend-Token).
func NewRouter(cfg config.Config, modules []modules.Module, lookup ApiKeyLookup, companyCreateHandler http.HandlerFunc, authOptions ...AuthOptions) http.Handler {
	r := chi.NewRouter()
	metrics := observability.DefaultMetrics()
	var auth AuthOptions
	if len(authOptions) > 0 {
		auth = authOptions[0]
	}

	r.Use(middleware.RequestID)
	r.Use(observability.HTTPMiddleware(metrics))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(LimitBody(maxBodyBytes))
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://0.0.0.0:3000", // <-- Añade este origen exacto
			"http://localhost:5173",
			"http://127.0.0.1:5173",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With", "X-API-Key", "X-Backend-Token", "X-Company-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	r.Use(corsMiddleware.Handler)

	r.Get("/health", healthHandler)
	if auth.SignedDownload != nil {
		r.Get("/storage/download", auth.SignedDownload)
	}
	r.Method(http.MethodGet, "/metrics", metrics.Handler())
	registerAuthRoutes(r, auth)

	// POST /internal/companies es el bootstrap interno con rate-limit por IP.
	r.Group(func(r chi.Router) {
		r.Use(InternalBootstrapMiddleware(cfg.BackendSecret))
		r.With(RateLimitIP(10/60, 10, 5*time.Minute)).Post("/internal/companies", companyCreateHandler)

	})

	// Las rutas sin versión se conservan durante la transición. /v1 es el
	// contrato público estable y permite que cada módulo adapte su payload.
	registerTenantRoutes(r, modules, lookup, auth, false)
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-API-Version", "v1")
				next.ServeHTTP(w, r)
			})
		})
		v1.Get("/health", healthHandler)
		v1.Method(http.MethodGet, "/metrics", metrics.Handler())
		registerAuthRoutes(v1, auth)
		v1.Group(func(internal chi.Router) {
			internal.Use(InternalBootstrapMiddleware(cfg.BackendSecret))
			internal.With(RateLimitIP(10, 60, 5*time.Minute)).Post("/internal/companies", companyCreateHandler)
		})
		registerTenantRoutes(v1, modules, lookup, auth, true)
	})

	return r
}
