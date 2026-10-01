package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/brandsrx/supay/internal/app"
	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Supay finalizó con error: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()
	app.SetupLogging()
	log.Println("Iniciando Supay API...")

	cfg := appconfig.Load()
	if err := cfg.ValidateRunMode(); err != nil {
		return fmt.Errorf("configuración de ejecución inválida: %w", err)
	}
	if cfg.RunMode.RunsWeb() {
		if err := cfg.ValidateAuth(); err != nil {
			return fmt.Errorf("configuración de autenticación inválida: %w", err)
		}
		if cfg.BackendSecret == "" {
			return fmt.Errorf("BACKEND_SECRET es obligatorio para proteger los endpoints internos")
		}
	}
	if err := cfg.ValidateStorage(); err != nil {
		return fmt.Errorf("configuración de storage inválida: %w", err)
	}
	autoMigrate := strings.EqualFold(strings.TrimSpace(os.Getenv("AUTO_MIGRATE")), "true")
	if err := cfg.ValidateAutoMigrate(autoMigrate); err != nil {
		return fmt.Errorf("configuración de migraciones inválida: %w", err)
	}
	db := database.ConnectDB()

	if autoMigrate {
		log.Println("AUTO_MIGRATE=true: ejecutando migraciones de esquema y datos...")
		if err := database.Migrate(); err != nil {
			return fmt.Errorf("ejecutar migraciones: %w", err)
		}
		log.Println("Migraciones completadas.")
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	container := app.NewContainer(cfg, db)
	_ = container.ObjectStorage()

	if cfg.RunMode.RunsWorker() {
		stopMaintenance := app.StartMaintenanceScheduler(ctx, container.MaintenanceService(), cfg.Maintenance)
		defer stopMaintenance()
		stopReaper := app.StartStaleEmissionReaper(ctx, container.InvoiceRepo())
		defer stopReaper()

		queue, err := container.EmissionQueue()
		if err != nil {
			return fmt.Errorf("configurar cola de emisión: %w", err)
		}
		stopQueue, err := app.StartEmissionQueue(ctx, queue, cfg.Queue.Enabled, cfg.Queue.SoftStopTimeout)
		if err != nil {
			return fmt.Errorf("iniciar cola de emisión: %w", err)
		}
		stopQueueLogged := func() {
			if err := stopQueue(); err != nil {
				log.Printf("cola de emisión: apagado incompleto: %v", err)
			}
		}
		// El servidor HTTP puede tardar más que la ventana de Cloud Run en
		// drenar. Iniciar el soft-stop de River apenas llega SIGTERM garantiza
		// que la cola dispone de sus 10 segundos completos en paralelo.
		go func() {
			<-ctx.Done()
			stopQueueLogged()
		}()
		defer stopQueueLogged()
	}

	if !cfg.RunMode.RunsWeb() {
		log.Printf("Proceso worker iniciado (RUN_MODE=%s)", cfg.RunMode)
		<-ctx.Done()
		return nil
	}

	log.Printf("Servidor escuchando en el puerto :%s (RUN_MODE=%s)", cfg.Port, cfg.RunMode)
	if err := app.RunServerContext(ctx, container.Server()); err != nil {
		return fmt.Errorf("iniciar servidor: %w", err)
	}
	return nil
}
