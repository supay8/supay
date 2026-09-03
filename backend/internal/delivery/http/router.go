package http

import (
	"net/http"
	"time"

	"github.com/brandsrx/supay/internal/delivery/http/modules"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// maxBodyBytes limita el tamaño del cuerpo de las peticiones (10 MB): los
// endpoints masivos aceptan hasta 500 facturas por paquete.
const maxBodyBytes int64 = 10 << 20

// NewRouter construye el router de Chi con todas las rutas de la API.
// lookup resuelve API keys a tenants. Si lookup es nil la API queda abierta
// (útil para tests de handlers aislados).
func NewRouter(modules []modules.Module, lookup ApiKeyLookup) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
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
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With", "X-API-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	r.Use(corsMiddleware.Handler)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","message":"Supay API running"}`))
	})

	r.Group(func(r chi.Router) {
		if lookup != nil {
			r.Use(TenantMiddleware(lookup))
		}

		for _, m := range modules {
			if prefix := m.PathPrefix(); prefix != "" {
				r.Route(prefix, m.RegisterRoutes)
			} else {
				m.RegisterRoutes(r)
			}
		}
	})

	return r
}
