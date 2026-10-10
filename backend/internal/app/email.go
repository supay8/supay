package app

import (
	"context"
	"fmt"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/brandsrx/supay/internal/invoiceemail"
)

func configureEmailDispatcher(c *App) error {
	if !c.cfg.InvoiceEmail.Enabled || c.cfg.RunMode.RunsEmailWorker() {
		return nil
	}

	client, err := cloudtasks.NewClient(context.Background())
	if err != nil {
		return fmt.Errorf("Cloud Tasks no disponible: %v", err)
	}
	publisher, err := invoiceemail.NewCloudTasksPublisher(client, invoiceemail.CloudTasksConfig{
		ProjectID: c.cfg.InvoiceEmail.ProjectID, Location: c.cfg.InvoiceEmail.Location,
		Queue: c.cfg.InvoiceEmail.Queue, WorkerURL: c.cfg.InvoiceEmail.WorkerURL,
		ServiceAccountEmail: c.cfg.InvoiceEmail.TaskServiceAccountEmail,
		Audience:            c.cfg.InvoiceEmail.Audience,
	})
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("configurar publicación de emails: %v", err)
	}
	c.emailTasksClient = client
	c.emailDispatcher = invoiceemail.NewDispatcher(c.emailNotificationRepo, publisher, c.cfg.InvoiceEmail.DispatchBatchSize, c.cfg.InvoiceEmail.PublishLockTimeout)
	return nil
}
func configureEmailWorker(c *App) error {

	sender, err := invoiceemail.NewSMTPSender(invoiceemail.SMTPConfig{
		Host: c.cfg.InvoiceEmail.SMTPHost, Port: c.cfg.InvoiceEmail.SMTPPort,
		Username: c.cfg.InvoiceEmail.SMTPUsername, Password: c.cfg.InvoiceEmail.SMTPPassword,
		From: c.cfg.InvoiceEmail.From, FromName: c.cfg.InvoiceEmail.FromName,
		Timeout: c.cfg.InvoiceEmail.SMTPTimeout,
	})
	if err != nil {
		return fmt.Errorf("configurar SMTP de facturas: %v", err)
	}
	c.emailProcessor = invoiceemail.NewProcessor(c.emailNotificationRepo, c.invoiceRepo, c.invoiceFileService, c.pdfService, sender, c.cfg.InvoiceEmail.DeliveryLockTimeout)

	c.emailTaskHandler = invoiceemail.NewHandler(c.emailProcessor, nil, c.cfg.InvoiceEmail.Audience, c.cfg.InvoiceEmail.TaskServiceAccountEmail)
	return nil
}
