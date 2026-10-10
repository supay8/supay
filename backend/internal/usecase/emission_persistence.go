package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

func (uc *InvoiceUsecase) prepareCredentialsForEmission(ctx context.Context, inv *domain.Invoice) error {
	if uc.credentials == nil {
		return nil
	}
	company := inv.Company
	pos := inv.PointOfSale
	if err := uc.credentials.EnsureCuis(ctx, &company, &pos); err != nil {
		return err
	}
	cufd, err := uc.credentials.EnsureCufd(ctx, &company, &pos)
	if err != nil {
		return err
	}
	if cufd == nil || cufd.ID == "" {
		return domain.NewConflictError("no se pudo obtener un cufd vigente antes de emitir")
	}
	inv.Company = company
	inv.PointOfSale = pos
	inv.CufdId = cufd.ID
	inv.CufdRecord = *cufd
	return nil
}

// persistResultadoConReintentos persiste el resultado de la emisión reintentando
// ante fallos transitorios del repositorio. Es crítico: el SIAT ya aceptó (o
// rechazó) la factura, así que perder el CUF dejaría la factura irrecuperable
// por API. Si aun así falla, se loguea a nivel crítico con los datos para
// conciliar manualmente (el reaper devolverá la factura a PENDING y el reenvío
// con el mismo numeroFactura/CUF es idempotente ante el SIAT).
func (uc *InvoiceUsecase) persistResultadoConReintentos(inv *domain.Invoice, result ports.FiscalResult, reason domain.InvoiceTransitionReason) error {
	const maxIntentos = 3
	var err error
	for intento := 1; intento <= maxIntentos; intento++ {
		fields := map[string]any{
			"cuf": inv.Cuf, "xml_hash": inv.XmlHash,
			"archivo": inv.Archivo, "hash_archivo": inv.HashArchivo,
			"siat_reception_code": inv.SiatReceptionCode, "siat_mensajes": inv.SiatMensajes,
			"cufd_id": inv.CufdId,
		}
		event := invoiceTransitionEvent(inv, domain.InvoiceSending, inv.Status, reason, map[string]any{
			"source": "Emit", "codigo_estado": result.CodigoEstado,
			"codigo_recepcion": result.CodigoRecepcion, "transaccion": result.Transaccion,
		})
		var claimed bool
		claimed, err = uc.invoiceRepo.TransitionStatus(inv.CompanyId, inv.ID, domain.InvoiceSending, inv.Status, reason, fields, event)
		if err == nil && !claimed {
			return domain.NewConflictError("la factura cambió de estado mientras se persistía la respuesta del SIAT")
		}
		if err == nil {
			return nil
		}
		slog.Error("emisión: fallo al persistir resultado del SIAT",
			"invoice_id", inv.ID, "intento", intento, "error", err)
		time.Sleep(time.Duration(intento) * 200 * time.Millisecond)
	}
	slog.Error("emisión: resultado SIAT NO PERSISTIDO tras reintentos — conciliar manualmente",
		"invoice_id", inv.ID,
		"numero_factura", inv.InvoiceNumber,
		"punto_venta_id", inv.PointOfSaleId,
		"cuf", result.Cuf,
		"codigo_recepcion", result.CodigoRecepcion,
		"codigo_estado", result.CodigoEstado,
		"transaccion", result.Transaccion)
	return fmt.Errorf("no se pudo persistir el resultado de la emisión tras %d intentos: %w", maxIntentos, err)
}

func (uc *InvoiceUsecase) persistSignedXML(ctx context.Context, inv *domain.Invoice) error {
	if inv == nil || inv.Xml == nil || strings.TrimSpace(*inv.Xml) == "" {
		return errors.New("el resultado fiscal no contiene XML firmado")
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return errors.New("el resultado fiscal no contiene CUF")
	}
	if uc.fileService == nil {
		return errors.New("storage de documentos fiscales no configurado")
	}
	file, err := uc.fileService.Save(ctx, inv.CompanyId, inv.ID, *inv.Cuf, "xml", strings.NewReader(*inv.Xml), int64(len(*inv.Xml)))
	if err != nil {
		return err
	}
	// El hash canónico es el calculado sobre los bytes realmente persistidos,
	// no un valor declarado por el adaptador fiscal.
	inv.XmlHash = &file.SHA256
	return nil
}

func invoiceTransitionEvent(inv *domain.Invoice, from, to domain.InvoiceStatus, reason domain.InvoiceTransitionReason, details map[string]any) *domain.InvoiceEvent {
	payload := map[string]any{
		"from_status": string(from),
		"to_status":   string(to),
		"reason":      string(reason),
	}
	for key, value := range details {
		payload[key] = value
	}
	encoded, _ := json.Marshal(payload)
	return &domain.InvoiceEvent{
		InvoiceID: inv.ID,
		TenantID:  inv.CompanyId,
		Type:      "STATUS_TRANSITION",
		Message:   fmt.Sprintf("invoice status changed from %s to %s", from, to),
		Payload:   encoded,
	}
}

// marshalMensajes serializa los mensajes de la respuesta SIAT a JSON para
// persistirlos en la factura (siat_mensajes) y poder diagnosticar observaciones.
func marshalMensajes(msgs []ports.FiscalMessage) (string, error) {
	if len(msgs) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(msgs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
