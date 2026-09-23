package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/storage"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/joho/godotenv"
)

type legacyInvoiceXML struct {
	ID        string `gorm:"column:id"`
	CompanyID string `gorm:"column:company_id"`
	CUF       string `gorm:"column:cuf"`
	XML       string `gorm:"column:xml"`
}

func main() {
	_ = godotenv.Load()
	cfg := appconfig.Load()
	db := database.ConnectDB()

	var legacyColumnCount int64
	if err := db.Raw(`
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'invoices' AND column_name = 'xml'
	`).Scan(&legacyColumnCount).Error; err != nil {
		log.Fatalf("verificar columna legacy: %v", err)
	}
	if legacyColumnCount == 0 {
		log.Print("backfill no requerido: invoices.xml ya fue retirado")
		return
	}

	var rows []legacyInvoiceXML
	if err := db.Raw(`
		WITH legacy_xml AS (
		    SELECT i.id, i.tenant_id AS company_id, COALESCE(i.cuf, '') AS cuf,
		           i.xml, i.created_at, 0 AS source_order
		    FROM invoices i
		    WHERE i.xml IS NOT NULL AND btrim(i.xml) <> ''
		    UNION ALL
		    SELECT i.id, i.tenant_id, COALESCE(i.cuf, ''),
		           d.content, d.created_at, 1
		    FROM invoice_documents d
		    JOIN invoices i ON i.id = d.invoice_id
		    WHERE d.document_type = 'XML'
		      AND d.content IS NOT NULL
		      AND btrim(d.content) <> ''
		)
		SELECT id, company_id, cuf, xml
		FROM legacy_xml
		ORDER BY created_at, id, source_order
	`).Scan(&rows).Error; err != nil {
		log.Fatalf("listar XML históricos: %v", err)
	}
	if len(rows) == 0 {
		log.Print("backfill completado: no existen XML históricos pendientes")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	objectStorage, err := storage.NewObjectStorageFromConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("inicializar object storage: %v", err)
	}
	if closer, ok := objectStorage.(interface{ Close() error }); ok {
		defer closer.Close()
	}
	fileRepo := postgres.NewPostgresInvoiceFileRepository(db)
	files := usecase.NewInvoiceFileService(
		objectStorage,
		fileRepo,
		cfg.StoragePresignTTL,
	)

	for i, row := range rows {
		if strings.TrimSpace(row.CUF) == "" {
			log.Fatalf("factura %s tiene XML pero no CUF; requiere conciliación manual", row.ID)
		}
		itemCtx, itemCancel := context.WithTimeout(ctx, 2*time.Minute)
		// Una caída antigua pudo dejar metadata sin objeto. Mientras invoices.xml
		// todavía existe, es seguro retirar solo esa metadata rota y reconstruirla.
		if existing, findErr := fileRepo.FindFile(itemCtx, row.CompanyID, row.ID, "xml"); findErr == nil {
			if _, statErr := objectStorage.Stat(itemCtx, existing.StorageKey); errors.Is(statErr, domain.ErrNotFound) {
				if deleteErr := fileRepo.DeleteFile(itemCtx, row.CompanyID, row.ID, "xml", existing.StorageKey); deleteErr != nil {
					itemCancel()
					log.Fatalf("retirar metadata rota de factura %s: %v", row.ID, deleteErr)
				}
			}
		}
		_, saveErr := files.Save(itemCtx, row.CompanyID, row.ID, row.CUF, "xml", strings.NewReader(row.XML), int64(len(row.XML)))
		if saveErr != nil {
			itemCancel()
			log.Fatalf("respaldar factura %s (%d/%d): %v", row.ID, i+1, len(rows), saveErr)
		}
		storedXML, _, readErr := files.ReadAll(itemCtx, row.CompanyID, row.ID, "xml")
		itemCancel()
		if readErr != nil {
			log.Fatalf("verificar bytes respaldados de factura %s (%d/%d): %v", row.ID, i+1, len(rows), readErr)
		}
		if string(storedXML) != row.XML {
			log.Fatalf("verificar bytes respaldados de factura %s (%d/%d): el contenido no coincide", row.ID, i+1, len(rows))
		}
		log.Printf("XML histórico respaldado %d/%d: %s", i+1, len(rows), row.ID)
	}

	var missing int64
	if err := db.Raw(`
		WITH legacy_xml AS (
		    SELECT i.tenant_id AS company_id, i.id AS invoice_id, i.xml AS content
		    FROM invoices i
		    WHERE i.xml IS NOT NULL AND btrim(i.xml) <> ''
		    UNION ALL
		    SELECT i.tenant_id, i.id, d.content
		    FROM invoice_documents d
		    JOIN invoices i ON i.id = d.invoice_id
		    WHERE d.document_type = 'XML'
		      AND d.content IS NOT NULL
		      AND btrim(d.content) <> ''
		)
		SELECT count(*)
		FROM legacy_xml x
		WHERE NOT EXISTS (
		      SELECT 1 FROM invoice_files f
		      WHERE f.company_id = x.company_id
		        AND f.invoice_id = x.invoice_id
		        AND f.kind = 'xml'
		        AND lower(btrim(f.sha256)) = encode(digest(x.content, 'sha256'), 'hex')
		  )
	`).Scan(&missing).Error; err != nil {
		log.Fatalf("verificar backfill: %v", err)
	}
	if missing != 0 {
		log.Fatalf("backfill incompleto: %d XML no tienen respaldo verificado", missing)
	}
	log.Printf("backfill completado: %d XML respaldados y verificados", len(rows))
}
