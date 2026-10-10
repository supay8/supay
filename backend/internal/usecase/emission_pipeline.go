package usecase

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

// dispatchInvoiceEmission explicitly orders the effectful stages. Adapters
// implementing the pipeline cannot contact SIAT until both storage writes pass.
func (uc *InvoiceUsecase) dispatchInvoiceEmission(ctx context.Context, inv *domain.Invoice, svc ports.FiscalSingle, doc ports.FiscalDocument) (result ports.FiscalResult, err error) {
	pipeline, ok := svc.(ports.FiscalEmissionPipeline)
	if !ok {
		return svc.Emit(ctx, doc)
	}
	dispatched := false
	defer func() {
		if err != nil && !dispatched {
			err = &emissionPreparationError{err}
		}
	}()
	store, ok := uc.invoiceRepo.(ports.EmissionPreparationStore)
	if !ok {
		return ports.FiscalResult{}, fmt.Errorf("persistencia de preparación fiscal no configurada")
	}
	prepared, err := restoreEmissionArtifacts(inv)
	if err != nil {
		return ports.FiscalResult{}, err
	}
	if prepared.Archivo == "" {
		prepared, err = pipeline.PrepareEmission(ctx, doc)
	}
	if err != nil {
		return ports.FiscalResult{}, err
	}
	applyEmissionArtifacts(inv, prepared)
	if uc.fileService == nil {
		return ports.FiscalResult{}, fmt.Errorf("storage de documentos fiscales no configurado")
	}
	file, created, err := uc.fileService.SaveWithStatus(ctx, inv.CompanyId, inv.ID, prepared.Cuf, "xml", strings.NewReader(prepared.Xml), int64(len(prepared.Xml)))
	if err != nil {
		return ports.FiscalResult{}, fmt.Errorf("persistir XML antes del envío: %w", err)
	}
	inv.XmlHash = &file.SHA256
	if err = store.StorePreparedEmission(ctx, inv); err != nil {
		if created {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			err = errors.Join(err, uc.fileService.RemoveCreated(cleanupCtx, file))
		}
		return ports.FiscalResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return ports.FiscalResult{}, err
	}
	dispatched = true
	return pipeline.DispatchEmission(ctx, doc, prepared)
}

// applyEmissionArtifacts is a pure projection: it never reads storage or sends requests.
func applyEmissionArtifacts(inv *domain.Invoice, result ports.FiscalResult) {
	if result.Cuf != "" {
		inv.Cuf = &result.Cuf
	}
	if result.Xml != "" {
		inv.Xml = &result.Xml
	}
	if result.XmlHash != "" {
		inv.XmlHash = &result.XmlHash
	}
	if result.Archivo != "" {
		inv.Archivo = result.Archivo
		inv.HashArchivo = result.XmlHash
	}
}

func emissionOutcome(result ports.FiscalResult) (domain.InvoiceStatus, domain.InvoiceTransitionReason) {
	if !result.Transaccion {
		return domain.InvoiceRejected, domain.TransitionSIATRejected
	}
	if result.CodigoEstado == 904 {
		return domain.InvoiceObserved, domain.TransitionSIATObserved
	}
	return domain.InvoiceAccepted, domain.TransitionSIATAccepted
}

// restoreEmissionArtifacts verifies and reuses the exact signed bytes reserved
// by a previous attempt. Re-signing the same CUF would change immutable objects.
func restoreEmissionArtifacts(inv *domain.Invoice) (ports.FiscalResult, error) {
	if inv.Archivo == "" {
		return ports.FiscalResult{}, nil
	}
	if inv.Cuf == nil || *inv.Cuf == "" || inv.HashArchivo == "" {
		return ports.FiscalResult{}, fmt.Errorf("preparación fiscal incompleta; requiere conciliación")
	}
	data, err := base64.StdEncoding.DecodeString(inv.Archivo)
	if err != nil {
		return ports.FiscalResult{}, fmt.Errorf("archivo fiscal inválido: %w", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != inv.HashArchivo {
		return ports.FiscalResult{}, fmt.Errorf("hash de preparación fiscal inválido")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return ports.FiscalResult{}, err
	}
	defer reader.Close()
	xml, err := io.ReadAll(io.LimitReader(reader, 16*1024*1024+1))
	if err != nil {
		return ports.FiscalResult{}, err
	}
	if len(xml) == 0 || len(xml) > 16*1024*1024 {
		return ports.FiscalResult{}, fmt.Errorf("tamaño XML de preparación inválido")
	}
	return ports.FiscalResult{Cuf: *inv.Cuf, Xml: string(xml), Archivo: inv.Archivo, XmlHash: inv.HashArchivo}, nil
}

// discardOnlinePreparation releases the immutable online file before replacing
// it with the distinct offline CUF. The atomic SENDING claim owns this operation.
func (uc *InvoiceUsecase) discardOnlinePreparation(ctx context.Context, inv *domain.Invoice) error {
	if inv.Archivo == "" {
		return nil
	}
	store, ok := uc.invoiceRepo.(ports.EmissionPreparationStore)
	if !ok {
		return nil
	} // legacy adapters do not stage online artifacts
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	_, file, err := uc.fileService.ReadAll(cleanupCtx, inv.CompanyId, inv.ID, "xml")
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if err == nil {
		if err = uc.fileService.RemoveCreated(cleanupCtx, file); err != nil {
			return err
		}
	}
	inv.Cuf, inv.Xml, inv.XmlHash = nil, nil, nil
	inv.Archivo, inv.HashArchivo = "", ""
	return store.StorePreparedEmission(cleanupCtx, inv)
}

type emissionPreparationError struct{ err error }

func (e *emissionPreparationError) Error() string { return e.err.Error() }

func (e *emissionPreparationError) Unwrap() error { return e.err }
