package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/invoiceemail"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("email dispatcher: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()
	cfg := config.Load()
	if err := cfg.ValidateInvoiceEmail(); err != nil {
		return err
	}
	if !cfg.InvoiceEmail.Enabled {
		return fmt.Errorf("INVOICE_EMAIL_ENABLED no está activo")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	publisher, err := invoiceemail.NewCloudTasksPublisher(client, invoiceemail.CloudTasksConfig{
		ProjectID: cfg.InvoiceEmail.ProjectID, Location: cfg.InvoiceEmail.Location,
		Queue: cfg.InvoiceEmail.Queue, WorkerURL: cfg.InvoiceEmail.WorkerURL,
		ServiceAccountEmail: cfg.InvoiceEmail.TaskServiceAccountEmail,
		Audience:            cfg.InvoiceEmail.Audience,
	})
	if err != nil {
		return err
	}
	db, err := database.ConnectDB()
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	dispatcher := invoiceemail.NewDispatcher(
		postgres.NewPostgresInvoiceEmailNotificationRepository(db),
		publisher, cfg.InvoiceEmail.DispatchBatchSize, cfg.InvoiceEmail.PublishLockTimeout,
	)
	for {
		count, dispatchErr := dispatcher.DispatchBatch(ctx)
		if dispatchErr != nil {
			return dispatchErr
		}
		if count < cfg.InvoiceEmail.DispatchBatchSize {
			return nil
		}
	}
}
