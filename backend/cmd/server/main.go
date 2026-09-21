package main

import (
	"context"
	"log"
	"os"

	"github.com/brandsrx/supay/internal/app"
	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	app.SetupLogging()
	log.Println("Iniciando Supay API...")

	cfg := appconfig.Load()
	if err := cfg.ValidateAuth(); err != nil {
		log.Fatalf("Configuración de autenticación inválida: %v", err)
	}
	if err := cfg.ValidateStorage(); err != nil {
		log.Fatalf("Configuración de storage inválida: %v", err)
	}
	db := database.ConnectDB()

	if os.Getenv("AUTO_MIGRATE") == "true" {
		log.Println("AUTO_MIGRATE=true: ejecutando migraciones de esquema y datos...")
		if err := database.Migrate(); err != nil {
			log.Fatalf("Error al ejecutar migraciones: %v", err)
		}
		log.Println("Migraciones completadas.")
	}
	if cfg.BackendSecret == "" {
		log.Fatalf("BACKEND_SECRET es obligatorio para proteger los endpoints internos")
	}

	container := app.NewContainer(cfg, db)
	_ = container.ObjectStorage()
	app.StartStaleEmissionReaper(container.InvoiceRepo())
	stopMaintenance := app.StartMaintenanceScheduler(context.Background(), container.MaintenanceService(), cfg.Maintenance)
	defer stopMaintenance()

	log.Printf("Servidor escuchando en el puerto :%s", cfg.Port)
	if err := app.RunServer(container.Server()); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
