package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"gorm.io/gorm"
)

// EmissionRejectedError indica que el SIAT respondió y rechazó la factura
// (Transaccion=false). La factura queda persistida como REJECTED.
type EmissionRejectedError struct {
	CodigoEstado    int
	CodigoRecepcion string
	Mensajes        []ports.FiscalMessage
}

// EmissionInProgressError maps concurrent emission attempts to HTTP 409.
type EmissionInProgressError struct{}

func (e *EmissionInProgressError) Error() string { return "la factura ya está en proceso de emisión" }
func (e *EmissionInProgressError) Unwrap() error { return domain.NewConflictError(e.Error()) }

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

func (e *EmissionRejectedError) Error() string {
	var parts []string
	for _, m := range e.Mensajes {
		parts = append(parts, fmt.Sprintf("[%d] %s", m.Codigo, m.Descripcion))
	}
	if len(parts) == 0 {
		return "la factura fue rechazada por el SIAT"
	}
	return "la factura fue rechazada por el SIAT: " + strings.Join(parts, "; ")
}

// Emit runs the complete emission inside the request. A normal POS sale must
// receive the SIAT result (or its offline contingency document) immediately;
// it must not wait for an internal retry queue.
func (uc *InvoiceUsecase) Emit(ctx context.Context, id string) (*domain.Invoice, error) {
	return uc.ProcessEmission(ctx, id)
}

// ProcessEmission emite al SIAT una factura en estado PENDING usando el SDK go-siat.
// El flujo cubre los prerrequisitos (CUIS/CUFD vigentes, sucursal/PV, cliente e
// ítems mapeados a catálogos SIN), la construcción con builders del SDK, la
// firma digital automática (modalidad electrónica) y la persistencia del
// resultado (CUF, XML, hash, código de recepción y estado).
func (uc *InvoiceUsecase) ProcessEmission(ctx context.Context, id string) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}
	if inv.CodigoDocumentoSector == 30 {
		return nil, domain.NewBadRequestError("el sector 30 requiere emisión masiva; use /v1/siat/masiva/{companyId}/{pointOfSaleId}")
	}
	if inv.Status != domain.InvoicePending {
		log.Println("DEBUG 1")

		if inv.Status == domain.InvoiceSending {
			log.Println("DEBUG 2")

			return nil, &EmissionInProgressError{}
		}
		return nil, domain.NewConflictError("solo se pueden emitir facturas en estado PENDING")
	}

	// La factura permanece PENDING mientras obtiene credenciales para no ocupar
	// SENDING con un documento que todavia no tiene CUIS/CUFD utilizable.
	if err := uc.prepareCredentialsForEmission(ctx, inv); err != nil {
		log.Println("DEBUG 3")

		return nil, err
	}

	// Claim atómico PENDING->SENDING: evita emisiones duplicadas concurrentes.
	claimed, err := uc.invoiceRepo.ClaimForEmission(id)
	if err != nil {
		log.Println("DEBUG 4")

		return nil, err
	}
	if !claimed {
		log.Println("DEBUG 5")

		return nil, &EmissionInProgressError{}
	}

	// Los errores que no son de conectividad revierten a PENDING. Un timeout o
	// una caida de red se resuelve mediante la contingencia oficial mas abajo.
	rollback := func() {
		event := invoiceTransitionEvent(inv, domain.InvoiceSending, domain.InvoicePending, domain.TransitionTransportFailure, map[string]any{"source": "Emit"})
		if _, err := uc.invoiceRepo.TransitionStatus(inv.ID, domain.InvoiceSending, domain.InvoicePending, domain.TransitionTransportFailure, nil, event); err != nil {
			log.Println("DEBUG 6")

			slog.Error("emisión: no se pudo revertir la factura a PENDING",
				"invoice_id", inv.ID, "error", err)
		}
		inv.Status = domain.InvoicePending
	}

	req, err := uc.buildSolicitudFactura(ctx, inv)
	if err != nil {
		log.Println("DEBUG 7")

		rollback()
		return nil, err
	}
	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		log.Println("DEBUG 8")

		rollback()
		return nil, err
	}
	result, err := svc.Emit(ctx, *req)
	if err != nil {
		log.Println("DEBUG 9")

		if isSIATConnectivityError(err) {
			log.Println("DEBUG 10")

			offline, contingencyErr := uc.processOfflineContingency(ctx, inv, svc, *req)
			if contingencyErr == nil {
				return offline, nil
			}
			log.Println("DEBUG 13")

			slog.Error("emisión: no se pudo activar contingencia offline",
				"invoice_id", inv.ID, "siat_error", err, "contingency_error", contingencyErr)
		}
		rollback()
		log.Println("DEBUG 12")
		return nil, fmt.Errorf("error de emisión: %w", err)
	}
	log.Println("14")
	if strings.TrimSpace(result.Cuf) != "" {
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
		if result.XmlHash != "" {
			inv.HashArchivo = result.XmlHash
		}
	}
	if result.CodigoRecepcion != "" {
		inv.SiatReceptionCode = &result.CodigoRecepcion
	}
	if msgs, err := marshalMensajes(result.Mensajes); err == nil {
		inv.SiatMensajes = &msgs
	}
	transitionReason := domain.TransitionSIATRejected
	if result.Transaccion {
		// Según el catálogo mensajesServicios del SIAT, 904 = RECEPCION OBSERVADA
		// (no es un caso correcto); solo 908 = RECEPCION VALIDADA.
		if result.CodigoEstado == 904 {
			inv.Status = domain.InvoiceObserved
			transitionReason = domain.TransitionSIATObserved
		} else {
			inv.Status = domain.InvoiceAccepted
			transitionReason = domain.TransitionSIATAccepted
		}
	} else {
		inv.Status = domain.InvoiceRejected
	}

	if err := uc.persistResultadoConReintentos(inv, result, transitionReason); err != nil {
		log.Println("DEBUG 11")

		return nil, err
	}
	if !result.Transaccion {
		return nil, &EmissionRejectedError{
			CodigoEstado:    result.CodigoEstado,
			CodigoRecepcion: result.CodigoRecepcion,
			Mensajes:        result.Mensajes,
		}
	}

	// El PDF forma parte del resultado sincrono que consume el POS.
	if uc.pdfService != nil && result.Transaccion {
		uc.pdfService.GenerateAndPersist(ctx, inv.ID)
	}

	return inv, nil
}

func isSIATConnectivityError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		return true
	}
	// Algunos clientes SOAP pierden el tipo concreto al envolver el error de
	// transporte. Se limita el fallback a mensajes inequívocos de conectividad.
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection refused", "connection reset", "network is unreachable",
		"no such host", "i/o timeout", "tls handshake timeout",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (uc *InvoiceUsecase) processOfflineContingency(
	ctx context.Context,
	inv *domain.Invoice,
	svc ports.FiscalService,
	req ports.FiscalDocument,
) (*domain.Invoice, error) {
	preparer, ok := svc.(ports.OfflineFiscalService)
	if !ok {
		return nil, errors.New("el adaptador SIAT no soporta generación offline")
	}
	if uc.contingencyRepo == nil {
		return nil, errors.New("repositorio de contingencias no configurado")
	}

	// El contexto HTTP puede haber vencido precisamente por el timeout del
	// SIAT. La construccion, firma y persistencia offline son operaciones locales.
	offlineCtx := context.WithoutCancel(ctx)
	result, err := preparer.PrepareOffline(offlineCtx, req)
	if err != nil {
		return nil, fmt.Errorf("generar factura offline: %w", err)
	}

	event, err := uc.openConnectivityContingency(inv)
	if err != nil {
		return nil, fmt.Errorf("registrar contingencia local: %w", err)
	}

	if strings.TrimSpace(result.Cuf) != "" {
		inv.Cuf = &result.Cuf
	}
	if result.Xml != "" {
		inv.Xml = &result.Xml
	}
	if result.XmlHash != "" {
		inv.XmlHash = &result.XmlHash
	}
	inv.Archivo = result.Archivo
	inv.HashArchivo = result.XmlHash
	inv.ContingencyEventId = &event.ID
	inv.EmissionType = "OFFLINE"
	inv.Status = domain.InvoiceOffline

	fields := map[string]any{
		"cuf": inv.Cuf, "xml": inv.Xml, "xml_hash": inv.XmlHash,
		"archivo": inv.Archivo, "hash_archivo": inv.HashArchivo,
		"cufd_id": inv.CufdId, "contingency_event_id": event.ID,
		"emission_type": inv.EmissionType,
	}
	transitionEvent := invoiceTransitionEvent(inv, domain.InvoiceSending, domain.InvoiceOffline, domain.TransitionContingency, map[string]any{
		"source": "Emit", "contingency_event_id": event.ID, "trigger": "SIAT_CONNECTIVITY_FAILURE",
	})
	claimed, err := uc.invoiceRepo.TransitionStatus(
		inv.ID, domain.InvoiceSending, domain.InvoiceOffline,
		domain.TransitionContingency, fields, transitionEvent,
	)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, domain.NewConflictError("la factura cambió de estado mientras se activaba la contingencia")
	}
	if uc.pdfService != nil {
		uc.pdfService.GenerateAndPersist(offlineCtx, inv.ID)
	}
	return inv, nil
}

func (uc *InvoiceUsecase) openConnectivityContingency(inv *domain.Invoice) (*domain.ContingencyEvent, error) {
	latest, err := uc.contingencyRepo.GetLatestByPointOfSale(inv.PointOfSaleId)
	if err == nil && latest != nil && !latest.IsSynced && latest.EndDate == nil {
		return latest, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	description := "Caída de red detectada durante la emisión; pendiente de registro ante el SIAT"
	start := inv.IssueDate
	if start.IsZero() {
		start = time.Now().In(siat.LaPaz)
	}
	event := &domain.ContingencyEvent{
		PointOfSaleID: inv.PointOfSaleId,
		Reason:        "FALLA_CONEXION_INTERNET",
		Description:   &description,
		StartDate:     start,
		IsSynced:      false,
	}
	if err := uc.contingencyRepo.Create(event); err != nil {
		return nil, err
	}
	return event, nil
}

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
			"cuf": inv.Cuf, "xml": inv.Xml, "xml_hash": inv.XmlHash,
			"archivo": inv.Archivo, "hash_archivo": inv.HashArchivo,
			"siat_reception_code": inv.SiatReceptionCode, "siat_mensajes": inv.SiatMensajes,
			"cufd_id": inv.CufdId,
		}
		event := invoiceTransitionEvent(inv, domain.InvoiceSending, inv.Status, reason, map[string]any{
			"source": "Emit", "codigo_estado": result.CodigoEstado,
			"codigo_recepcion": result.CodigoRecepcion, "transaccion": result.Transaccion,
		})
		var claimed bool
		claimed, err = uc.invoiceRepo.TransitionStatus(inv.ID, domain.InvoiceSending, inv.Status, reason, fields, event)
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

// VerifyStatus consulta al SIAT el estado real de un documento emitido
// (verificacionEstadoFactura) y reconcilia el estado local de la factura con el
// CodigoEstado devuelto (según el catálogo mensajesServicios). Un error de
// transporte no modifica el estado local.
func (uc *InvoiceUsecase) VerifyStatus(ctx context.Context, id string) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, domain.NewConflictError("la factura no ha sido emitida (no tiene cuf asignado)")
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		return nil, err
	}
	result, err := svc.VerifyStatus(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("error de verificación: %w", err)
	}

	if estado, ok := siatEstadoToDomain(result.CodigoEstado); ok && inv.Status != estado {
		event := invoiceTransitionEvent(inv, inv.Status, estado, domain.TransitionSIATReconciliation, map[string]any{"source": "VerifyStatus", "codigo_estado": result.CodigoEstado})
		claimed, err := uc.invoiceRepo.TransitionStatus(inv.ID, inv.Status, estado, domain.TransitionSIATReconciliation, nil, event)
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, domain.NewConflictError("la factura cambió de estado durante la reconciliación SIAT")
		}
		inv.Status = estado
	}

	return inv, nil
}

// Annul anula ante el SIAT una factura emitida (anulacionFactura) con el motivo
// del catálogo sincronizado motivoAnulacion. Al ser aceptada (905 ANULACION
// CONFIRMADA) se persiste el estado, el motivo y la fecha de anulación.
func (uc *InvoiceUsecase) Annul(ctx context.Context, id string, codigoMotivo int) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}

	if inv.Status != domain.InvoiceAccepted {
		return nil, domain.NewConflictError("solo se pueden anular facturas en estado ACCEPTED")
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, domain.NewConflictError("la factura no tiene cuf asignado")
	}
	if err := uc.validateMotivoAnulacion(inv.CompanyId, codigoMotivo); err != nil {
		return nil, err
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}

	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		return nil, err
	}
	result, err := svc.Annul(ctx, *req, codigoMotivo)
	if err != nil {
		return nil, fmt.Errorf("error de anulación: %w", err)
	}

	if !result.Transaccion {
		return nil, &EmissionRejectedError{
			CodigoEstado:    result.CodigoEstado,
			CodigoRecepcion: result.CodigoRecepcion,
			Mensajes:        result.Mensajes,
		}
	}

	now := time.Now()
	fields := map[string]any{
		"motivo_anulacion": codigoMotivo,
		"fecha_anulacion":  now,
	}
	if result.CodigoRecepcion != "" {
		fields["siat_reception_code"] = result.CodigoRecepcion
	}
	// Transición condicional ACCEPTED->CANCELLED: si otra anulación concurrente
	// ya la aplicó, aquí llega false en lugar de pisar el estado.
	event := invoiceTransitionEvent(inv, domain.InvoiceAccepted, domain.InvoiceCancelled, domain.TransitionCancellation, map[string]any{"source": "Annul", "codigo_motivo": codigoMotivo})
	claimed, err := uc.invoiceRepo.TransitionStatus(id, domain.InvoiceAccepted, domain.InvoiceCancelled, domain.TransitionCancellation, fields, event)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, domain.NewConflictError("la factura ya no está en estado ACCEPTED (posible anulación concurrente)")
	}

	inv.Status = domain.InvoiceCancelled
	inv.MotivoAnulacion = &codigoMotivo
	inv.FechaAnulacion = &now
	if code, ok := fields["siat_reception_code"]; ok {
		codeStr := code.(string)
		inv.SiatReceptionCode = &codeStr
	}

	return inv, nil
}

// RevertAnnul revierte una anulación aceptada por el SIAT
// (reversionAnulacionFactura; 907 REVERSION DE ANULACION CONFIRMADA),
// devolviendo la factura a ACCEPTED y limpiando el motivo y la fecha de
// anulación persistidos.
func (uc *InvoiceUsecase) RevertAnnul(ctx context.Context, id string) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("factura no encontrada")
		}
		return nil, err
	}
	if inv.Status != domain.InvoiceCancelled {
		return nil, domain.NewConflictError("solo se pueden revertir anulaciones de facturas en estado CANCELLED")
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, domain.NewConflictError("la factura no tiene cuf asignado")
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveEmissionService(ctx, inv.CompanyId)
	if err != nil {
		return nil, err
	}
	result, err := svc.RevertAnnul(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("error de reversión de anulación: %w", err)
	}

	if !result.Transaccion {
		return nil, &EmissionRejectedError{
			CodigoEstado:    result.CodigoEstado,
			CodigoRecepcion: result.CodigoRecepcion,
			Mensajes:        result.Mensajes,
		}
	}

	fields := map[string]any{
		"motivo_anulacion": nil,
		"fecha_anulacion":  nil,
	}
	if result.CodigoRecepcion != "" {
		fields["siat_reception_code"] = result.CodigoRecepcion
	}
	// Transición condicional CANCELLED->ACCEPTED (simétrica a Annul).
	event := invoiceTransitionEvent(inv, domain.InvoiceCancelled, domain.InvoiceAccepted, domain.TransitionCancellationRevert, map[string]any{"source": "RevertAnnul"})
	claimed, err := uc.invoiceRepo.TransitionStatus(id, domain.InvoiceCancelled, domain.InvoiceAccepted, domain.TransitionCancellationRevert, fields, event)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, domain.NewConflictError("la factura ya no está en estado CANCELLED (posible reversión concurrente)")
	}

	inv.Status = domain.InvoiceAccepted
	inv.MotivoAnulacion = nil
	inv.FechaAnulacion = nil
	if code, ok := fields["siat_reception_code"]; ok {
		codeStr := code.(string)
		inv.SiatReceptionCode = &codeStr
	}

	return inv, nil
}

// buildSolicitudDocumento reúne la identificación del documento ya emitido para
// las operaciones de verificación, anulación y reversión de anulación. El CUFD
// que se envía es el VIGENTE del punto de venta (GetActiveByPos), porque el SIAT
// rechaza operaciones firmadas con un CUFD vencido (vigencia ~24h); si no hay
// CUFD vigente registrado se cae al CUFD con el que se emitió la factura.
func (uc *InvoiceUsecase) buildSolicitudDocumento(inv *domain.Invoice) (*ports.FiscalDocumentQuery, error) {
	pos := inv.PointOfSale
	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		return nil, domain.NewConflictError("el punto de venta no tiene cuis activo")
	}

	// Obtener CUFD vigente del punto de venta, o caer al registrado con la factura.
	var cufd *domain.Cufd
	if uc.cufdRepo != nil {
		if active, err := uc.cufdRepo.GetActiveByPos(inv.PointOfSaleId); err == nil && active != nil {
			cufd = active
		}
	}
	if cufd == nil {
		if inv.CufdRecord.ID != "" && strings.TrimSpace(inv.CufdRecord.Cufd) != "" {
			cufd = &inv.CufdRecord
		}
	}
	if cufd == nil || strings.TrimSpace(cufd.Cufd) == "" {
		return nil, domain.NewConflictError("la factura no tiene un cufd vigente asociado; solicítelo primero")
	}

	// Validar vigencia del CUFD: el SIAT rechaza operaciones con CUFD vencido.
	now := time.Now().In(siat.LaPaz)
	if now.Before(cufd.ValidFrom) || now.After(cufd.ValidTo) {
		return nil, fmt.Errorf("el CUFD está vencido (válido desde %s hasta %s); solicite uno nuevo (POST /siat/cufd/...)",
			cufd.ValidFrom.In(siat.LaPaz).Format("2006-01-02 15:04"),
			cufd.ValidTo.In(siat.LaPaz).Format("2006-01-02 15:04"))
	}

	codigoPuntoVenta := pos.CodigoPuntoVenta
	if pos.SiatCode != nil {
		codigoPuntoVenta = *pos.SiatCode
	}

	modalidad := inv.Modalidad
	if modalidad <= 0 {
		modalidad = uc.effectiveModalidadForCompany(&inv.Company)
	}
	if modalidad <= 0 {
		modalidad = siat.ModalidadElectronica
	}

	return &ports.FiscalDocumentQuery{
		CodigoAmbiente:        inv.Company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         "",
		Nit:                   inv.Company.Nit,
		Modalidad:             modalidad,
		Layout:                inv.Layout,
		Cuf:                   *inv.Cuf,
		CodigoSucursal:        pos.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pos.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoDocumentoSector: inv.CodigoDocumentoSector,
		CodigoTipoFactura:     inv.CodigoTipoFactura,
	}, nil
}

// validateMotivoAnulacion valida que el motivo exista en el catálogo
// sincronizado motivoAnulacion del SIAT.
func (uc *InvoiceUsecase) validateMotivoAnulacion(companyID string, codigoMotivo int) error {
	if uc.catalogRepo == nil {
		return nil
	}
	items, err := uc.catalogRepo.List(companyID, "motivoAnulacion")
	if err != nil {
		return fmt.Errorf("no se pudo consultar el catálogo de motivos de anulación: %w", err)
	}
	for _, item := range items {
		if item.Codigo == codigoMotivo {
			return nil
		}
	}
	return domain.NewBadRequestError(fmt.Sprintf("motivo de anulación %d no es válido; consulte el catálogo motivoAnulacion", codigoMotivo))
}

// siatEstadoToDomain mapea el CodigoEstado de una respuesta de verificación del
// SIAT al estado de dominio local, según el catálogo mensajesServicios:
// 902 RECEPCION RECHAZADA, 904 RECEPCION OBSERVADA, 905 ANULACION CONFIRMADA,
// 907 REVERSION DE ANULACION CONFIRMADA, 908 RECEPCION VALIDADA.
func siatEstadoToDomain(codigoEstado int) (domain.InvoiceStatus, bool) {
	switch codigoEstado {
	case 908, 907:
		return domain.InvoiceAccepted, true
	case 904:
		return domain.InvoiceObserved, true
	case 902, 906, 909:
		return domain.InvoiceRejected, true
	case 905:
		return domain.InvoiceCancelled, true
	default:
		return "", false
	}
}

// clienteFromCustomer construye el bloque ClienteFactura del SIAT desde el
// snapshot embebido en la factura. Nunca consulta la dimensión customers.
func clienteFromCustomer(c domain.Customer) (ports.FiscalCustomer, error) {
	if strings.TrimSpace(c.DocumentNumber) == "" || strings.TrimSpace(c.Name) == "" {
		return ports.FiscalCustomer{}, domain.NewConflictError("factura sin cliente asociado; toda factura debe referenciar un cliente")
	}
	codigoDoc, err := codigoTipoDocumentoIdentidad(c.DocumentType)
	if err != nil {
		return ports.FiscalCustomer{}, err
	}
	complemento := c.Complement
	var codigoCliente *string
	if strings.TrimSpace(c.CodigoCliente) != "" {
		codigo := c.CodigoCliente
		codigoCliente = &codigo
		// SIAT XSD requiere que 'complemento' esté presente antes que
		// 'codigoCliente'. Si hay codigoCliente pero no complemento, se envía
		// string vacío (no nil) para mantener el orden del XSD y evitar el
		// rechazo 920.
		if complemento == nil {
			empty := ""
			complemento = &empty
		}
	}
	return ports.FiscalCustomer{
		NombreRazonSocial:            c.Name,
		CodigoTipoDocumentoIdentidad: codigoDoc,
		NumeroDocumento:              c.DocumentNumber,
		Complemento:                  complemento,
		CodigoCliente:                codigoCliente,
	}, nil
}

// buildSolicitudFactura reúne los prerrequisitos de la factura y los mapea a
// los códigos de catálogo SIN esperados por el SDK.
func (uc *InvoiceUsecase) buildSolicitudFactura(ctx context.Context, inv *domain.Invoice) (*ports.FiscalDocument, error) {
	company := inv.Company
	pos := inv.PointOfSale

	// CUIS lazy: si falta y hay servicio de credenciales, se solicita en
	// línea; si no, se mantiene el error accionable.
	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		if uc.credentials != nil {
			if err := uc.credentials.EnsureCuis(ctx, &company, &pos); err != nil {
				return nil, err
			}
		}
	}
	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		return nil, domain.NewConflictError("el punto de venta no tiene cuis activo; solicítelo primero")
	}

	// CUFD: con credenciales se resuelve lazy (renueva si venció); sin
	// ellas se usa el vigente del punto de venta o el de la factura.
	var cufd domain.Cufd
	if uc.credentials != nil {
		ensured, err := uc.credentials.EnsureCufd(ctx, &company, &pos)
		if err != nil {
			return nil, err
		}
		cufd = *ensured
	} else {
		cufd = inv.CufdRecord
		if uc.cufdRepo != nil {
			if active, err := uc.cufdRepo.GetActiveByPos(inv.PointOfSaleId); err == nil && active != nil {
				cufd = *active
			}
		}
		if cufd.ID == "" || !cufd.Active {
			return nil, domain.NewConflictError("la factura no tiene un cufd vigente asociado; solicítelo primero")
		}
		now := time.Now().In(siat.LaPaz)
		if now.Before(cufd.ValidFrom) || now.After(cufd.ValidTo) {
			return nil, domain.NewConflictError("el cufd asociado a la factura está vencido; solicite uno nuevo")
		}
	}
	if cufd.ID != "" {
		inv.CufdId = cufd.ID
		inv.CufdRecord = cufd
	}

	if company.CodigoActividad == nil || strings.TrimSpace(*company.CodigoActividad) == "" {
		return nil, domain.NewConflictError("la empresa no tiene definida su actividad económica (codigo_actividad)")
	}
	actividad := strings.TrimSpace(*company.CodigoActividad)

	// El código de punto de venta que usa el CUFD/CUIS debe ser el mismo que
	// viaja en el CUF y en la recepción (preferir el código registrado ante SIAT).
	codigoPuntoVenta := pos.CodigoPuntoVenta
	if pos.SiatCode != nil {
		codigoPuntoVenta = *pos.SiatCode
	}

	modalidad := inv.Modalidad
	if modalidad <= 0 {
		modalidad = uc.effectiveModalidadForCompany(&company)
	}
	if modalidad <= 0 {
		modalidad = siat.ModalidadElectronica
	}

	leyenda, err := uc.resolveLeyenda(inv.CompanyId, actividad)
	if err != nil {
		return nil, err
	}

	telefono := strings.TrimSpace(company.Telefono)
	if telefono == "" {
		telefono = "0000000"
	}
	telefonoPtr := &telefono

	habilitadas := uc.actividadesHabilitadas(&company)
	items := make([]ports.FiscalItem, 0, len(inv.Items))
	for i, it := range inv.Items {
		itemActividad := actividad
		if it.CodigoActividad != nil && strings.TrimSpace(*it.CodigoActividad) != "" {
			itemActividad = strings.TrimSpace(*it.CodigoActividad)
		} else {
			// Herencia por defecto de actividad principal si no viene definida
			itemActividad = actividad
		}
		// Multiactividad controlada: validar existencia en habilitadas (Warn, no bloquea)
		if len(habilitadas) > 0 && !habilitadas[itemActividad] {
			slog.Warn("emission: actividad item no habilitada en padrón, se emite con advertencia (evitar 1017)",
				"invoice_id", inv.ID, "item", i+1, "actividad_item", itemActividad, "actividad_principal", actividad, "habilitadas", habilitadas)
		}

		var codigoProductoSin int64
		if it.CodigoProductoSin != nil {
			if parsed, perr := strconv.ParseInt(strings.TrimSpace(*it.CodigoProductoSin), 10, 64); perr == nil && parsed > 0 {
				codigoProductoSin = parsed
			}
		}
		if codigoProductoSin <= 0 {
			return nil, domain.NewBadRequestError(fmt.Sprintf("el ítem %d (%s) no tiene un codigo_producto_sin válido; sincronice el catálogo y asigne el código SIN", i+1, it.Description))
		}

		unidadMedida := 1
		if it.UnitCode != nil && *it.UnitCode > 0 {
			unidadMedida = *it.UnitCode
		}

		descuento := it.Discount
		var descuentoPtr *float64
		if descuento > 0 {
			descuentoPtr = &descuento
		}

		// Subtotal dinámico estricto: corrige valores quemados/desalineados (1013/1018)
		subtotalCalc := siat.CalcularSubtotal(it.Quantity, it.UnitPrice, descuentoPtr)
		if it.Subtotal != 0 && round2(it.Subtotal) != subtotalCalc {
			slog.Warn("emission: subtotal item auto-corregido",
				"invoice_id", inv.ID, "item", i+1, "descripcion", it.Description,
				"subtotal_previo", it.Subtotal, "subtotal_corregido", subtotalCalc)
		}
		items = append(items, ports.FiscalItem{
			ActividadEconomica: itemActividad,
			CodigoProductoSin:  codigoProductoSin,
			CodigoProducto:     it.Code,
			Descripcion:        it.Description,
			Cantidad:           it.Quantity,
			UnidadMedida:       unidadMedida,
			PrecioUnitario:     it.UnitPrice,
			MontoDescuento:     descuentoPtr,
			SubTotal:           subtotalCalc,
			DatosSector:        it.SectorData,
		})
	}
	// MontoTotal dinámico: suma estricta de subtotales (auto-corrección con Warn)
	var montoCorregido float64
	for _, it := range items {
		montoCorregido += it.SubTotal
	}
	montoCorregido = round2(montoCorregido)
	if inv.CodigoDocumentoSector == 30 && len(items) == 0 {
		montoCorregido = inv.Subtotal
	}
	montoCorregido, err = siat.TotalDocumento(montoCorregido, inv.SectorData)
	if err != nil {
		return nil, domain.NewBadRequestError(err.Error())
	}
	if round2(inv.Total) != montoCorregido {
		slog.Warn("emission: montoTotal auto-corregido",
			"invoice_id", inv.ID, "monto_previo", inv.Total, "monto_corregido", montoCorregido)
	}

	// La dirección del XML debe coincidir con la registrada en padrón ante el
	// SIAT (la trae el CUFD); si no, la factura se observa (código 1007).
	direccion := strings.TrimSpace(cufd.Direccion)
	if direccion == "" {
		direccion = company.Direccion
	}

	sector := inv.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	perfil, err := siat.PerfilSectorLayout(sector, inv.Layout)
	if err != nil {
		return nil, domain.NewBadRequestError(fmt.Sprintf("factura %s: %v", inv.ID, err))
	}
	var numeroFacturaOriginal int64
	var originalItems []ports.FiscalItem
	if perfil.EsAjuste() {
		if strings.TrimSpace(valueOrEmpty(inv.AjustaFacturaId)) == "" {
			return nil, domain.NewConflictError("el documento de ajuste no tiene factura original asociada")
		}
		original, originalErr := uc.invoiceRepo.GetByID(*inv.AjustaFacturaId)
		if originalErr != nil {
			return nil, fmt.Errorf("no se pudo cargar la factura original del ajuste: %w", originalErr)
		}
		slog.Info("siat ajuste: factura original cargada",
			"invoice_id", inv.ID,
			"original_id", original.ID,
			"nota_invoice_number", inv.InvoiceNumber,
			"original_invoice_number", original.InvoiceNumber,
			"original_cuf", valueOrEmpty(original.Cuf),
			"layout", inv.Layout,
			"sector", sector)
		if original.InvoiceNumber <= 0 {
			return nil, domain.NewConflictError("la factura original del ajuste no tiene un número válido")
		}
		numeroFacturaOriginal = int64(original.InvoiceNumber)
		if perfil.DetallePar || perfil.Codigo == 29 || perfil.Codigo == siat.SectorNotaCreditoDebito {
			for _, item := range original.Items {
				if item.CodigoProductoSin == nil || item.CodigoActividad == nil || item.UnitCode == nil {
					return nil, domain.NewConflictError("la factura original no tiene datos fiscales completos en sus ítems")
				}
				code, err := strconv.ParseInt(*item.CodigoProductoSin, 10, 64)
				if err != nil || code <= 0 {
					return nil, domain.NewConflictError("código SIN inválido en factura original")
				}
				data, err := datosDetalleOriginal(perfil, item.SectorData)
				if err != nil {
					return nil, err
				}
				discount := item.Discount
				originalItems = append(originalItems, ports.FiscalItem{
					ActividadEconomica: *item.CodigoActividad, CodigoProductoSin: code,
					CodigoProducto: item.Code, Descripcion: item.Description,
					Cantidad: item.Quantity, PrecioUnitario: item.UnitPrice, UnidadMedida: *item.UnitCode,
					MontoDescuento: &discount, SubTotal: item.Subtotal, DatosSector: data,
				})
			}
		}
	}
	tipoFactura := perfil.TipoDocumentoResuelto(inv.CodigoTipoFactura)

	// Los campos legados educativos viajan siempre; el paquete siat los ignora
	// fuera de los sectores 11/46 y los fusiona a datos_sector cuando aplica.
	var nombreEstudiante, periodoFacturado string
	if inv.NombreEstudiante != nil {
		nombreEstudiante = strings.TrimSpace(*inv.NombreEstudiante)
	}
	if inv.PeriodoFacturado != nil {
		periodoFacturado = strings.TrimSpace(*inv.PeriodoFacturado)
	}

	usuario := "SUPAY"
	if company.UsuarioSiat != "" {
		usuario = company.UsuarioSiat
	}

	// El bloque de cliente del SIAT se construye desde el snapshot de la factura.
	cliente, err := clienteFromCustomer(inv.Customer)
	if err != nil {
		return nil, err
	}

	return &ports.FiscalDocument{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         "",
		Nit:                   company.Nit,
		Modalidad:             modalidad,
		NumeroFactura:         int64(inv.InvoiceNumber),
		NumeroFacturaOriginal: numeroFacturaOriginal,
		CodigoSucursal:        pos.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pos.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		FechaEmision:          inv.IssueDate,
		Usuario:               usuario,
		Leyenda:               leyenda,
		RazonSocialEmisor:     company.BusinessName,
		Municipio:             company.Municipio,
		Direccion:             direccion,
		Telefono:              telefonoPtr,
		CodigoMetodoPago:      inv.CodigoMetodoPago,
		CodigoMoneda:          inv.CodigoMoneda,
		TipoCambio:            inv.TipoCambio,
		MontoTotal:            montoCorregido,
		CodigoDocumentoSector: sector,
		Layout:                inv.Layout,
		CodigoTipoFactura:     tipoFactura,
		NombreEstudiante:      nombreEstudiante,
		PeriodoFacturado:      periodoFacturado,
		DatosSector:           inv.SectorData,
		Archivo:               inv.Archivo,
		HashArchivo:           inv.HashArchivo,
		Cuf:                   valueOrEmpty(inv.Cuf),
		Cliente:               cliente,
		Items:                 items,
		OriginalItems:         originalItems,
	}, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// codigoTipoDocumentoIdentidad mapea el tipo de documento del cliente al código
// del catálogo sincronizado tipoDocumentoIdentidad del SIN.
func codigoTipoDocumentoIdentidad(documentType string) (int, error) {
	switch strings.ToUpper(strings.TrimSpace(documentType)) {
	case "CI":
		return 1, nil
	case "CEX":
		return 2, nil
	case "PAS":
		return 3, nil
	case "NIT":
		return 4, nil
	case "OD":
		return 5, nil
	default:
		return 0, domain.NewBadRequestError(fmt.Sprintf("tipo de documento de identidad no soportado: %q", documentType))
	}
}

// resolveLeyenda busca en el catálogo sincronizado leyendasFactura (tabla
// siat_leyendas_factura) la leyenda oficial del SIAT para la actividad
// económica de la empresa. Si el catálogo no está sincronizado o no contiene
// la actividad, usa la leyenda genérica de la Ley 453.
func (uc *InvoiceUsecase) resolveLeyenda(companyID, actividad string) (string, error) {
	if uc.leyendaRepo != nil && strings.TrimSpace(actividad) != "" {
		if items, err := uc.leyendaRepo.ListByActividad(companyID, strings.TrimSpace(actividad)); err == nil {
			for _, item := range items {
				leyenda := strings.TrimSpace(item.DescripcionLeyenda)
				if leyenda != "" {
					return leyenda, nil
				}
			}
		}
	}
	return "Ley N° 453: Tienes derecho a recibir información sobre el Sistema de Facturación, Ley N° 453, de 4 de diciembre de 2013.", nil
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
