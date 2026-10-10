package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/brandsrx/supay/internal/app"
	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Supay finalizó con error: %v", err)
	}
}

func run() (err error) {
	_ = godotenv.Load()
	app.SetupLogging()
	log.Println("Iniciando Supay API...")

	cfg := appconfig.Load()

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	application, err := app.NewApp(cfg)
	if err != nil {
		return fmt.Errorf("inicializar aplicación: %w", err)
	}
	defer func() { err = errors.Join(err, application.Close()) }()

	if cfg.RunMode.RunsWorker() {
		stopMaintenance := app.StartMaintenanceScheduler(ctx, application.MaintenanceService(), cfg.Maintenance)
		defer stopMaintenance()
		stopReaper := app.StartStaleEmissionReaper(ctx, application.InvoiceRepo())
		defer stopReaper()

		queue := application.EmissionQueue()
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
	if err := app.RunServerContext(ctx, application.Server()); err != nil {
		return fmt.Errorf("iniciar servidor: %w", err)
	}
	return nil
}
