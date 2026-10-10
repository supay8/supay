package usecase

import (
	"context"
	"fmt"
	"github.com/brandsrx/supay/internal/domain"
	"log/slog"
	"strings"
	"time"
)

func (uc *SiatUsecase) persistBatchDocuments(ctx context.Context, batch invoiceBatch) ([]*domain.InvoiceFile, error) {
	if uc.fileService == nil {
		return nil, domain.NewConflictError("El storage de documentos fiscales no está configurado")
	}
	created := make([]*domain.InvoiceFile, 0, len(batch.pkg.Documents))
	for i, document := range batch.pkg.Documents {
		file, wasCreated, err := uc.fileService.SaveWithStatus(
			ctx, batch.pkg.CompanyId, batch.pkg.InvoiceIDs[i], document.Cuf, "xml",
			strings.NewReader(document.Xml), int64(len(document.Xml)),
		)
		if err != nil {
			uc.cleanupBatchDocuments(created)
			return nil, fmt.Errorf("persistir XML firmado de factura %s: %w", batch.pkg.InvoiceIDs[i], err)
		}
		if file.SHA256 != document.XmlHash {
			if wasCreated {
				uc.cleanupBatchDocuments([]*domain.InvoiceFile{file})
			}
			uc.cleanupBatchDocuments(created)
			return nil, fmt.Errorf("hash del XML persistido de factura %s inconsistente", batch.pkg.InvoiceIDs[i])
		}
		if wasCreated {
			created = append(created, file)
		}
	}
	return created, nil
}

func (uc *SiatUsecase) cleanupBatchDocuments(files []*domain.InvoiceFile) {
	if uc.fileService == nil {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, file := range files {
		if err := uc.fileService.RemoveCreated(cleanupCtx, file); err != nil {
			// La metadata conserva suficiente información para una conciliación;
			// nunca ocultar un fallo de compensación de un documento fiscal.
			slog.Error("no se pudo compensar XML preparado", "storage_key", file.StorageKey, "error", err)
		}
	}
}
