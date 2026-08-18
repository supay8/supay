package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
	"gorm.io/gorm"
)

// SiatEmissionService es el contrato de operaciones de facturación sobre el SDK
// go-siat: emisión de facturas, verificación de estado, anulación y reversión de
// anulación de documentos ya emitidos.
type SiatEmissionService interface {
	EmitirFactura(ctx context.Context, req siat.SolicitudFactura) (*siat.ResultadoEmision, error)
	VerificarEstado(ctx context.Context, req siat.SolicitudDocumento) (*siat.ResultadoDocumento, error)
	AnularFactura(ctx context.Context, req siat.SolicitudDocumento, codigoMotivo int) (*siat.ResultadoDocumento, error)
	RevertirAnulacion(ctx context.Context, req siat.SolicitudDocumento) (*siat.ResultadoDocumento, error)
}

// EmissionRejectedError indica que el SIAT respondió y rechazó la factura
// (Transaccion=false). La factura queda persistida como REJECTED.
type EmissionRejectedError struct {
	CodigoEstado    int
	CodigoRecepcion string
	Mensajes        []siat.Mensaje
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

// Emit emite al SIAT una factura en estado PENDING usando el SDK go-siat.
// El flujo cubre los prerrequisitos (CUIS/CUFD vigentes, sucursal/PV, cliente e
// ítems mapeados a catálogos SIN), la construcción con builders del SDK, la
// firma digital automática (modalidad electrónica) y la persistencia del
// resultado (CUF, XML, hash, código de recepción y estado).
func (uc *InvoiceUsecase) Emit(ctx context.Context, id string) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("factura no encontrada")
		}
		return nil, err
	}
	if inv.Status != domain.InvoicePending {
		if inv.Status == domain.InvoiceSending {
			return nil, errors.New("la factura ya está en proceso de emisión")
		}
		return nil, errors.New("solo se pueden emitir facturas en estado PENDING")
	}

	// Claim atómico PENDING->SENDING: evita emisiones duplicadas concurrentes.
	claimed, err := uc.invoiceRepo.ClaimForEmission(id)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, errors.New("la factura ya está en proceso de emisión")
	}

	// Un fallo de transporte o de prerrequisitos revierte a PENDING (reintentable);
	// un rechazo del SIAT queda persistido como REJECTED.
	rollback := func() {
		inv.Status = domain.InvoicePending
		_ = uc.invoiceRepo.Update(inv)
	}

	if uc.siatService == nil {
		rollback()
		return nil, errors.New("el servicio SIAT no está disponible")
	}

	req, err := uc.buildSolicitudFactura(inv)
	if err != nil {
		rollback()
		return nil, err
	}

	result, err := uc.siatService.EmitirFactura(ctx, *req)
	if err != nil {
		rollback()
		return nil, fmt.Errorf("error de emisión: %w", err)
	}

	inv.Cuf = &result.Cuf
	if result.Xml != "" {
		inv.Xml = &result.Xml
	}
	if result.XmlHash != "" {
		inv.XmlHash = &result.XmlHash
	}
	if result.CodigoRecepcion != "" {
		inv.SiatReceptionCode = &result.CodigoRecepcion
	}
	if msgs, err := marshalMensajes(result.Mensajes); err == nil {
		inv.SiatMensajes = &msgs
	}
	if result.Transaccion {
		// Según el catálogo mensajesServicios del SIAT, 904 = RECEPCION OBSERVADA
		// (no es un caso correcto); solo 908 = RECEPCION VALIDADA.
		if result.CodigoEstado == 904 {
			inv.Status = domain.InvoiceObserved
		} else {
			inv.Status = domain.InvoiceAccepted
		}
	} else {
		inv.Status = domain.InvoiceRejected
	}
	if err := uc.invoiceRepo.Update(inv); err != nil {
		return nil, err
	}

	if !result.Transaccion {
		return nil, &EmissionRejectedError{
			CodigoEstado:    result.CodigoEstado,
			CodigoRecepcion: result.CodigoRecepcion,
			Mensajes:        result.Mensajes,
		}
	}

	return inv, nil
}

// VerifyStatus consulta al SIAT el estado real de un documento emitido
// (verificacionEstadoFactura) y reconcilia el estado local de la factura con el
// CodigoEstado devuelto (según el catálogo mensajesServicios). Un error de
// transporte no modifica el estado local.
func (uc *InvoiceUsecase) VerifyStatus(ctx context.Context, id string) (*domain.Invoice, error) {
	inv, err := uc.invoiceRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("factura no encontrada")
		}
		return nil, err
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, errors.New("la factura no ha sido emitida (no tiene CUF asignado)")
	}
	if uc.siatService == nil {
		return nil, errors.New("el servicio SIAT no está disponible")
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}

	result, err := uc.siatService.VerificarEstado(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("error de verificación: %w", err)
	}

	if estado, ok := siatEstadoToDomain(result.CodigoEstado); ok && inv.Status != estado {
		inv.Status = estado
		if err := uc.invoiceRepo.Update(inv); err != nil {
			return nil, err
		}
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
			return nil, errors.New("factura no encontrada")
		}
		return nil, err
	}

	if inv.Status != domain.InvoiceAccepted {
		return nil, errors.New("solo se pueden anular facturas en estado ACCEPTED")
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, errors.New("la factura no tiene CUF asignado")
	}

	if err := uc.validateMotivoAnulacion(inv.CompanyId, codigoMotivo); err != nil {
		return nil, err
	}

	if uc.siatService == nil {
		return nil, errors.New("el servicio SIAT no está disponible")
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}

	result, err := uc.siatService.AnularFactura(ctx, *req, codigoMotivo)
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
	inv.Status = domain.InvoiceCancelled
	inv.MotivoAnulacion = &codigoMotivo
	inv.FechaAnulacion = &now
	if result.CodigoRecepcion != "" {
		inv.SiatReceptionCode = &result.CodigoRecepcion
	}
	if err := uc.invoiceRepo.Update(inv); err != nil {
		return nil, err
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
			return nil, errors.New("factura no encontrada")
		}
		return nil, err
	}
	if inv.Status != domain.InvoiceCancelled {
		return nil, errors.New("solo se pueden revertir anulaciones de facturas en estado CANCELLED")
	}
	if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
		return nil, errors.New("la factura no tiene CUF asignado")
	}
	if uc.siatService == nil {
		return nil, errors.New("el servicio SIAT no está disponible")
	}

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		return nil, err
	}

	result, err := uc.siatService.RevertirAnulacion(ctx, *req)
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

	inv.Status = domain.InvoiceAccepted
	inv.MotivoAnulacion = nil
	inv.FechaAnulacion = nil
	if result.CodigoRecepcion != "" {
		inv.SiatReceptionCode = &result.CodigoRecepcion
	}
	if err := uc.invoiceRepo.Update(inv); err != nil {
		return nil, err
	}

	return inv, nil
}

// buildSolicitudDocumento reúne la identificación del documento ya emitido para
// las operaciones de verificación, anulación y reversión de anulación. El CUFD
// que se envía es el VIGENTE del punto de venta (GetActiveByPos), porque el SIAT
// rechaza operaciones firmadas con un CUFD vencido (vigencia ~24h); si no hay
// CUFD vigente registrado se cae al CUFD con el que se emitió la factura.
func (uc *InvoiceUsecase) buildSolicitudDocumento(inv *domain.Invoice) (*siat.SolicitudDocumento, error) {
	pos := inv.PointOfSale
	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		return nil, errors.New("el punto de venta no tiene CUIS activo")
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
		return nil, errors.New("la factura no tiene CUFD asociado y el punto de venta no tiene CUFD vigente; solicite uno nuevo (POST /siat/cufd/...)")
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

	modalidad := uc.modalidad
	if modalidad <= 0 {
		modalidad = siat.ModalidadElectronica
	}

	return &siat.SolicitudDocumento{
		CodigoAmbiente:        inv.Company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         inv.Company.CodigoSistema,
		Nit:                   inv.Company.Nit,
		Modalidad:             modalidad,
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
	return fmt.Errorf("motivo de anulación %d no es válido; consulte el catálogo motivoAnulacion", codigoMotivo)
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

// buildSolicitudFactura reúne los prerrequisitos de la factura y los mapea a
// los códigos de catálogo SIN esperados por el SDK.
func (uc *InvoiceUsecase) buildSolicitudFactura(inv *domain.Invoice) (*siat.SolicitudFactura, error) {
	company := inv.Company
	pos := inv.PointOfSale

	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		return nil, errors.New("el punto de venta no tiene CUIS activo; solicítelo primero (POST /siat/cuis/{companyId}/{pointOfSaleId})")
	}

	// Intentar usar el CUFD más reciente del punto de venta; si no hay,
	// caer al CUFD registrado con la factura.
	cufd := inv.CufdRecord
	if uc.cufdRepo != nil {
		if active, err := uc.cufdRepo.GetActiveByPos(inv.PointOfSaleId); err == nil && active != nil {
			cufd = *active
		}
	}
	if cufd.ID == "" || !cufd.Active {
		return nil, errors.New("la factura no tiene un CUFD vigente asociado; solicítelo primero (POST /siat/cufd/{companyId}/{pointOfSaleId})")
	}
	now := time.Now().In(siat.LaPaz)
	if now.Before(cufd.ValidFrom) || now.After(cufd.ValidTo) {
		return nil, errors.New("el CUFD asociado a la factura está vencido; solicite uno nuevo")
	}

	if company.CodigoActividad == nil || strings.TrimSpace(*company.CodigoActividad) == "" {
		return nil, errors.New("la empresa no tiene definida su actividad económica (codigo_actividad)")
	}
	actividad := strings.TrimSpace(*company.CodigoActividad)

	// El código de punto de venta que usa el CUFD/CUIS debe ser el mismo que
	// viaja en el CUF y en la recepción (preferir el código registrado ante SIAT).
	codigoPuntoVenta := pos.CodigoPuntoVenta
	if pos.SiatCode != nil {
		codigoPuntoVenta = *pos.SiatCode
	}

	modalidad := uc.modalidad
	if modalidad <= 0 {
		modalidad = siat.ModalidadElectronica
	}

	codigoDoc, err := codigoTipoDocumentoIdentidad(inv.Customer.DocumentType)
	if err != nil {
		return nil, err
	}

	leyenda, err := uc.resolveLeyenda(inv.CompanyId, actividad)
	if err != nil {
		return nil, err
	}

	telefono := company.Telefono
	var telefonoPtr *string
	if strings.TrimSpace(telefono) != "" {
		telefonoPtr = &telefono
	}

	items := make([]siat.ItemFactura, 0, len(inv.Items))
	for i, it := range inv.Items {
		itemActividad := actividad
		if it.CodigoActividad != nil && strings.TrimSpace(*it.CodigoActividad) != "" {
			itemActividad = strings.TrimSpace(*it.CodigoActividad)
		}

		var codigoProductoSin int64
		if it.CodigoProductoSin != nil {
			if parsed, perr := strconv.ParseInt(strings.TrimSpace(*it.CodigoProductoSin), 10, 64); perr == nil && parsed > 0 {
				codigoProductoSin = parsed
			}
		}
		if codigoProductoSin <= 0 {
			return nil, fmt.Errorf("el ítem %d (%s) no tiene un codigoProductoSin válido; sincronice el catálogo y asigne el código SIN", i+1, it.Description)
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

		items = append(items, siat.ItemFactura{
			ActividadEconomica: itemActividad,
			CodigoProductoSin:  codigoProductoSin,
			CodigoProducto:     it.Code,
			Descripcion:        it.Description,
			Cantidad:           it.Quantity,
			UnidadMedida:       unidadMedida,
			PrecioUnitario:     it.UnitPrice,
			MontoDescuento:     descuentoPtr,
			SubTotal:           it.Subtotal,
		})
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
	tipoFactura := inv.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	var nombreEstudiante, periodoFacturado string
	if inv.NombreEstudiante != nil {
		nombreEstudiante = strings.TrimSpace(*inv.NombreEstudiante)
	}
	if inv.PeriodoFacturado != nil {
		periodoFacturado = strings.TrimSpace(*inv.PeriodoFacturado)
	}

	return &siat.SolicitudFactura{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		Modalidad:             modalidad,
		NumeroFactura:         int64(inv.InvoiceNumber),
		CodigoSucursal:        pos.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pos.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		FechaEmision:          inv.IssueDate,
		Usuario:               "SUPAY",
		Leyenda:               leyenda,
		RazonSocialEmisor:     company.BusinessName,
		Municipio:             company.Municipio,
		Direccion:             direccion,
		Telefono:              telefonoPtr,
		CodigoMetodoPago:      inv.CodigoMetodoPago,
		CodigoMoneda:          inv.CodigoMoneda,
		TipoCambio:            inv.TipoCambio,
		MontoTotal:            inv.Total,
		CodigoDocumentoSector: sector,
		CodigoTipoFactura:     tipoFactura,
		NombreEstudiante:      nombreEstudiante,
		PeriodoFacturado:      periodoFacturado,
		Cliente: siat.ClienteFactura{
			NombreRazonSocial:            inv.Customer.Name,
			CodigoTipoDocumentoIdentidad: codigoDoc,
			NumeroDocumento:              inv.Customer.DocumentNumber,
			Complemento:                  inv.Customer.Complement,
			CodigoCliente:                inv.Customer.ID,
		},
		Items: items,
	}, nil
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
		return 0, fmt.Errorf("tipo de documento de identidad no soportado: %q", documentType)
	}
}

// resolveLeyenda busca en el catálogo sincronizado leyendasFactura la leyenda
// oficial del SIAT para la actividad económica de la empresa. Si el catálogo no
// está sincronizado o no contiene la actividad, usa la leyenda genérica de la
// Ley 453.
func (uc *InvoiceUsecase) resolveLeyenda(companyID, actividad string) (string, error) {
	if uc.catalogRepo != nil {
		if items, err := uc.catalogRepo.List(companyID, "leyendasFactura"); err == nil {
			prefix := actividad + ":"
			for _, item := range items {
				if strings.HasPrefix(item.Descripcion, prefix) {
					return strings.TrimSpace(strings.TrimPrefix(item.Descripcion, prefix)), nil
				}
			}
		}
	}
	return "Ley N° 453: Tienes derecho a recibir información sobre el Sistema de Facturación, Ley N° 453, de 4 de diciembre de 2013.", nil
}

// marshalMensajes serializa los mensajes de la respuesta SIAT a JSON para
// persistirlos en la factura (siat_mensajes) y poder diagnosticar observaciones.
func marshalMensajes(msgs []siat.Mensaje) (string, error) {
	if len(msgs) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(msgs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// resolveDocumentoSector determina el documento-sector del SIAT para la
// actividad económica de la empresa consultando el catálogo sincronizado
// actividadesDocumentoSector (descripción "actividad|FCV|FSEDU|NCD|NCDDE").
// Prefiere la factura de compraventa (FCV); si la actividad solo está asociada
// a sectores educativos (p.ej. 8549100 -> FSEDU), usa ese sector.
func (uc *InvoiceUsecase) resolveDocumentoSector(companyID, actividad string) int {
	if uc.catalogRepo != nil {
		if items, err := uc.catalogRepo.List(companyID, "actividadesDocumentoSector"); err == nil {
			found := 0
			for _, item := range items {
				fields := strings.Split(item.Descripcion, "|")
				if len(fields) < 2 || strings.TrimSpace(fields[0]) != actividad {
					continue
				}
				tipo := strings.TrimSpace(fields[1])
				if tipo == "FCV" {
					return item.Codigo
				}
				if tipo == "FSEDU" {
					found = item.Codigo
				}
			}
			if found > 0 {
				return found
			}
		}
	}
	return 1
}
