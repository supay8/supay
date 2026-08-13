package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
	"gorm.io/gorm"
)

// SiatEmissionService es el contrato de emisión de facturas sobre el SDK go-siat.
type SiatEmissionService interface {
	EmitirFactura(ctx context.Context, req siat.SolicitudFactura) (*siat.ResultadoEmision, error)
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
	if result.Transaccion {
		if result.CodigoEstado == 905 {
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

// buildSolicitudFactura reúne los prerrequisitos de la factura y los mapea a
// los códigos de catálogo SIN esperados por el SDK.
func (uc *InvoiceUsecase) buildSolicitudFactura(inv *domain.Invoice) (*siat.SolicitudFactura, error) {
	company := inv.Company
	pos := inv.PointOfSale

	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		return nil, errors.New("el punto de venta no tiene CUIS activo; solicítelo primero (POST /siat/cuis/{companyId}/{pointOfSaleId})")
	}

	cufd := inv.CufdRecord
	if cufd.ID == "" || !cufd.Active {
		return nil, errors.New("la factura no tiene un CUFD vigente asociado; solicítelo primero (POST /siat/cufd/{companyId}/{pointOfSaleId})")
	}
	now := time.Now()
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

	return &siat.SolicitudFactura{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		Modalidad:        modalidad,
		NumeroFactura:    int64(inv.InvoiceNumber),
		CodigoSucursal:   pos.CodigoSucursal,
		CodigoPuntoVenta: codigoPuntoVenta,
		Cuis:             *pos.Cuis,
		Cufd:             cufd.Cufd,
		CodigoControl:    cufd.ControlCode,
		FechaEmision:     inv.IssueDate,
		Usuario:          "SUPAY",
		Leyenda:          leyenda,
		RazonSocialEmisor: company.BusinessName,
		Municipio:        company.Municipio,
		Direccion:        company.Direccion,
		Telefono:         telefonoPtr,
		CodigoMetodoPago: inv.CodigoMetodoPago,
		CodigoMoneda:     inv.CodigoMoneda,
		TipoCambio:       inv.TipoCambio,
		MontoTotal:       inv.Total,
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
