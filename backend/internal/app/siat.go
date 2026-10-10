package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/brandsrx/supay/internal/adapters/notification"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/adapters/siat/sandbox"
	"github.com/brandsrx/supay/internal/crypto"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/storage"
	"github.com/brandsrx/supay/internal/usecase"
)

type infrastructure struct {
	// Servicios de infraestructura
	cryptoSvc          *crypto.Service
	cipher             ports.SecretCipher
	certStorage        storage.CertStorage
	pdfStorage         pdf.Storage
	objectStorage      ports.Storage
	invoiceFileService *usecase.InvoiceFileService
	siatProvider       ports.FiscalServiceProvider
	pdfService         *pdf.Service
	notifier           ports.Notifier

	fiscalService ports.FiscalService
}

func configureStorage(c *App) error {

	var err error
	c.objectStorage, err = storage.NewObjectStorageFromConfig(context.Background(), c.cfg)
	if err != nil {
		return fmt.Errorf("Object storage no disponible: %v", err)
	}
	c.invoiceFileService = usecase.NewInvoiceFileService(c.objectStorage, postgres.NewPostgresInvoiceFileRepository(c.db), c.cfg.StoragePresignTTL)
	c.pdfService = pdf.NewService(c.db)
	c.pdfService.SetFileWriter(c.invoiceFileService)
	c.notifier = notification.NewWebhookNotifier(c.cfg.Maintenance.NotificationTimeout)
	return nil
}
func configureSIAT(c *App) error {
	if key := c.cfg.EncryptionKey; key != "" {
		var err error
		c.cryptoSvc, err = crypto.New(key)
		if err != nil {
			return fmt.Errorf("ENCRYPTION_KEY inválida: %v", err)
		}
		c.cipher = c.cryptoSvc
	}
	var err error
	c.certStorage, err = storage.NewCertStorageFromConfig(c.cfg)
	if err != nil {
		return fmt.Errorf("Cert storage no disponible (STORAGE_DRIVER=%s): %v", c.cfg.StorageDriver, err)
	}
	if c.cfg.SiatSandbox {
		c.fiscalService = sandbox.NewFiscalService()
		return nil
	}
	c.siatProvider = siat.NewFiscalProvider(siat.NewSiatClientProviderWithStorage(
		c.companyRepo,
		c.certificateRepo,
		c.cryptoSvc,
		c.certStorage,
		siat.ProviderInfra{
			BaseURL:        c.cfg.SiatInfra.BaseURL,
			CodigoAmbiente: c.cfg.SiatInfra.CodigoAmbiente,
			CodigoSistema:  c.cfg.SiatInfra.CodigoSistema,
			Timeout:        c.cfg.SiatInfra.Timeout,
			TraceId:        c.cfg.SiatInfra.TraceId,
			UserAgent:      c.cfg.SiatInfra.UserAgent,
			Modalidad:      c.cfg.SiatInfra.Modalidad,
		},
	))
	// Deprecated: puente de Config.SIAT para consumidores embebidos.
	// Sunset: 2026-12-31. Migrar credenciales/certificados por empresa y usar
	// exclusivamente siatProvider; retirar este bloque y Config.SIAT juntos.
	legacy := c.cfg.SIAT
	if legacy.Token != "" || legacy.Nit != 0 {
		if err := legacy.Validate(); err != nil {
			return fmt.Errorf("configuración SIAT: %w", err)
		}
		svc, err := siat.NewService(legacy)
		if err != nil {
			return err
		}
		c.fiscalService = siat.NewFiscalAdapter(svc)
		slog.Warn("puente Config.SIAT legacy activo; migrar credenciales por empresa a FiscalServiceProvider", "sunset", "2026-12-31")
	}
	return nil

}
