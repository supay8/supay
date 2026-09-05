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
	db := database.ConnectDB()

	if os.Getenv("AUTO_MIGRATE") == "true" {
		log.Println("AUTO_MIGRATE=true: ejecutando migraciones de esquema y datos...")
		if err := database.Migrate(); err != nil {
			log.Fatalf("Error al ejecutar migraciones: %v", err)
		}
		log.Println("Migraciones completadas.")
	}
	if os.Getenv("BACKEND_SECRET") == "false" {
		log.Fatalf("BACKEND_SECRET required")
	}

	container := app.NewContainer(cfg, db)
	app.StartStaleEmissionReaper(container.InvoiceRepo())
	stopMaintenance := app.StartMaintenanceScheduler(context.Background(), container.MaintenanceService(), cfg.Maintenance)
	defer stopMaintenance()

	log.Printf("Servidor escuchando en el puerto :%s", cfg.Port)
	if err := app.RunServer(container.Server()); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
