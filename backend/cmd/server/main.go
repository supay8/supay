package main

import (
	"fmt"
	"log"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	fmt.Println("🚀 Iniciando motor SIAT en Go con Chi Router...")

	// 1. Conectar a PostgreSQL y migrar
	database.ConnectDB()

	// 2. Inyección de dependencias
	companyRepo := postgres.NewPostgresCompanyRepository(database.DB)
	companyUsecase := usecase.NewCompanyUsecase(companyRepo)
	companyHandler := deliveryHttp.NewCompanyHandler(companyUsecase)

	posRepo := postgres.NewPostgresPointOfSaleRepository(database.DB)
	posUsecase := usecase.NewPointOfSaleUsecase(posRepo, companyRepo)
	posHandler := deliveryHttp.NewPosHandler(posUsecase)

	// 3. Inicializar el Router de Chi
	r := chi.NewRouter()

	// Middlewares globales útiles
	r.Use(middleware.Logger)    // Muestra las peticiones HTTP en consola
	r.Use(middleware.Recoverer) // Evita que un panic tumbe el servidor

	// Ruta de salud
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "message": "SIAT Engine & Database running"}`))
	})

	// Agrupando rutas para Company
	r.Route("/companies", func(r chi.Router) {
		r.Post("/", companyHandler.Create)  // POST /companies
		r.Get("/", companyHandler.GetByNit) // GET /companies?nit=... o por query
		r.Patch("/{id}", companyHandler.Update)
		r.Delete("/{id}", companyHandler.Delete)
	})

	// Agrupando rutas para PointOfSale
	r.Route("/point-of-sale", func(r chi.Router) {
		r.Post("/", posHandler.Create)       // POST /point-of-sale
		r.Get("/", posHandler.List)          // GET /point-of-sale?companyId=...
		r.Get("/{id}", posHandler.GetByID)   // GET /point-of-sale/{id}
		r.Patch("/{id}", posHandler.Update)  // PATCH /point-of-sale/{id}
		r.Delete("/{id}", posHandler.Delete) // DELETE /point-of-sale/{id}
	})

	port := ":8080"
	log.Printf("Servidor escuchando en el puerto %s", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
